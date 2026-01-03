-- Covering index for graph loading optimization
--
-- Query: SELECT p.title, l.target_title
--        FROM links l
--        JOIN pages p ON p.id = l.source_id
--        WHERE p.fetch_status = 'success'
--
-- The original idx_links_source_id only indexed source_id, so SQLite had to:
-- 1. Scan idx_links_source_id to find matching rows
-- 2. Look up each row in the links table to get target_title
--
-- This covering index includes both source_id AND target_title, so SQLite can:
-- 1. Scan the index to get BOTH values directly (no table lookup needed)
--
-- Result: Eliminates millions of table lookups when loading large graphs (8M+ edges)
CREATE INDEX IF NOT EXISTS idx_links_source_target_covering
    ON links(source_id, target_title);

-- Update query planner statistics so it knows about the new index
ANALYZE;

INSERT INTO schema_migrations (version, name) VALUES (3, 'graph_optimization');
