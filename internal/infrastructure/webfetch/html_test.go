package webfetch

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTMLToMarkdown_extractsTitle(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{"from the title element", "<html><head><title> My  Article </title></head><body><p>x</p></body></html>", "My Article"},
		{"falls back to the first h1", "<html><body><h1>Heading One</h1><p>x</p></body></html>", "Heading One"},
		{"empty when neither exists", "<html><body><p>x</p></body></html>", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given an HTML document
			// When converting it to Markdown
			title, _, err := htmlToMarkdown(strings.NewReader(tt.html))

			// Then the title is extracted from the best available source
			assert.NoError(t, err)
			assert.Equal(t, tt.want, title)
		})
	}
}

func TestHTMLToMarkdown_convertsStructure(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			"headings and paragraphs",
			"<body><h1>Title</h1><p>First paragraph.</p><h2>Section</h2><p>Second   paragraph.</p></body>",
			"# Title\n\nFirst paragraph.\n\n## Section\n\nSecond paragraph.",
		},
		{
			"unordered list",
			"<body><ul><li>one</li><li>two</li></ul></body>",
			"- one\n- two",
		},
		{
			"ordered list",
			"<body><ol><li>one</li><li>two</li></ol></body>",
			"1. one\n2. two",
		},
		{
			"inline code",
			"<body><p>Use <code>fmt.Println</code> here.</p></body>",
			"Use `fmt.Println` here.",
		},
		{
			"preformatted block keeps its line breaks",
			"<body><pre>line one\n  line two</pre></body>",
			"```\nline one\n  line two\n```",
		},
		{
			"link text without its href",
			"<body><p>See <a href=\"https://x.test\">the docs</a>.</p></body>",
			"See the docs.",
		},
		{
			"html entities are decoded",
			"<body><p>a &amp; b &lt; c</p></body>",
			"a & b < c",
		},
		{
			"line break element",
			"<body><p>one<br>two</p></body>",
			"one\ntwo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given an HTML fragment
			// When converting it to Markdown
			_, got, err := htmlToMarkdown(strings.NewReader(tt.html))

			// Then the structure is rendered as Markdown
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHTMLToMarkdown_dropsNoiseElements(t *testing.T) {
	// Given a page with scripts, styles, navigation, footer, aside and form noise
	page := `<html><head><title>T</title><style>p{color:red}</style></head><body>
		<nav>Home | About</nav>
		<script>alert('x')</script>
		<noscript>enable js</noscript>
		<aside>related links</aside>
		<form><input value="q"></form>
		<p>Real content.</p>
		<footer>copyright</footer></body></html>`

	// When converting it to Markdown
	_, got, err := htmlToMarkdown(strings.NewReader(page))

	// Then only the readable content remains
	assert.NoError(t, err)
	assert.Equal(t, "Real content.", got)
}

func TestHTMLToMarkdown_prefersTheMainArticleRegion(t *testing.T) {
	// Given a page whose article is surrounded by unrelated body text
	page := `<body><div>Sidebar chatter</div>
		<article><h1>Post</h1><p>Body of the post.</p></article>
		<div>More chatter</div></body>`

	// When converting it to Markdown
	_, got, err := htmlToMarkdown(strings.NewReader(page))

	// Then only the article's content is kept
	assert.NoError(t, err)
	assert.Equal(t, "# Post\n\nBody of the post.", got)
}

func TestHTMLToMarkdown_returnsEmptyMarkdown_whenThereIsNoReadableText(t *testing.T) {
	// Given a page that only holds a script mount point
	page := `<html><body><div id="root"></div><script>render()</script></body></html>`

	// When converting it to Markdown
	_, got, err := htmlToMarkdown(strings.NewReader(page))

	// Then the Markdown is empty
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestHTMLToMarkdown_dropsRegionsMarkedAsChromeOrHidden(t *testing.T) {
	// Given a page whose navigation, banner, search and hidden regions are
	// marked up with ARIA roles or the hidden attributes rather than tags
	page := `<body>
		<div role="navigation">Languages: English Deutsch</div>
		<div role="banner">Site header</div>
		<div role="search">Search box</div>
		<div role="contentinfo">Legal</div>
		<div role="complementary">Related</div>
		<div hidden>Hidden text</div>
		<div aria-hidden="true">Decorative</div>
		<p>Real content.</p></body>`

	// When converting it to Markdown
	_, got, err := htmlToMarkdown(strings.NewReader(page))

	// Then only the readable content remains
	assert.NoError(t, err)
	assert.Equal(t, "Real content.", got)
}

func TestHTMLToMarkdown_keepsRegionsWithOtherRolesOrAriaAttributes(t *testing.T) {
	// Given content marked with roles that are not page chrome
	page := `<body><div role="article">Kept article.</div><div aria-hidden="false">Kept too.</div></body>`

	// When converting it to Markdown
	_, got, err := htmlToMarkdown(strings.NewReader(page))

	// Then nothing is dropped
	assert.NoError(t, err)
	assert.Equal(t, "Kept article.\n\nKept too.", got)
}
