package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wikigraph-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Open the database
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	if db.Path() != dbPath {
		t.Errorf("Path() = %q, want %q", db.Path(), dbPath)
	}

	var one int
	if err := db.QueryRow("SELECT 1").Scan(&one); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if one != 1 {
		t.Errorf("SELECT 1 = %d, want 1", one)
	}

	var fkEnabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatalf("checking foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Error("foreign keys not enabled")
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("checking journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want 'wal'", journalMode)
	}
}

func TestMigrate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wikigraph-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	var version int
	var name string
	err = db.QueryRow("SELECT version, name FROM schema_migrations WHERE version = 1").Scan(&version, &name)
	if err != nil {
		t.Fatalf("querying schema_migrations: %v", err)
	}
	if version != 1 || name != "initial_schema" {
		t.Errorf("migration = (%d, %q), want (1, 'initial_schema')", version, name)
	}

	_, err = db.Exec(`
		INSERT INTO pages (title, fetch_status, fetched_at)
		VALUES ('Test Page', 'success', '2024-01-01T00:00:00Z')
	`)
	if err != nil {
		t.Fatalf("inserting into pages: %v", err)
	}

	var pageID int64
	if err := db.QueryRow("SELECT id FROM pages WHERE title = 'Test Page'").Scan(&pageID); err != nil {
		t.Fatalf("getting page id: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO links (source_id, target_title, anchor_text)
		VALUES (?, 'Target Page', 'click here')
	`, pageID)
	if err != nil {
		t.Fatalf("inserting into links: %v", err)
	}

	_, err = db.Exec("DELETE FROM pages WHERE id = ?", pageID)
	if err != nil {
		t.Fatalf("deleting page: %v", err)
	}

	var linkCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM links").Scan(&linkCount); err != nil {
		t.Fatalf("counting links: %v", err)
	}
	if linkCount != 0 {
		t.Errorf("links not cascade deleted: count = %d, want 0", linkCount)
	}

	// Migrations should be idempotent
	if err := db.Migrate(); err != nil {
		t.Fatalf("re-running migrations: %v", err)
	}
}

func TestMigrate_CheckConstraints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wikigraph-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO pages (title, fetch_status)
		VALUES ('Test', 'invalid_status')
	`)
	if err == nil {
		t.Error("expected error for invalid fetch_status, got nil")
	}

	_, err = db.Exec(`
		INSERT INTO pages (title, fetch_status, fetched_at)
		VALUES ('Test Redirect', 'redirect', '2024-01-01T00:00:00Z')
	`)
	if err == nil {
		t.Error("expected error for redirect without redirect_to, got nil")
	}

	_, err = db.Exec(`
		INSERT INTO pages (title, fetch_status)
		VALUES ('Test Success', 'success')
	`)
	if err == nil {
		t.Error("expected error for success without fetched_at, got nil")
	}

	_, err = db.Exec(`
		INSERT INTO pages (title, fetch_status, redirect_to, fetched_at)
		VALUES ('Einstein', 'redirect', 'Albert Einstein', '2024-01-01T00:00:00Z')
	`)
	if err != nil {
		t.Errorf("valid redirect failed: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO pages (title, fetch_status)
		VALUES ('Pending Page', 'pending')
	`)
	if err != nil {
		t.Errorf("pending page failed: %v", err)
	}
}

func TestStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wikigraph-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO pages (title, fetch_status, fetched_at, redirect_to) VALUES
			('Page 1', 'success', '2024-01-01T00:00:00Z', NULL),
			('Page 2', 'success', '2024-01-02T00:00:00Z', NULL),
			('Page 3', 'pending', NULL, NULL),
			('Page 4', 'redirect', '2024-01-01T00:00:00Z', 'Page 1'),
			('Page 5', 'error', NULL, NULL)
	`)
	if err != nil {
		t.Fatalf("inserting test pages: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO links (source_id, target_title) VALUES
			(1, 'Page 2'),
			(1, 'Page 3'),
			(2, 'Page 1')
	`)
	if err != nil {
		t.Fatalf("inserting test links: %v", err)
	}

	stats, err := db.Stats()
	if err != nil {
		t.Fatalf("getting stats: %v", err)
	}

	if stats.TotalPages != 5 {
		t.Errorf("TotalPages = %d, want 5", stats.TotalPages)
	}
	if stats.FetchedPages != 2 {
		t.Errorf("FetchedPages = %d, want 2", stats.FetchedPages)
	}
	if stats.PendingPages != 1 {
		t.Errorf("PendingPages = %d, want 1", stats.PendingPages)
	}
	if stats.RedirectPages != 1 {
		t.Errorf("RedirectPages = %d, want 1", stats.RedirectPages)
	}
	if stats.ErrorPages != 1 {
		t.Errorf("ErrorPages = %d, want 1", stats.ErrorPages)
	}
	if stats.TotalLinks != 3 {
		t.Errorf("TotalLinks = %d, want 3", stats.TotalLinks)
	}
	if stats.DatabaseSizeBytes <= 0 {
		t.Errorf("DatabaseSizeBytes = %d, want > 0", stats.DatabaseSizeBytes)
	}
}
