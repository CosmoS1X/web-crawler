package crawler_test

import (
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/CosmoS1X/web-crawler/crawler"
)

func TestAnalyzeInvalidOptions(t *testing.T) {
	client := &http.Client{}

	tests := []struct {
		name    string
		opts    crawler.Options
		wantErr error
	}{
		{
			name:    "empty url",
			opts:    crawler.Options{URL: "", HTTPClient: client},
			wantErr: crawler.ErrInvalidURL,
		},
		{
			name:    "unsupported scheme",
			opts:    crawler.Options{URL: "ftp://example.com", HTTPClient: client},
			wantErr: crawler.ErrInvalidURL,
		},
		{
			name:    "malformed url",
			opts:    crawler.Options{URL: "http://[::1", HTTPClient: client},
			wantErr: crawler.ErrInvalidURL,
		},
		{
			name:    "missing host",
			opts:    crawler.Options{URL: "https://", HTTPClient: client},
			wantErr: crawler.ErrInvalidURL,
		},
		{
			name:    "nil client",
			opts:    crawler.Options{URL: exampleURL},
			wantErr: crawler.ErrNilHTTPClient,
		},
		{
			name:    "negative depth",
			opts:    crawler.Options{URL: exampleURL, Depth: -1, HTTPClient: client},
			wantErr: crawler.ErrInvalidOptions,
		},
		{
			name:    "negative timeout",
			opts:    crawler.Options{URL: exampleURL, Timeout: -time.Second, HTTPClient: client},
			wantErr: crawler.ErrInvalidOptions,
		},
		{
			name:    "negative retries",
			opts:    crawler.Options{URL: exampleURL, Retries: -1, HTTPClient: client},
			wantErr: crawler.ErrInvalidOptions,
		},
		{
			name:    "negative delay",
			opts:    crawler.Options{URL: exampleURL, Delay: -time.Second, HTTPClient: client},
			wantErr: crawler.ErrInvalidOptions,
		},
		{
			name:    "negative concurrency",
			opts:    crawler.Options{URL: exampleURL, Concurrency: -1, HTTPClient: client},
			wantErr: crawler.ErrInvalidOptions,
		},
		{
			name:    "negative rps",
			opts:    crawler.Options{URL: exampleURL, RPS: -1, HTTPClient: client},
			wantErr: crawler.ErrInvalidOptions,
		},
		{
			name:    "nan rps",
			opts:    crawler.Options{URL: exampleURL, RPS: math.NaN(), HTTPClient: client},
			wantErr: crawler.ErrInvalidOptions,
		},
		{
			name:    "infinite rps",
			opts:    crawler.Options{URL: exampleURL, RPS: math.Inf(1), HTTPClient: client},
			wantErr: crawler.ErrInvalidOptions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := crawler.Analyze(t.Context(), tt.opts)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
