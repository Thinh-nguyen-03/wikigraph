-- WikiGraph initial schema
-- IMPORTANT: Run "PRAGMA foreign_keys = ON;" on every connection

CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    applied_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

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

-- target_title is TEXT (not FK) to allow links to unfetched pages
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

CREATE INDEX IF NOT EXISTS idx_pages_fetched_at ON pages(fetched_at) WHERE fetch_status = 'success';
CREATE INDEX IF NOT EXISTS idx_pages_fetch_status ON pages(fetch_status);
CREATE INDEX IF NOT EXISTS idx_links_source_id ON links(source_id);
CREATE INDEX IF NOT EXISTS idx_links_target_title ON links(target_title);

INSERT INTO schema_migrations (version, name) VALUES (1, 'initial_schema');
