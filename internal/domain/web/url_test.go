package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanonicalURL_succeeds_normalizingEquivalentForms(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"lowercases scheme and host", "HTTPS://Example.COM/Path", "https://example.com/Path"},
		{"removes the fragment", "https://example.com/a#section", "https://example.com/a"},
		{"trims surrounding whitespace", "  https://example.com/a \n", "https://example.com/a"},
		{"keeps path, query and port", "http://example.com:8080/a?b=1", "http://example.com:8080/a?b=1"},
		{"drops the default https port", "https://example.com:443/a", "https://example.com/a"},
		{"drops the default http port", "http://example.com:80/a", "http://example.com/a"},
		{"adds a root path to a bare host", "https://example.com", "https://example.com/"},
		{"keeps the brackets of an IPv6 host", "http://[2001:DB8::1]:8080/a", "http://[2001:db8::1]:8080/a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a raw URL in one of several equivalent spellings
			// When canonicalizing it
			got, err := CanonicalURL(tt.raw)

			// Then the canonical form is returned
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCanonicalURL_fails_whenURLIsNotAnAbsoluteHTTPURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"blank", "   "},
		{"no scheme", "example.com/a"},
		{"unsupported scheme", "ftp://example.com/a"},
		{"file scheme", "file:///etc/passwd"},
		{"javascript scheme", "javascript:alert(1)"},
		{"missing host", "https:///a"},
		{"unparseable", "http://exa mple.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a value that is not an absolute http(s) URL
			// When canonicalizing it
			got, err := CanonicalURL(tt.raw)

			// Then it is rejected with ErrInvalidURL
			assert.ErrorIs(t, err, ErrInvalidURL)
			assert.Empty(t, got)
		})
	}
}

func TestCanonicalURL_fails_whenURLCarriesCredentials(t *testing.T) {
	// Given a URL with embedded user:password credentials
	// When canonicalizing it
	got, err := CanonicalURL("https://user:secret@example.com/a")

	// Then it is rejected, so credentials never become a stored source identity
	assert.ErrorIs(t, err, ErrInvalidURL)
	assert.Empty(t, got)
}

func TestCanonicalURL_isIdempotent(t *testing.T) {
	// Given an already canonical URL
	first, err := CanonicalURL("HTTPS://Example.com:443/a#x")
	assert.NoError(t, err)

	// When canonicalizing it again
	second, err := CanonicalURL(first)

	// Then the result does not change
	assert.NoError(t, err)
	assert.Equal(t, first, second)
}
