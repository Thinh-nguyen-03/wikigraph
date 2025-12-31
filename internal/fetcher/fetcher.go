// Package fetcher provides Wikipedia page fetching with rate limiting.
package fetcher

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"golang.org/x/time/rate"

	"github.com/Thinh-nguyen-03/wikigraph/internal/parser"
)

type Fetcher struct {
	collector *colly.Collector
	limiter   *rate.Limiter
}

type Result struct {
	Title       string
	ContentHash string
	Links       []parser.Link
	RedirectTo  string
	StatusCode  int
	Error       error
}

type Config struct {
	RateLimit      float64
	RequestTimeout time.Duration
	UserAgent      string
	BaseURL        string
}

func New(cfg Config) *Fetcher {
	c := colly.NewCollector(
		colly.UserAgent(cfg.UserAgent),
		colly.AllowedDomains("en.wikipedia.org"),
	)

	c.SetRequestTimeout(cfg.RequestTimeout)

	return &Fetcher{
		collector: c,
		limiter:   rate.NewLimiter(rate.Limit(cfg.RateLimit), 1),
	}
}

// Fetch retrieves a Wikipedia page and extracts its links.
func (f *Fetcher) Fetch(ctx context.Context, title string) *Result {
	result := &Result{Title: title}

	if err := f.limiter.Wait(ctx); err != nil {
		result.Error = fmt.Errorf("rate limit: %w", err)
		return result
	}

	pageURL := f.buildURL(title)

	c := f.collector.Clone()

	var html string
	var finalURL string

	c.OnResponse(func(r *colly.Response) {
		result.StatusCode = r.StatusCode
		finalURL = r.Request.URL.String()
		html = string(r.Body)
	})

	c.OnError(func(r *colly.Response, err error) {
		result.StatusCode = r.StatusCode
		result.Error = err
	})

	if err := c.Visit(pageURL); err != nil {
		result.Error = err
		return result
	}

	c.Wait()

	if result.Error != nil {
		return result
	}

	if result.StatusCode == 404 {
		return result
	}

	redirectTo := detectRedirect(pageURL, finalURL)
	if redirectTo != "" {
		result.RedirectTo = redirectTo
		return result
	}

	links, err := parser.ExtractLinksFromHTML(html)
	if err != nil {
		result.Error = fmt.Errorf("parsing html: %w", err)
		return result
	}

	result.Links = links
	result.ContentHash = hashContent(html)

	return result
}

func (f *Fetcher) buildURL(title string) string {
	encoded := url.PathEscape(strings.ReplaceAll(title, " ", "_"))
	return fmt.Sprintf("https://en.wikipedia.org/wiki/%s", encoded)
}

func detectRedirect(originalURL, finalURL string) string {
	if originalURL == finalURL {
		return ""
	}

	parsed, err := url.Parse(finalURL)
	if err != nil {
		return ""
	}

	path := parsed.Path
	if !strings.HasPrefix(path, "/wiki/") {
		return ""
	}

	title := strings.TrimPrefix(path, "/wiki/")
	decoded, err := url.PathUnescape(title)
	if err != nil {
		return title
	}

	return strings.ReplaceAll(decoded, "_", " ")
}

func hashContent(content string) string {
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:])
}
