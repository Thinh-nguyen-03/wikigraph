-- Restore the covering index dropped by migration 4.
-- Migration 4 recreated the links table but omitted idx_links_source_target_covering
-- which is required by the GetGraphData query (INDEXED BY hint in cache.go).

CREATE INDEX IF NOT EXISTS idx_links_source_target_covering
    ON links(source_id, target_title);

INSERT INTO schema_migrations (version, name) VALUES (5, 'restore_covering_index');
