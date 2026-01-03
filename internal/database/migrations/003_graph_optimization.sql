-- migrations/003_graph_optimization.sql
-- Covering index for graph data queries

-- ============================================================================
-- Covering index for GetGraphData edge query
-- ============================================================================
-- Optimizes: SELECT p.title, l.target_title FROM links l JOIN pages p ...
-- Eliminates table lookups during JOIN by including target_title in the index
CREATE INDEX IF NOT EXISTS idx_links_source_target_covering
    ON links(source_id, target_title);

-- Run ANALYZE to update query planner statistics
ANALYZE;

-- ============================================================================
-- Record this migration
-- ============================================================================
INSERT INTO schema_migrations (version, name) VALUES (3, 'graph_optimization');
