// Package parser extracts Wikipedia article links from HTML.
package parser

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Link struct {
	Title      string
	AnchorText string
}

var excludedPrefixes = []string{
	"Wikipedia:",
	"Help:",
	"File:",
	"Category:",
	"Template:",
	"Template_talk:",
	"Portal:",
	"Special:",
	"Talk:",
	"User:",
	"User_talk:",
	"Wikipedia_talk:",
	"MediaWiki:",
	"Draft:",
	"Module:",
}

// ExtractLinks parses HTML and returns all internal Wikipedia article links.
func ExtractLinks(doc *goquery.Document) []Link {
	seen := make(map[string]bool)
	var links []Link

	// Only look for links in the main content area
	doc.Find("#mw-content-text a[href^='/wiki/']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		title := extractTitle(href)
		if title == "" || seen[title] || shouldExclude(title) {
			return
		}

		seen[title] = true
		links = append(links, Link{
			Title:      title,
			AnchorText: strings.TrimSpace(s.Text()),
		})
	})

	return links
}

// ExtractLinksFromHTML parses HTML string and returns all internal Wikipedia article links.
func ExtractLinksFromHTML(html string) ([]Link, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	return ExtractLinks(doc), nil
}

func extractTitle(href string) string {
	// href format: /wiki/Article_Name or /wiki/Article_Name#section
	if !strings.HasPrefix(href, "/wiki/") {
		return ""
	}

	path := strings.TrimPrefix(href, "/wiki/")

	if idx := strings.Index(path, "#"); idx != -1 {
		path = path[:idx]
	}

	decoded, err := url.PathUnescape(path)
	if err != nil {
		return path
	}

	// Replace underscores with spaces (Wikipedia convention)
	return strings.ReplaceAll(decoded, "_", " ")
}

func shouldExclude(title string) bool {
	for _, prefix := range excludedPrefixes {
		if strings.HasPrefix(title, prefix) {
			return true
		}
	}

	if strings.HasSuffix(title, " (disambiguation)") {
		return true
	}

	return false
}
