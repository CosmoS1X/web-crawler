package crawler

import "context"

// Analyze fetches the root page and returns a JSON report.
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	c, err := newCrawler(opts)
	if err != nil {
		return nil, err
	}

	return c.run(ctx).marshal(opts.IndentJSON)
}
