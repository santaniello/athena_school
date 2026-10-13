package webfetch

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/web"
)

// allowAll lets tests reach httptest servers, which always listen on loopback.
func allowAll(net.IP) bool { return false }

func newTestClient(opts ...Option) *Client {
	return New(append([]Option{WithIPBlocker(allowAll)}, opts...)...)
}

func serve(t *testing.T, contentType, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestClientFetch_succeeds_forAnHTMLPage(t *testing.T) {
	// Given a server returning an HTML article
	server := serve(t, "text/html; charset=utf-8",
		"<html><head><title>Hello</title></head><body><h1>Hello</h1><p>World.</p></body></html>")

	// When fetching it
	page, err := newTestClient().Fetch(context.Background(), server.URL+"/post#frag")

	// Then the page carries its canonical URL, title and Markdown
	require.NoError(t, err)
	assert.Equal(t, server.URL+"/post", page.URL)
	assert.Equal(t, "Hello", page.Title)
	assert.Equal(t, "# Hello\n\nWorld.", page.Markdown)
}

func TestClientFetch_succeeds_forAPlainTextPage(t *testing.T) {
	// Given a server returning plain text
	server := serve(t, "text/plain", "  just some text\n")

	// When fetching it
	page, err := newTestClient().Fetch(context.Background(), server.URL)

	// Then the text is used as is, trimmed, with no title
	require.NoError(t, err)
	assert.Equal(t, "just some text", page.Markdown)
	assert.Empty(t, page.Title)
}

func TestClientFetch_decodesNonUTF8Pages(t *testing.T) {
	// Given a server returning an ISO-8859-1 page declared in its Content-Type
	latin1 := "<html><body><p>a\xe7\xe3o</p></body></html>"
	server := serve(t, "text/html; charset=iso-8859-1", latin1)

	// When fetching it
	page, err := newTestClient().Fetch(context.Background(), server.URL)

	// Then the accented characters are decoded correctly
	require.NoError(t, err)
	assert.Equal(t, "ação", page.Markdown)
}

func TestClientFetch_sendsADescriptiveUserAgent(t *testing.T) {
	// Given a server recording the User-Agent it receives
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.UserAgent()
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "ok")
	}))
	t.Cleanup(server.Close)

	// When fetching a page
	_, err := newTestClient().Fetch(context.Background(), server.URL)

	// Then the request identifies Athena
	require.NoError(t, err)
	assert.Contains(t, got, "Athena")
}

func TestClientFetch_fails_forAnInvalidURL(t *testing.T) {
	// Given a URL that is not http(s)
	// When fetching it
	_, err := newTestClient().Fetch(context.Background(), "ftp://example.com/a")

	// Then it is rejected before any request is made
	assert.ErrorIs(t, err, web.ErrInvalidURL)
}

func TestClientFetch_fails_forUnsupportedContentTypes(t *testing.T) {
	for _, contentType := range []string{"application/pdf", "image/png", "application/json", ""} {
		t.Run(contentType, func(t *testing.T) {
			// Given a server returning a non-text content type
			server := serve(t, contentType, "binary")

			// When fetching it
			_, err := newTestClient().Fetch(context.Background(), server.URL)

			// Then the content is rejected
			assert.ErrorIs(t, err, web.ErrUnsupportedContent)
		})
	}
}

func TestClientFetch_fails_whenBodyExceedsTheLimit(t *testing.T) {
	// Given a server returning more than the configured body limit
	server := serve(t, "text/plain", strings.Repeat("a", 101))

	// When fetching it with a 100-byte limit
	_, err := newTestClient(WithMaxBodyBytes(100)).Fetch(context.Background(), server.URL)

	// Then the page is rejected as too large
	assert.ErrorIs(t, err, web.ErrTooLarge)
}

func TestClientFetch_succeeds_whenBodyIsExactlyAtTheLimit(t *testing.T) {
	// Given a server returning exactly the configured body limit
	server := serve(t, "text/plain", strings.Repeat("a", 100))

	// When fetching it with a 100-byte limit
	page, err := newTestClient(WithMaxBodyBytes(100)).Fetch(context.Background(), server.URL)

	// Then it is accepted
	require.NoError(t, err)
	assert.Len(t, page.Markdown, 100)
}

func TestClientFetch_truncatesTextToTheCharacterLimit(t *testing.T) {
	// Given a page with more readable characters than the limit
	server := serve(t, "text/plain", strings.Repeat("é", 50))

	// When fetching it with a 10-character limit
	page, err := newTestClient(WithMaxTextChars(10)).Fetch(context.Background(), server.URL)

	// Then the text is cut on a character boundary, not mid-rune
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("é", 10), page.Markdown)
}

func TestClientFetch_fails_whenThePageHasNoReadableText(t *testing.T) {
	// Given a page that only renders client-side
	server := serve(t, "text/html", `<html><body><div id="root"></div><script>render()</script></body></html>`)

	// When fetching it
	_, err := newTestClient().Fetch(context.Background(), server.URL)

	// Then it is reported as empty
	assert.ErrorIs(t, err, web.ErrEmptyContent)
}

func TestClientFetch_fails_forNon2xxStatuses(t *testing.T) {
	// Given a server answering 404
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)

	// When fetching it
	_, err := newTestClient().Fetch(context.Background(), server.URL)

	// Then the status is reported
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestClientFetch_followsRedirects_upToTheLimit(t *testing.T) {
	// Given a server that redirects to itself forever
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server.URL+"/loop", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	// When fetching it
	_, err := newTestClient().Fetch(context.Background(), server.URL)

	// Then the redirect chain is cut off with an error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirect")
}

func TestClientFetch_followsASingleRedirect(t *testing.T) {
	// Given a server redirecting /old to /new
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "moved here")
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// When fetching the old URL
	page, err := newTestClient().Fetch(context.Background(), server.URL+"/old")

	// Then the final content is returned under the requested canonical URL
	require.NoError(t, err)
	assert.Equal(t, "moved here", page.Markdown)
	assert.Equal(t, server.URL+"/old", page.URL)
}

func TestClientFetch_fails_whenTheContextIsCanceled(t *testing.T) {
	// Given a canceled context
	server := serve(t, "text/plain", "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When fetching with it
	_, err := newTestClient().Fetch(ctx, server.URL)

	// Then the fetch stops with the context's error
	assert.ErrorIs(t, err, context.Canceled)
}

func TestClientFetch_fails_whenTheServerIsTooSlow(t *testing.T) {
	// Given a server slower than the client's timeout
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	t.Cleanup(server.Close)

	// When fetching with a 50ms timeout
	_, err := newTestClient(WithTimeout(50*time.Millisecond)).Fetch(context.Background(), server.URL)

	// Then the fetch fails instead of hanging
	assert.Error(t, err)
}

func TestClientFetch_blocksLoopback_byDefault(t *testing.T) {
	// Given a server on loopback and a client with the default blocker
	server := serve(t, "text/plain", "secret")

	// When fetching it
	_, err := New().Fetch(context.Background(), server.URL)

	// Then the connection is refused as a blocked host
	assert.ErrorIs(t, err, web.ErrBlockedHost)
}

func TestClientFetch_blocksARedirectToABlockedAddress(t *testing.T) {
	// Given a server that redirects to 127.0.0.2, while only that address is blocked
	blockOnly := func(ip net.IP) bool { return ip.Equal(net.ParseIP("127.0.0.2")) }
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
		http.Redirect(w, r, "http://127.0.0.2:"+port+"/", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	// When fetching the allowed address
	_, err := New(WithIPBlocker(blockOnly)).Fetch(context.Background(), server.URL)

	// Then the redirect target is refused at dial time
	assert.ErrorIs(t, err, web.ErrBlockedHost)
}

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.1.2.3", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},
		{"100.64.0.1", true},
		{"0.0.0.0", true},
		{"224.0.0.1", true},
		{"fe80::1", true},
		{"fc00::1", true},
		{"::ffff:127.0.0.1", true},
		{"192.0.0.8", true},
		{"198.18.0.1", true},
		{"198.19.255.255", true},
		{"240.0.0.1", true},
		{"255.255.255.255", true},
		{"64:ff9b::a00:1", true},
		{"64:ff9b::7f00:1", true},
		{"64:ff9b::a9fe:a9fe", true},
		{"64:ff9b::808:808", false},
		{"8.8.8.8", false},
		{"93.184.216.34", false},
		{"2606:4700:4700::1111", false},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			// Given an IP address
			// When checking whether the default blocker refuses it
			got := isBlockedIP(net.ParseIP(tt.ip))

			// Then only non-public addresses are blocked
			assert.Equal(t, tt.blocked, got)
		})
	}
}
