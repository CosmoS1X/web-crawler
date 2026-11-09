package crawler

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

type linkKind int

const (
	linkNavigation linkKind = iota
	linkImage
	linkScript
	linkStyle
)

const (
	attrHref = "href"
	attrSrc  = "src"
	attrRel  = "rel"
)

type linkSpec struct {
	attr string
	kind linkKind
	rel  string
}

var linkSpecs = map[string]linkSpec{
	"a":      {attr: attrHref, kind: linkNavigation},
	"img":    {attr: attrSrc, kind: linkImage},
	"script": {attr: attrSrc, kind: linkScript},
	"link":   {attr: attrHref, kind: linkStyle, rel: "stylesheet"},
}

type extractedLink struct {
	URL  string
	Kind linkKind
}

func (c *crawler) findBrokenLinks(
	ctx context.Context,
	pageURL, base *url.URL,
	depth int,
	rawLinks []extractedLink,
) []BrokenLink {
	broken := make([]BrokenLink, 0, len(rawLinks))
	seen := make(map[string]struct{}, len(rawLinks))
	canSchedule := depth+1 < c.opts.Depth
	pageKey := canonicalURL(pageURL)
	for _, link := range rawLinks {
		if ctx.Err() != nil {
			return broken
		}

		linkURL, ok := resolveNavigationLink(base, link)
		if !ok {
			continue
		}
		if canonicalURL(linkURL) == pageKey {
			continue
		}
		linkKey := linkURL.String()
		if _, exists := seen[linkKey]; exists {
			continue
		}
		seen[linkKey] = struct{}{}

		if c.isInternal(linkURL) && canSchedule {
			continue
		}

		item, isBroken := c.probeLink(ctx, linkURL)
		if ctx.Err() != nil {
			return broken
		}
		if isBroken {
			broken = append(broken, item)
		}
	}
	return broken
}

func (c *crawler) probeLink(ctx context.Context, linkURL *url.URL) (BrokenLink, bool) {
	rawURL := linkURL.String()
	result := c.fetchWithGETFallback(ctx, rawURL, fetchStatusOnly, func(result fetchResult) bool {
		return result.StatusCode == http.StatusMethodNotAllowed
	})
	if statusCode, errText, failed := fetchFailure(result); failed {
		return BrokenLink{URL: rawURL, StatusCode: statusCode, Error: errText}, true
	}
	return BrokenLink{}, false
}

// resolveNavigationLink resolves a raw link and rejects non-navigation kinds.
func resolveNavigationLink(base *url.URL, link extractedLink) (*url.URL, bool) {
	if link.Kind != linkNavigation {
		return nil, false
	}
	return resolveLink(base, link.URL)
}

func extractLinks(body []byte) (links []extractedLink, baseHrefs []string) {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	links = make([]extractedLink, 0)

	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			return links, baseHrefs
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}

		tagName, _ := tokenizer.TagName()
		if string(tagName) == "base" {
			if href, ok := tagAttr(tokenizer, attrHref); ok && href != "" {
				baseHrefs = append(baseHrefs, href)
			}
			continue
		}

		spec, ok := linkSpecs[string(tagName)]
		if !ok {
			continue
		}

		var rawURL, rel string
		for {
			key, value, more := tokenizer.TagAttr()
			switch string(key) {
			case spec.attr:
				rawURL = string(value)
			case attrRel:
				rel = string(value)
			}
			if !more {
				break
			}
		}

		if spec.rel != "" && !hasRelToken(rel, spec.rel) {
			continue
		}
		if rawURL == "" {
			continue
		}
		links = append(links, extractedLink{URL: rawURL, Kind: spec.kind})
	}
}

// tagAttr consumes the tag attributes and returns the named one when present.
func tagAttr(tokenizer *html.Tokenizer, name string) (string, bool) {
	var value string
	var found bool
	for {
		key, attrValue, more := tokenizer.TagAttr()
		if string(key) == name {
			value = string(attrValue)
			found = true
		}
		if !more {
			break
		}
	}
	return value, found
}

func hasRelToken(rel, token string) bool {
	return slices.Contains(strings.Fields(strings.ToLower(rel)), token)
}
