# Phase 1: Scraper & Cache Implementation

## Overview

Phase 1 implements the foundation of WikiGraph: fetching Wikipedia pages, parsing their internal links, and caching results in SQLite to avoid redundant requests.

---

## Table of Contents

- [Architecture](#architecture)
- [Components](#components)
  - [Scraper](#scraper)
  - [Parser](#parser)
  - [Cache](#cache)
  - [Database](#database)
- [API Reference](#api-reference)
- [Configuration](#configuration)
- [Usage](#usage)
- [Testing](#testing)
- [Edge Cases](#edge-cases)
- [Performance](#performance)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         CLI                                 │
│                   wikigraph fetch <title>                   │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      Scraper                                │
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │    Cache     │───▶│   Fetcher    │───▶│    Parser    │  │
│  │   (check)    │    │   (HTTP)     │    │   (HTML)     │  │
│  └──────────────┘    └──────────────┘    └──────────────┘  │
│          │                                      │           │
│          │                                      │           │
│          ▼                                      ▼           │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                     SQLite                            │  │
│  │              pages / links tables                     │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow

1. User requests a Wikipedia page title via CLI
2. Cache layer checks if page exists in database
3. If cached and fresh, return stored links
4. If not cached, Fetcher retrieves HTML from Wikipedia
5. Parser extracts internal Wikipedia links from HTML
6. Results stored in database for future requests
7. Links returned to user

---

## Components

### Scraper

The scraper package orchestrates the fetching and parsing workflow.

**Location**: `internal/scraper/`

**Files**:
- `scraper.go` - Main scraper logic and public API
- `fetcher.go` - HTTP client for Wikipedia requests
- `parser.go` - HTML parsing and link extraction
- `scraper_test.go` - Unit tests

#### Scraper Interface

```go
type Scraper interface {
    // Fetch retrieves a Wikipedia page and returns its links
    // Returns cached results if available and fresh
    Fetch(ctx context.Context, title string) (*PageResult, error)
    
    // FetchWithOptions allows customizing fetch behavior
    FetchWithOptions(ctx context.Context, title string, opts FetchOptions) (*PageResult, error)
}

type PageResult struct {
    Title      string
    Links      []Link
    FetchedAt  time.Time
    FromCache  bool
}

type Link struct {
    TargetTitle string
    AnchorText  string
}

type FetchOptions struct {
    BypassCache    bool
    MaxLinks       int
    IncludeAnchors bool
}
```

#### Implementation Details

**Rate Limiting**:
- Default: 1 request per second
- Configurable via `WithRateLimit(duration)`
- Uses `golang.org/x/time/rate` for token bucket

**User-Agent**:
- Must identify as a bot per Wikipedia policy
- Format: `WikiGraph/1.0 (https://github.com/user/wikigraph; contact@email.com)`

**Timeouts**:
- Connection timeout: 10 seconds
- Read timeout: 30 seconds
- Total request timeout: 45 seconds

---

### Parser

The parser extracts internal Wikipedia links from HTML content.

**Location**: `internal/scraper/parser.go`

#### Parsing Rules

**Include**:
- Links matching `/wiki/[Title]` pattern
- Links within the main content area (`#mw-content-text`)
- Links with valid article titles

**Exclude**:
- External links (non-Wikipedia URLs)
- Special namespaces: `Wikipedia:`, `Help:`, `File:`, `Template:`, `Category:`, `Portal:`, `Talk:`, `User:`, `Special:`, `MediaWiki:`
- Citation links (`#cite_note-*`)
- Fragment-only links (`#section`)
- Red links (non-existent pages)
- Links in navigation, footer, sidebars
- Links in infobox external references

#### Link Normalization

```go
// NormalizeTitle converts a Wikipedia URL or title to canonical form
func NormalizeTitle(input string) string {
    // Remove URL prefix if present
    // /wiki/Albert_Einstein -> Albert_Einstein
    
    // Decode URL encoding
    // Albert%20Einstein -> Albert Einstein
    
    // Replace underscores with spaces for storage
    // Albert_Einstein -> Albert Einstein
    
    // Handle special characters
    // Keep: letters, numbers, spaces, hyphens, parentheses
}
```

#### Parser Interface

```go
type Parser interface {
    // Parse extracts links from HTML content
    Parse(html []byte) ([]Link, error)
    
    // ParseWithSelector extracts links from a specific CSS selector
    ParseWithSelector(html []byte, selector string) ([]Link, error)
}
```

---

### Cache

The cache layer manages SQLite storage and retrieval.

**Location**: `internal/cache/`

**Files**:
- `cache.go` - Cache interface and implementation
- `cache_test.go` - Unit tests

#### Cache Interface

```go
type Cache interface {
    // GetPage retrieves a cached page if it exists and is fresh
    GetPage(ctx context.Context, title string) (*CachedPage, error)
    
    // SetPage stores a page and its links
    SetPage(ctx context.Context, page *CachedPage) error
    
    // GetLinks retrieves links for a cached page
    GetLinks(ctx context.Context, pageID int64) ([]Link, error)
    
    // IsStale checks if a cached page needs refreshing
    IsStale(page *CachedPage) bool
    
    // Invalidate removes a page from cache
    Invalidate(ctx context.Context, title string) error
    
    // Stats returns cache statistics
    Stats(ctx context.Context) (*CacheStats, error)
}

type CachedPage struct {
    ID          int64
    Title       string
    ContentHash string
    FetchedAt   time.Time
    Links       []Link
}

type CacheStats struct {
    TotalPages   int64
    TotalLinks   int64
    OldestPage   time.Time
    NewestPage   time.Time
    DatabaseSize int64
}
```

#### Cache Freshness

Default TTL: 7 days (configurable)

```go
func (c *cache) IsStale(page *CachedPage) bool {
    return time.Since(page.FetchedAt) > c.ttl
}
```

---

### Database

SQLite database schema and migrations.

**Location**: `migrations/`

#### Schema

See [database-schema.md](./database-schema.md) for the complete schema documentation.

**Key tables:**

```sql
-- Pages table with fetch status tracking
CREATE TABLE IF NOT EXISTS pages (
    id            INTEGER PRIMARY KEY,
    title         TEXT UNIQUE NOT NULL,
    content_hash  TEXT,
    fetch_status  TEXT NOT NULL DEFAULT 'pending'
                  CHECK(fetch_status IN ('pending', 'success', 'redirect', 'not_found', 'error')),
    redirect_to   TEXT,
    fetched_at    TEXT,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Links table with length constraints
CREATE TABLE IF NOT EXISTS links (
    id            INTEGER PRIMARY KEY,
    source_id     INTEGER NOT NULL,
    target_title  TEXT NOT NULL CHECK(length(target_title) <= 512),
    anchor_text   TEXT CHECK(anchor_text IS NULL OR length(anchor_text) <= 1024),
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    FOREIGN KEY (source_id) REFERENCES pages(id) ON DELETE CASCADE,
    UNIQUE(source_id, target_title)
);
```

**Design decisions:**
- **ISO8601 timestamps**: Portable, sortable, human-readable
- **fetch_status**: Tracks success/redirect/not_found/error states
- **redirect_to**: Preserves redirect chain information
- **No AUTOINCREMENT**: Unnecessary overhead in SQLite
- **CHECK constraints**: Prevent unbounded text fields

#### Why `target_title` Instead of `target_id`?

We store `target_title` as text rather than a foreign key because:

1. We may have links to pages we haven't fetched yet
2. Avoids creating placeholder page records
3. Simplifies the crawling logic in Phase 2
4. Trade-off: ~50 bytes per link vs ~8 bytes, acceptable at scale

---

## API Reference

### CLI Commands

```bash
# Fetch a single page and display its links
wikigraph fetch <title> [flags]

Flags:
  -n, --max-links int      Maximum links to display (default: all)
  -f, --format string      Output format: text, json, csv (default: text)
  -v, --verbose            Show detailed fetch information
      --bypass-cache       Force fetch from Wikipedia
      --include-anchors    Include anchor text in output

# Examples
wikigraph fetch "Albert Einstein"
wikigraph fetch "World_War_II" --max-links=50 --format=json
wikigraph fetch "Go_(programming_language)" --bypass-cache
```

### Programmatic API

```go
package main

import (
    "context"
    "log"
    
    "github.com/user/wikigraph/internal/cache"
    "github.com/user/wikigraph/internal/scraper"
)

func main() {
    // Initialize cache
    c, err := cache.New("wikigraph.db")
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    // Initialize scraper
    s := scraper.New(
        scraper.WithCache(c),
        scraper.WithRateLimit(time.Second),
        scraper.WithUserAgent("WikiGraph/1.0"),
    )
    
    // Fetch a page
    ctx := context.Background()
    result, err := s.Fetch(ctx, "Albert Einstein")
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Found %d links (cached: %v)", len(result.Links), result.FromCache)
    for _, link := range result.Links {
        log.Printf("  - %s", link.TargetTitle)
    }
}
```

---

## Configuration

### Environment Variables

```bash
# Database
WIKIGRAPH_DB_PATH=./data/wikigraph.db

# Scraper
WIKIGRAPH_RATE_LIMIT=1s
WIKIGRAPH_REQUEST_TIMEOUT=45s
WIKIGRAPH_USER_AGENT="WikiGraph/1.0 (contact@email.com)"

# Cache
WIKIGRAPH_CACHE_TTL=168h  # 7 days
```

### Config File (Optional)

```yaml
# config.yaml
database:
  path: ./data/wikigraph.db

scraper:
  rate_limit: 1s
  request_timeout: 45s
  user_agent: "WikiGraph/1.0 (contact@email.com)"

cache:
  ttl: 168h
```

---

## Usage

### Basic Usage

```bash
# Fetch a page
./wikigraph fetch "Albert Einstein"

# Output:
# Fetched "Albert Einstein" in 0.84s
# Found 347 links
# 
# Sample links:
#   - Physics
#   - Germany
#   - Nobel Prize in Physics
#   - Theory of relativity
#   - Princeton University
#   ... (342 more)
```

### JSON Output

```bash
./wikigraph fetch "Albert Einstein" --format=json
```

```json
{
  "title": "Albert Einstein",
  "link_count": 347,
  "fetched_at": "2024-01-15T10:30:00Z",
  "from_cache": false,
  "fetch_duration_ms": 840,
  "links": [
    {"target_title": "Physics", "anchor_text": "physics"},
    {"target_title": "Germany", "anchor_text": "Germany"},
    ...
  ]
}
```

### Cache Statistics

```bash
./wikigraph cache stats

# Output:
# Cache Statistics
# ----------------
# Total pages:    142
# Total links:    48,293
# Database size:  12.4 MB
# Oldest entry:   2024-01-08 14:22:00
# Newest entry:   2024-01-15 10:30:00
```

---

## Testing

### Running Tests

```bash
# All tests
make test

# With coverage
go test -cover ./...

# Specific package
go test ./internal/scraper/...

# Verbose
go test -v ./internal/scraper/...
```

### Test Structure

```
internal/
├── scraper/
│   ├── scraper_test.go      # Integration tests
│   ├── fetcher_test.go      # HTTP client tests
│   ├── parser_test.go       # HTML parsing tests
│   └── testdata/
│       ├── albert_einstein.html
│       ├── disambiguation.html
│       └── redirect.html
├── cache/
│   └── cache_test.go        # Cache tests
```

### Example Tests

```go
// parser_test.go
func TestParser_ExtractsInternalLinks(t *testing.T) {
    html := loadTestData(t, "albert_einstein.html")
    
    p := NewParser()
    links, err := p.Parse(html)
    
    require.NoError(t, err)
    assert.Greater(t, len(links), 100)
    assert.Contains(t, linkTitles(links), "Physics")
    assert.Contains(t, linkTitles(links), "Germany")
}

func TestParser_ExcludesSpecialNamespaces(t *testing.T) {
    html := []byte(`
        <div id="mw-content-text">
            <a href="/wiki/Physics">Physics</a>
            <a href="/wiki/Wikipedia:About">About</a>
            <a href="/wiki/Help:Contents">Help</a>
            <a href="/wiki/File:Einstein.jpg">Image</a>
        </div>
    `)
    
    p := NewParser()
    links, err := p.Parse(html)
    
    require.NoError(t, err)
    assert.Len(t, links, 1)
    assert.Equal(t, "Physics", links[0].TargetTitle)
}

func TestParser_HandlesURLEncoding(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"/wiki/Albert%20Einstein", "Albert Einstein"},
        {"/wiki/Go_(programming_language)", "Go (programming language)"},
        {"/wiki/Caf%C3%A9", "Café"},
    }
    
    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            result := NormalizeTitle(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

---

## Edge Cases

### Handled Edge Cases

| Case | Behavior |
|------|----------|
| **Redirect pages** | Follow redirect, store canonical title |
| **Disambiguation pages** | Treat as normal page, extract all links |
| **Missing pages** | Return `ErrPageNotFound` |
| **Rate limited (429)** | Exponential backoff, max 3 retries |
| **Network timeout** | Return `ErrTimeout` after configured duration |
| **Invalid title** | Return `ErrInvalidTitle` |
| **Empty page** | Return empty links slice, not an error |
| **Non-English Wikipedia** | Only en.wikipedia.org supported |

### Error Types

```go
var (
    ErrPageNotFound  = errors.New("page not found")
    ErrInvalidTitle  = errors.New("invalid page title")
    ErrRateLimited   = errors.New("rate limited by Wikipedia")
    ErrTimeout       = errors.New("request timeout")
    ErrNetworkError  = errors.New("network error")
)
```

### Redirect Handling

```go
// Fetcher follows redirects and returns canonical title
result, err := scraper.Fetch(ctx, "Einstein")
// result.Title == "Albert Einstein" (redirected)
```

---

## Performance

### Benchmarks

```bash
go test -bench=. ./internal/scraper/
```

```
BenchmarkParser_SmallPage-8      10000    105234 ns/op    48KB alloc
BenchmarkParser_LargePage-8       2000    892341 ns/op   156KB alloc
BenchmarkCache_Get-8            100000     15234 ns/op     2KB alloc
BenchmarkCache_Set-8             50000     31234 ns/op     4KB alloc
```

### Optimization Notes

1. **HTML Parsing**: Using `goquery` for CSS selectors; consider `golang.org/x/net/html` for lower-level control if needed

2. **Database**: 
   - Batch inserts for links (single transaction)
   - Prepared statements for repeated queries
   - WAL mode enabled for better concurrency

3. **Memory**:
   - Stream large HTML responses instead of loading entirely
   - Limit stored anchor text length to 256 chars

### Database Optimization

```sql
-- Enable WAL mode for better concurrent access
PRAGMA journal_mode=WAL;

-- Optimize for our read-heavy workload
PRAGMA synchronous=NORMAL;
PRAGMA cache_size=10000;
PRAGMA temp_store=MEMORY;
```

---

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/gocolly/colly/v2` | v2.1.0 | Web scraping framework |
| `github.com/PuerkitoBio/goquery` | v1.8.1 | HTML parsing |
| `modernc.org/sqlite` | v1.28.0 | Pure Go SQLite driver |
| `golang.org/x/time/rate` | latest | Rate limiting |
| `github.com/stretchr/testify` | v1.8.4 | Testing assertions |

---

## Known Limitations

1. **English Wikipedia only**: Other language editions not supported
2. **No JavaScript rendering**: Pages requiring JS won't parse correctly
3. **Table/infobox links**: Some structured data links may be missed
4. **Rate limit**: Single request per second limits throughput

---

## Next Steps (Phase 2)

Phase 1 provides the foundation. Phase 2 will add:

- BFS crawler to fetch multiple pages
- Graph construction from stored links
- Pathfinding algorithms
- Crawl job management

See [Phase 2 Documentation](./phase2-graph-construction.md) for details.
