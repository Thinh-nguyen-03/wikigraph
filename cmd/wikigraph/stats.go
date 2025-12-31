package main

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show database statistics",
	RunE:  runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	stats, err := db.Stats()
	if err != nil {
		return fmt.Errorf("getting stats: %w", err)
	}

	fmt.Printf("Database: %s\n", cfg.Database.Path)
	fmt.Printf("Size:     %s\n\n", humanize.Bytes(uint64(stats.DatabaseSizeBytes)))

	fmt.Printf("Pages:\n")
	fmt.Printf("  Total:     %d\n", stats.TotalPages)
	fmt.Printf("  Fetched:   %d\n", stats.FetchedPages)
	fmt.Printf("  Pending:   %d\n", stats.PendingPages)
	fmt.Printf("  Redirects: %d\n", stats.RedirectPages)
	fmt.Printf("  Not Found: %d\n", stats.NotFoundPages)
	fmt.Printf("  Errors:    %d\n", stats.ErrorPages)
	fmt.Printf("\nLinks:     %d\n", stats.TotalLinks)

	if stats.OldestFetch.Valid {
		fmt.Printf("\nOldest fetch: %s\n", stats.OldestFetch.String)
	}
	if stats.NewestFetch.Valid {
		fmt.Printf("Newest fetch: %s\n", stats.NewestFetch.String)
	}

	return nil
}
