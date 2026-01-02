// Package scraper orchestrates Wikipedia crawling.
package scraper

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
	"github.com/Thinh-nguyen-03/wikigraph/internal/fetcher"
)

type Scraper struct {
	cache   *cache.Cache
	fetcher *fetcher.Fetcher
	cfg     Config
}

type Config struct {
	MaxDepth    int
	BatchSize   int
	MaxPages    int
	StopOnError bool
	Workers     int // Number of concurrent fetch workers
}

type Stats struct {
	PagesFetched int
	PagesSkipped int
	LinksFound   int
	Errors       int
	Duration     time.Duration
}

func New(c *cache.Cache, f *fetcher.Fetcher, cfg Config) *Scraper {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 5
	}
	return &Scraper{
		cache:   c,
		fetcher: f,
		cfg:     cfg,
	}
}

// Starts crawling from the given seed pages.
func (s *Scraper) Crawl(ctx context.Context, seeds []string) (*Stats, error) {
	start := time.Now()
	stats := &Stats{}

	for _, seed := range seeds {
		if _, err := s.cache.GetOrCreatePage(seed); err != nil {
			return stats, fmt.Errorf("creating seed page %q: %w", seed, err)
		}
	}

	slog.Info("starting crawl", "seeds", len(seeds), "max_depth", s.cfg.MaxDepth)

	for depth := 0; depth < s.cfg.MaxDepth; depth++ {
		select {
		case <-ctx.Done():
			stats.Duration = time.Since(start)
			return stats, ctx.Err()
		default:
		}

		processed, err := s.processDepth(ctx, stats)
		if err != nil {
			stats.Duration = time.Since(start)
			if s.cfg.StopOnError {
				return stats, fmt.Errorf("crawl at depth %d: %w", depth, err)
			}
			slog.Warn("error during crawl", "error", err)
		}

		if processed == 0 {
			slog.Info("no more pending pages, stopping")
			break
		}

		if s.cfg.MaxPages > 0 && stats.PagesFetched >= s.cfg.MaxPages {
			slog.Info("reached max pages limit", "limit", s.cfg.MaxPages)
			break
		}
	}

	stats.Duration = time.Since(start)
	slog.Info("crawl complete",
		"pages_fetched", stats.PagesFetched,
		"links_found", stats.LinksFound,
		"errors", stats.Errors,
		"duration", stats.Duration,
	)

	return stats, nil
}

type pageResult struct {
	page    *cache.Page
	targets []string
	fetched bool
	skipped bool
	links   int
	err     error
}

func (s *Scraper) processDepth(ctx context.Context, stats *Stats) (int, error) {
	limit := s.cfg.BatchSize
	if s.cfg.MaxPages > 0 {
		remaining := s.cfg.MaxPages - stats.PagesFetched
		if remaining < limit {
			limit = remaining
		}
		if limit <= 0 {
			return 0, nil
		}
	}

	pages, err := s.cache.GetPendingPages(limit)
	if err != nil {
		return 0, fmt.Errorf("getting pending pages: %w", err)
	}

	if len(pages) == 0 {
		return 0, nil
	}

	numWorkers := s.cfg.Workers
	if numWorkers > len(pages) {
		numWorkers = len(pages)
	}

	jobs := make(chan *cache.Page, len(pages))
	results := make(chan pageResult, len(pages))

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for page := range jobs {
				select {
				case <-ctx.Done():
					results <- pageResult{page: page, err: ctx.Err()}
					return
				default:
				}
				targets, fetched, skipped, links, err := s.processPageWorker(ctx, page)
				results <- pageResult{
					page:    page,
					targets: targets,
					fetched: fetched,
					skipped: skipped,
					links:   links,
					err:     err,
				}
			}
		}()
	}

	go func() {
		for _, page := range pages {
			jobs <- page
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	allTargets := make(map[string]struct{})
	var firstError error

	for result := range results {
		if result.err != nil {
			stats.Errors++
			if s.cfg.StopOnError && firstError == nil {
				firstError = result.err
			}
			slog.Warn("failed to process page", "title", result.page.Title, "error", result.err)
			continue
		}

		if result.fetched {
			stats.PagesFetched++
			stats.LinksFound += result.links
		}
		if result.skipped {
			stats.PagesSkipped++
		}

		for _, t := range result.targets {
			allTargets[t] = struct{}{}
		}
	}

	if firstError != nil && s.cfg.StopOnError {
		return len(pages), firstError
	}

	if len(allTargets) > 0 {
		targetSlice := make([]string, 0, len(allTargets))
		for t := range allTargets {
			targetSlice = append(targetSlice, t)
		}
		if err := s.cache.EnsureTargetPagesExist(targetSlice); err != nil {
			return len(pages), fmt.Errorf("creating target pages: %w", err)
		}
	}

	return len(pages), nil
}

// processPageWorker processes a single page and returns individual stats instead of updating shared Stats.
func (s *Scraper) processPageWorker(ctx context.Context, page *cache.Page) (targets []string, fetched, skipped bool, links int, err error) {
	slog.Debug("fetching page", "title", page.Title)

	result := s.fetcher.Fetch(ctx, page.Title)

	if result.Error != nil {
		if updateErr := s.cache.UpdatePageStatus(page.Title, cache.StatusError, "", ""); updateErr != nil {
			return nil, false, false, 0, fmt.Errorf("updating error status: %w", updateErr)
		}
		return nil, false, false, 0, result.Error
	}

	if result.StatusCode == 404 {
		if updateErr := s.cache.UpdatePageStatus(page.Title, cache.StatusNotFound, "", ""); updateErr != nil {
			return nil, false, false, 0, fmt.Errorf("updating not_found status: %w", updateErr)
		}
		return nil, false, true, 0, nil
	}

	if result.RedirectTo != "" {
		if updateErr := s.cache.UpdatePageStatus(page.Title, cache.StatusRedirect, "", result.RedirectTo); updateErr != nil {
			return nil, false, false, 0, fmt.Errorf("updating redirect status: %w", updateErr)
		}
		return []string{result.RedirectTo}, false, true, 0, nil
	}

	contentUnchanged := page.ContentHash.Valid && page.ContentHash.String == result.ContentHash
	if contentUnchanged {
		slog.Debug("content unchanged, skipping link update", "title", page.Title)
		if updateErr := s.cache.UpdatePageStatus(page.Title, cache.StatusSuccess, result.ContentHash, ""); updateErr != nil {
			return nil, false, false, 0, fmt.Errorf("updating success status: %w", updateErr)
		}
		return nil, false, true, 0, nil
	}

	cacheLinks := make([]cache.Link, len(result.Links))
	targetTitles := make([]string, len(result.Links))
	for i, link := range result.Links {
		cacheLinks[i] = cache.Link{
			TargetTitle: link.Title,
			AnchorText:  sql.NullString{String: link.AnchorText, Valid: link.AnchorText != ""},
		}
		targetTitles[i] = link.Title
	}

	if deleteErr := s.cache.DeleteLinksFromPage(page.ID); deleteErr != nil {
		return nil, false, false, 0, fmt.Errorf("clearing old links: %w", deleteErr)
	}
	if addErr := s.cache.AddLinks(page.ID, cacheLinks); addErr != nil {
		return nil, false, false, 0, fmt.Errorf("adding links: %w", addErr)
	}

	if updateErr := s.cache.UpdatePageStatus(page.Title, cache.StatusSuccess, result.ContentHash, ""); updateErr != nil {
		return nil, false, false, 0, fmt.Errorf("updating success status: %w", updateErr)
	}

	slog.Debug("processed page", "title", page.Title, "links", len(result.Links))

	return targetTitles, true, false, len(result.Links), nil
}

// Fetches a single page without BFS expansion.
func (s *Scraper) FetchSingle(ctx context.Context, title string) (*Stats, error) {
	start := time.Now()
	stats := &Stats{}

	page, err := s.cache.GetOrCreatePage(title)
	if err != nil {
		return stats, fmt.Errorf("creating page: %w", err)
	}

	if page.FetchStatus != cache.StatusPending {
		slog.Info("page already fetched", "title", title, "status", page.FetchStatus)
		stats.PagesSkipped = 1
		stats.Duration = time.Since(start)
		return stats, nil
	}

	targets, fetched, skipped, links, err := s.processPageWorker(ctx, page)
	if err != nil {
		stats.Errors = 1
		stats.Duration = time.Since(start)
		return stats, fmt.Errorf("fetching page %q: %w", title, err)
	}

	if fetched {
		stats.PagesFetched = 1
		stats.LinksFound = links
	}
	if skipped {
		stats.PagesSkipped = 1
	}

	// For single fetch, insert targets immediately
	if len(targets) > 0 {
		if err := s.cache.EnsureTargetPagesExist(targets); err != nil {
			return stats, fmt.Errorf("creating target pages: %w", err)
		}
	}

	stats.Duration = time.Since(start)
	return stats, nil
}
