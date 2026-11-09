package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/net/html/charset"
)

const (
	retryDelay   = 100 * time.Millisecond
	maxPageBytes = 10 << 20
)

type fetchMode int

const (
	fetchStatusOnly fetchMode = iota
	fetchFullBody
	fetchAssetMeta
)

type fetchResult struct {
	StatusCode int
	Body       []byte
	AssetSize  int64
	Err        error
}

func (c *crawler) fetch(ctx context.Context, method, rawURL string, mode fetchMode) fetchResult {
	var result fetchResult
	for attemptIndex := 0; ; attemptIndex++ {
		var retryable bool
		result, retryable = c.doRequest(ctx, method, rawURL, mode)

		if !retryable || ctx.Err() != nil || attemptIndex >= c.opts.Retries {
			return result
		}

		if waitErr := waitForRetry(ctx); waitErr != nil {
			result.Body = nil
			result.Err = waitErr
			return result
		}
	}
}

// fetchWithGETFallback probes a URL with HEAD and repeats the request with GET
// when needsFallback reports that the HEAD response is not usable.
func (c *crawler) fetchWithGETFallback(
	ctx context.Context,
	rawURL string,
	mode fetchMode,
	needsFallback func(fetchResult) bool,
) fetchResult {
	result := c.fetch(ctx, http.MethodHead, rawURL, mode)
	if result.Err != nil || !needsFallback(result) {
		return result
	}
	return c.fetch(ctx, http.MethodGet, rawURL, mode)
}

// doRequest performs a single HTTP attempt and reports whether a failure is
// transient and therefore retryable.
func (c *crawler) doRequest(
	ctx context.Context,
	method, rawURL string,
	mode fetchMode,
) (fetchResult, bool) {
	if c.limiter != nil {
		if waitErr := c.limiter.Wait(ctx); waitErr != nil {
			return fetchResult{Err: waitErr}, false
		}
	}

	req, cancel, err := c.buildRequest(ctx, method, rawURL)
	if err != nil {
		return fetchResult{Err: err}, false
	}
	defer cancel()

	resp, err := c.opts.HTTPClient.Do(req)
	if err != nil {
		return fetchResult{Err: err}, true
	}
	defer resp.Body.Close()

	return readResponse(resp, method, mode)
}

func (c *crawler) buildRequest(
	ctx context.Context,
	method, rawURL string,
) (*http.Request, context.CancelFunc, error) {
	reqCtx := ctx
	cancel := context.CancelFunc(func() {})
	if c.opts.Timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.opts.Timeout)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, rawURL, http.NoBody)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	if c.opts.UserAgent != "" {
		req.Header.Set("User-Agent", c.opts.UserAgent)
	}
	return req, cancel, nil
}

// readResponse extracts the result payload for the requested fetch mode.
func readResponse(resp *http.Response, method string, mode fetchMode) (fetchResult, bool) {
	result := fetchResult{StatusCode: resp.StatusCode}
	switch mode {
	case fetchStatusOnly:
	case fetchFullBody:
		body, err := readPageBody(resp)
		if err != nil {
			return fetchResult{StatusCode: resp.StatusCode, Err: err}, true
		}
		result.Body = body
	case fetchAssetMeta:
		size, err := readAssetSize(resp, method)
		if err != nil {
			return fetchResult{StatusCode: resp.StatusCode, Err: err}, true
		}
		result.AssetSize = size
	}
	return result, isTransientStatus(resp.StatusCode)
}

func readPageBody(resp *http.Response) ([]byte, error) {
	reader, err := charset.NewReader(
		io.LimitReader(resp.Body, maxPageBytes),
		resp.Header.Get("Content-Type"),
	)
	if errors.Is(err, io.EOF) {
		reader = http.NoBody
	} else if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

func readAssetSize(resp *http.Response, method string) (int64, error) {
	if method == http.MethodHead || resp.ContentLength >= 0 {
		return resp.ContentLength, nil
	}

	size, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}
	return size, nil
}

// fetchFailure classifies a failed probe and returns its status code and error
// text. The boolean is false when the probe succeeded.
func fetchFailure(result fetchResult) (statusCode int, errText string, failed bool) {
	if result.Err != nil {
		return 0, result.Err.Error(), true
	}
	if result.StatusCode >= http.StatusBadRequest {
		return result.StatusCode, statusErrorText(result.StatusCode), true
	}
	return 0, "", false
}

func statusErrorText(statusCode int) string {
	if text := http.StatusText(statusCode); text != "" {
		return text
	}
	return fmt.Sprintf("unexpected http status: %d", statusCode)
}

func isTransientStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}

func waitForRetry(ctx context.Context) error {
	timer := time.NewTimer(retryDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
