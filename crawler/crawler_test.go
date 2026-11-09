package crawler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmoS1X/web-crawler/crawler"
)

func TestAnalyzeDepthZero(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<a href="/inner">inner</a>`)
		case "/inner":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	data, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:        srv.URL,
		Depth:      0,
		Timeout:    time.Second,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	rep := decodeReport(t, data)
	require.Len(t, rep.Pages, 1)
	assert.Equal(t, srv.URL, rep.Pages[0].URL)
	assert.Equal(t, 0, rep.Pages[0].Depth)
	assert.Equal(t, "ok", rep.Pages[0].Status)
}

func TestAnalyzeDuplicateInternalLinks(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<a href="/page">1</a><a href="/page">2</a><a href="/page/">3</a>`)
		case pagePath, pagePath + "/":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	data, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:        srv.URL,
		Depth:      2,
		Timeout:    time.Second,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	rep := decodeReport(t, data)
	require.Len(t, rep.Pages, 2)
	assert.Equal(t, srv.URL, rep.Pages[0].URL)
	assert.Equal(t, 0, rep.Pages[0].Depth)
	assert.Equal(t, "ok", rep.Pages[0].Status)
	assert.Equal(t, 1, rep.Pages[1].Depth)
	assert.Equal(t, "ok", rep.Pages[1].Status)
	assert.NotEqual(t, rep.Pages[1].URL, rep.Pages[0].URL)
}

type externalLinkTransport struct{}

func (externalLinkTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.Host {
	case exampleHost:
		switch req.URL.Path {
		case "", "/":
			return testResponse(req, `<a href="/ok">ok</a><a href="https://other.example/x">ext</a>`), nil
		case "/ok":
			return testResponse(req, ""), nil
		}
	case "other.example":
		return responseWithStatus(req, http.StatusNotFound, ""), nil
	}
	return nil, fmt.Errorf("unexpected request: %s", req.URL)
}

func TestAnalyzeExternalLinkNotTraversed(t *testing.T) {
	t.Parallel()

	rep := analyzeReport(t, crawler.Options{
		URL:        exampleURL,
		Depth:      2,
		Timeout:    time.Second,
		HTTPClient: &http.Client{Transport: externalLinkTransport{}},
	})
	require.Len(t, rep.Pages, 2)

	assert.Equal(t, exampleURL, rep.Pages[0].URL)
	assert.Equal(t, 0, rep.Pages[0].Depth)
	assert.Equal(t, exampleURL+"/ok", rep.Pages[1].URL)
	assert.Equal(t, 1, rep.Pages[1].Depth)
	assert.Equal(t, "ok", rep.Pages[1].Status)

	for _, page := range rep.Pages {
		assert.NotContains(t, page.URL, "other.example")
	}
}

func TestAnalyzeMalformedNavigationLink(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<a href="http://%zz">bad</a>`)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})

	data, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:        srv.URL,
		Depth:      2,
		Timeout:    time.Second,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	rep := decodeReport(t, data)
	require.Len(t, rep.Pages, 1)
	assert.Equal(t, srv.URL, rep.Pages[0].URL)
}

func TestAnalyzeContextCancelMidCrawl(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	var signalOnce sync.Once
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<a href="/a">a</a><a href="/b">b</a>`)
		case "/a":
			signalOnce.Do(func() { close(started) })
			<-r.Context().Done()
		default:
			w.WriteHeader(http.StatusOK)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
		}
		cancel()
	}()

	data, err := crawler.Analyze(ctx, crawler.Options{
		URL:        srv.URL,
		Depth:      2,
		Timeout:    time.Second,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)
	require.True(t, json.Valid(data))

	var rep crawler.Report
	require.NoError(t, json.Unmarshal(data, &rep))
	require.NotEmpty(t, rep.Pages)
	assert.Equal(t, srv.URL, rep.Pages[0].URL)
}

func TestAnalyzeAlreadyCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	data, err := crawler.Analyze(ctx, crawler.Options{
		URL:        exampleURL,
		Depth:      1,
		Timeout:    time.Second,
		HTTPClient: &http.Client{Transport: failingTransport{}},
	})
	require.NoError(t, err)
	require.True(t, json.Valid(data))
	assert.NotContains(t, string(data), context.Canceled.Error())

	rep := decodeReport(t, data)
	assert.Empty(t, rep.Pages)
}

func TestAnalyzeSelfLinkIgnored(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<a href="/">self</a><a href="/#top">fragment</a><a href="/page">page</a>`)
		case pagePath:
			writeBody(t, w, `<a href="/page">self</a>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	data, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:        srv.URL,
		Depth:      2,
		Timeout:    time.Second,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	rep := decodeReport(t, data)
	require.Len(t, rep.Pages, 2)
	assert.Equal(t, srv.URL, rep.Pages[0].URL)
	assert.Equal(t, srv.URL+pagePath, rep.Pages[1].URL)
	for _, page := range rep.Pages {
		assert.Empty(t, page.BrokenLinks)
	}
}

func TestAnalyzePagesSortedByDepthThenURL(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<a href="/z">z</a><a href="/a">a</a>`)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})

	data, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:        srv.URL,
		Depth:      2,
		Timeout:    time.Second,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	rep := decodeReport(t, data)
	require.Len(t, rep.Pages, 3)
	assert.Equal(t, []string{srv.URL, srv.URL + "/a", srv.URL + "/z"}, []string{
		rep.Pages[0].URL,
		rep.Pages[1].URL,
		rep.Pages[2].URL,
	})
}

type pageCancelingTransport struct {
	cancel context.CancelFunc
}

func (tr pageCancelingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.cancel()
	return testResponse(req, `<img src="`+logoPath+`"><a href="/broken">broken</a>`), nil
}

func TestAnalyzeSkipsOptionalWorkWhenContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data, err := crawler.Analyze(ctx, crawler.Options{
		URL:        exampleURL,
		Depth:      1,
		Timeout:    time.Second,
		HTTPClient: &http.Client{Transport: pageCancelingTransport{cancel: cancel}},
	})
	require.NoError(t, err)

	rep := decodeReport(t, data)
	require.Len(t, rep.Pages, 1)
	assert.Equal(t, "ok", rep.Pages[0].Status)
	assert.Empty(t, rep.Pages[0].Assets)
	assert.Empty(t, rep.Pages[0].BrokenLinks)
	assert.NotContains(t, string(data), context.Canceled.Error())
}
