package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
	"github.com/Thinh-nguyen-03/wikigraph/internal/fetcher"
	"github.com/Thinh-nguyen-03/wikigraph/internal/scraper"
)

var (
	maxDepth  int
	maxPages  int
	batchSize int
)

var fetchCmd = &cobra.Command{
	Use:   "fetch [pages...]",
	Short: "Fetch Wikipedia pages and extract links",
	Long: `Fetch one or more Wikipedia pages and extract their links.

Examples:
  wikigraph fetch "Albert Einstein"
  wikigraph fetch "Physics" "Mathematics" --depth 2
  wikigraph fetch "Computer Science" --depth 3 --max-pages 100`,
	Args: cobra.MinimumNArgs(1),
	RunE: runFetch,
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	fetchCmd.Flags().IntVarP(&maxDepth, "depth", "d", 1, "maximum crawl depth (1 = only seed pages)")
	fetchCmd.Flags().IntVarP(&maxPages, "max-pages", "m", 0, "maximum pages to fetch (0 = unlimited)")
	fetchCmd.Flags().IntVarP(&batchSize, "batch", "b", 10, "pages to fetch per batch")
}

func runFetch(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nInterrupted, shutting down...")
		cancel()
	}()

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	c := cache.New(db)
	f := fetcher.New(fetcher.Config{
		RateLimit:      cfg.Scraper.RateLimit,
		RequestTimeout: cfg.Scraper.RequestTimeout,
		UserAgent:      cfg.Scraper.UserAgent,
		BaseURL:        cfg.Scraper.WikipediaAPIURL,
	})

	s := scraper.New(c, f, scraper.Config{
		MaxDepth:  maxDepth,
		BatchSize: batchSize,
		MaxPages:  maxPages,
	})

	stats, err := s.Crawl(ctx, args)
	if err != nil && err != context.Canceled {
		return err
	}

	fmt.Printf("\nCrawl complete:\n")
	fmt.Printf("  Pages fetched: %d\n", stats.PagesFetched)
	fmt.Printf("  Pages skipped: %d\n", stats.PagesSkipped)
	fmt.Printf("  Links found:   %d\n", stats.LinksFound)
	fmt.Printf("  Errors:        %d\n", stats.Errors)
	fmt.Printf("  Duration:      %s\n", stats.Duration.Truncate(time.Millisecond))

	return nil
}
