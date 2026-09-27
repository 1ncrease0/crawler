package parser

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parse(t *testing.T, base, body string) (string, []string) {
	t.Helper()
	baseURL, err := url.Parse(base)
	require.NoError(t, err)
	page, err := ParsePage(strings.NewReader(body), baseURL)
	require.NoError(t, err)
	return page.Title, page.Links
}

func TestParsePage(t *testing.T) {
	t.Parallel()

	body := `<html><head><title>  Hello &amp; World  </title></head><body>
		<a href="/a">a</a>
		<a href="/a">duplicate</a>
		<area href="/b">
		<a href="/c">c</a>
		<img src="/img.png">
		<script src="/s.js"></script>
		<link href="/style.css">
	</body></html>`

	title, links := parse(t, "https://example.com/dir/page", body)

	assert.Equal(t, "Hello & World", title)
	assert.Equal(t, []string{
		"https://example.com/a",
		"https://example.com/b",
		"https://example.com/c",
	}, links)
}

func TestParseLinkResolution(t *testing.T) {
	t.Parallel()

	const base = "https://example.com/dir/page"

	tests := []struct {
		name string
		href string
		want string
		skip bool
	}{
		{name: "relative", href: "sub", want: "https://example.com/dir/sub"},
		{name: "root relative", href: "/about", want: "https://example.com/about"},
		{name: "absolute", href: "https://other.com/x", want: "https://other.com/x"},
		{name: "protocol relative", href: "//cdn.example.com/x", want: "https://cdn.example.com/x"},
		{name: "query kept", href: "/a?b=1", want: "https://example.com/a?b=1"},
		{name: "fragment stripped", href: "/a#section", want: "https://example.com/a"},
		{name: "trimmed", href: "  /about  ", want: "https://example.com/about"},
		{name: "empty", href: "", skip: true},
		{name: "whitespace", href: "   ", skip: true},
		{name: "mailto", href: "mailto:a@example.com", skip: true},
		{name: "javascript", href: "javascript:void(0)", skip: true},
		{name: "tel", href: "tel:+123", skip: true},
		{name: "data", href: "data:text/plain,hi", skip: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := fmt.Sprintf(`<a href=%q>link</a>`, tt.href)
			_, links := parse(t, base, body)

			if tt.skip {
				assert.Empty(t, links)
				return
			}
			assert.Equal(t, []string{tt.want}, links)
		})
	}
}

func TestParsePageBaseTag(t *testing.T) {
	t.Parallel()

	body := `<html><head>
		<base href="https://cdn.example.com/assets/">
		<base href="https://ignored.example.com/">
	</head><body>
		<a href="img.png">relative</a>
		<a href="/root">root</a>
	</body></html>`

	_, links := parse(t, "https://example.com/dir/page", body)

	assert.Equal(t, []string{
		"https://cdn.example.com/assets/img.png",
		"https://cdn.example.com/root",
	}, links)
}

func TestParsePageTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing", body: `<html><body>no title</body></html>`, want: ""},
		{name: "empty", body: `<html><head><title>   </title></head></html>`, want: ""},
		{name: "first wins", body: `<html><head><title>first</title><title>second</title></head></html>`, want: "first"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			title, _ := parse(t, "https://example.com/", tt.body)
			assert.Equal(t, tt.want, title)
		})
	}
}
