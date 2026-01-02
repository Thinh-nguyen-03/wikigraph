# WikiGraph Optimization Plan

> **Status**: Complete
> **Created**: 2026-01-01
> **Updated**: 2026-01-01
> **Scope**: Phase 1 (Scraper/Cache) and Phase 2 (Graph/Pathfinding) optimizations
>
> **Phase A Complete** - Quick wins implemented (OPT-002, OPT-003, OPT-006, OPT-011, OPT-013)
> **Phase B Complete** - Medium effort optimizations implemented (OPT-004, OPT-007, OPT-012, CQ-003)
> **Phase C Complete** - Larger changes implemented (OPT-001, OPT-005, OPT-008)
> **Phase D Complete** - Polish optimizations implemented (OPT-009, OPT-010, CQ-001, CQ-002)

This document details all identified inefficiencies and suboptimal patterns in the current implementation, with concrete solutions for each.

---

## Table of Contents

- [Executive Summary](#executive-summary)
- [High Priority Issues](#high-priority-issues)
  - [OPT-001: Sequential Page Fetching](#opt-001-sequential-page-fetching)
  - [OPT-002: GetGraphData Slice Allocation](#opt-002-getgraphdata-slice-allocation)
  - [OPT-003: Redundant Link Deletion](#opt-003-redundant-link-deletion)
- [Medium Priority Issues](#medium-priority-issues)
  - [OPT-004: N+1 Query Pattern in Target Page Creation](#opt-004-n1-query-pattern-in-target-page-creation)
  - [OPT-005: Collector Cloning Per Fetch](#opt-005-collector-cloning-per-fetch)
  - [OPT-006: Linear Prefix Search in Parser](#opt-006-linear-prefix-search-in-parser)
  - [OPT-007: Two-Query Graph Loading](#opt-007-two-query-graph-loading)
  - [OPT-008: Full HTML in Memory](#opt-008-full-html-in-memory)
- [Low Priority Issues](#low-priority-issues)
  - [OPT-009: Queue Slice Management in Pathfinder](#opt-009-queue-slice-management-in-pathfinder)
  - [OPT-010: Path Reconstruction Inefficiency](#opt-010-path-reconstruction-inefficiency)
  - [OPT-011: Stats Query Subqueries](#opt-011-stats-query-subqueries)
  - [OPT-012: Duplicate Edge Prevention](#opt-012-duplicate-edge-prevention)
  - [OPT-013: Rate Limiter Burst Size](#opt-013-rate-limiter-burst-size)
- [Code Quality Issues](#code-quality-issues)
  - [CQ-001: Inconsistent Error Wrapping](#cq-001-inconsistent-error-wrapping)
  - [CQ-002: Code Duplication in Cache](#cq-002-code-duplication-in-cache)
  - [CQ-003: Missing Database Indexes](#cq-003-missing-database-indexes)
- [Testing Gaps](#testing-gaps)
- [Implementation Order](#implementation-order)

---

## Executive Summary

The WikiGraph codebase is well-structured with correct algorithms and good separation of concerns. However, several inefficiencies exist that would become critical at scale (1M+ pages). This document prioritizes fixes by impact and implementation complexity.

| Priority | Count | Est. Effort | Impact |
|----------|-------|-------------|--------|
| High | 3 | 4-6 hours | Blocking at scale |
| Medium | 5 | 6-10 hours | Performance degradation |
| Low | 5 | 3-5 hours | Minor inefficiencies |
| Code Quality | 3 | 2-3 hours | Maintainability |

---

## High Priority Issues

### OPT-001: Sequential Page Fetching

**Problem**

Pages within a batch are fetched sequentially, blocking on each network request before starting the next. Even with rate limiting, there's no parallelism for processing/parsing.

**Source**

```
File: internal/scraper/scraper.go
Lines: 120-134
```

**Current Code**

```go
for _, title := range batch {
    // Rate limit wait
    if err := s.fetcher.WaitForRateLimit(ctx); err != nil {
        return fmt.Errorf("rate limit wait: %w", err)
    }

    // Fetch page - BLOCKS until complete
    result := s.fetcher.Fetch(ctx, title)

    // Process result...
}
```

**Impact**

- 5-10x throughput loss
- Network latency dominates processing time
- Rate limiter already controls request frequency; parallelism within limits would help

**Solution**

Implement a worker pool with the rate limiter as a shared resource. Workers wait for rate limit token before fetching, but multiple workers can process results concurrently.

**Proposed Implementation**

```go
func (s *Scraper) processBatchConcurrent(ctx context.Context, batch []string) error {
    const numWorkers = 5

    jobs := make(chan string, len(batch))
    results := make(chan fetchResult, len(batch))

    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for title := range jobs {
                // Rate limiter is thread-safe
                if err := s.fetcher.WaitForRateLimit(ctx); err != nil {
                    results <- fetchResult{title: title, err: err}
                    continue
                }
                result := s.fetcher.Fetch(ctx, title)
                results <- fetchResult{title: title, result: result}
            }
        }()
    }

    // Send jobs
    go func() {
        for _, title := range batch {
            jobs <- title
        }
        close(jobs)
    }()

    // Wait and close results
    go func() {
        wg.Wait()
        close(results)
    }()

    // Process results (can also be parallelized)
    for result := range results {
        if err := s.processResult(ctx, result); err != nil {
            slog.Error("failed to process", "title", result.title, "error", err)
        }
    }

    return nil
}

type fetchResult struct {
    title  string
    result *fetcher.Result
    err    error
}
```

**Complexity**: Medium (2-3 hours)

**Testing Required**:
- Unit test with mock fetcher
- Integration test with rate limit verification
- Concurrency stress test

---

### OPT-002: GetGraphData Slice Allocation

**Problem**

`GetGraphData()` builds node and edge slices by repeatedly calling `append()` without pre-allocation. For large graphs (1M+ pages), this causes:
- Repeated memory allocations
- GC pressure
- Potential OOM

**Source**

```
File: internal/cache/cache.go
Lines: 294-339
```

**Current Code**

```go
func (c *Cache) GetGraphData(ctx context.Context) (*GraphData, error) {
    data := &GraphData{
        Nodes: []string{},        // No capacity
        Edges: []Edge{},          // No capacity
    }

    // Query pages
    rows, err := c.db.QueryContext(ctx, `SELECT title FROM pages WHERE fetch_status = 'success'`)
    // ...

    for rows.Next() {
        var title string
        rows.Scan(&title)
        data.Nodes = append(data.Nodes, title)  // Repeated reallocation
    }

    // Similar pattern for edges...
}
```

**Impact**

- Memory: O(n) extra allocations during growth
- For 1M pages: ~20 reallocations, each copying existing data
- GC pressure causes latency spikes
- Risk of OOM if graph is very large

**Solution**

Query counts first, then pre-allocate slices with exact capacity.

**Proposed Implementation**

```go
func (c *Cache) GetGraphData(ctx context.Context) (*GraphData, error) {
    // Get counts first
    var nodeCount, edgeCount int
    err := c.db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM pages WHERE fetch_status = 'success'`).Scan(&nodeCount)
    if err != nil {
        return nil, fmt.Errorf("count pages: %w", err)
    }

    err = c.db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM links l
         JOIN pages p ON p.id = l.source_id
         WHERE p.fetch_status = 'success'`).Scan(&edgeCount)
    if err != nil {
        return nil, fmt.Errorf("count edges: %w", err)
    }

    // Pre-allocate with exact capacity
    data := &GraphData{
        Nodes: make([]string, 0, nodeCount),
        Edges: make([]Edge, 0, edgeCount),
    }

    // Now append is O(1) - no reallocations
    rows, err := c.db.QueryContext(ctx,
        `SELECT title FROM pages WHERE fetch_status = 'success'`)
    if err != nil {
        return nil, fmt.Errorf("query pages: %w", err)
    }
    defer rows.Close()

    for rows.Next() {
        var title string
        if err := rows.Scan(&title); err != nil {
            return nil, fmt.Errorf("scan page: %w", err)
        }
        data.Nodes = append(data.Nodes, title)
    }

    // Same for edges...
    return data, nil
}
```

**Complexity**: Easy (30 minutes)

**Testing Required**:
- Unit test verifying correct counts
- Benchmark comparing old vs new allocation patterns
- Memory profiling with large dataset

---

### OPT-003: Redundant Link Deletion

**Problem**

Every time a page is processed, all its links are deleted and re-inserted, even if the page content hasn't changed. This wastes database writes.

**Source**

```
File: internal/scraper/scraper.go
Lines: 180-185
```

**Current Code**

```go
// Always delete and re-insert, regardless of whether page changed
if err := s.cache.DeleteLinksFromPage(ctx, page.ID); err != nil {
    return fmt.Errorf("delete links: %w", err)
}

if err := s.cache.AddLinks(ctx, page.ID, links); err != nil {
    return fmt.Errorf("add links: %w", err)
}
```

**Impact**

- 2x database writes for unchanged pages
- Unnecessary index updates
- Transaction overhead
- For re-crawls, potentially 90%+ of pages are unchanged

**Solution**

Check if the page content hash has changed before updating links. If unchanged, skip the delete/insert cycle entirely.

**Proposed Implementation**

```go
func (s *Scraper) processPage(ctx context.Context, page *cache.Page, result *fetcher.Result) error {
    // Parse links from HTML
    links, err := s.parser.Parse(result.HTML)
    if err != nil {
        return fmt.Errorf("parse links: %w", err)
    }

    // Check if content has changed
    newHash := result.ContentHash
    if page.ContentHash != nil && *page.ContentHash == newHash {
        // Content unchanged - skip link update
        slog.Debug("content unchanged, skipping link update", "title", page.Title)

        // Still update fetched_at timestamp
        if err := s.cache.UpdatePageTimestamp(ctx, page.ID); err != nil {
            return fmt.Errorf("update timestamp: %w", err)
        }
        return nil
    }

    // Content changed - update links
    if err := s.cache.DeleteLinksFromPage(ctx, page.ID); err != nil {
        return fmt.Errorf("delete links: %w", err)
    }

    if err := s.cache.AddLinks(ctx, page.ID, links); err != nil {
        return fmt.Errorf("add links: %w", err)
    }

    // Update content hash
    if err := s.cache.UpdatePageHash(ctx, page.ID, newHash); err != nil {
        return fmt.Errorf("update hash: %w", err)
    }

    return nil
}
```

**New Cache Methods Required**

```go
// UpdatePageTimestamp updates only the fetched_at timestamp
func (c *Cache) UpdatePageTimestamp(ctx context.Context, pageID int64) error {
    _, err := c.db.ExecContext(ctx,
        `UPDATE pages SET fetched_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
                          updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
         WHERE id = ?`, pageID)
    return err
}

// UpdatePageHash updates the content hash
func (c *Cache) UpdatePageHash(ctx context.Context, pageID int64, hash string) error {
    _, err := c.db.ExecContext(ctx,
        `UPDATE pages SET content_hash = ?,
                          updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
         WHERE id = ?`, hash, pageID)
    return err
}
```

**Complexity**: Easy (1 hour)

**Testing Required**:
- Unit test: unchanged content skips link update
- Unit test: changed content triggers link update
- Integration test: verify database state after re-crawl

---

## Medium Priority Issues

### OPT-004: N+1 Query Pattern in Target Page Creation

**Problem**

`EnsureTargetPagesExist()` is called once per page inside the batch loop, causing N database operations instead of 1.

**Source**

```
File: internal/scraper/scraper.go
Line: 187
```

**Current Code**

```go
for _, title := range batch {
    // ... fetch and process ...

    // Called for EACH page in batch - N+1 pattern
    if err := s.cache.EnsureTargetPagesExist(ctx, links); err != nil {
        return fmt.Errorf("ensure targets: %w", err)
    }
}
```

**Impact**

- For batch size 50: 50 separate INSERT operations
- Transaction overhead multiplied by batch size
- Could be single bulk INSERT

**Solution**

Collect all target titles from the batch, deduplicate, and insert in one operation after the batch completes.

**Proposed Implementation**

```go
func (s *Scraper) processBatch(ctx context.Context, batch []string) error {
    allTargets := make(map[string]struct{})

    for _, title := range batch {
        result := s.fetcher.Fetch(ctx, title)
        links, _ := s.parser.Parse(result.HTML)

        // Collect targets instead of inserting immediately
        for _, link := range links {
            allTargets[link.TargetTitle] = struct{}{}
        }

        // Process page...
    }

    // Single bulk insert after batch completes
    targetSlice := make([]string, 0, len(allTargets))
    for target := range allTargets {
        targetSlice = append(targetSlice, target)
    }

    if err := s.cache.EnsureTargetPagesExistBulk(ctx, targetSlice); err != nil {
        return fmt.Errorf("ensure targets: %w", err)
    }

    return nil
}
```

**New Cache Method**

```go
// EnsureTargetPagesExistBulk creates pending pages for all targets in one transaction
func (c *Cache) EnsureTargetPagesExistBulk(ctx context.Context, titles []string) error {
    if len(titles) == 0 {
        return nil
    }

    tx, err := c.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    stmt, err := tx.PrepareContext(ctx,
        `INSERT OR IGNORE INTO pages (title, fetch_status) VALUES (?, 'pending')`)
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, title := range titles {
        if _, err := stmt.ExecContext(ctx, title); err != nil {
            return err
        }
    }

    return tx.Commit()
}
```

**Complexity**: Easy (1 hour)

**Testing Required**:
- Unit test: bulk insert creates all pages
- Unit test: duplicates are handled correctly
- Benchmark: compare N inserts vs bulk insert

---

### OPT-005: Collector Cloning Per Fetch

**Problem**

A new Colly collector is cloned for every single fetch operation, causing unnecessary object allocation overhead.

**Source**

```
File: internal/fetcher/fetcher.go
Line: 65
```

**Current Code**

```go
func (f *Fetcher) Fetch(ctx context.Context, title string) *Result {
    result := &Result{Title: title}

    // Clone collector for each request
    c := f.collector.Clone()  // Allocates new collector

    c.OnResponse(func(r *colly.Response) {
        result.HTML = string(r.Body)
        // ...
    })

    c.Visit(url)
    c.Wait()

    return result
}
```

**Impact**

- Memory allocation per fetch
- GC pressure
- ~5-10% overhead

**Solution**

Colly collectors are thread-safe. Instead of cloning, use a single collector with request-scoped context for result capture.

**Proposed Implementation**

```go
type Fetcher struct {
    collector *colly.Collector
    rateLimiter *rate.Limiter
    mu sync.Mutex
    pendingResults map[string]*Result
}

func (f *Fetcher) Fetch(ctx context.Context, title string) *Result {
    result := &Result{Title: title}
    url := buildURL(title)

    // Store result reference for callback
    f.mu.Lock()
    f.pendingResults[url] = result
    f.mu.Unlock()

    defer func() {
        f.mu.Lock()
        delete(f.pendingResults, url)
        f.mu.Unlock()
    }()

    // Use shared collector
    if err := f.collector.Visit(url); err != nil {
        result.Error = err
    }

    return result
}

// Setup once in NewFetcher
func NewFetcher(cfg Config) *Fetcher {
    c := colly.NewCollector(
        colly.UserAgent(cfg.UserAgent),
        colly.Async(false),  // Synchronous for simpler result handling
    )

    f := &Fetcher{
        collector: c,
        pendingResults: make(map[string]*Result),
    }

    c.OnResponse(func(r *colly.Response) {
        f.mu.Lock()
        if result, ok := f.pendingResults[r.Request.URL.String()]; ok {
            result.HTML = string(r.Body)
            result.StatusCode = r.StatusCode
        }
        f.mu.Unlock()
    })

    return f
}
```

**Alternative**: Keep cloning but use `sync.Pool` for collector reuse.

```go
var collectorPool = sync.Pool{
    New: func() interface{} {
        return baseCollector.Clone()
    },
}

func (f *Fetcher) Fetch(ctx context.Context, title string) *Result {
    c := collectorPool.Get().(*colly.Collector)
    defer collectorPool.Put(c)
    // ...
}
```

**Complexity**: Medium (2 hours)

**Testing Required**:
- Concurrency test: multiple fetches don't interfere
- Memory profiling: verify reduced allocations
- Correctness test: results match expected

---

### OPT-006: Linear Prefix Search in Parser

**Problem**

For each link, the parser linearly scans an array of excluded prefixes to check if it should be filtered.

**Source**

```
File: internal/parser/parser.go
Lines: 91-96
```

**Current Code**

```go
var excludedPrefixes = []string{
    "Wikipedia:", "Help:", "File:", "Template:",
    "Category:", "Portal:", "Talk:", "User:",
    "Special:", "MediaWiki:", "Draft:", "Module:",
}

func (p *Parser) shouldExclude(title string) bool {
    for _, prefix := range excludedPrefixes {
        if strings.HasPrefix(title, prefix) {  // O(n) scan
            return true
        }
    }
    return false
}
```

**Impact**

- O(n) prefix checks per link
- For page with 500 links: 500 * 12 = 6000 string comparisons
- Minor but adds up over millions of pages

**Solution**

Use a map for O(1) lookup after extracting the namespace prefix.

**Proposed Implementation**

```go
var excludedNamespaces = map[string]bool{
    "Wikipedia":  true,
    "Help":       true,
    "File":       true,
    "Template":   true,
    "Category":   true,
    "Portal":     true,
    "Talk":       true,
    "User":       true,
    "Special":    true,
    "MediaWiki":  true,
    "Draft":      true,
    "Module":     true,
}

func (p *Parser) shouldExclude(title string) bool {
    // Extract namespace (text before first colon)
    if idx := strings.Index(title, ":"); idx != -1 {
        namespace := title[:idx]
        return excludedNamespaces[namespace]
    }
    return false
}
```

**Complexity**: Easy (15 minutes)

**Testing Required**:
- Unit test: all namespaces correctly excluded
- Unit test: regular titles not excluded
- Benchmark: compare old vs new performance

---

### OPT-007: Two-Query Graph Loading

**Problem**

Graph loading executes two separate queries: one for nodes, one for edges. This causes two database round-trips and intermediate storage of all node titles.

**Source**

```
File: internal/graph/loader.go
Lines: 16-31
```

**Current Code**

```go
func (l *Loader) Load(ctx context.Context) (*Graph, error) {
    // Query 1: Get all nodes
    data, err := l.cache.GetGraphData(ctx)
    if err != nil {
        return nil, err
    }

    g := NewWithCapacity(len(data.Nodes))

    // Add all nodes
    for _, title := range data.Nodes {
        g.AddNode(title)
    }

    // Query 2: Get all edges (implicit in GetGraphData)
    for _, edge := range data.Edges {
        g.AddEdge(edge.Source, edge.Target)
    }

    return g, nil
}
```

**Impact**

- Two database round-trips
- Intermediate slice storage for nodes
- Nodes without edges still loaded

**Solution**

Use a single query with JOIN that returns edges with source titles. Build graph directly from edges, creating nodes on demand.

**Proposed Implementation**

```go
func (c *Cache) GetEdgesWithSources(ctx context.Context) ([]EdgeWithSource, error) {
    rows, err := c.db.QueryContext(ctx, `
        SELECT p.title, l.target_title
        FROM links l
        JOIN pages p ON p.id = l.source_id
        WHERE p.fetch_status = 'success'
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var edges []EdgeWithSource
    for rows.Next() {
        var e EdgeWithSource
        if err := rows.Scan(&e.Source, &e.Target); err != nil {
            return nil, err
        }
        edges = append(edges, e)
    }
    return edges, nil
}

// In loader:
func (l *Loader) Load(ctx context.Context) (*Graph, error) {
    edges, err := l.cache.GetEdgesWithSources(ctx)
    if err != nil {
        return nil, err
    }

    g := New()
    for _, e := range edges {
        g.AddEdge(e.Source, e.Target)  // AddEdge creates nodes if needed
    }

    return g, nil
}
```

**Note**: This changes behavior slightly - nodes without outgoing edges won't be in the graph. For pathfinding, this is usually fine since you can't traverse from them anyway. If needed, add a second query for isolated nodes only.

**Complexity**: Medium (1 hour)

**Testing Required**:
- Unit test: graph structure matches expected
- Integration test: pathfinding works correctly
- Verify isolated nodes handling

---

### OPT-008: Full HTML in Memory

**Problem**

The fetcher stores the entire HTML response as a string in the Result struct, which is then passed to the parser and hashed. For large Wikipedia pages (500KB+), this causes memory spikes.

**Source**

```
File: internal/fetcher/fetcher.go
Lines: 70-73, 102-109
```

**Current Code**

```go
c.OnResponse(func(r *colly.Response) {
    result.HTML = string(r.Body)  // Entire HTML in memory
    result.StatusCode = r.StatusCode
})

// Later in scraper:
links, err := parser.Parse(result.HTML)  // Parse full HTML
hash := sha256.Sum256([]byte(result.HTML))  // Hash full HTML again
```

**Impact**

- Memory spike for large pages
- HTML traversed twice (parse + hash)
- String conversion from bytes

**Solution**

Stream the HTML: hash during download, parse directly from bytes without string conversion.

**Proposed Implementation**

```go
type Result struct {
    Title       string
    Links       []Link     // Parsed directly during fetch
    ContentHash string     // Computed during download
    StatusCode  int
    Error       error
}

c.OnResponse(func(r *colly.Response) {
    // Hash the body directly (no string conversion)
    hash := sha256.Sum256(r.Body)
    result.ContentHash = hex.EncodeToString(hash[:])

    // Parse directly from bytes
    links, err := parser.ParseBytes(r.Body)
    if err != nil {
        result.Error = err
        return
    }
    result.Links = links

    result.StatusCode = r.StatusCode
})
```

**Parser Change**

```go
// Add ParseBytes method that works with []byte directly
func (p *Parser) ParseBytes(html []byte) ([]Link, error) {
    doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
    if err != nil {
        return nil, err
    }
    return p.extractLinks(doc)
}
```

**Complexity**: Medium (1-2 hours)

**Testing Required**:
- Unit test: parsing works with bytes
- Memory profiling: verify reduced allocations
- Large page test: 1MB+ HTML

---

## Low Priority Issues

### OPT-009: Queue Slice Management in Pathfinder

**Problem**

The BFS queue uses slice append and reslicing, which causes memory churn and cache inefficiency for large graphs.

**Source**

```
File: internal/graph/pathfinder.go
Lines: 34, 68
```

**Current Code**

```go
queue := []*Node{from}

for len(queue) > 0 {
    levelSize := len(queue)

    for i := 0; i < levelSize; i++ {
        current := queue[i]
        for _, neighbor := range current.OutLinks {
            queue = append(queue, neighbor)  // Grows slice
        }
    }

    queue = queue[levelSize:]  // Reslice (doesn't free memory)
}
```

**Impact**

- Memory not released during traversal
- Repeated allocations as queue grows
- Poor cache locality

**Solution**

Use a ring buffer or double-buffer approach.

**Proposed Implementation**

```go
type nodeQueue struct {
    items []*Node
    head  int
    tail  int
}

func newNodeQueue(capacity int) *nodeQueue {
    return &nodeQueue{
        items: make([]*Node, capacity),
    }
}

func (q *nodeQueue) Push(n *Node) {
    if q.tail >= len(q.items) {
        // Grow or wrap
        newItems := make([]*Node, len(q.items)*2)
        copy(newItems, q.items[q.head:q.tail])
        q.items = newItems
        q.tail -= q.head
        q.head = 0
    }
    q.items[q.tail] = n
    q.tail++
}

func (q *nodeQueue) Pop() *Node {
    if q.head >= q.tail {
        return nil
    }
    n := q.items[q.head]
    q.items[q.head] = nil  // Help GC
    q.head++
    return n
}

func (q *nodeQueue) Len() int {
    return q.tail - q.head
}
```

**Complexity**: Medium (1 hour)

**Testing Required**:
- Unit test: queue operations correct
- Benchmark: compare with slice approach
- Large graph test

---

### OPT-010: Path Reconstruction Inefficiency

**Problem**

Path reconstruction builds a slice from target to source, then reverses it. This requires two passes over the path.

**Source**

```
File: internal/graph/pathfinder.go
Lines: 205-213
```

**Current Code**

```go
func reconstructPath(parent map[*Node]*Node, from, to *Node) []*Node {
    path := []*Node{}
    for n := to; n != nil; n = parent[n] {
        path = append(path, n)
    }

    // Reverse the path
    for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
        path[i], path[j] = path[j], path[i]
    }

    return path
}
```

**Impact**

- Two passes over path data
- Extra slice operations
- Minor for short paths, noticeable for very long paths

**Solution**

Pre-allocate and build in correct order using indices, or accept reversed order for internal use.

**Proposed Implementation**

```go
func reconstructPath(parent map[*Node]*Node, from, to *Node) []*Node {
    // Count path length first
    length := 0
    for n := to; n != nil; n = parent[n] {
        length++
    }

    // Pre-allocate and fill in reverse order
    path := make([]*Node, length)
    i := length - 1
    for n := to; n != nil; n = parent[n] {
        path[i] = n
        i--
    }

    return path
}
```

**Complexity**: Easy (15 minutes)

**Testing Required**:
- Unit test: paths correct
- Benchmark: compare approaches

---

### OPT-011: Stats Query Subqueries

**Problem**

The database stats query uses 9 separate COUNT subqueries, each scanning the entire table.

**Source**

```
File: internal/database/database.go
Lines: 142-153
```

**Current Code**

```go
query := `
SELECT
    (SELECT COUNT(*) FROM pages) as total_pages,
    (SELECT COUNT(*) FROM pages WHERE fetch_status = 'success') as fetched,
    (SELECT COUNT(*) FROM pages WHERE fetch_status = 'pending') as pending,
    (SELECT COUNT(*) FROM pages WHERE fetch_status = 'error') as errors,
    (SELECT COUNT(*) FROM pages WHERE fetch_status = 'redirect') as redirects,
    (SELECT COUNT(*) FROM pages WHERE fetch_status = 'not_found') as not_found,
    (SELECT COUNT(*) FROM links) as total_links,
    (SELECT MIN(fetched_at) FROM pages WHERE fetched_at IS NOT NULL) as oldest,
    (SELECT MAX(fetched_at) FROM pages WHERE fetched_at IS NOT NULL) as newest
`
```

**Impact**

- 9 table scans instead of 1
- For 1M pages: noticeable latency (~100ms vs ~20ms)
- Minor issue since stats are queried infrequently

**Solution**

Use conditional aggregation in a single scan.

**Proposed Implementation**

```sql
SELECT
    COUNT(*) as total_pages,
    COUNT(CASE WHEN fetch_status = 'success' THEN 1 END) as fetched,
    COUNT(CASE WHEN fetch_status = 'pending' THEN 1 END) as pending,
    COUNT(CASE WHEN fetch_status = 'error' THEN 1 END) as errors,
    COUNT(CASE WHEN fetch_status = 'redirect' THEN 1 END) as redirects,
    COUNT(CASE WHEN fetch_status = 'not_found' THEN 1 END) as not_found,
    MIN(fetched_at) as oldest,
    MAX(fetched_at) as newest
FROM pages;

-- Links count in separate query (unavoidable)
SELECT COUNT(*) FROM links;
```

**Complexity**: Easy (15 minutes)

**Testing Required**:
- Unit test: counts match expected
- Verify NULL handling for oldest/newest

---

### OPT-012: Duplicate Edge Prevention

**Problem**

The graph structure allows duplicate edges if `AddEdge(A, B)` is called multiple times with the same arguments.

**Source**

```
File: internal/graph/graph.go
Lines: 46-56
```

**Current Code**

```go
func (g *Graph) AddEdge(sourceTitle, targetTitle string) {
    g.mu.Lock()
    defer g.mu.Unlock()

    source := g.addNodeLocked(sourceTitle)
    target := g.addNodeLocked(targetTitle)

    // No duplicate check - will add multiple times
    source.OutLinks = append(source.OutLinks, target)
    target.InLinks = append(target.InLinks, source)
}
```

**Impact**

- Graph corruption if same link added twice
- Inflated edge counts
- Pathfinding still works (just less efficient)

**Solution**

Check for existing edge before adding, or deduplicate at the source (cache layer).

**Proposed Implementation**

```go
func (g *Graph) AddEdge(sourceTitle, targetTitle string) {
    g.mu.Lock()
    defer g.mu.Unlock()

    source := g.addNodeLocked(sourceTitle)
    target := g.addNodeLocked(targetTitle)

    // Check if edge already exists
    for _, existing := range source.OutLinks {
        if existing == target {
            return  // Edge already exists
        }
    }

    source.OutLinks = append(source.OutLinks, target)
    target.InLinks = append(target.InLinks, source)
}
```

**Alternative**: Use map for edges (O(1) lookup but more memory):

```go
type Node struct {
    Title    string
    OutLinks map[*Node]bool
    InLinks  map[*Node]bool
}
```

**Complexity**: Easy (30 minutes)

**Testing Required**:
- Unit test: duplicate edges not added
- Verify edge counts correct

---

### OPT-013: Rate Limiter Burst Size

**Problem**

Rate limiter is configured with burst size of 1, preventing any burst handling for momentary spikes.

**Source**

```
File: internal/fetcher/fetcher.go
Line: 50
```

**Current Code**

```go
rateLimiter: rate.NewLimiter(rate.Limit(cfg.RateLimit), 1)  // burst = 1
```

**Impact**

- Slightly lower throughput than possible
- Can't absorb momentary request bursts
- Minor issue

**Solution**

Allow small burst (3-5) while maintaining average rate.

**Proposed Implementation**

```go
// Allow burst of 3 requests while maintaining 1 req/sec average
rateLimiter: rate.NewLimiter(rate.Every(cfg.RateLimitInterval), 3)
```

**Complexity**: Trivial (5 minutes)

**Testing Required**:
- Verify Wikipedia doesn't rate limit with burst=3
- Test burst behavior

---

## Code Quality Issues

### CQ-001: Inconsistent Error Wrapping

**Problem**

Some errors use `fmt.Errorf("...: %w", err)` for wrapping, others return raw errors. This makes debugging harder.

**Locations**

- `internal/cache/cache.go`: Mixed usage
- `internal/scraper/scraper.go`: Sometimes wraps, sometimes not
- `internal/fetcher/fetcher.go`: Often loses error context

**Solution**

Standardize on always wrapping errors with context.

**Example**

```go
// Before
return err

// After
return fmt.Errorf("failed to query page %q: %w", title, err)
```

**Complexity**: Easy (1 hour across codebase)

---

### CQ-002: Code Duplication in Cache

**Problem**

`GetPage()` and `getPageByID()` have duplicated scanning logic.

**Source**

```
File: internal/cache/cache.go
Lines: 50-64 and 263-273
```

**Solution**

Extract common scanning logic into helper function.

**Proposed Implementation**

```go
func scanPage(row interface{ Scan(...interface{}) error }) (*Page, error) {
    var p Page
    var contentHash, redirectTo, fetchedAt sql.NullString

    err := row.Scan(
        &p.ID, &p.Title, &contentHash, &p.FetchStatus,
        &redirectTo, &fetchedAt, &p.CreatedAt, &p.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }

    if contentHash.Valid {
        p.ContentHash = &contentHash.String
    }
    // ... rest of NULL handling

    return &p, nil
}

func (c *Cache) GetPage(ctx context.Context, title string) (*Page, error) {
    row := c.db.QueryRowContext(ctx, selectPageQuery, title)
    return scanPage(row)
}

func (c *Cache) getPageByID(ctx context.Context, id int64) (*Page, error) {
    row := c.db.QueryRowContext(ctx, selectPageByIDQuery, id)
    return scanPage(row)
}
```

**Complexity**: Easy (30 minutes)

---

### CQ-003: Missing Database Indexes

**Problem**

Some common query patterns lack optimal indexes.

**Missing Indexes**

```sql
-- For GetPendingPages ordered by creation
CREATE INDEX idx_pages_pending_created
ON pages(fetch_status, created_at)
WHERE fetch_status = 'pending';

-- For graph loading with status filter
CREATE INDEX idx_pages_success
ON pages(fetch_status)
WHERE fetch_status = 'success';
```

**Impact**

- Full table scans for ordered pagination
- Slower graph loading as database grows

**Complexity**: Easy (15 minutes)

**Location**: `migrations/001_initial_schema.sql` or new migration

---

## Testing Gaps

| Gap | Description | Priority |
|-----|-------------|----------|
| **Stress tests** | No tests with 100K+ nodes | Medium |
| **Concurrency tests** | No race condition tests for pathfinder | High |
| **Integration tests** | No end-to-end scraper→graph→path tests | Medium |
| **Memory profiling** | No profiling with large datasets | Low |
| **Error path tests** | Limited coverage of error scenarios | Medium |

---

## Implementation Order

### Phase A: Quick Wins (2-3 hours) - COMPLETED

1. [x] [OPT-002](#opt-002-getgraphdata-slice-allocation) - Pre-allocate slices
2. [x] [OPT-006](#opt-006-linear-prefix-search-in-parser) - Map-based prefix lookup
3. [x] [OPT-003](#opt-003-redundant-link-deletion) - Skip unchanged pages
4. [x] [OPT-011](#opt-011-stats-query-subqueries) - Single-pass stats query
5. [x] [OPT-013](#opt-013-rate-limiter-burst-size) - Increase burst size

### Phase B: Medium Effort (4-6 hours) - COMPLETED

6. [x] [OPT-004](#opt-004-n1-query-pattern-in-target-page-creation) - Batch target creation
7. [x] [OPT-007](#opt-007-two-query-graph-loading) - Single-query graph load
8. [x] [OPT-012](#opt-012-duplicate-edge-prevention) - Deduplicate edges
9. [x] [CQ-003](#cq-003-missing-database-indexes) - Add indexes

### Phase C: Larger Changes (4-6 hours) - COMPLETED

10. [x] [OPT-001](#opt-001-sequential-page-fetching) - Concurrent fetching
11. [x] [OPT-005](#opt-005-collector-cloning-per-fetch) - Collector reuse
12. [x] [OPT-008](#opt-008-full-html-in-memory) - Stream HTML processing

### Phase D: Polish (2-3 hours) - COMPLETED

13. [x] [OPT-009](#opt-009-queue-slice-management-in-pathfinder) - Ring buffer queue
14. [x] [OPT-010](#opt-010-path-reconstruction-inefficiency) - Optimized reconstruction
15. [x] [CQ-001](#cq-001-inconsistent-error-wrapping) - Error wrapping
16. [x] [CQ-002](#cq-002-code-duplication-in-cache) - Extract helpers

---

## Metrics to Track

After implementing optimizations, measure:

| Metric | Before | After | Target |
|--------|--------|-------|--------|
| Pages/second (crawl) | ~1 | - | 3-5 |
| Graph load time (10K nodes) | - | - | <1s |
| Graph load time (100K nodes) | - | - | <10s |
| Path search (10K nodes) | - | - | <50ms |
| Memory usage (10K nodes) | - | - | <100MB |
| Memory usage (100K nodes) | - | - | <1GB |

---

## Notes

- All changes should maintain backward compatibility
- Each optimization should have before/after benchmarks
- Consider feature flags for major changes during testing
- Update documentation after significant changes
