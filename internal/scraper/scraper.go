// Package scraper orchestrates Wikipedia crawling.
package scraper

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
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
	MaxDepth      int
	BatchSize     int
	MaxPages      int
	StopOnError   bool
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
				return stats, err
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

	for _, page := range pages {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		if err := s.processPage(ctx, page, stats); err != nil {
			stats.Errors++
			if s.cfg.StopOnError {
				return 0, err
			}
			slog.Warn("failed to process page", "title", page.Title, "error", err)
		}
	}

	return len(pages), nil
}

func (s *Scraper) processPage(ctx context.Context, page *cache.Page, stats *Stats) error {
	slog.Debug("fetching page", "title", page.Title)

	result := s.fetcher.Fetch(ctx, page.Title)

	if result.Error != nil {
		if err := s.cache.UpdatePageStatus(page.Title, cache.StatusError, "", ""); err != nil {
			return fmt.Errorf("updating error status: %w", err)
		}
		return result.Error
	}

	if result.StatusCode == 404 {
		if err := s.cache.UpdatePageStatus(page.Title, cache.StatusNotFound, "", ""); err != nil {
			return fmt.Errorf("updating not_found status: %w", err)
		}
		stats.PagesSkipped++
		return nil
	}

	if result.RedirectTo != "" {
		if err := s.cache.UpdatePageStatus(page.Title, cache.StatusRedirect, "", result.RedirectTo); err != nil {
			return fmt.Errorf("updating redirect status: %w", err)
		}
		if _, err := s.cache.GetOrCreatePage(result.RedirectTo); err != nil {
			return fmt.Errorf("creating redirect target: %w", err)
		}
		stats.PagesSkipped++
		return nil
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

	if err := s.cache.DeleteLinksFromPage(page.ID); err != nil {
		return fmt.Errorf("clearing old links: %w", err)
	}
	if err := s.cache.AddLinks(page.ID, cacheLinks); err != nil {
		return fmt.Errorf("adding links: %w", err)
	}

	if err := s.cache.EnsureTargetPagesExist(targetTitles); err != nil {
		return fmt.Errorf("creating target pages: %w", err)
	}

	if err := s.cache.UpdatePageStatus(page.Title, cache.StatusSuccess, result.ContentHash, ""); err != nil {
		return fmt.Errorf("updating success status: %w", err)
	}

	stats.PagesFetched++
	stats.LinksFound += len(result.Links)

	slog.Debug("processed page", "title", page.Title, "links", len(result.Links))

	return nil
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

	if err := s.processPage(ctx, page, stats); err != nil {
		stats.Errors = 1
		stats.Duration = time.Since(start)
		return stats, err
	}

	stats.Duration = time.Since(start)
	return stats, nil
}
