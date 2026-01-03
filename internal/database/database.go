// Package database provides SQLite database connection and migration management.
package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	*sql.DB
	path string
}

func Open(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating database directory: %w", err)
		}
	}

	// modernc.org/sqlite requires pragmas via SQL, not DSN parameters
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// SQLite only supports one writer at a time
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// foreign_keys must be set on every connection (not persistent)
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA cache_size = -64000",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA mmap_size = 268435456",
		"PRAGMA wal_autocheckpoint = 1000",
		"PRAGMA journal_size_limit = 67108864",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("setting pragma %q: %w", pragma, err)
		}
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{DB: db, path: path}, nil
}

func (db *DB) Path() string {
	return db.path
}

func (db *DB) Migrate() error {
	migrations := []struct {
		version int
		file    string
		name    string
	}{
		{1, "migrations/001_initial_schema.sql", "initial_schema"},
		{2, "migrations/002_optimization_indexes.sql", "optimization_indexes"},
		{3, "migrations/003_graph_optimization.sql", "graph_optimization"},
	}

	var currentVersion int
	err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&currentVersion)
	if err != nil {
		currentVersion = 0
	}

	slog.Debug("checking migrations", "current_version", currentVersion)

	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}

		content, err := migrationsFS.ReadFile(m.file)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", m.file, err)
		}

		slog.Info("applying migration", "version", m.version, "name", m.name)

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("executing migration %s: %w", m.file, err)
		}
	}

	slog.Debug("migrations complete")
	return nil
}

func (db *DB) Size() (int64, error) {
	var pageCount, pageSize int64

	if err := db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0, fmt.Errorf("getting page count: %w", err)
	}

	if err := db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, fmt.Errorf("getting page size: %w", err)
	}

	return pageCount * pageSize, nil
}

type Stats struct {
	TotalPages     int64
	FetchedPages   int64
	PendingPages   int64
	RedirectPages  int64
	ErrorPages     int64
	NotFoundPages  int64
	TotalLinks     int64
	OldestFetch    sql.NullString
	NewestFetch    sql.NullString
	DatabaseSizeBytes int64
}

func (db *DB) Stats() (*Stats, error) {
	stats := &Stats{}

	pageQuery := `
		SELECT
			COUNT(*) as total_pages,
			COUNT(CASE WHEN fetch_status = 'success' THEN 1 END) as fetched_pages,
			COUNT(CASE WHEN fetch_status = 'pending' THEN 1 END) as pending_pages,
			COUNT(CASE WHEN fetch_status = 'redirect' THEN 1 END) as redirect_pages,
			COUNT(CASE WHEN fetch_status = 'error' THEN 1 END) as error_pages,
			COUNT(CASE WHEN fetch_status = 'not_found' THEN 1 END) as not_found_pages,
			MIN(fetched_at) as oldest_fetch,
			MAX(fetched_at) as newest_fetch
		FROM pages
	`

	err := db.QueryRow(pageQuery).Scan(
		&stats.TotalPages,
		&stats.FetchedPages,
		&stats.PendingPages,
		&stats.RedirectPages,
		&stats.ErrorPages,
		&stats.NotFoundPages,
		&stats.OldestFetch,
		&stats.NewestFetch,
	)
	if err != nil {
		return nil, fmt.Errorf("querying page stats: %w", err)
	}

	err = db.QueryRow(`SELECT COUNT(*) FROM links`).Scan(&stats.TotalLinks)
	if err != nil {
		return nil, fmt.Errorf("querying link count: %w", err)
	}

	stats.DatabaseSizeBytes, err = db.Size()
	if err != nil {
		return nil, fmt.Errorf("getting database size: %w", err)
	}

	return stats, nil
}

func (db *DB) Checkpoint() error {
	_, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		return fmt.Errorf("checkpointing WAL: %w", err)
	}
	return nil
}

func (db *DB) Vacuum() error {
	_, err := db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("vacuuming database: %w", err)
	}
	return nil
}

func (db *DB) Analyze() error {
	_, err := db.Exec("ANALYZE")
	if err != nil {
		return fmt.Errorf("analyzing database: %w", err)
	}
	return nil
}

func (db *DB) Optimize() error {
	_, err := db.Exec("PRAGMA optimize")
	if err != nil {
		return fmt.Errorf("optimizing database: %w", err)
	}
	return nil
}
