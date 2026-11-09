package crawler_test

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmoS1X/web-crawler/crawler"
)

const (
	sectionChildPath = "/section/child"
	dirPagePath      = "/dir/page"
)

func TestAnalyzeBaseHrefNavigation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pagePath  string
		pageHTML  string
		wantPages []string
	}{
		{
			name:      "valid base changes navigation",
			pagePath:  "/page",
			pageHTML:  `<base href="/section/"><a href="child">child</a>`,
			wantPages: []string{"/page", sectionChildPath},
		},
		{
			name:      "invalid base falls back to page url",
			pagePath:  dirPagePath,
			pageHTML:  `<base href="ftp://example.com/"><a href="next">next</a>`,
			wantPages: []string{dirPagePath, "/dir/next"},
		},
		{
			name:      "first base wins",
			pagePath:  dirPagePath,
			pageHTML:  `<base href="/first/"><base href="/second/"><a href="child">child</a>`,
			wantPages: []string{dirPagePath, "/first/child"},
		},
		{
			name:      "invalid base tags are ignored",
			pagePath:  "/",
			pageHTML:  `<base target="_blank"><base href=""><base href="ftp://example.com/"><base href="/section/"><a href="child">child</a>`,
			wantPages: []string{"/", sectionChildPath},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newRouteServer(t, map[string]string{
				tt.pagePath:     tt.pageHTML,
				tt.wantPages[1]: "",
			})
			rep := analyzeReport(t, crawler.Options{
				URL:        srv.URL + tt.pagePath,
				Depth:      2,
				Timeout:    time.Second,
				HTTPClient: srv.Client(),
			})

			require.Len(t, rep.Pages, len(tt.wantPages))
			assert.Equal(t, absoluteURLs(srv.URL, tt.wantPages), pageURLs(rep))
		})
	}
}

func TestAnalyzeBaseHrefAffectsAssets(t *testing.T) {
	t.Parallel()

	srv := newRouteServer(t, map[string]string{
		"/":                `<base href="/static/"><img src="logo.png">`,
		"/static/logo.png": "logo",
	})

	rep := analyzeReport(t, crawler.Options{
		URL:        srv.URL,
		Depth:      0,
		Timeout:    time.Second,
		HTTPClient: srv.Client(),
	})
	require.Len(t, rep.Pages, 1)
	require.Len(t, rep.Pages[0].Assets, 1)

	asset := rep.Pages[0].Assets[0]
	assert.Equal(t, srv.URL+"/static/logo.png", asset.URL)
	assert.Equal(t, assetTypeImage, asset.Type)
	assert.Equal(t, int64(len("logo")), asset.SizeBytes)
}

type normalizationTransport struct {
	pages             map[string]string
	internalHeadCalls atomic.Int32
}

func newNormalizationTransport(rootKey, pageHTML string, internalPaths ...string) *normalizationTransport {
	pages := make(map[string]string, len(internalPaths)+1)
	pages[rootKey] = pageHTML
	for _, path := range internalPaths {
		pages[path] = ""
	}
	return &normalizationTransport{pages: pages}
}

func (tr *normalizationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := strings.ToLower(req.URL.Hostname())
	path := req.URL.Path
	if path == "" {
		path = "/"
	}

	if body, ok := tr.pages[host+path]; ok {
		if req.Method == http.MethodHead {
			tr.internalHeadCalls.Add(1)
		}
		return testResponse(req, body), nil
	}
	if host == externalHost {
		return responseWithStatus(req, http.StatusNotFound, ""), nil
	}
	return nil, fmt.Errorf("unexpected request: %s", req.URL)
}

func TestAnalyzeHostNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		rootURL       string
		rootKey       string
		pageHTML      string
		internalPaths []string
		wantPages     []string
		wantBrokenURL string
	}{
		{
			name:          "case and default https port",
			rootURL:       exampleURL,
			rootKey:       "example.com/",
			pageHTML:      `<a href="https://EXAMPLE.com/upper">upper</a><a href="https://example.com:443/port">port</a><a href="https://external.example/x">ext</a>`,
			internalPaths: []string{"example.com/upper", "example.com/port"},
			wantPages: []string{
				"https://example.com",
				"https://EXAMPLE.com/upper",
				"https://example.com:443/port",
			},
			wantBrokenURL: "https://external.example/x",
		},
		{
			name:          "default ipv6 port",
			rootURL:       "http://[::1]/",
			rootKey:       "::1/",
			pageHTML:      `<a href="/page">page</a><a href="http://[::1]:80/port">port</a><a href="http://external.example/x">ext</a>`,
			internalPaths: []string{"::1/page", "::1/port"},
			wantPages: []string{
				"http://[::1]/",
				"http://[::1]/page",
				"http://[::1]:80/port",
			},
			wantBrokenURL: "http://external.example/x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tr := newNormalizationTransport(tt.rootKey, tt.pageHTML, tt.internalPaths...)
			rep := analyzeReport(t, crawler.Options{
				URL:        tt.rootURL,
				Depth:      2,
				Timeout:    time.Second,
				HTTPClient: &http.Client{Transport: tr},
			})

			require.Len(t, rep.Pages, len(tt.wantPages))
			assert.Equal(t, tt.wantPages, pageURLs(rep))

			wantBroken := []crawler.BrokenLink{{
				URL:        tt.wantBrokenURL,
				StatusCode: http.StatusNotFound,
				Error:      http.StatusText(http.StatusNotFound),
			}}
			assert.Equal(t, wantBroken, rep.Pages[0].BrokenLinks)
			assert.Zero(t, tr.internalHeadCalls.Load(), "internal links must not be probed")
		})
	}
}
