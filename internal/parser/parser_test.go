package parser

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestExtractLinks(t *testing.T) {
	html := `
	<html>
	<body>
	<div id="mw-content-text">
		<p>This is about <a href="/wiki/Physics">physics</a> and
		<a href="/wiki/Albert_Einstein">Albert Einstein</a>.</p>
		<p>See also <a href="/wiki/Quantum_mechanics#History">quantum mechanics</a>.</p>
	</div>
	</body>
	</html>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	links := ExtractLinks(doc)

	if len(links) != 3 {
		t.Fatalf("got %d links, want 3", len(links))
	}

	expected := map[string]bool{
		"Physics":           true,
		"Albert Einstein":   true,
		"Quantum mechanics": true,
	}

	for _, link := range links {
		if !expected[link.Title] {
			t.Errorf("unexpected link: %q", link.Title)
		}
	}
}

func TestExtractLinks_DeduplicatesLinks(t *testing.T) {
	html := `
	<div id="mw-content-text">
		<a href="/wiki/Test">Test</a>
		<a href="/wiki/Test">Test again</a>
		<a href="/wiki/Test#section">Test section</a>
	</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	links := ExtractLinks(doc)

	if len(links) != 1 {
		t.Errorf("got %d links, want 1 (deduplicated)", len(links))
	}
}

func TestExtractLinks_ExcludesNamespaces(t *testing.T) {
	html := `
	<div id="mw-content-text">
		<a href="/wiki/Real_Article">Real Article</a>
		<a href="/wiki/Wikipedia:About">About Wikipedia</a>
		<a href="/wiki/File:Example.jpg">Image</a>
		<a href="/wiki/Category:Science">Science category</a>
		<a href="/wiki/Help:Contents">Help</a>
		<a href="/wiki/Template:Infobox">Template</a>
		<a href="/wiki/Special:Search">Search</a>
		<a href="/wiki/Talk:Article">Talk page</a>
		<a href="/wiki/User:Example">User page</a>
	</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	links := ExtractLinks(doc)

	if len(links) != 1 {
		t.Errorf("got %d links, want 1 (only real article)", len(links))
	}
	if len(links) > 0 && links[0].Title != "Real Article" {
		t.Errorf("Title = %q, want 'Real Article'", links[0].Title)
	}
}

func TestExtractLinks_ExcludesDisambiguation(t *testing.T) {
	html := `
	<div id="mw-content-text">
		<a href="/wiki/Mercury_(planet)">Mercury (planet)</a>
		<a href="/wiki/Mercury_(disambiguation)">Mercury (disambiguation)</a>
	</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	links := ExtractLinks(doc)

	if len(links) != 1 {
		t.Errorf("got %d links, want 1", len(links))
	}
}

func TestExtractLinks_URLDecodes(t *testing.T) {
	html := `
	<div id="mw-content-text">
		<a href="/wiki/Schr%C3%B6dinger%27s_cat">Schrödinger's cat</a>
	</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	links := ExtractLinks(doc)

	if len(links) != 1 {
		t.Fatalf("got %d links, want 1", len(links))
	}
	if links[0].Title != "Schrödinger's cat" {
		t.Errorf("Title = %q, want %q", links[0].Title, "Schrödinger's cat")
	}
}

func TestExtractLinks_IgnoresExternalLinks(t *testing.T) {
	html := `
	<div id="mw-content-text">
		<a href="/wiki/Article">Internal</a>
		<a href="https://example.com">External</a>
		<a href="//example.com">Protocol-relative</a>
	</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	links := ExtractLinks(doc)

	if len(links) != 1 {
		t.Errorf("got %d links, want 1", len(links))
	}
}

func TestExtractLinks_OnlyMainContent(t *testing.T) {
	html := `
	<html>
	<body>
	<div id="sidebar">
		<a href="/wiki/Sidebar_Link">Sidebar</a>
	</div>
	<div id="mw-content-text">
		<a href="/wiki/Content_Link">Content</a>
	</div>
	<div id="footer">
		<a href="/wiki/Footer_Link">Footer</a>
	</div>
	</body>
	</html>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	links := ExtractLinks(doc)

	if len(links) != 1 {
		t.Errorf("got %d links, want 1 (only main content)", len(links))
	}
	if len(links) > 0 && links[0].Title != "Content Link" {
		t.Errorf("Title = %q, want 'Content Link'", links[0].Title)
	}
}

func TestExtractLinksFromHTML(t *testing.T) {
	html := `<div id="mw-content-text"><a href="/wiki/Test">Test</a></div>`

	links, err := ExtractLinksFromHTML(html)
	if err != nil {
		t.Fatalf("ExtractLinksFromHTML error: %v", err)
	}
	if len(links) != 1 {
		t.Errorf("got %d links, want 1", len(links))
	}
}
