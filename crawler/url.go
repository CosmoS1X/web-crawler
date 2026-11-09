package crawler

import (
	"net"
	"net/url"
	"strings"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

func resolveLink(base *url.URL, raw string) (*url.URL, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return nil, false
	}

	ref, err := url.Parse(raw)
	if err != nil {
		return nil, false
	}

	absolute := base.ResolveReference(ref)
	absolute.Fragment = ""
	absolute.RawFragment = ""
	if absolute.Scheme != schemeHTTP && absolute.Scheme != schemeHTTPS {
		return nil, false
	}
	if absolute.Host == "" {
		return nil, false
	}
	return absolute, true
}

// resolveBase returns the first valid base URL, falling back to the page URL.
func resolveBase(pageURL *url.URL, baseHrefs []string) *url.URL {
	for _, href := range baseHrefs {
		if resolved, ok := resolveLink(pageURL, href); ok {
			return resolved
		}
	}
	return pageURL
}

func canonicalURL(u *url.URL) string {
	canonical := *u
	canonical.Fragment = ""
	canonical.RawFragment = ""
	canonical.Host = normalizedHost(u)
	if canonical.Path == "" {
		canonical.Path = "/"
	} else if canonical.Path != "/" {
		canonical.Path = strings.TrimSuffix(canonical.Path, "/")
	}
	return canonical.String()
}

func normalizedHost(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" ||
		(u.Scheme == schemeHTTP && port == "80") ||
		(u.Scheme == schemeHTTPS && port == "443") {
		if strings.Contains(host, ":") {
			return "[" + host + "]"
		}
		return host
	}
	return net.JoinHostPort(host, port)
}
