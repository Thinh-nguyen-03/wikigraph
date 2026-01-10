-- Remove anchor_text column to reduce database size
-- anchor_text is not used in pathfinding or any queries
-- This will reduce database size by ~50%

-- SQLite doesn't support DROP COLUMN directly
-- We need to recreate the table

CREATE TABLE IF NOT EXISTS links_new (
    id            INTEGER PRIMARY KEY,
    source_id     INTEGER NOT NULL,
    target_title  TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    CHECK(length(target_title) <= 512),
    FOREIGN KEY (source_id) REFERENCES pages(id) ON DELETE CASCADE,
    UNIQUE(source_id, target_title)
);

-- Copy data (excluding anchor_text)
INSERT INTO links_new (id, source_id, target_title, created_at)
SELECT id, source_id, target_title, created_at FROM links;

-- Drop old table
DROP TABLE links;

-- Rename new table
ALTER TABLE links_new RENAME TO links;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_links_source_id
    ON links(source_id);

CREATE INDEX IF NOT EXISTS idx_links_target_title
    ON links(target_title);

INSERT INTO schema_migrations (version, name) VALUES (4, 'remove_anchor_text');
