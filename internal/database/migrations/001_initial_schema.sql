-- WikiGraph Database Schema
--
-- Design principles:
-- - ISO8601 TEXT timestamps for SQLite compatibility and sortability
-- - CHECK constraints enforce data integrity at database level
-- - No AUTOINCREMENT (unnecessary overhead in SQLite)
-- - CASCADE deletes maintain referential integrity
--
-- IMPORTANT: Foreign keys are OFF by default in SQLite!
-- Application must run: PRAGMA foreign_keys = ON

-- Tracks applied migrations for safe incremental schema changes
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    applied_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Pages: Metadata for crawled Wikipedia pages
--
-- fetch_status state machine:
--   pending   -> Initial state when page is discovered via link
--   success   -> Page fetched successfully, links extracted
--   redirect  -> Page redirects to another (redirect_to contains target)
--   not_found -> HTTP 404, page doesn't exist
--   error     -> Network error, rate limit, or other failure
--
-- Constraints ensure redirect_to is set when status is redirect,
-- and fetched_at is set for completed fetches

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

    CHECK((fetch_status != 'redirect') OR (redirect_to IS NOT NULL)),
    CHECK((fetch_status IN ('pending', 'error')) OR (fetched_at IS NOT NULL))
);

-- Links: Directed edges representing hyperlinks between Wikipedia pages
--
-- Design decision: target_title is TEXT, not a foreign key to pages.id
-- This allows storing links to pages we haven't crawled yet without
-- creating placeholder records. The graph loader resolves these dynamically.
--
-- UNIQUE constraint on (source_id, target_title) prevents duplicate edges
CREATE TABLE IF NOT EXISTS links (
    id            INTEGER PRIMARY KEY,
    source_id     INTEGER NOT NULL,
    target_title  TEXT NOT NULL,
    anchor_text   TEXT,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    CHECK(length(target_title) <= 512),
    CHECK(anchor_text IS NULL OR length(anchor_text) <= 1024),
    FOREIGN KEY (source_id) REFERENCES pages(id) ON DELETE CASCADE,
    UNIQUE(source_id, target_title)
);

-- Indexes for common query patterns
-- Note: pages.title already has implicit index from UNIQUE constraint

-- Partial index for cache refresh queries (only successful pages can be stale)
CREATE INDEX IF NOT EXISTS idx_pages_fetched_at
    ON pages(fetched_at)
    WHERE fetch_status = 'success';

-- General status lookup for crawl queue management
CREATE INDEX IF NOT EXISTS idx_pages_fetch_status
    ON pages(fetch_status);

-- Forward graph traversal: get all outgoing links from a page
CREATE INDEX IF NOT EXISTS idx_links_source_id
    ON links(source_id);

-- Backward graph traversal: find all pages linking to this page
CREATE INDEX IF NOT EXISTS idx_links_target_title
    ON links(target_title);

INSERT INTO schema_migrations (version, name) VALUES (1, 'initial_schema');
