// Package webfetch implements web.Fetcher over net/http, refusing to contact
// non-public addresses and bounding every dimension of the response.
package webfetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html/charset"

	"github.com/santaniello/athena/internal/domain/web"
)

const (
	defaultTimeout      = 10 * time.Second
	defaultMaxBodyBytes = 2 << 20
	defaultMaxTextChars = 200_000
	maxRedirects        = 5
	userAgent           = "Mozilla/5.0 (compatible; Athena/1.0; study-notes reader)"
)

// Client implements web.Fetcher.
type Client struct {
	timeout      time.Duration
	maxBodyBytes int64
	maxTextChars int
	blocked      func(net.IP) bool
	http         *http.Client
}

var _ web.Fetcher = (*Client)(nil)

// Option customizes a Client.
type Option func(*Client)

// WithTimeout bounds the whole request, redirects included.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// WithMaxBodyBytes bounds the response body before any conversion.
func WithMaxBodyBytes(n int64) Option { return func(c *Client) { c.maxBodyBytes = n } }

// WithMaxTextChars bounds the extracted text, in characters.
func WithMaxTextChars(n int) Option { return func(c *Client) { c.maxTextChars = n } }

// WithIPBlocker replaces the check that refuses an address. Tests use it to
// reach servers on loopback; production code keeps the default.
func WithIPBlocker(blocked func(net.IP) bool) Option { return func(c *Client) { c.blocked = blocked } }

// New creates a Client that only contacts public addresses unless told
// otherwise.
func New(opts ...Option) *Client {
	c := &Client{
		timeout:      defaultTimeout,
		maxBodyBytes: defaultMaxBodyBytes,
		maxTextChars: defaultMaxTextChars,
		blocked:      isBlockedIP,
	}
	for _, opt := range opts {
		opt(c)
	}

	dialer := &net.Dialer{Timeout: c.timeout, Control: c.guardDial}
	c.http = &http.Client{
		Timeout: c.timeout,
		Transport: &http.Transport{
			// Never route through a proxy from the environment: the dial
			// guard would then only see the proxy's address.
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   c.timeout,
			ResponseHeaderTimeout: c.timeout,
			DisableKeepAlives:     true,
		},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("webfetch: stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
	return c
}

// guardDial runs on every connection attempt with the already-resolved
// address, so it covers redirects and DNS answers that change between a
// check and the connection.
func (c *Client) guardDial(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return web.ErrBlockedHost
	}
	if c.blocked(net.ParseIP(host)) {
		return web.ErrBlockedHost
	}
	return nil
}

// Fetch implements web.Fetcher.
func (c *Client) Fetch(ctx context.Context, rawURL string) (web.Page, error) {
	canonical, err := web.CanonicalURL(rawURL)
	if err != nil {
		return web.Page{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, canonical, http.NoBody)
	if err != nil {
		return web.Page{}, fmt.Errorf("webfetch: building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9")

	resp, err := c.http.Do(req)
	if err != nil {
		return web.Page{}, fmt.Errorf("webfetch: fetching page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return web.Page{}, fmt.Errorf("webfetch: unexpected status %d", resp.StatusCode)
	}

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	isHTML := mediaType == "text/html" || mediaType == "application/xhtml+xml"
	if !isHTML && mediaType != "text/plain" {
		return web.Page{}, web.ErrUnsupportedContent
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBodyBytes+1))
	if err != nil {
		return web.Page{}, fmt.Errorf("webfetch: reading body: %w", err)
	}
	if int64(len(body)) > c.maxBodyBytes {
		return web.Page{}, web.ErrTooLarge
	}

	text, err := charset.NewReader(bytes.NewReader(body), resp.Header.Get("Content-Type"))
	if err != nil {
		return web.Page{}, fmt.Errorf("webfetch: decoding body: %w", err)
	}

	var title, markdown string
	if isHTML {
		title, markdown, err = htmlToMarkdown(text)
	} else {
		var raw []byte
		raw, err = io.ReadAll(text)
		markdown = strings.TrimSpace(string(raw))
	}
	if err != nil {
		return web.Page{}, fmt.Errorf("webfetch: reading page text: %w", err)
	}
	if markdown == "" {
		return web.Page{}, web.ErrEmptyContent
	}

	return web.Page{URL: canonical, Title: title, Markdown: truncateRunes(markdown, c.maxTextChars)}, nil
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return strings.TrimSpace(string(runes[:limit]))
}

// nat64Prefix is the well-known NAT64 prefix; hosts behind DNS64 resolve to it with an
// IPv4 address embedded in the last 32 bits.
var nat64Prefix = netip.MustParsePrefix("64:ff9b::/96")

// blockedPrefixes lists the non-routable ranges that the netip.Addr predicates
// (loopback, private, link-local, multicast, unspecified) do not cover.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),  // IETF protocol assignments
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),   // reserved, includes the broadcast address
}

// isBlockedIP reports whether ip is anything but a public unicast address.
func isBlockedIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	return isBlockedAddr(addr.Unmap())
}

func isBlockedAddr(addr netip.Addr) bool {
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	if nat64Prefix.Contains(addr) {
		b := addr.As16()
		return isBlockedAddr(netip.AddrFrom4([4]byte(b[12:])))
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
