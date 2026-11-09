package crawler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmoS1X/web-crawler/crawler"
)

const (
	referenceLogoSize   = 12345
	referenceScriptSize = 32
	referenceStyleSize  = 24
)

type referenceTransport struct {
	pageHTML string
}

func (tr referenceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.URL.Host == exampleHost && (req.URL.Path == "" || req.URL.Path == "/"):
		return testResponse(req, tr.pageHTML), nil
	case req.URL.Host == exampleHost && req.URL.Path == "/about":
		return responseWithStatus(req, http.StatusOK, ""), nil
	case req.URL.Host == exampleHost && req.URL.Path == "/missing":
		return responseWithStatus(req, http.StatusNotFound, ""), nil
	case req.URL.Host == externalHost && req.URL.Path == "/missing":
		return responseWithStatus(req, http.StatusNotFound, ""), nil
	case req.URL.Host == exampleHost && req.URL.Path == "/static/logo.png":
		return responseWithContentLength(req, referenceLogoSize), nil
	case req.URL.Host == exampleHost && req.URL.Path == "/static/app.js":
		return responseWithContentLength(req, referenceScriptSize), nil
	case req.URL.Host == exampleHost && req.URL.Path == "/static/app.css":
		return responseWithContentLength(req, referenceStyleSize), nil
	default:
		return nil, fmt.Errorf("unexpected request: %s", req.URL)
	}
}

func referenceOptions(t *testing.T, indent bool) crawler.Options {
	t.Helper()

	return crawler.Options{
		URL:        exampleURL,
		Depth:      1,
		Timeout:    time.Second,
		IndentJSON: indent,
		HTTPClient: &http.Client{
			Transport: referenceTransport{pageHTML: loadFixture(t, "reference_page.html")},
		},
	}
}

var timeFieldPattern = regexp.MustCompile(`("(?:generated_at|discovered_at)":\s*)"[^"]*"`)

func normalizeTimes(data string) string {
	return timeFieldPattern.ReplaceAllString(data, `$1"__TIME__"`)
}

func TestAnalyzeMatchesReferenceReport(t *testing.T) {
	t.Parallel()

	data, err := crawler.Analyze(t.Context(), referenceOptions(t, true))
	require.NoError(t, err)

	report := decodeReport(t, data)
	assert.Equal(t, exampleURL, report.RootURL)
	assert.Equal(t, 1, report.Depth)
	require.Len(t, report.Pages, 1)
	assert.Equal(t, exampleURL, report.Pages[0].URL)

	want := normalizeTimes(strings.TrimSuffix(loadFixture(t, "report_reference.json"), "\n"))
	assert.Equal(t, want, normalizeTimes(string(data)))
}

func TestAnalyzeIndentJSONOnlyChangesFormatting(t *testing.T) {
	t.Parallel()

	compact, err := crawler.Analyze(t.Context(), referenceOptions(t, false))
	require.NoError(t, err)

	pretty, err := crawler.Analyze(t.Context(), referenceOptions(t, true))
	require.NoError(t, err)

	normalizedPretty := normalizeTimes(string(pretty))
	var compactedPretty bytes.Buffer
	require.NoError(t, json.Compact(&compactedPretty, []byte(normalizedPretty)))

	assert.Equal(t, normalizeTimes(string(compact)), compactedPretty.String())
	assert.NotContains(t, string(compact), "\n")
	assert.Contains(t, string(pretty), "\n")

	wantCompact := normalizeTimes(strings.TrimSuffix(loadFixture(t, "report_reference.json"), "\n"))
	var compactFixture bytes.Buffer
	require.NoError(t, json.Compact(&compactFixture, []byte(wantCompact)))
	assert.Equal(t, compactFixture.String(), normalizeTimes(string(compact)))
}

func TestAnalyzeTimestampsISO8601(t *testing.T) {
	t.Parallel()

	data, err := crawler.Analyze(t.Context(), referenceOptions(t, true))
	require.NoError(t, err)

	var raw struct {
		GeneratedAt string `json:"generated_at"`
		Pages       []struct {
			DiscoveredAt string `json:"discovered_at"`
		} `json:"pages"`
	}
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Len(t, raw.Pages, 1)

	for _, value := range []string{raw.GeneratedAt, raw.Pages[0].DiscoveredAt} {
		parsed, parseErr := time.Parse(time.RFC3339, value)
		require.NoError(t, parseErr)
		assert.Equal(t, time.UTC, parsed.Location())
		assert.Equal(t, value, parsed.UTC().Format(time.RFC3339))
	}
}
