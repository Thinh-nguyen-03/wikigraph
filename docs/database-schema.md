# Database Schema Documentation

## Overview

WikiGraph uses SQLite for persistent storage of fetched Wikipedia pages and their link relationships. This document describes the schema design, rationale, and usage patterns.

**Design Principles:**
- ISO8601 timestamps for portability and clarity
- Explicit state tracking for fetch operations
- Schema versioning for safe migrations
- Optimized indexes for common query patterns
- CHECK constraints for data validation

---

## Entity Relationship Diagram

```
┌─────────────────────────────────────────┐
│           schema_migrations             │
├─────────────────────────────────────────┤
│ version      INTEGER PK                 │
│ name         TEXT NOT NULL              │
│ applied_at   TEXT NOT NULL              │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│                pages                    │
├─────────────────────────────────────────┤
│ id            INTEGER PK                │
│ title         TEXT UNIQUE NOT NULL      │
│ content_hash  TEXT                      │
│ fetch_status  TEXT NOT NULL             │
│ redirect_to   TEXT                      │
│ fetched_at    TEXT                      │
│ created_at    TEXT NOT NULL             │
│ updated_at    TEXT NOT NULL             │
└─────────────────────────────────────────┘
                    │
                    │ 1:N
                    ▼
┌─────────────────────────────────────────┐
│                links                    │
├─────────────────────────────────────────┤
│ id            INTEGER PK                │
│ source_id     INTEGER FK → pages.id     │
│ target_title  TEXT NOT NULL             │
│ anchor_text   TEXT                      │
│ created_at    TEXT NOT NULL             │
└─────────────────────────────────────────┘
```

---

## Tables

### schema_migrations

Tracks which migrations have been applied. Enables safe, incremental schema changes.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `version` | INTEGER | PRIMARY KEY | Migration version number |
| `name` | TEXT | NOT NULL | Human-readable migration name |
| `applied_at` | TEXT | NOT NULL | ISO8601 timestamp when migration was applied |

**Why this matters:** Without migration tracking, you can't safely evolve the schema or know what state a database is in.

---

### pages

Stores metadata about fetched Wikipedia pages.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PRIMARY KEY | Auto-incrementing unique identifier |
| `title` | TEXT | UNIQUE NOT NULL | Canonical Wikipedia page title |
| `content_hash` | TEXT | | SHA-256 hash of page content (change detection) |
| `fetch_status` | TEXT | NOT NULL, CHECK constraint | State: `pending`, `success`, `redirect`, `not_found`, `error` |
| `redirect_to` | TEXT | | If status is `redirect`, the canonical title it points to |
| `fetched_at` | TEXT | | ISO8601 timestamp of last successful fetch (NULL if never fetched) |
| `created_at` | TEXT | NOT NULL | ISO8601 timestamp when record was created |
| `updated_at` | TEXT | NOT NULL | ISO8601 timestamp when record was last modified |

**Design Notes:**

1. **No AUTOINCREMENT**: SQLite's `INTEGER PRIMARY KEY` already auto-increments. AUTOINCREMENT adds overhead to prevent rowid reuse, which we don't need.

2. **TEXT for timestamps**: SQLite has no native datetime type. Using TEXT with ISO8601 format (`YYYY-MM-DDTHH:MM:SSZ`) is:
   - Portable across systems
   - Human-readable in queries
   - Sortable as strings
   - Standard practice at Google, Stripe, and other companies

3. **fetch_status state machine**:
   ```
   pending → success    (fetched successfully)
   pending → redirect   (page redirects to another)
   pending → not_found  (404 from Wikipedia)
   pending → error      (network error, rate limited, etc.)
   ```

4. **redirect_to**: When "Einstein" redirects to "Albert Einstein", we store `redirect_to = "Albert Einstein"`. This enables:
   - Following redirect chains
   - Avoiding re-fetching known redirects
   - Analytics on redirect patterns

5. **updated_at**: Essential for:
   - Cache invalidation logic
   - Debugging data issues
   - Audit trails

---

### links

Stores directed edges from source pages to target pages.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PRIMARY KEY | Auto-incrementing unique identifier |
| `source_id` | INTEGER | NOT NULL, FK → pages.id | Source page (the page containing the link) |
| `target_title` | TEXT | NOT NULL, max 512 chars | Target page title (may not exist in pages table) |
| `anchor_text` | TEXT | max 1024 chars | The clickable text of the link |
| `created_at` | TEXT | NOT NULL | ISO8601 timestamp when record was created |

**Constraints:**
- `UNIQUE(source_id, target_title)` prevents duplicate links
- `ON DELETE CASCADE` removes links when source page is deleted
- `CHECK(length(target_title) <= 512)` prevents unbounded titles
- `CHECK(length(anchor_text) <= 1024)` prevents unbounded anchor text

**Design Decision - Why `target_title` instead of `target_id`?**

We intentionally store the target as a title string rather than a foreign key because:

1. **Unfetched pages**: A page may link to pages we haven't crawled yet
2. **No placeholders**: Avoids creating empty page records just to satisfy FK
3. **Simpler crawling**: Phase 2 crawler can easily find "pages linked but not fetched"
4. **Query flexibility**: Can analyze link patterns without fetching every page

Trade-off: ~50 bytes per link vs ~8 bytes for integer FK. At 1M links, this is ~42MB extra - acceptable for the benefits.

---

## Indexes

```sql
-- NOTE: idx_pages_title is NOT needed - UNIQUE constraint creates implicit index

-- Find stale cache entries for refresh
CREATE INDEX idx_pages_fetched_at ON pages(fetched_at)
    WHERE fetch_status = 'success';

-- Find pages by status (e.g., all pending pages for crawl queue)
CREATE INDEX idx_pages_fetch_status ON pages(fetch_status);

-- Get all links from a page (most common query in pathfinding)
CREATE INDEX idx_links_source_id ON links(source_id);

-- Find all pages linking to a target (backlinks, reverse lookup)
CREATE INDEX idx_links_target_title ON links(target_title);

-- Covering index for "find unfetched targets" query (Phase 2 crawler)
-- This is a partial index that only includes links to pages not yet in the pages table
CREATE INDEX idx_links_target_for_crawl ON links(target_title)
    WHERE target_title NOT IN (SELECT title FROM pages);
```

**Index Design Notes:**

1. **No index on `pages.title`**: The `UNIQUE` constraint automatically creates an index. Adding another is wasteful.

2. **Partial index on `fetched_at`**: Only indexes successful fetches, which are the only ones we check for staleness.

3. **Status index**: Essential for queries like "get all pending pages" during crawling.

4. **Covering index consideration**: For high-volume deployments, consider:
   ```sql
   CREATE INDEX idx_links_source_covering ON links(source_id, target_title, anchor_text);
   ```
   This allows queries to be satisfied entirely from the index without table lookups.

---

## Migrations

### Location

```
migrations/
├── 001_initial_schema.sql
└── ...
```

### Migration 001: Initial Schema

```sql
-- migrations/001_initial_schema.sql
-- WikiGraph Database Schema v1.0
--
-- Design principles:
-- - ISO8601 TEXT timestamps for portability
-- - CHECK constraints for data validation
-- - No AUTOINCREMENT (unnecessary overhead)
-- - Explicit state tracking for fetch operations

-- ============================================================================
-- Schema Migrations Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    applied_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- ============================================================================
-- Pages Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS pages (
    id            INTEGER PRIMARY KEY,
    title         TEXT UNIQUE NOT NULL,
    content_hash  TEXT,
    fetch_status  TEXT NOT NULL DEFAULT 'pending'
                  CHECK(fetch_status IN ('pending', 'success', 'redirect', 'not_found', 'error')),
    redirect_to   TEXT,
    fetched_at    TEXT,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    -- Ensure redirect_to is set when status is 'redirect'
    CHECK(
        (fetch_status != 'redirect') OR
        (fetch_status = 'redirect' AND redirect_to IS NOT NULL)
    )
);

-- ============================================================================
-- Links Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS links (
    id            INTEGER PRIMARY KEY,
    source_id     INTEGER NOT NULL,
    target_title  TEXT NOT NULL CHECK(length(target_title) <= 512),
    anchor_text   TEXT CHECK(anchor_text IS NULL OR length(anchor_text) <= 1024),
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    FOREIGN KEY (source_id) REFERENCES pages(id) ON DELETE CASCADE,
    UNIQUE(source_id, target_title)
);

-- ============================================================================
-- Indexes
-- ============================================================================
-- Note: UNIQUE on pages.title already creates an implicit index

-- Find stale cache entries (only for successfully fetched pages)
CREATE INDEX IF NOT EXISTS idx_pages_fetched_at
    ON pages(fetched_at)
    WHERE fetch_status = 'success';

-- Find pages by status (for crawl queue management)
CREATE INDEX IF NOT EXISTS idx_pages_fetch_status
    ON pages(fetch_status);

-- Get all outgoing links from a page
CREATE INDEX IF NOT EXISTS idx_links_source_id
    ON links(source_id);

-- Find all incoming links to a page (backlinks)
CREATE INDEX IF NOT EXISTS idx_links_target_title
    ON links(target_title);

-- ============================================================================
-- Record this migration
-- ============================================================================
INSERT INTO schema_migrations (version, name) VALUES (1, 'initial_schema');
```

### Running Migrations

```go
// internal/database/migrate.go

package database

import (
    "database/sql"
    "embed"
    "fmt"
    "log/slog"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate runs all pending database migrations
func Migrate(db *sql.DB) error {
    // Get current version
    var currentVersion int
    err := db.QueryRow(`
        SELECT COALESCE(MAX(version), 0)
        FROM schema_migrations
    `).Scan(&currentVersion)

    // Table might not exist yet
    if err != nil {
        currentVersion = 0
    }

    migrations := []struct {
        version int
        file    string
    }{
        {1, "001_initial_schema.sql"},
        // Add future migrations here:
        // {2, "002_add_embeddings.sql"},
    }

    for _, m := range migrations {
        if m.version <= currentVersion {
            continue // Already applied
        }

        content, err := migrationFS.ReadFile("migrations/" + m.file)
        if err != nil {
            return fmt.Errorf("reading migration %s: %w", m.file, err)
        }

        slog.Info("applying migration", "version", m.version, "file", m.file)

        if _, err := db.Exec(string(content)); err != nil {
            return fmt.Errorf("executing migration %s: %w", m.file, err)
        }
    }

    return nil
}
```

---

## Common Queries

### Insert/Update a Page (Upsert)

```sql
-- Insert new page or update existing on successful fetch
INSERT INTO pages (title, content_hash, fetch_status, fetched_at, updated_at)
VALUES (
    ?,
    ?,
    'success',
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
)
ON CONFLICT(title) DO UPDATE SET
    content_hash = excluded.content_hash,
    fetch_status = excluded.fetch_status,
    fetched_at = excluded.fetched_at,
    updated_at = excluded.updated_at;
```

### Insert a Redirect

```sql
INSERT INTO pages (title, fetch_status, redirect_to, updated_at)
VALUES (?, 'redirect', ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
ON CONFLICT(title) DO UPDATE SET
    fetch_status = 'redirect',
    redirect_to = excluded.redirect_to,
    updated_at = excluded.updated_at;
```

### Insert Links (Batch)

```sql
INSERT INTO links (source_id, target_title, anchor_text)
VALUES (?, ?, ?)
ON CONFLICT(source_id, target_title) DO NOTHING;
```

### Get Page with Links

```sql
SELECT p.id, p.title, p.content_hash, p.fetch_status,
       p.redirect_to, p.fetched_at,
       l.target_title, l.anchor_text
FROM pages p
LEFT JOIN links l ON l.source_id = p.id
WHERE p.title = ?;
```

### Find Stale Pages (for cache refresh)

```sql
SELECT id, title, fetched_at
FROM pages
WHERE fetch_status = 'success'
  AND fetched_at < datetime('now', '-7 days')
ORDER BY fetched_at ASC
LIMIT ?;
```

### Get Outgoing Links

```sql
SELECT target_title, anchor_text
FROM links
WHERE source_id = ?
ORDER BY target_title;
```

### Get Incoming Links (Backlinks)

```sql
SELECT p.title as source_title, l.anchor_text
FROM links l
JOIN pages p ON p.id = l.source_id
WHERE l.target_title = ?;
```

### Find Unfetched Targets (Crawl Queue)

```sql
-- Pages that are linked to but not yet fetched
SELECT DISTINCT l.target_title
FROM links l
LEFT JOIN pages p ON p.title = l.target_title
WHERE p.id IS NULL
LIMIT ?;
```

### Get Pages by Status

```sql
-- Get all pages pending fetch
SELECT id, title, created_at
FROM pages
WHERE fetch_status = 'pending'
ORDER BY created_at ASC
LIMIT ?;

-- Get all failed pages for retry
SELECT id, title, updated_at
FROM pages
WHERE fetch_status = 'error'
ORDER BY updated_at ASC
LIMIT ?;
```

### Follow Redirect Chain

```sql
-- Resolve a redirect to its final target
WITH RECURSIVE redirect_chain(title, redirect_to, depth) AS (
    SELECT title, redirect_to, 0
    FROM pages
    WHERE title = ?

    UNION ALL

    SELECT p.title, p.redirect_to, rc.depth + 1
    FROM pages p
    JOIN redirect_chain rc ON p.title = rc.redirect_to
    WHERE p.fetch_status = 'redirect' AND rc.depth < 10
)
SELECT title, redirect_to
FROM redirect_chain
WHERE redirect_to IS NULL OR redirect_to NOT IN (SELECT title FROM pages WHERE fetch_status = 'redirect');
```

### Cache Statistics

```sql
SELECT
    (SELECT COUNT(*) FROM pages) as total_pages,
    (SELECT COUNT(*) FROM pages WHERE fetch_status = 'success') as fetched_pages,
    (SELECT COUNT(*) FROM pages WHERE fetch_status = 'pending') as pending_pages,
    (SELECT COUNT(*) FROM pages WHERE fetch_status = 'redirect') as redirect_pages,
    (SELECT COUNT(*) FROM pages WHERE fetch_status = 'error') as error_pages,
    (SELECT COUNT(*) FROM links) as total_links,
    (SELECT MIN(fetched_at) FROM pages WHERE fetched_at IS NOT NULL) as oldest_fetch,
    (SELECT MAX(fetched_at) FROM pages WHERE fetched_at IS NOT NULL) as newest_fetch;
```

### Database Size

```go
// Get database file size
func (db *Database) Size() (int64, error) {
    var pageCount, pageSize int64
    
    row := db.QueryRow("PRAGMA page_count")
    if err := row.Scan(&pageCount); err != nil {
        return 0, err
    }
    
    row = db.QueryRow("PRAGMA page_size")
    if err := row.Scan(&pageSize); err != nil {
        return 0, err
    }
    
    return pageCount * pageSize, nil
}
```

---

## Performance Tuning

### SQLite Pragmas (CRITICAL)

These pragmas must be set on **every connection** to the database:

```sql
-- Enable foreign keys (OFF by default in SQLite!)
PRAGMA foreign_keys = ON;

-- Enable WAL mode for better concurrency (persistent, only needs to be set once)
PRAGMA journal_mode = WAL;

-- Synchronous mode (NORMAL is safe with WAL, faster than FULL)
PRAGMA synchronous = NORMAL;

-- Increase cache size (negative = KB, positive = pages)
-- 64MB cache for better read performance
PRAGMA cache_size = -64000;

-- Store temp tables in memory
PRAGMA temp_store = MEMORY;

-- Enable memory-mapped I/O for reads (256MB)
PRAGMA mmap_size = 268435456;

-- Busy timeout (wait up to 5 seconds if database is locked)
PRAGMA busy_timeout = 5000;
```

### Go Database Initialization

```go
// internal/database/database.go

package database

import (
    "database/sql"
    "fmt"

    _ "modernc.org/sqlite"
)

// Open creates a new database connection with optimal settings
func Open(path string) (*sql.DB, error) {
    // Connection string with URI parameters
    dsn := fmt.Sprintf("file:%s?_fk=1&_journal=WAL&_sync=NORMAL&_timeout=5000", path)

    db, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("opening database: %w", err)
    }

    // Set connection pool settings
    db.SetMaxOpenConns(1) // SQLite only supports one writer at a time
    db.SetMaxIdleConns(1)

    // Apply pragmas that can't be set via DSN
    pragmas := []string{
        "PRAGMA cache_size = -64000",
        "PRAGMA temp_store = MEMORY",
        "PRAGMA mmap_size = 268435456",
    }

    for _, pragma := range pragmas {
        if _, err := db.Exec(pragma); err != nil {
            db.Close()
            return nil, fmt.Errorf("setting pragma: %w", err)
        }
    }

    return db, nil
}
```

### Batch Insert Pattern

```go
func (db *Database) InsertLinks(pageID int64, links []Link) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    stmt, err := tx.Prepare(`
        INSERT INTO links (source_id, target_title, anchor_text)
        VALUES (?, ?, ?)
        ON CONFLICT(source_id, target_title) DO NOTHING
    `)
    if err != nil {
        return err
    }
    defer stmt.Close()
    
    for _, link := range links {
        _, err := stmt.Exec(pageID, link.TargetTitle, link.AnchorText)
        if err != nil {
            return err
        }
    }
    
    return tx.Commit()
}
```

### Query with Prepared Statements

```go
type Database struct {
    db           *sql.DB
    stmtGetPage  *sql.Stmt
    stmtGetLinks *sql.Stmt
}

func (d *Database) Prepare() error {
    var err error
    
    d.stmtGetPage, err = d.db.Prepare(`
        SELECT id, title, content_hash, fetched_at
        FROM pages WHERE title = ?
    `)
    if err != nil {
        return err
    }
    
    d.stmtGetLinks, err = d.db.Prepare(`
        SELECT target_title, anchor_text
        FROM links WHERE source_id = ?
    `)
    if err != nil {
        return err
    }
    
    return nil
}
```

---

## Data Integrity

### Constraints

1. **Unique titles**: `pages.title` is unique
2. **Unique links**: `(source_id, target_title)` is unique
3. **Cascade delete**: Deleting a page removes its links
4. **Not null**: Required fields enforced at schema level

### Validation (Application Level)

```go
func ValidateTitle(title string) error {
    if title == "" {
        return errors.New("title cannot be empty")
    }
    if len(title) > 256 {
        return errors.New("title too long")
    }
    if strings.ContainsAny(title, "<>{}[]|") {
        return errors.New("title contains invalid characters")
    }
    return nil
}
```

---

## Backup and Recovery

### Backup

```bash
# Simple file copy (with WAL checkpoint first)
sqlite3 wikigraph.db "PRAGMA wal_checkpoint(TRUNCATE);"
cp wikigraph.db wikigraph.db.backup

# Or use .backup command
sqlite3 wikigraph.db ".backup 'wikigraph.db.backup'"
```

### Recovery

```bash
# Restore from backup
cp wikigraph.db.backup wikigraph.db

# Integrity check
sqlite3 wikigraph.db "PRAGMA integrity_check;"
```

---

## Schema Evolution

### Adding Columns

```sql
-- migrations/002_add_page_size.sql
ALTER TABLE pages ADD COLUMN content_size INTEGER;

-- Record this migration
INSERT INTO schema_migrations (version, name) VALUES (2, 'add_page_size');
```

### Adding Tables (Phase 3 - Embeddings)

```sql
-- migrations/003_add_embeddings.sql
CREATE TABLE IF NOT EXISTS embeddings (
    id          INTEGER PRIMARY KEY,
    page_id     INTEGER UNIQUE NOT NULL,
    vector      BLOB NOT NULL,
    model       TEXT NOT NULL,
    dimensions  INTEGER NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    FOREIGN KEY (page_id) REFERENCES pages(id) ON DELETE CASCADE
);

-- Index for joining with pages
CREATE INDEX IF NOT EXISTS idx_embeddings_page_id ON embeddings(page_id);

-- Record this migration
INSERT INTO schema_migrations (version, name) VALUES (3, 'add_embeddings');
```

### Migration Best Practices

1. **Always use transactions**: Wrap migrations in BEGIN/COMMIT
2. **Backwards compatible**: Add columns as nullable, add defaults
3. **Test on copy first**: Never run untested migrations on production data
4. **Include rollback**: Document how to undo each migration

---

## Troubleshooting

### Common Issues

**"database is locked"**
- Enable WAL mode
- Reduce transaction duration
- Use connection pooling

**Slow queries**
- Check indexes with `EXPLAIN QUERY PLAN`
- Run `ANALYZE` after bulk inserts
- Consider vacuuming: `VACUUM`

**Large database size**
- Check for duplicate data
- Consider compressing anchor text
- Run `VACUUM` to reclaim space

### Debug Queries

```sql
-- Check table sizes
SELECT name, SUM(pgsize) as size
FROM dbstat
GROUP BY name
ORDER BY size DESC;

-- Analyze query plan
EXPLAIN QUERY PLAN
SELECT * FROM links WHERE target_title = 'Physics';

-- Index usage statistics
SELECT * FROM sqlite_stat1;
```
