package scraper

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
	"github.com/Thinh-nguyen-03/wikigraph/internal/fetcher"
)

func setupTest(t *testing.T) (*cache.Cache, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "wikigraph-scraper-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}

	db, err := database.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("opening database: %v", err)
	}

	if err := db.Migrate(); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("running migrations: %v", err)
	}

	return cache.New(db), func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}
}

func TestNew(t *testing.T) {
	c, cleanup := setupTest(t)
	defer cleanup()

	f := fetcher.New(fetcher.Config{
		RateLimit:      1.0,
		RequestTimeout: 5 * time.Second,
		UserAgent:      "Test",
		BaseURL:        "https://en.wikipedia.org",
	})

	s := New(c, f, Config{
		MaxDepth:  3,
		BatchSize: 10,
	})

	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.cfg.BatchSize != 10 {
		t.Errorf("BatchSize = %d, want 10", s.cfg.BatchSize)
	}
}

func TestNew_DefaultBatchSize(t *testing.T) {
	c, cleanup := setupTest(t)
	defer cleanup()

	f := fetcher.New(fetcher.Config{
		RateLimit:      1.0,
		RequestTimeout: 5 * time.Second,
		UserAgent:      "Test",
		BaseURL:        "https://en.wikipedia.org",
	})

	s := New(c, f, Config{MaxDepth: 3})

	if s.cfg.BatchSize != 10 {
		t.Errorf("default BatchSize = %d, want 10", s.cfg.BatchSize)
	}
}

func TestCrawl_ContextCancellation(t *testing.T) {
	c, cleanup := setupTest(t)
	defer cleanup()

	f := fetcher.New(fetcher.Config{
		RateLimit:      1.0,
		RequestTimeout: 5 * time.Second,
		UserAgent:      "Test",
		BaseURL:        "https://en.wikipedia.org",
	})

	s := New(c, f, Config{MaxDepth: 3, BatchSize: 10})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.Crawl(ctx, []string{"Test"})
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestCrawl_CreatesSeeds(t *testing.T) {
	c, cleanup := setupTest(t)
	defer cleanup()

	f := fetcher.New(fetcher.Config{
		RateLimit:      0.1, // Very slow to prevent actual fetches
		RequestTimeout: 1 * time.Second,
		UserAgent:      "Test",
		BaseURL:        "https://en.wikipedia.org",
	})

	s := New(c, f, Config{MaxDepth: 1, BatchSize: 1, MaxPages: 0})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	s.Crawl(ctx, []string{"Seed1", "Seed2"})

	page1, _ := c.GetPage("Seed1")
	page2, _ := c.GetPage("Seed2")

	if page1 == nil {
		t.Error("Seed1 page should exist")
	}
	if page2 == nil {
		t.Error("Seed2 page should exist")
	}
}
