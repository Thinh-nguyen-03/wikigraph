-- migrations/001_initial_schema.sql
-- WikiGraph Database Schema v1.0
--
-- Design principles:
-- - ISO8601 TEXT timestamps for portability and sorting
-- - CHECK constraints for data validation at the database level
-- - No AUTOINCREMENT (unnecessary overhead in SQLite)
-- - Explicit state tracking for fetch operations
-- - Foreign key constraints with CASCADE delete
--
-- IMPORTANT: Foreign keys are OFF by default in SQLite!
-- The application MUST run: PRAGMA foreign_keys = ON;

-- ============================================================================
-- Schema Migrations Table
-- ============================================================================
-- Tracks which migrations have been applied to this database.
-- This enables safe, incremental schema changes and rollbacks.

CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    applied_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- ============================================================================
-- Pages Table
-- ============================================================================
-- Stores metadata about fetched Wikipedia pages.
--
-- State machine for fetch_status:
--   pending   -> Initial state, page has not been fetched yet
--   success   -> Page was fetched successfully, links extracted
--   redirect  -> Page redirects to another page (redirect_to contains target)
--   not_found -> Page does not exist (HTTP 404)
--   error     -> Fetch failed due to network error, rate limit, etc.

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

    -- Business rule: redirect_to must be set when status is 'redirect'
    CHECK(
        (fetch_status != 'redirect') OR
        (fetch_status = 'redirect' AND redirect_to IS NOT NULL)
    ),

    -- Business rule: fetched_at should be set for success/redirect/not_found
    CHECK(
        (fetch_status IN ('pending', 'error')) OR
        (fetch_status IN ('success', 'redirect', 'not_found') AND fetched_at IS NOT NULL)
    )
);

-- ============================================================================
-- Links Table
-- ============================================================================
-- Stores directed edges from source pages to target pages.
--
-- Design decision: target_title is TEXT, not a foreign key to pages.id
-- This allows storing links to pages we haven't crawled yet without
-- creating placeholder records.

CREATE TABLE IF NOT EXISTS links (
    id            INTEGER PRIMARY KEY,
    source_id     INTEGER NOT NULL,
    target_title  TEXT NOT NULL,
    anchor_text   TEXT,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    -- Length constraints to prevent unbounded storage
    CHECK(length(target_title) <= 512),
    CHECK(anchor_text IS NULL OR length(anchor_text) <= 1024),

    -- Foreign key with cascade delete
    FOREIGN KEY (source_id) REFERENCES pages(id) ON DELETE CASCADE,

    -- Prevent duplicate links from same source to same target
    UNIQUE(source_id, target_title)
);

-- ============================================================================
-- Indexes
-- ============================================================================
-- Note: UNIQUE constraint on pages.title already creates an implicit index,
-- so we don't need to create idx_pages_title explicitly.

-- Find stale cache entries for refresh (only for successfully fetched pages)
-- This is a partial index - only indexes rows where fetch_status = 'success'
CREATE INDEX IF NOT EXISTS idx_pages_fetched_at
    ON pages(fetched_at)
    WHERE fetch_status = 'success';

-- Find pages by status (for crawl queue management)
-- Used by: "SELECT * FROM pages WHERE fetch_status = 'pending' LIMIT 100"
CREATE INDEX IF NOT EXISTS idx_pages_fetch_status
    ON pages(fetch_status);

-- Get all outgoing links from a page (most common query in pathfinding)
-- Used by: "SELECT target_title FROM links WHERE source_id = ?"
CREATE INDEX IF NOT EXISTS idx_links_source_id
    ON links(source_id);

-- Find all incoming links to a page (backlinks, reverse lookup)
-- Used by: "SELECT source_id FROM links WHERE target_title = ?"
CREATE INDEX IF NOT EXISTS idx_links_target_title
    ON links(target_title);

-- ============================================================================
-- Record this migration
-- ============================================================================
INSERT INTO schema_migrations (version, name) VALUES (1, 'initial_schema');
