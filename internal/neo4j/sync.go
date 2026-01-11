package neo4j

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Syncer handles synchronization between SQLite and Neo4j
type Syncer struct {
	client *Client
	db     *sql.DB
	logger *log.Logger
}

// NewSyncer creates a new syncer instance
func NewSyncer(client *Client, db *sql.DB, logger *log.Logger) *Syncer {
	if logger == nil {
		logger = log.Default()
	}
	return &Syncer{
		client: client,
		db:     db,
		logger: logger,
	}
}

// SyncStats holds statistics about a sync operation
type SyncStats struct {
	NodesCreated int64
	EdgesCreated int64
	Duration     time.Duration
	StartTime    time.Time
	EndTime      time.Time
}

// InitialSync performs a full sync from SQLite to Neo4j
// This is a one-time operation to populate Neo4j with all existing data
func (s *Syncer) InitialSync(ctx context.Context, batchSize int) (*SyncStats, error) {
	if batchSize == 0 {
		batchSize = 10000 // Default batch size
	}

	stats := &SyncStats{
		StartTime: time.Now(),
	}

	s.logger.Println("Starting initial sync from SQLite to Neo4j...")

	// Step 1: Initialize schema
	s.logger.Println("Initializing Neo4j schema...")
	if err := s.client.InitializeSchema(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Step 2: Sync all successful pages as nodes
	s.logger.Println("Syncing page nodes...")
	nodesCreated, err := s.syncNodes(ctx, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to sync nodes: %w", err)
	}
	stats.NodesCreated = nodesCreated
	s.logger.Printf("Created %d nodes", nodesCreated)

	// Step 3: Sync all links as edges
	s.logger.Println("Syncing link edges...")
	edgesCreated, err := s.syncEdges(ctx, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to sync edges: %w", err)
	}
	stats.EdgesCreated = edgesCreated
	s.logger.Printf("Created %d edges", edgesCreated)

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	s.logger.Printf("Initial sync completed in %s", stats.Duration)
	return stats, nil
}

// syncNodes syncs all successful pages from SQLite to Neo4j
func (s *Syncer) syncNodes(ctx context.Context, batchSize int) (int64, error) {
	// Query all successful pages
	query := `
		SELECT title
		FROM pages
		WHERE fetch_status = 'success'
		ORDER BY id
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to query pages: %w", err)
	}
	defer rows.Close()

	var totalCreated int64
	batch := make([]string, 0, batchSize)

	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return totalCreated, fmt.Errorf("failed to scan row: %w", err)
		}

		batch = append(batch, title)

		if len(batch) >= batchSize {
			if err := s.client.CreateNodesBatch(ctx, batch); err != nil {
				return totalCreated, fmt.Errorf("failed to create node batch: %w", err)
			}
			totalCreated += int64(len(batch))

			// Log progress every batch
			if totalCreated%100000 == 0 {
				s.logger.Printf("Progress: %d nodes synced...", totalCreated)
			}

			batch = batch[:0] // Clear batch
		}
	}

	// Create remaining batch
	if len(batch) > 0 {
		if err := s.client.CreateNodesBatch(ctx, batch); err != nil {
			return totalCreated, fmt.Errorf("failed to create final node batch: %w", err)
		}
		totalCreated += int64(len(batch))
	}

	return totalCreated, rows.Err()
}

// syncEdges syncs all links from SQLite to Neo4j
func (s *Syncer) syncEdges(ctx context.Context, batchSize int) (int64, error) {
	// Query all links with source page titles
	query := `
		SELECT p.title AS source_title, l.target_title
		FROM links l
		JOIN pages p ON p.id = l.source_id
		WHERE p.fetch_status = 'success'
		ORDER BY l.id
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to query links: %w", err)
	}
	defer rows.Close()

	var totalCreated int64
	batch := make([]EdgeInput, 0, batchSize)

	for rows.Next() {
		var sourceTitle, targetTitle string
		if err := rows.Scan(&sourceTitle, &targetTitle); err != nil {
			return totalCreated, fmt.Errorf("failed to scan row: %w", err)
		}

		batch = append(batch, EdgeInput{
			SourceTitle: sourceTitle,
			TargetTitle: targetTitle,
		})

		if len(batch) >= batchSize {
			if err := s.client.CreateEdgesBatch(ctx, batch); err != nil {
				return totalCreated, fmt.Errorf("failed to create edge batch: %w", err)
			}
			totalCreated += int64(len(batch))

			// Log progress every batch
			if totalCreated%1000000 == 0 {
				s.logger.Printf("Progress: %d edges synced...", totalCreated)
			}

			batch = batch[:0] // Clear batch
		}
	}

	// Create remaining batch
	if len(batch) > 0 {
		if err := s.client.CreateEdgesBatch(ctx, batch); err != nil {
			return totalCreated, fmt.Errorf("failed to create final edge batch: %w", err)
		}
		totalCreated += int64(len(batch))
	}

	return totalCreated, rows.Err()
}

// IncrementalSync syncs only new or updated data since the given timestamp
func (s *Syncer) IncrementalSync(ctx context.Context, since time.Time, batchSize int) (*SyncStats, error) {
	if batchSize == 0 {
		batchSize = 5000
	}

	stats := &SyncStats{
		StartTime: time.Now(),
	}

	s.logger.Printf("Starting incremental sync (since %s)...", since.Format(time.RFC3339))

	// Sync new pages
	nodesCreated, err := s.syncNewNodes(ctx, since, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to sync new nodes: %w", err)
	}
	stats.NodesCreated = nodesCreated

	// Sync new links
	edgesCreated, err := s.syncNewEdges(ctx, since, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to sync new edges: %w", err)
	}
	stats.EdgesCreated = edgesCreated

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	if stats.NodesCreated > 0 || stats.EdgesCreated > 0 {
		s.logger.Printf("Incremental sync completed: %d nodes, %d edges in %s",
			stats.NodesCreated, stats.EdgesCreated, stats.Duration)
	}

	return stats, nil
}

// syncNewNodes syncs pages created after the given timestamp
func (s *Syncer) syncNewNodes(ctx context.Context, since time.Time, batchSize int) (int64, error) {
	query := `
		SELECT title
		FROM pages
		WHERE fetch_status = 'success' AND created_at > ?
		ORDER BY id
	`

	rows, err := s.db.QueryContext(ctx, query, since.Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("failed to query new pages: %w", err)
	}
	defer rows.Close()

	var totalCreated int64
	batch := make([]string, 0, batchSize)

	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return totalCreated, fmt.Errorf("failed to scan row: %w", err)
		}

		batch = append(batch, title)

		if len(batch) >= batchSize {
			if err := s.client.CreateNodesBatch(ctx, batch); err != nil {
				return totalCreated, fmt.Errorf("failed to create node batch: %w", err)
			}
			totalCreated += int64(len(batch))
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := s.client.CreateNodesBatch(ctx, batch); err != nil {
			return totalCreated, fmt.Errorf("failed to create final node batch: %w", err)
		}
		totalCreated += int64(len(batch))
	}

	return totalCreated, rows.Err()
}

// syncNewEdges syncs links created after the given timestamp
func (s *Syncer) syncNewEdges(ctx context.Context, since time.Time, batchSize int) (int64, error) {
	query := `
		SELECT p.title AS source_title, l.target_title
		FROM links l
		JOIN pages p ON p.id = l.source_id
		WHERE p.fetch_status = 'success' AND l.created_at > ?
		ORDER BY l.id
	`

	rows, err := s.db.QueryContext(ctx, query, since.Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("failed to query new links: %w", err)
	}
	defer rows.Close()

	var totalCreated int64
	batch := make([]EdgeInput, 0, batchSize)

	for rows.Next() {
		var sourceTitle, targetTitle string
		if err := rows.Scan(&sourceTitle, &targetTitle); err != nil {
			return totalCreated, fmt.Errorf("failed to scan row: %w", err)
		}

		batch = append(batch, EdgeInput{
			SourceTitle: sourceTitle,
			TargetTitle: targetTitle,
		})

		if len(batch) >= batchSize {
			if err := s.client.CreateEdgesBatch(ctx, batch); err != nil {
				return totalCreated, fmt.Errorf("failed to create edge batch: %w", err)
			}
			totalCreated += int64(len(batch))
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := s.client.CreateEdgesBatch(ctx, batch); err != nil {
			return totalCreated, fmt.Errorf("failed to create final edge batch: %w", err)
		}
		totalCreated += int64(len(batch))
	}

	return totalCreated, rows.Err()
}

// VerifySync compares counts between SQLite and Neo4j to check consistency
func (s *Syncer) VerifySync(ctx context.Context) error {
	s.logger.Println("Verifying sync consistency...")

	// Get SQLite counts
	var sqliteNodes, sqliteEdges int64

	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pages WHERE fetch_status = 'success'").Scan(&sqliteNodes)
	if err != nil {
		return fmt.Errorf("failed to count SQLite nodes: %w", err)
	}

	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM links").Scan(&sqliteEdges)
	if err != nil {
		return fmt.Errorf("failed to count SQLite edges: %w", err)
	}

	// Get Neo4j counts
	neo4jStats, err := s.client.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Neo4j stats: %w", err)
	}

	s.logger.Printf("SQLite: %d nodes, %d edges", sqliteNodes, sqliteEdges)
	s.logger.Printf("Neo4j:  %d nodes, %d edges", neo4jStats.NodeCount, neo4jStats.EdgeCount)

	if sqliteNodes != neo4jStats.NodeCount {
		return fmt.Errorf("node count mismatch: SQLite has %d, Neo4j has %d", sqliteNodes, neo4jStats.NodeCount)
	}

	if sqliteEdges != neo4jStats.EdgeCount {
		return fmt.Errorf("edge count mismatch: SQLite has %d, Neo4j has %d", sqliteEdges, neo4jStats.EdgeCount)
	}

	s.logger.Println("Sync verification passed!")
	return nil
}
