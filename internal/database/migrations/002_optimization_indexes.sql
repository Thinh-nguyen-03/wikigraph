-- Performance optimization indexes
--
-- These indexes target the most common query patterns identified during profiling:
-- 1. GetPendingPages: Fetch next batch of pages to crawl (with pagination)
-- 2. GetGraphData: Bulk load all successful pages for graph construction

-- Composite index for pending page queries with ordering
-- Query: SELECT ... FROM pages WHERE fetch_status = 'pending' ORDER BY created_at LIMIT N
-- Without this, SQLite would need separate index lookups for filter + sort
-- With this, it can do a single index scan (fetch_status, created_at)
CREATE INDEX IF NOT EXISTS idx_pages_pending_ordered
    ON pages(fetch_status, created_at)
    WHERE fetch_status = 'pending';

-- Partial index for successful pages only
-- Query: SELECT ... FROM pages WHERE fetch_status = 'success'
-- More efficient than general idx_pages_fetch_status because it only
-- indexes successful pages, making it smaller and faster for graph loading
CREATE INDEX IF NOT EXISTS idx_pages_success
    ON pages(fetch_status)
    WHERE fetch_status = 'success';

INSERT INTO schema_migrations (version, name) VALUES (2, 'optimization_indexes');
