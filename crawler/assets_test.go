package crawler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmoS1X/web-crawler/crawler"
)

func TestAnalyzeAssetFieldPresence(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<img src="/ok.png"><script src="/missing.js"></script>`)
		case "/ok.png":
			writeBody(t, w, "png")
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

	var raw struct {
		Pages []struct {
			Assets []map[string]json.RawMessage `json:"assets"`
		} `json:"pages"`
	}
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Len(t, raw.Pages, 1)
	require.Len(t, raw.Pages[0].Assets, 2)

	for _, rawAsset := range raw.Pages[0].Assets {
		assert.Contains(t, rawAsset, "url")
		assert.Contains(t, rawAsset, "type")
	}

	okAsset := raw.Pages[0].Assets[0]
	assert.Contains(t, okAsset, "status_code")
	assert.Contains(t, okAsset, "size_bytes")
	assert.NotContains(t, okAsset, "error")

	missingAsset := raw.Pages[0].Assets[1]
	assert.Contains(t, missingAsset, "status_code")
	assert.NotContains(t, missingAsset, "size_bytes")
	assert.Contains(t, missingAsset, "error")
}

func TestAnalyzeAssetRequestsCachedAcrossPages(t *testing.T) {
	t.Parallel()

	var logoCalls atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<a href="/second">second</a><img src="`+logoPath+`">`)
		case "/second":
			writeBody(t, w, `<img src="`+logoPath+`">`)
		case logoPath:
			logoCalls.Add(1)
			writeBody(t, w, "logo")
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
	assert.Equal(t, int32(1), logoCalls.Load(), "a cached asset must be requested only once")

	var rootAsset, secondAsset crawler.Asset
	for _, p := range rep.Pages {
		require.Len(t, p.Assets, 1)
		switch p.URL {
		case srv.URL:
			rootAsset = p.Assets[0]
		case srv.URL + "/second":
			secondAsset = p.Assets[0]
		}
	}

	assert.Equal(t, rootAsset, secondAsset)
	assert.Equal(t, srv.URL+logoPath, rootAsset.URL)
	assert.Equal(t, assetTypeImage, rootAsset.Type)
	assert.Equal(t, int64(len("logo")), rootAsset.SizeBytes)
}

func TestAnalyzeAssetWithoutContentLength(t *testing.T) {
	t.Parallel()

	const logoBody = "0123456789"
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<img src="`+logoPath+`">`)
		case logoPath:
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("response writer does not support flushing")
				return
			}
			flusher.Flush()
			writeBody(t, w, logoBody)
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
	require.Len(t, rep.Pages[0].Assets, 1)

	got := rep.Pages[0].Assets[0]
	assert.Equal(t, http.StatusOK, got.StatusCode)
	assert.Equal(t, int64(len(logoBody)), got.SizeBytes)
	assert.Empty(t, got.Error)
}

func TestAnalyzeAssetHTTPError(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<script src="/missing.js"></script>`)
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
	require.Len(t, rep.Pages[0].Assets, 1)

	got := rep.Pages[0].Assets[0]
	assert.Equal(t, srv.URL+"/missing.js", got.URL)
	assert.Equal(t, assetTypeScript, got.Type)
	assert.Equal(t, http.StatusNotFound, got.StatusCode)
	assert.Zero(t, got.SizeBytes)
	assert.Equal(t, http.StatusText(http.StatusNotFound), got.Error)
}

func TestAnalyzeAssetTypesAndDeduplication(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<img src="/a.png"><img src="/a.png"><script src="/app.js"></script><link rel="stylesheet" href="/s.css"><link rel="icon" href="/f.ico">`)
		default:
			writeBody(t, w, "asset")
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

	assets := rep.Pages[0].Assets
	require.Len(t, assets, 3, "duplicate asset URLs must be reported once")
	assert.Equal(t, []string{
		srv.URL + "/a.png",
		srv.URL + "/app.js",
		srv.URL + "/s.css",
	}, []string{assets[0].URL, assets[1].URL, assets[2].URL})
	assert.Equal(t, []string{assetTypeImage, assetTypeScript, assetTypeStyle},
		[]string{assets[0].Type, assets[1].Type, assets[2].Type})
}

type assetFailingTransport struct {
	pageHTML string
}

func (tr assetFailingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == exampleHost {
		return testResponse(req, tr.pageHTML), nil
	}
	return nil, errors.New("connection failed")
}

func TestAnalyzeAssetNetworkError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pageHTML string
		wantURL  string
		wantType string
	}{
		{
			name:     "image",
			pageHTML: `<img src="https://cdn.invalid/logo.png">`,
			wantURL:  "https://cdn.invalid/logo.png",
			wantType: assetTypeImage,
		},
		{
			name:     "script",
			pageHTML: `<script src="https://cdn.invalid/app.js"></script>`,
			wantURL:  "https://cdn.invalid/app.js",
			wantType: assetTypeScript,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := crawler.Analyze(t.Context(), crawler.Options{
				URL:        exampleURL,
				Depth:      0,
				Timeout:    time.Second,
				HTTPClient: &http.Client{Transport: assetFailingTransport{pageHTML: tt.pageHTML}},
			})
			require.NoError(t, err)

			rep := decodeReport(t, data)
			require.Len(t, rep.Pages, 1)
			require.Len(t, rep.Pages[0].Assets, 1)

			got := rep.Pages[0].Assets[0]
			assert.Equal(t, tt.wantURL, got.URL)
			assert.Equal(t, tt.wantType, got.Type)
			assert.Zero(t, got.StatusCode)
			assert.Zero(t, got.SizeBytes)
			assert.Contains(t, got.Error, "connection failed")
		})
	}
}

func TestAnalyzeAssetSameURLDifferentTypes(t *testing.T) {
	t.Parallel()

	var assetCalls atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<img src="/shared"><script src="/shared"></script>`)
		case "/shared":
			assetCalls.Add(1)
			w.Header().Set("Content-Length", "5")
			w.WriteHeader(http.StatusOK)
			writeBody(t, w, "12345")
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
	require.Len(t, rep.Pages[0].Assets, 2)

	image, script := rep.Pages[0].Assets[0], rep.Pages[0].Assets[1]
	assert.Equal(t, assetTypeImage, image.Type)
	assert.Equal(t, assetTypeScript, script.Type)
	assert.Equal(t, srv.URL+"/shared", image.URL)
	assert.Equal(t, srv.URL+"/shared", script.URL)
	assert.Equal(t, int64(5), image.SizeBytes)
	assert.Equal(t, int64(5), script.SizeBytes)
	assert.Equal(t, http.StatusOK, image.StatusCode)
	assert.Equal(t, http.StatusOK, script.StatusCode)
	assert.Equal(t, int32(1), assetCalls.Load(), "shared asset URL must be fetched once")
}

type assetCancelingTransport struct {
	cancel context.CancelFunc
	calls  atomic.Int32
}

func (tr *assetCancelingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.calls.Add(1)
	if req.URL.Path == logoPath {
		tr.cancel()
		return nil, context.Canceled
	}
	return testResponse(req, `<img src="`+logoPath+`">`), nil
}

func TestAnalyzeAssetsContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data, err := crawler.Analyze(ctx, crawler.Options{
		URL:        exampleURL,
		Depth:      0,
		Timeout:    time.Second,
		HTTPClient: &http.Client{Transport: &assetCancelingTransport{cancel: cancel}},
	})
	require.NoError(t, err)

	rep := decodeReport(t, data)
	require.Len(t, rep.Pages, 1)
	assert.Empty(t, rep.Pages[0].Assets)
	assert.NotContains(t, string(data), context.Canceled.Error())
}

func TestAnalyzeMalformedAssetURLIgnored(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			writeBody(t, w, `<img src="http://[::1"><img src="`+logoPath+`">`)
		case logoPath:
			w.Header().Set("Content-Length", "4")
			writeBody(t, w, "logo")
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
	require.Len(t, rep.Pages[0].Assets, 1)
	assert.Equal(t, srv.URL+logoPath, rep.Pages[0].Assets[0].URL)
}
