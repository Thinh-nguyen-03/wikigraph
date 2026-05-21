package neostore

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// Syncer handles synchronization between SQLite and Neo4j
type Syncer struct {
	client *Client
	db     *sql.DB
}

// NewSyncer creates a new syncer instance
func NewSyncer(client *Client, db *sql.DB) *Syncer {
	return &Syncer{client: client, db: db}
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
func (s *Syncer) InitialSync(ctx context.Context, batchSize int) (*SyncStats, error) {
	if batchSize == 0 {
		batchSize = 10000
	}

	stats := &SyncStats{StartTime: time.Now()}

	slog.Info("starting initial sync from sqlite to neo4j")

	slog.Info("initializing neo4j schema")
	if err := s.client.InitializeSchema(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	slog.Info("syncing page nodes")
	nodesCreated, err := s.syncNodes(ctx, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to sync nodes: %w", err)
	}
	stats.NodesCreated = nodesCreated
	slog.Info("nodes synced", "count", nodesCreated)

	slog.Info("syncing link edges")
	edgesCreated, err := s.syncEdges(ctx, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to sync edges: %w", err)
	}
	stats.EdgesCreated = edgesCreated
	slog.Info("edges synced", "count", edgesCreated)

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)
	slog.Info("initial sync complete", "duration", stats.Duration)
	return stats, nil
}

// syncNodes syncs all successful pages from SQLite to Neo4j
func (s *Syncer) syncNodes(ctx context.Context, batchSize int) (int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT title FROM pages WHERE fetch_status = 'success' ORDER BY id
	`)
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
			if totalCreated%100000 == 0 {
				slog.Info("sync progress", "nodes_synced", totalCreated)
			}
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

// syncEdges syncs all links from SQLite to Neo4j
func (s *Syncer) syncEdges(ctx context.Context, batchSize int) (int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.title AS source_title, l.target_title
		FROM links l
		JOIN pages p ON p.id = l.source_id
		WHERE p.fetch_status = 'success'
		ORDER BY l.id
	`)
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
		batch = append(batch, EdgeInput{SourceTitle: sourceTitle, TargetTitle: targetTitle})

		if len(batch) >= batchSize {
			if err := s.client.CreateEdgesBatch(ctx, batch); err != nil {
				return totalCreated, fmt.Errorf("failed to create edge batch: %w", err)
			}
			totalCreated += int64(len(batch))
			if totalCreated%1000000 == 0 {
				slog.Info("sync progress", "edges_synced", totalCreated)
			}
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

// IncrementalSync syncs only new data since the given timestamp
func (s *Syncer) IncrementalSync(ctx context.Context, since time.Time, batchSize int) (*SyncStats, error) {
	if batchSize == 0 {
		batchSize = 5000
	}

	stats := &SyncStats{StartTime: time.Now()}

	slog.Info("starting incremental sync", "since", since)

	nodesCreated, err := s.syncNewNodes(ctx, since, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to sync new nodes: %w", err)
	}
	stats.NodesCreated = nodesCreated

	edgesCreated, err := s.syncNewEdges(ctx, since, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to sync new edges: %w", err)
	}
	stats.EdgesCreated = edgesCreated

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	if stats.NodesCreated > 0 || stats.EdgesCreated > 0 {
		slog.Info("incremental sync complete",
			"nodes", stats.NodesCreated,
			"edges", stats.EdgesCreated,
			"duration", stats.Duration,
		)
	}

	return stats, nil
}

// syncNewNodes syncs pages created after the given timestamp
func (s *Syncer) syncNewNodes(ctx context.Context, since time.Time, batchSize int) (int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT title FROM pages WHERE fetch_status = 'success' AND created_at > ? ORDER BY id
	`, since.Format(time.RFC3339))
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
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.title AS source_title, l.target_title
		FROM links l
		JOIN pages p ON p.id = l.source_id
		WHERE p.fetch_status = 'success' AND l.created_at > ?
		ORDER BY l.id
	`, since.Format(time.RFC3339))
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
		batch = append(batch, EdgeInput{SourceTitle: sourceTitle, TargetTitle: targetTitle})

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
	slog.Info("verifying sync consistency")

	var sqliteNodes, sqliteEdges int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pages WHERE fetch_status = 'success'").Scan(&sqliteNodes); err != nil {
		return fmt.Errorf("failed to count sqlite nodes: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM links").Scan(&sqliteEdges); err != nil {
		return fmt.Errorf("failed to count sqlite edges: %w", err)
	}

	neo4jStats, err := s.client.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get neo4j stats: %w", err)
	}

	slog.Info("sqlite counts", "nodes", sqliteNodes, "edges", sqliteEdges)
	slog.Info("neo4j counts", "nodes", neo4jStats.NodeCount, "edges", neo4jStats.EdgeCount)

	if sqliteNodes != neo4jStats.NodeCount {
		return fmt.Errorf("node count mismatch: sqlite has %d, neo4j has %d", sqliteNodes, neo4jStats.NodeCount)
	}
	if sqliteEdges != neo4jStats.EdgeCount {
		return fmt.Errorf("edge count mismatch: sqlite has %d, neo4j has %d", sqliteEdges, neo4jStats.EdgeCount)
	}

	slog.Info("sync verification passed")
	return nil
}
