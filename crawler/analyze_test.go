package crawler_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmoS1X/web-crawler/crawler"
)

func TestAnalyzeSuccess(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	data, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:        srv.URL,
		Depth:      1,
		Timeout:    time.Second,
		IndentJSON: true,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	rep := decodeReport(t, data)
	assert.Equal(t, srv.URL, rep.RootURL)
	assert.Equal(t, 1, rep.Depth)
	assert.False(t, rep.GeneratedAt.IsZero())
	require.Len(t, rep.Pages, 1)

	got := rep.Pages[0]
	assert.Equal(t, srv.URL, got.URL)
	assert.Equal(t, 0, got.Depth)
	assert.Equal(t, http.StatusOK, got.HTTPStatus)
	assert.Equal(t, "ok", got.Status)
	assert.Empty(t, got.Error)
	assert.NotNil(t, got.BrokenLinks)
	assert.Empty(t, got.BrokenLinks)
	assert.NotNil(t, got.Assets)
	assert.Empty(t, got.Assets)
	assert.False(t, got.DiscoveredAt.IsZero())
}

func TestAnalyzeNetworkError(t *testing.T) {
	t.Parallel()

	data, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:        exampleURL,
		Depth:      1,
		Timeout:    time.Second,
		IndentJSON: true,
		HTTPClient: &http.Client{Transport: failingTransport{}},
	})
	require.NoError(t, err, "network errors must not fail Analyze")

	rep := decodeReport(t, data)
	require.Len(t, rep.Pages, 1)

	got := rep.Pages[0]
	assert.Zero(t, got.HTTPStatus)
	assert.Equal(t, "error", got.Status)
	assert.NotEmpty(t, got.Error)
}

func TestAnalyzePageHTTPError(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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

	got := rep.Pages[0]
	assert.Equal(t, http.StatusInternalServerError, got.HTTPStatus)
	assert.Equal(t, "error", got.Status)
	assert.Equal(t, http.StatusText(http.StatusInternalServerError), got.Error)
	assert.NotNil(t, got.BrokenLinks)
	assert.Empty(t, got.BrokenLinks)
	assert.NotNil(t, got.Assets)
	assert.Empty(t, got.Assets)
}
