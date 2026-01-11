package main

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Rebuild database query statistics",
	Long: `Run ANALYZE on the database to rebuild query planner statistics.

This command updates SQLite's internal statistics about the distribution of data
in indexes, which helps the query optimizer choose better execution plans.

When to run:
  - After importing large amounts of data
  - After significant crawl operations
  - If queries seem slower than expected
  - Periodically (e.g., weekly) for large databases

This is NOT run on every server startup to avoid long startup times.
For databases with millions of rows, ANALYZE can take 2-10 minutes.

Examples:
  wikigraph analyze
  wikigraph analyze --database /path/to/wikigraph.db`,
	RunE: runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	slog.Info("opening database", "path", cfg.Database.Path)

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	slog.Info("running ANALYZE - this may take several minutes for large databases...")
	start := time.Now()

	if err := db.Analyze(); err != nil {
		return fmt.Errorf("ANALYZE failed: %w", err)
	}

	duration := time.Since(start)
	slog.Info("ANALYZE complete",
		"duration", duration.Round(time.Millisecond),
	)

	fmt.Printf("\nDatabase statistics updated successfully in %s\n", duration.Round(time.Millisecond))
	return nil
}
