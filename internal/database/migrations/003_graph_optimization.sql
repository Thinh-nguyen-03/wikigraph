-- Covering index for graph loading optimization.
-- Includes both source_id and target_title to eliminate table lookups during bulk loads.
CREATE INDEX IF NOT EXISTS idx_links_source_target_covering
    ON links(source_id, target_title);

-- ANALYZE moved to separate command (wikigraph analyze) to avoid startup delays.
-- Run manually after large data imports.

INSERT INTO schema_migrations (version, name) VALUES (3, 'graph_optimization');
