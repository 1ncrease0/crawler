package parser

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/1ncrease0/crawler/internal/model"
)

func ParsePage(r io.Reader, baseURL *url.URL) (model.Page, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return model.Page{}, fmt.Errorf("parser: parse html: %w", err)
	}

	b := &pageBuilder{
		base: baseURL,
		seen: make(map[string]struct{}),
	}
	b.findBase(doc)
	b.walk(doc)

	return b.page, nil
}

type pageBuilder struct {
	base      *url.URL
	page      model.Page
	titleDone bool
	baseDone  bool
	seen      map[string]struct{}
}

func (b *pageBuilder) findBase(n *html.Node) {
	if b.baseDone {
		return
	}

	if n.Type == html.ElementNode && n.Namespace == "" && n.Data == "base" {
		if href, ok := findAttr(n, "href"); ok {
			b.baseDone = true
			if abs, ok := resolveRef(b.base, href); ok {
				b.base = abs
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.findBase(c)
	}
}

func (b *pageBuilder) walk(n *html.Node) {
	if n.Type == html.ElementNode && n.Namespace == "" {
		switch n.Data {
		case "title":
			b.visitTitle(n)
		case "a", "area":
			b.visitLink(n)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.walk(c)
	}
}

func (b *pageBuilder) visitTitle(n *html.Node) {
	if b.titleDone {
		return
	}
	b.titleDone = true

	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
	}
	b.page.Title = strings.TrimSpace(sb.String())
}

func (b *pageBuilder) visitLink(n *html.Node) {
	href, ok := findAttr(n, "href")
	if !ok {
		return
	}

	abs, ok := resolveRef(b.base, href)
	if !ok {
		return
	}

	link := abs.String()
	if _, dup := b.seen[link]; dup {
		return
	}
	b.seen[link] = struct{}{}
	b.page.Links = append(b.page.Links, link)
}

func findAttr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func resolveRef(baseURL *url.URL, href string) (*url.URL, bool) {
	href = strings.TrimSpace(href)
	if href == "" {
		return nil, false
	}

	ref, err := url.Parse(href)
	if err != nil {
		return nil, false
	}

	abs := baseURL.ResolveReference(ref)
	abs.Fragment = ""
	abs.RawFragment = ""
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return nil, false
	}
	if abs.Host == "" {
		return nil, false
	}

	return abs, true
}
