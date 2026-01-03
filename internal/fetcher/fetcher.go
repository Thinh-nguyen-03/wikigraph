// Package fetcher provides Wikipedia page fetching with rate limiting.
package fetcher

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"golang.org/x/time/rate"

	"github.com/Thinh-nguyen-03/wikigraph/internal/parser"
)

type pendingRequest struct {
	result   *Result
	html     []byte
	finalURL string
	done     chan struct{}
}

type Fetcher struct {
	collector *colly.Collector
	limiter   *rate.Limiter
	pending   sync.Map
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
	// Burst size accommodates concurrent workers for parallel requests
	burstSize := 50
	if cfg.RateLimit < 50 {
		burstSize = int(cfg.RateLimit)
	}

	f := &Fetcher{
		limiter: rate.NewLimiter(rate.Limit(cfg.RateLimit), burstSize),
	}

	c := colly.NewCollector(
		colly.UserAgent(cfg.UserAgent),
		colly.AllowedDomains("en.wikipedia.org"),
		colly.Async(true),
	)

	c.SetRequestTimeout(cfg.RequestTimeout)

	c.OnResponse(func(r *colly.Response) {
		urlStr := r.Request.URL.String()
		if val, ok := f.pending.Load(urlStr); ok {
			req := val.(*pendingRequest)
			req.result.StatusCode = r.StatusCode
			req.finalURL = r.Request.URL.String()
			req.html = make([]byte, len(r.Body))
			copy(req.html, r.Body)
			close(req.done)
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		urlStr := r.Request.URL.String()
		if val, ok := f.pending.Load(urlStr); ok {
			req := val.(*pendingRequest)
			req.result.StatusCode = r.StatusCode
			req.result.Error = err
			close(req.done)
		}
	})

	f.collector = c
	return f
}

func (f *Fetcher) Fetch(ctx context.Context, title string) *Result {
	result := &Result{Title: title}

	if err := f.limiter.Wait(ctx); err != nil {
		result.Error = fmt.Errorf("rate limit: %w", err)
		return result
	}

	pageURL := f.buildURL(title)

	req := &pendingRequest{
		result: result,
		done:   make(chan struct{}),
	}

	f.pending.Store(pageURL, req)
	defer f.pending.Delete(pageURL)

	if err := f.collector.Visit(pageURL); err != nil {
		result.Error = err
		return result
	}

	select {
	case <-req.done:
	case <-ctx.Done():
		result.Error = ctx.Err()
		return result
	}

	if result.Error != nil {
		return result
	}

	if result.StatusCode == 404 {
		return result
	}

	redirectTo := detectRedirect(pageURL, req.finalURL)
	if redirectTo != "" {
		result.RedirectTo = redirectTo
		return result
	}

	links, err := parser.ExtractLinksFromBytes(req.html)
	if err != nil {
		result.Error = fmt.Errorf("parsing html: %w", err)
		return result
	}

	result.Links = links
	result.ContentHash = hashContentBytes(req.html)

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

func hashContentBytes(content []byte) string {
	hash := md5.Sum(content)
	return hex.EncodeToString(hash[:])
}
