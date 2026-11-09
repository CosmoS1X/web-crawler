package crawler

import (
	"cmp"
	"context"
	"net/http"
	"net/url"
	"slices"
	"time"

	"golang.org/x/time/rate"
)

type crawler struct {
	opts       Options
	root       *url.URL
	limiter    *rate.Limiter
	assetCache map[string]assetMeta
}

func newCrawler(opts Options) (*crawler, error) {
	root, err := opts.validate()
	if err != nil {
		return nil, err
	}

	var limiter *rate.Limiter
	if opts.RPS > 0 {
		// RPS has priority over Delay
		limiter = rate.NewLimiter(rate.Limit(opts.RPS), 1)
	} else if opts.Delay > 0 {
		limiter = rate.NewLimiter(rate.Every(opts.Delay), 1)
	}

	return &crawler{
		opts:       opts,
		root:       root,
		limiter:    limiter,
		assetCache: make(map[string]assetMeta),
	}, nil
}

func (c *crawler) run(ctx context.Context) *Report {
	generatedAt := time.Now().UTC().Truncate(time.Second)

	visited := make(map[string]struct{})
	pages := make([]Page, 0)

	type queueItem struct {
		url   *url.URL
		depth int
	}

	queue := make([]queueItem, 0)
	visited[canonicalURL(c.root)] = struct{}{}
	queue = append(queue, queueItem{url: c.root, depth: 0})

	for len(queue) > 0 {
		if ctx.Err() != nil {
			break
		}

		current := queue[0]
		queue = queue[1:]

		page, base, links := c.visitPage(ctx, current.url, current.depth)
		pages = append(pages, page)

		if ctx.Err() != nil {
			break
		}

		if current.depth+1 >= c.opts.Depth {
			continue
		}

		for _, link := range links {
			if ctx.Err() != nil {
				break
			}
			linkURL, ok := resolveNavigationLink(base, link)
			if !ok {
				continue
			}
			if !c.isInternal(linkURL) {
				continue
			}

			key := canonicalURL(linkURL)
			if _, exists := visited[key]; exists {
				continue
			}
			visited[key] = struct{}{}
			queue = append(queue, queueItem{url: linkURL, depth: current.depth + 1})
		}
	}

	slices.SortFunc(pages, func(a, b Page) int {
		return cmp.Or(
			cmp.Compare(a.Depth, b.Depth),
			cmp.Compare(a.URL, b.URL),
		)
	})

	return &Report{
		RootURL:     c.opts.URL,
		Depth:       c.opts.Depth,
		GeneratedAt: generatedAt,
		Pages:       pages,
	}
}

func (c *crawler) visitPage(
	ctx context.Context,
	pageURL *url.URL,
	depth int,
) (page Page, base *url.URL, links []extractedLink) {
	discoveredAt := time.Now().UTC().Truncate(time.Second)
	result := c.fetch(ctx, http.MethodGet, pageURL.String(), fetchFullBody)

	page = Page{
		URL:          pageURL.String(),
		Depth:        depth,
		HTTPStatus:   result.StatusCode,
		Status:       statusOK,
		DiscoveredAt: discoveredAt,
	}
	if result.Err != nil {
		page.Status = statusError
		page.Error = result.Err.Error()
		return page, pageURL, nil
	}

	if result.StatusCode >= http.StatusBadRequest {
		page.Status = statusError
		page.Error = statusErrorText(result.StatusCode)
	}

	links, baseHrefs := extractLinks(result.Body)
	base = resolveBase(pageURL, baseHrefs)

	page.SEO = extractSEO(result.Body)
	page.Assets = c.collectAssets(ctx, base, links)
	page.BrokenLinks = c.findBrokenLinks(ctx, pageURL, base, depth, links)
	return page, base, links
}

// isInternal reports whether the URL points to the crawl root host.
func (c *crawler) isInternal(u *url.URL) bool {
	return normalizedHost(u) == normalizedHost(c.root)
}
