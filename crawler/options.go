package crawler

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"
)

var (
	ErrInvalidURL     = errors.New("invalid url")
	ErrNilHTTPClient  = errors.New("http client is required")
	ErrInvalidOptions = errors.New("invalid options")
)

type Options struct {
	URL         string
	Depth       int
	Timeout     time.Duration
	IndentJSON  bool
	HTTPClient  *http.Client
	Retries     int
	Delay       time.Duration
	UserAgent   string
	Concurrency int
	RPS         float64
}

func (o Options) validate() (*url.URL, error) {
	if o.HTTPClient == nil {
		return nil, ErrNilHTTPClient
	}

	checks := []struct {
		invalid bool
		message string
	}{
		{invalid: o.Depth < 0, message: "depth must not be negative"},
		{invalid: o.Timeout < 0, message: "timeout must not be negative"},
		{invalid: o.Retries < 0, message: "retries must not be negative"},
		{invalid: o.Delay < 0, message: "delay must not be negative"},
		{invalid: o.Concurrency < 0, message: "concurrency must not be negative"},
		{invalid: math.IsNaN(o.RPS) || math.IsInf(o.RPS, 0), message: "rps must be finite"},
		{invalid: o.RPS < 0, message: "rps must not be negative"},
	}
	for _, check := range checks {
		if check.invalid {
			return nil, fmt.Errorf("%w: %s", ErrInvalidOptions, check.message)
		}
	}
	return parseURL(o.URL)
}

func parseURL(rawURL string) (*url.URL, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("%w: empty url", ErrInvalidURL)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidURL, err)
	}
	if parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS {
		return nil, fmt.Errorf("%w: unsupported scheme %q", ErrInvalidURL, parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	return parsed, nil
}
