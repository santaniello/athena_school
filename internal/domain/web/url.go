package web

import (
	"net/url"
	"strings"
)

// CanonicalURL returns the canonical spelling of rawURL: lowercase scheme
// and host, no fragment, no default port, and a root path for a bare host.
// It is the identity of a link source, so two spellings of the same page
// replace each other instead of duplicating. Anything that is not an
// absolute http(s) URL with a host, or that carries credentials, is
// ErrInvalidURL.
func CanonicalURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", ErrInvalidURL
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", ErrInvalidURL
	}
	if parsed.Hostname() == "" || parsed.User != nil {
		return "", ErrInvalidURL
	}

	host := strings.ToLower(parsed.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	port := parsed.Port()
	if port == "80" && scheme == "http" || port == "443" && scheme == "https" {
		port = ""
	}
	if port != "" {
		host += ":" + port
	}

	parsed.Scheme = scheme
	parsed.Host = host
	parsed.Fragment = ""
	parsed.RawFragment = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
}
