-- migrations/002_optimization_indexes.sql
-- Optimization indexes for improved query performance (CQ-003)
--
-- These indexes are designed for the most common query patterns:
-- 1. GetPendingPages: ordered pagination of pending pages
-- 2. GetGraphData: bulk loading of successful pages for graph construction

-- ============================================================================
-- Composite index for pending pages with ordering
-- ============================================================================
-- Optimizes: SELECT ... FROM pages WHERE fetch_status = 'pending' ORDER BY created_at
-- The composite index allows SQLite to use a single index scan for both filter and sort
CREATE INDEX IF NOT EXISTS idx_pages_pending_ordered
    ON pages(fetch_status, created_at)
    WHERE fetch_status = 'pending';

-- ============================================================================
-- Partial index for successful pages
-- ============================================================================
-- Optimizes: SELECT ... FROM pages WHERE fetch_status = 'success'
-- This is more efficient than the general fetch_status index for graph loading
-- since it only indexes the rows we care about
CREATE INDEX IF NOT EXISTS idx_pages_success
    ON pages(fetch_status)
    WHERE fetch_status = 'success';

-- ============================================================================
-- Record this migration
-- ============================================================================
INSERT INTO schema_migrations (version, name) VALUES (2, 'optimization_indexes');
