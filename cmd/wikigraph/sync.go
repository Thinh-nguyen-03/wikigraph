package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Thinh-nguyen-03/wikigraph/internal/config"
	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
	"github.com/Thinh-nguyen-03/wikigraph/internal/neostore"
	"github.com/spf13/cobra"
)

var (
	syncBatchSize int
	clearDB       bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync data from SQLite to Neo4j",
	Long: `Synchronize data from the SQLite database to Neo4j graph database.

This command performs a one-time sync of data from SQLite to Neo4j.`,
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

	syncCmd.Flags().IntVar(&syncBatchSize, "batch-size", 10000, "Number of nodes/edges to sync per batch")
	syncCmd.Flags().BoolVar(&clearDB, "clear", false, "Clear Neo4j database before syncing")
}

func runSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.Neo4j.Enabled {
		slog.Warn("neo4j is disabled in config; set neostore.enabled=true to use neo4j")
	}

	slog.Info("opening sqlite database", "path", cfg.Database.Path)
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	slog.Info("connecting to neo4j", "uri", cfg.Neo4j.URI)
	neo4jClient, err := neostore.NewClient(neostore.Config{
		URI:                          cfg.Neo4j.URI,
		Username:                     cfg.Neo4j.Username,
		Password:                     cfg.Neo4j.Password,
		MaxConnectionPoolSize:        cfg.Neo4j.MaxConnectionPoolSize,
		ConnectionAcquisitionTimeout: cfg.Neo4j.ConnectionAcquisitionTimeout,
	})
	if err != nil {
		return fmt.Errorf("failed to create neo4j client: %w", err)
	}
	defer neo4jClient.Close(ctx)

	if err := neo4jClient.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("neo4j connectivity check failed: %w", err)
	}
	slog.Info("connected to neo4j successfully")

	if clearDB {
		slog.Info("clearing neo4j database")
		if err := neo4jClient.ClearDatabase(ctx); err != nil {
			return fmt.Errorf("failed to clear database: %w", err)
		}
		slog.Info("database cleared")
	}

	syncer := neostore.NewSyncer(neo4jClient, db.DB)

	slog.Info("starting full sync")
	stats, err := syncer.InitialSync(ctx, syncBatchSize)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}
	printSyncStats(stats)

	neo4jStats, err := neo4jClient.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}
	slog.Info("neo4j final counts", "nodes", neo4jStats.NodeCount, "edges", neo4jStats.EdgeCount)

	return nil
}

func runVerifySync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	neo4jClient, err := neostore.NewClient(neostore.Config{
		URI:                          cfg.Neo4j.URI,
		Username:                     cfg.Neo4j.Username,
		Password:                     cfg.Neo4j.Password,
		MaxConnectionPoolSize:        cfg.Neo4j.MaxConnectionPoolSize,
		ConnectionAcquisitionTimeout: cfg.Neo4j.ConnectionAcquisitionTimeout,
	})
	if err != nil {
		return fmt.Errorf("failed to create neo4j client: %w", err)
	}
	defer neo4jClient.Close(ctx)

	syncer := neostore.NewSyncer(neo4jClient, db.DB)
	return syncer.VerifySync(ctx)
}

func printSyncStats(stats *neostore.SyncStats) {
	sep := strings.Repeat("=", 60)
	fmt.Println(sep)
	fmt.Println("Sync complete")
	fmt.Println(sep)
	fmt.Printf("Duration:      %s\n", stats.Duration)
	fmt.Printf("Nodes created: %d\n", stats.NodesCreated)
	fmt.Printf("Edges created: %d\n", stats.EdgesCreated)
	if stats.Duration.Seconds() > 0 {
		fmt.Printf("Throughput:    %.0f nodes/sec, %.0f edges/sec\n",
			float64(stats.NodesCreated)/stats.Duration.Seconds(),
			float64(stats.EdgesCreated)/stats.Duration.Seconds(),
		)
	}
	fmt.Println(sep)
}
