package crawler

import (
	"context"
	"net/http"
	"net/url"
)

const (
	assetTypeImage  = "image"
	assetTypeScript = "script"
	assetTypeStyle  = "style"
)

type assetMeta struct {
	StatusCode int
	SizeBytes  int64
	Error      string
}

var assetKinds = []struct {
	kind     linkKind
	typeName string
}{
	{kind: linkImage, typeName: assetTypeImage},
	{kind: linkScript, typeName: assetTypeScript},
	{kind: linkStyle, typeName: assetTypeStyle},
}

func (c *crawler) collectAssets(ctx context.Context, base *url.URL, rawLinks []extractedLink) []Asset {
	byKind := make(map[linkKind][]extractedLink, len(assetKinds))
	for _, link := range rawLinks {
		byKind[link.Kind] = append(byKind[link.Kind], link)
	}

	seen := make(map[string]struct{}, len(rawLinks))
	assets := make([]Asset, 0, len(rawLinks))

	for _, assetKind := range assetKinds {
		for _, link := range byKind[assetKind.kind] {
			if ctx.Err() != nil {
				return assets
			}

			resolved, ok := resolveLink(base, link.URL)
			if !ok {
				continue
			}
			assetURL := resolved.String()

			key := assetKind.typeName + "::" + assetURL
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			meta, exists := c.assetCache[assetURL]
			if !exists {
				meta = c.fetchAsset(ctx, assetURL)
				if ctx.Err() != nil {
					return assets
				}
				c.assetCache[assetURL] = meta
			}

			assets = append(assets, Asset{
				URL:        assetURL,
				Type:       assetKind.typeName,
				SizeBytes:  meta.SizeBytes,
				StatusCode: meta.StatusCode,
				Error:      meta.Error,
			})
		}
	}
	return assets
}

func (c *crawler) fetchAsset(ctx context.Context, assetURL string) assetMeta {
	result := c.fetchWithGETFallback(ctx, assetURL, fetchAssetMeta, func(result fetchResult) bool {
		return result.StatusCode == http.StatusMethodNotAllowed ||
			(result.StatusCode < http.StatusBadRequest && result.AssetSize < 0)
	})

	if statusCode, errText, failed := fetchFailure(result); failed {
		return assetMeta{StatusCode: statusCode, Error: errText}
	}
	return assetMeta{
		StatusCode: result.StatusCode,
		SizeBytes:  result.AssetSize,
	}
}
