package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Thinh-nguyen-03/wikigraph/internal/config"
	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
	"github.com/Thinh-nguyen-03/wikigraph/internal/neo4j"
	"github.com/spf13/cobra"
)

var (
	syncLimit     int
	syncBatchSize int
	clearDB       bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync data from SQLite to Neo4j",
	Long: `Synchronize data from the SQLite database to Neo4j graph database.

This command performs a one-time sync of data from SQLite to Neo4j.
Use --limit to test with a small subset of data first.`,
	RunE: runSync,
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify sync consistency between SQLite and Neo4j",
	Long:  `Check that the node and edge counts match between SQLite and Neo4j.`,
	RunE:  runVerifySync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(verifyCmd)

	syncCmd.Flags().IntVar(&syncLimit, "limit", 0, "Limit the number of pages to sync (0 = all pages)")
	syncCmd.Flags().IntVar(&syncBatchSize, "batch-size", 10000, "Number of nodes/edges to sync per batch")
	syncCmd.Flags().BoolVar(&clearDB, "clear", false, "Clear Neo4j database before syncing")
}

func runSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if Neo4j is enabled
	if !cfg.Neo4j.Enabled {
		log.Println("Warning: Neo4j is disabled in config. Set neo4j.enabled=true to use Neo4j.")
		log.Println("Continuing anyway for testing purposes...")
	}

	// Open SQLite database
	log.Printf("Opening SQLite database: %s", cfg.Database.Path)
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Connect to Neo4j
	log.Printf("Connecting to Neo4j at %s", cfg.Neo4j.URI)
	neo4jClient, err := neo4j.NewClient(neo4j.Config{
		URI:                          cfg.Neo4j.URI,
		Username:                     cfg.Neo4j.Username,
		Password:                     cfg.Neo4j.Password,
		MaxConnectionPoolSize:        cfg.Neo4j.MaxConnectionPoolSize,
		ConnectionAcquisitionTimeout: cfg.Neo4j.ConnectionAcquisitionTimeout,
	})
	if err != nil {
		return fmt.Errorf("failed to create Neo4j client: %w", err)
	}
	defer neo4jClient.Close(ctx)

	// Verify connectivity
	log.Println("Verifying Neo4j connectivity...")
	if err := neo4jClient.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("Neo4j connectivity check failed: %w", err)
	}
	log.Println("✓ Connected to Neo4j successfully")

	// Clear database if requested
	if clearDB {
		log.Println("Clearing Neo4j database...")
		if err := neo4jClient.ClearDatabase(ctx); err != nil {
			return fmt.Errorf("failed to clear database: %w", err)
		}
		log.Println("✓ Database cleared")
	}

	// Create syncer
	syncer := neo4j.NewSyncer(neo4jClient, db.DB, log.Default())

	// Perform sync
	if syncLimit > 0 {
		log.Printf("Warning: --limit flag not fully implemented yet. Syncing all data.")
		log.Printf("For testing, use a smaller test database instead.")
	}

	log.Println("Starting full sync...")
	stats, err := syncer.InitialSync(ctx, syncBatchSize)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}
	printStats(stats)

	// Get final stats
	log.Println("\nFetching Neo4j statistics...")
	neo4jStats, err := neo4jClient.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	log.Printf("✓ Neo4j now contains:")
	log.Printf("  - %d nodes", neo4jStats.NodeCount)
	log.Printf("  - %d edges", neo4jStats.EdgeCount)

	return nil
}

func runVerifySync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open SQLite database
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Connect to Neo4j
	neo4jClient, err := neo4j.NewClient(neo4j.Config{
		URI:                          cfg.Neo4j.URI,
		Username:                     cfg.Neo4j.Username,
		Password:                     cfg.Neo4j.Password,
		MaxConnectionPoolSize:        cfg.Neo4j.MaxConnectionPoolSize,
		ConnectionAcquisitionTimeout: cfg.Neo4j.ConnectionAcquisitionTimeout,
	})
	if err != nil {
		return fmt.Errorf("failed to create Neo4j client: %w", err)
	}
	defer neo4jClient.Close(ctx)

	// Create syncer and verify
	syncer := neo4j.NewSyncer(neo4jClient, db.DB, log.Default())
	if err := syncer.VerifySync(ctx); err != nil {
		return err
	}

	return nil
}

func printStats(stats *neo4j.SyncStats) {
	separator := strings.Repeat("=", 60)
	log.Println("\n" + separator)
	log.Println("Sync completed successfully!")
	log.Println(separator)
	log.Printf("Duration:      %s", stats.Duration)
	log.Printf("Nodes created: %d", stats.NodesCreated)
	log.Printf("Edges created: %d", stats.EdgesCreated)

	if stats.Duration > 0 {
		nodesPerSec := float64(stats.NodesCreated) / stats.Duration.Seconds()
		edgesPerSec := float64(stats.EdgesCreated) / stats.Duration.Seconds()
		log.Printf("Throughput:    %.0f nodes/sec, %.0f edges/sec", nodesPerSec, edgesPerSec)
	}
	log.Println(separator)
}
