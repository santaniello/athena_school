// Package web defines the port through which the application reads a web
// page, and the rules that decide which URLs are acceptable sources.
package web

import (
	"context"
	"errors"
)

var (
	// ErrInvalidURL is returned for anything that is not an absolute http(s)
	// URL without embedded credentials.
	ErrInvalidURL = errors.New("web: invalid URL")
	// ErrBlockedHost is returned when the URL resolves to an address the
	// fetcher refuses to contact (loopback, private, link-local, metadata).
	ErrBlockedHost = errors.New("web: host is not allowed")
	// ErrUnsupportedContent is returned when the response is not text/html
	// or text/plain.
	ErrUnsupportedContent = errors.New("web: unsupported content type")
	// ErrTooLarge is returned when the response body exceeds the size limit.
	ErrTooLarge = errors.New("web: page is too large")
	// ErrEmptyContent is returned when no readable text could be extracted,
	// typically a page that only renders with JavaScript or sits behind a
	// login.
	ErrEmptyContent = errors.New("web: page has no readable text")
)

// Page is a fetched web page, already converted to Markdown.
type Page struct {
	// URL is the canonical URL of the page that was requested.
	URL string
	// Title is the page's title, empty when the page has none.
	Title string
	// Markdown is the page's readable text as Markdown.
	Markdown string
}

// Fetcher reads a web page. Implemented by
// internal/infrastructure/webfetch.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) (Page, error)
}
