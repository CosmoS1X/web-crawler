package crawler_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmoS1X/web-crawler/crawler"
)

func TestAnalyzeSEO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fixture string
		want    crawler.SEO
	}{
		{
			name:    "all tags present",
			fixture: "seo_full.html",
			want: crawler.SEO{
				HasTitle:       true,
				Title:          "Example Test",
				HasDescription: true,
				Description:    "A description",
				HasH1:          true,
			},
		},
		{
			name:    "tags missing",
			fixture: "seo_missing.html",
			want:    crawler.SEO{},
		},
		{
			name:    "html entities decoded",
			fixture: "seo_entities.html",
			want: crawler.SEO{
				HasTitle:       true,
				Title:          "Fish & Chips",
				HasDescription: true,
				Description:    `Tom "Jerry" <cartoon>`,
				HasH1:          true,
			},
		},
		{
			name:    "duplicate title uses first tag",
			fixture: "seo_duplicate_title.html",
			want: crawler.SEO{
				HasTitle: true,
				Title:    "First Title",
				HasH1:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := loadFixture(t, tt.fixture)
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeBody(t, w, body)
			})

			data, err := crawler.Analyze(t.Context(), crawler.Options{
				URL:        srv.URL,
				Depth:      1,
				Timeout:    time.Second,
				HTTPClient: srv.Client(),
			})
			require.NoError(t, err)

			rep := decodeReport(t, data)
			require.Len(t, rep.Pages, 1)
			assert.Equal(t, tt.want, rep.Pages[0].SEO)
		})
	}
}
