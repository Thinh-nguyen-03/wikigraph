# Graph Database Migration: Architectural Decision Document

## Executive Summary

At 162 million edges, WikiGraph's current architecture exhibits severe performance degradation during server startup (15+ minutes) and cache operations (30+ minutes or timeout). After analysis, it was determined that SQLite, while excellent for OLTP workloads, is fundamentally unsuited for large-scale graph traversal operations. This document proposes migrating to a dual-database architecture using Neo4j for graph queries while retaining SQLite for crawl data persistence.

**Key Metrics:**
- Current startup time: 15 minutes (unacceptable for development/deployment)
- Current cache serialization: 10+ minutes
- Current cache deserialization: 30+ minutes or timeout
- Target startup time: <1 second
- Target query latency: <10ms for typical path queries

---

## 1. Problem Statement

### 1.1 Current Architecture

WikiGraph currently uses a single SQLite database for both:
1. **Crawler data persistence** (pages, links, fetch metadata)
2. **Graph query operations** (pathfinding, connection analysis)

```
┌─────────────────────────────────────┐
│        SQLite Database              │
│  ┌──────────┐      ┌──────────┐    │
│  │  pages   │      │  links   │    │
│  │  5.6M    │◄────►│  162M    │    │
│  └──────────┘      └──────────┘    │
└─────────────────────────────────────┘
           │
           ▼
  In-Memory Graph Structure
  (Loaded on startup)
```

### 1.2 Observed Performance Issues

**Issue 1: Startup Bottleneck**
- **Symptom:** Server takes 15 minutes to start
- **Impact:** Blocks deployments, makes development iteration slow
- **Scale dependency:** Linear with edge count (O(E))

**Issue 2: Cache Serialization Failure**
- **Symptom:** Gob serialization creates 7GB file, takes 10+ minutes
- **Impact:** Extended shutdown time, resource waste

**Issue 3: Cache Deserialization Timeout**
- **Symptom:** Loading 7GB gob file hangs indefinitely (30+ minutes observed)
- **Impact:** Cache system completely ineffective at scale
- **Root cause:** Gob decoder overhead + memory allocation for 5.6M nodes

**Issue 4: Memory Footprint**
- **Symptom:** ~12-15GB RAM required for full graph
- **Impact:** Limits deployment options, expensive for cloud hosting

### 1.3 Performance Measurements

| Operation | Small Graph (1M edges) | Large Graph (162M edges) | Degradation Factor |
|-----------|----------------------|--------------------------|-------------------|
| Database load | 5 seconds | 15 minutes (900s) | 180x |
| Cache save | 2 seconds | 10+ minutes (600s+) | 300x+ |
| Cache load | 1 second | Timeout (1800s+) | 1800x+ |
| Memory usage | 500MB | 12-15GB | 24-30x |

**Observation:** Performance degrades **super-linearly** with edge count, indicating algorithmic inefficiency.

---

## 2. Root Cause Analysis

### 2.1 Bottleneck 1: Database Query Pattern

**Current approach:**
```sql
SELECT p.title, l.target_title
FROM links l
JOIN pages p ON p.id = l.source_id
WHERE p.fetch_status = 'success'
```

**Problem:**
- **162 million row scan** even with covering index
- Sequential I/O bound operation
- SQLite must materialize entire result set (1GB+ memory)

**Why it's slow:**
SQLite is optimized for:
- OLTP (Online Transaction Processing)
- Point queries (`WHERE id = ?`)
- Small result sets
- NOT for bulk scans of millions of rows
- NOT for graph traversal algorithms

**Graph query characteristics:**
```
Pathfinding: BFS from source to target
- Requires: Recursive neighbor lookups
- Pattern: Many small random-access queries
- SQLite weakness: Each hop = separate query with join overhead
```

### 2.2 Bottleneck 2: Gob Serialization

**Why gob fails at scale:**

```go
type SerializableGraph struct {
    Nodes map[string]*SerializableNode  // 5.6M entries
    // Each node contains:
    //   - Title (string)
    //   - OutLinkTitles []string (avg 29 links)
    //   - InLinkTitles []string (avg 29 links)
}
```

**Memory layout issue:**
1. **String duplication:** Each edge stores target title as string (not pointer)
   - 162M edges × avg 20 bytes/title = ~3.2GB just for edge titles
   - Additional 3.2GB for reverse edges = **6.4GB minimum**

2. **Gob encoding overhead:**
   - Type information encoded with each struct
   - Map overhead (hash table metadata)
   - Slice headers for each node's link arrays
   - Result: 7GB file (10% overhead)

3. **Deserialization cost:**
   - Must allocate 5.6M map entries
   - Must allocate 11.2M slices (in-links + out-links)
   - Must reconstruct 162M string references
   - Result: **Extensive garbage collector pressure**

**CPU profiling during deserialization would show:**
- 40% time in `encoding/gob.Decoder.Decode()`
- 30% time in memory allocation
- 20% time in garbage collection
- 10% time in actual data copy

### 2.3 Bottleneck 3: In-Memory Graph Construction

**Current implementation:**
```go
// Two-pass reconstruction
// Pass 1: Create all nodes
for title, sn := range sg.Nodes {  // 5.6M iterations
    node := &Node{
        Title: sn.Title,
        OutLinks: make([]*Node, 0, len(sn.OutLinkTitles)),
        InLinks: make([]*Node, 0, len(sn.InLinkTitles)),
    }
    g.nodes[title] = node
}

// Pass 2: Wire up connections
for title, sn := range sg.Nodes {  // 5.6M iterations
    for _, outTitle := range sn.OutLinkTitles {  // 162M total lookups
        target := g.nodes[outTitle]  // Map lookup
        node.OutLinks = append(node.OutLinks, target)
    }
}
```

**Complexity analysis:**
- Pass 1: O(N) where N = nodes (5.6M)
- Pass 2: O(E) where E = edges (162M) × map lookup O(1)
- Total: O(N + E) = O(167.6M operations)

**Why it's slow:**
- 167.6M map lookups = cache misses
- 162M slice append operations = reallocation overhead
- 162M pointer assignments = memory writes

**Memory allocation pattern:**
```
Total allocations during reconstruction:
- 5.6M node structs (×64 bytes) = 358MB
- 11.2M slice headers (×24 bytes) = 269MB
- 162M pointers (×8 bytes) = 1.3GB
- Map overhead (~50% of entries) = 179MB
Total: ~2GB allocations (triggers GC multiple times)
```

### 2.4 Bottleneck 4: All-or-Nothing Loading

**Fundamental architectural issue:**

The current design requires loading the **entire graph** into memory before serving any requests.

**Reality of usage patterns:**
- Wikipedia has ~6M pages
- User queries typically involve <100 pages
- The system loads **60,000x more data than needed** per query

**Example:**
```
Query: Find path from "Python_(programming_language)" to "Java_(programming_language)"
- Typical path length: 3-4 hops
- Nodes touched: ~100-500
- Nodes loaded: 5,600,000
- Efficiency: 0.009% of loaded data used
```

This is like **downloading the entire Wikipedia dump to read one article**.

---

## 3. Why SQLite is the Wrong Tool

### 3.1 SQLite Design Goals vs. Our Requirements

| Requirement | SQLite Design | Our Need | Match |
|-------------|---------------|----------|-------|
| Query pattern | Row-oriented (OLTP) | Graph-oriented | No |
| Access pattern | Sequential scans | Random neighbor lookups | No |
| Data structure | Tables with foreign keys | Nodes with edges | No |
| Traversal | Recursive CTEs (slow) | Native graph algorithms | No |
| Index type | B-tree | Graph-specific (adjacency) | No |
| Write pattern | Frequent updates | Bulk sync | Yes |
| Transaction model | ACID compliance | Eventual consistency OK | Yes |

**Conclusion:** SQLite is optimized for the crawler workload, NOT the query workload.

### 3.2 Graph Query Complexity in SQL

**Pathfinding in SQL requires:**
```sql
-- Breadth-first search in SQL (pseudo-code)
WITH RECURSIVE path AS (
    -- Base case: start node
    SELECT source_id, target_title, 1 as depth
    FROM links WHERE source_id = (SELECT id FROM pages WHERE title = ?)

    UNION ALL

    -- Recursive case: expand frontier
    SELECT l.source_id, l.target_title, p.depth + 1
    FROM links l
    JOIN path p ON l.source_id = (
        SELECT id FROM pages WHERE title = p.target_title
    )
    WHERE p.depth < 6  -- Max depth limit
)
SELECT * FROM path WHERE target_title = ?;
```

**Problems with this approach:**
1. **Recursive CTE overhead:** Each level creates a temporary result set
2. **Join costs:** Must join through pages table for ID ↔ title mapping
3. **No early termination:** Cannot stop when target found (SQL set-based)
4. **Memory explosion:** Frontier grows exponentially (branching factor ~29)
5. **No index utilization:** Graph algorithms don't map to B-tree indexes

**Performance:**
- Depth 1: ~29 rows
- Depth 2: ~841 rows (29²)
- Depth 3: ~24,389 rows (29³)
- Depth 4: ~707,281 rows (29⁴)
- SQLite must materialize all before filtering

### 3.3 What Is Actually Needed

**Graph database characteristics:**
1. **Native graph storage:** Nodes and edges as first-class citizens
2. **Index-free adjacency:** O(1) neighbor access via pointers
3. **Graph algorithms:** Built-in BFS, Dijkstra, etc.
4. **Lazy evaluation:** Only load subgraphs needed for query
5. **Cypher/Gremlin:** Graph query languages designed for traversal

**Example in Cypher (Neo4j):**
```cypher
MATCH path = shortestPath(
    (start:Page {title: $from})-[:LINKS_TO*1..6]-(end:Page {title: $to})
)
RETURN path
```

This is:
- More readable
- Optimized for graph traversal
- Uses specialized indexes
- Early termination when path found

---

## 4. Solutions Evaluated

### 4.1 Option 1: Optimize Current Approach

**Attempted optimizations:**
- Covering indexes: `idx_links_source_target_covering`
- Bulk loading with `AddEdgeUnchecked()`
- Gob caching: Failed at scale (7GB file, 30min+ load)
- Graph persistence: Fundamental serialization issue

**Conclusion:** Diminishing returns. Optimizations help up to ~10M edges, fail beyond.

### 4.2 Option 2: Lazy Loading (Workaround)

**Approach:** Load subgraphs on-demand from SQLite

**Pros:**
- Fast startup (<1s)
- Memory-efficient (only load what's needed)
- No new dependencies

**Cons:**
- SQLite still slow for graph queries (recursive joins)
- Complex cache invalidation logic
- Doesn't scale query performance, only startup time
- Workaround, not a solution

**Verdict:** Band-aid on architectural mismatch

### 4.3 Option 3: Better Serialization (MessagePack, etc.)

**Approach:** Replace gob with faster format

**Estimated improvement:**
- MessagePack: 5-10x faster than gob
- Protobuf: 3-5x faster than gob
- Still limited by fundamental serialization model

**Analysis:**
- 7GB file → maybe 3-4GB with better format
- 30min load → maybe 5-10min load
- Still unacceptable for production

**Verdict:** Incremental improvement, doesn't solve root cause

### 4.4 Option 4: Graph Database (RECOMMENDED)

**Approach:** Use Neo4j for graph queries, keep SQLite for crawler data

**Architecture:**
```
┌─────────────┐           ┌──────────────┐
│   SQLite    │           │    Neo4j     │
│ (Crawl DB)  │──sync────▶│  (Graph DB)  │
│             │           │              │
│ - pages     │           │ - Nodes      │
│ - links     │           │ - Edges      │
│ - metadata  │           │ - Indexes    │
└─────────────┘           └──────────────┘
       │                         │
       ▼                         ▼
  Crawler API              Path Query API
```

**Pros:**
- Right tool for the job
- Scales to billions of edges
- 10-100x faster queries
- Industry-standard solution
- Rich query language (Cypher)
- Built-in graph algorithms

**Cons:**
- Adds operational dependency (Docker)
- Synchronization logic needed
- Slightly more complex deployment

**Performance expectations:**

| Operation | Current (SQLite) | With Neo4j | Improvement |
|-----------|-----------------|------------|-------------|
| Startup time | 15 minutes | <1 second | 900x |
| Path query (cold) | 100-500ms | 5-20ms | 10-50x |
| Path query (warm) | 50-100ms | 1-5ms | 20-50x |
| Memory usage | 12-15GB | 2-4GB | 3-4x |
| Scalability | Limited (~200M edges) | Billions | Unlimited |

**Verdict:** Best long-term solution

---

## 5. Proposed Solution: Dual-Database Architecture

### 5.1 Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      WikiGraph System                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌────────────────┐                  ┌──────────────────┐  │
│  │  Crawler API   │                  │   Query API      │  │
│  │                │                  │                  │  │
│  │  POST /crawl   │                  │  GET /path       │  │
│  │  GET  /stats   │                  │  GET /connections│  │
│  └────────┬───────┘                  └────────┬─────────┘  │
│           │                                    │            │
│           ▼                                    ▼            │
│  ┌─────────────────┐            ┌──────────────────────┐  │
│  │  SQLite DB      │            │     Neo4j            │  │
│  │  (Source of     │   sync     │   (Query Engine)     │  │
│  │   Truth)        │───────────▶│                      │  │
│  │                 │            │                      │  │
│  │ - pages table   │            │ - Page nodes         │  │
│  │ - links table   │            │ - LINKS_TO edges     │  │
│  │ - crawl meta    │            │ - Graph indexes      │  │
│  └─────────────────┘            └──────────────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 Separation of Concerns

**SQLite Responsibilities:**
- Crawler state management (pages, fetch_status, timestamps)
- Link storage and deduplication
- Crawl statistics and metadata
- Write-heavy operations (crawler inserts/updates)
- Source of truth for all data

**Neo4j Responsibilities:**
- Graph query operations (pathfinding, connections)
- Graph analytics (centrality, clustering, etc.)
- Read-heavy traversal operations
- Optimized graph algorithms
- Derived data (synchronized from SQLite)

### 5.3 Data Synchronization Strategy

**Initial Sync:**
```go
// Bulk load from SQLite to Neo4j on first startup
func (s *Syncer) InitialSync() error {
    // 1. Load all successful pages as nodes
    pages := db.Query("SELECT title FROM pages WHERE fetch_status = 'success'")
    neo4j.BulkCreateNodes(pages)  // ~5.6M nodes

    // 2. Load all links as edges
    links := db.Query("SELECT p.title, l.target_title FROM links l JOIN pages p...")
    neo4j.BulkCreateEdges(links)  // ~162M edges

    // Uses Neo4j's LOAD CSV or Bolt batch API
    // Expected time: 5-10 minutes (one-time)
}
```

**Incremental Sync:**
```go
// Sync only new/updated data periodically
func (s *Syncer) IncrementalSync(since time.Time) error {
    // 1. Sync new pages
    newPages := db.Query("SELECT title FROM pages WHERE created_at > ?", since)
    neo4j.CreateNodes(newPages)

    // 2. Sync new links
    newLinks := db.Query("SELECT ... FROM links WHERE created_at > ?", since)
    neo4j.CreateEdges(newLinks)

    // Runs every N minutes (configurable)
    // Typical sync time: <1 second for normal crawl rate
}
```

**Sync frequency options:**
1. **Real-time:** After each crawl operation (high consistency, more overhead)
2. **Batched:** Every 1-5 minutes (balanced)
3. **On-demand:** Manual trigger or before queries (lazy sync)

**Recommendation:** Batched (every 5 minutes) - balances freshness and performance

### 5.4 Neo4j Schema Design

```cypher
// Node: Page
CREATE CONSTRAINT page_title_unique ON (p:Page) ASSERT p.title IS UNIQUE;

// Properties: title (indexed automatically by constraint)
(:Page {title: "Python_(programming_language)"})

// Edge: LINKS_TO
// Properties: None needed (link existence is the data)
(:Page)-[:LINKS_TO]->(:Page)

// Indexes
CREATE INDEX page_title_lookup FOR (p:Page) ON (p.title);

// This simple schema is sufficient because:
// - Neo4j's index-free adjacency makes traversal O(1)
// - No need for created_at on edges (not used in queries)
// - Source of truth remains in SQLite
```

### 5.5 Query Examples

**Before (SQLite + In-Memory Graph):**
```go
// Requires 15-minute startup to load graph
graph := loader.Load()  // Blocks server startup
path := graph.FindPath(from, to)  // Fast once loaded
```

**After (Neo4j):**
```go
// Server starts immediately, queries hit Neo4j directly
session := neo4j.NewSession()
result := session.Run(`
    MATCH path = shortestPath(
        (start:Page {title: $from})-[:LINKS_TO*1..6]-(end:Page {title: $to})
    )
    RETURN [node in nodes(path) | node.title] AS titles
`, map[string]interface{}{
    "from": from,
    "to": to,
})
// Returns in ~5-20ms
```

### 5.6 Deployment Architecture

**Development (Docker Compose):**
```yaml
version: '3.8'
services:
  neo4j:
    image: neo4j:5.15
    ports:
      - "7474:7474"  # Browser UI
      - "7687:7687"  # Bolt protocol
    environment:
      - NEO4J_AUTH=neo4j/wikigraph
      - NEO4J_dbms_memory_heap_max__size=4G
      - NEO4J_dbms_memory_pagecache_size=2G
    volumes:
      - neo4j-data:/data

  wikigraph:
    build: .
    depends_on:
      - neo4j
    environment:
      - NEO4J_URI=bolt://neo4j:7687
      - NEO4J_USER=neo4j
      - NEO4J_PASSWORD=wikigraph
    volumes:
      - ./wikigraph.db:/app/wikigraph.db

volumes:
  neo4j-data:
```

**Production considerations:**
- Neo4j Enterprise for clustering (high availability)
- Separate Neo4j instance from app server
- Monitoring: Neo4j metrics + query logging
- Backup: Neo4j backup + SQLite backup

---

## 6. Implementation Plan

### Phase 1: Setup & Proof of Concept (Day 1)
- [ ] Set up Neo4j with Docker
- [ ] Create Go Neo4j client wrapper
- [ ] Implement basic node/edge creation
- [ ] Test bulk import with small dataset (1K pages)
- [ ] Verify query performance

### Phase 2: Bulk Sync Implementation (Day 2)
- [ ] Implement initial bulk sync from SQLite
- [ ] Create sync progress tracking
- [ ] Handle sync errors and retries
- [ ] Test with full 162M edge dataset
- [ ] Measure sync time and optimize

### Phase 3: API Integration (Day 3)
- [ ] Modify path query handler to use Neo4j
- [ ] Modify connections handler to use Neo4j
- [ ] Add fallback logic (if Neo4j unavailable)
- [ ] Update health check to include Neo4j status
- [ ] Integration tests

### Phase 4: Incremental Sync (Day 4)
- [ ] Implement change tracking in SQLite
- [ ] Create incremental sync service
- [ ] Add sync scheduling (every 5 minutes)
- [ ] Handle concurrent crawl + sync
- [ ] Test data consistency

### Phase 5: Documentation & Cleanup (Day 5)
- [ ] Update README with Neo4j setup
- [ ] Document sync process
- [ ] Add architecture diagrams
- [ ] Performance benchmarks
- [ ] Remove old in-memory graph loading code

---

## 7. Risk Analysis & Mitigation

### Risk 1: Data Consistency
**Risk:** SQLite and Neo4j data might diverge

**Mitigation:**
- SQLite is source of truth (always)
- Neo4j can be rebuilt from SQLite at any time
- Add consistency check command: `wikigraph verify-sync`
- Monitor sync lag metrics

### Risk 2: Neo4j Operational Complexity
**Risk:** Adds dependency, potential failure point

**Mitigation:**
- Docker makes deployment simple
- Fallback to "503 Service Unavailable" if Neo4j down
- Keep SQLite path queries as emergency fallback
- Document recovery procedures

### Risk 3: Sync Performance
**Risk:** Syncing 162M edges might be slow

**Mitigation:**
- Use Neo4j's `UNWIND` batch API (handles 10K+ items/query)
- Parallel sync workers (partition by page ID ranges)
- Monitor and alert on sync lag
- Optimize with periodic full rebuild (weekly) vs continuous incremental

### Risk 4: Learning Curve
**Risk:** Team needs to learn Cypher query language

**Mitigation:**
- Cypher is simpler than SQL for graph queries
- Extensive Neo4j documentation
- Small query surface area (mainly shortest path)
- Can abstract behind Go interface

---

## 8. Success Metrics

### Performance Metrics
- **Startup time:** 15 min → <1 second (target: <500ms)
- **Path query latency:** 100ms → <10ms (p99)
- **Memory usage:** 12-15GB → 2-4GB
- **Sync time (initial):** N/A → <10 minutes
- **Sync time (incremental):** N/A → <1 second

### Reliability Metrics
- **Query success rate:** >99.9%
- **Sync lag:** <5 minutes (p95)
- **Data consistency:** 100% (verified daily)

### Development Metrics
- **Deployment time:** 15 min (startup) → <30 seconds
- **Development iteration speed:** Restart required → Hot reload capable

---

## 9. Future Enhancements

Once Neo4j is integrated, this unlocks additional capabilities:

### 9.1 Advanced Graph Analytics
```cypher
// PageRank algorithm (identify important pages)
CALL gds.pageRank.stream('wiki-graph')
YIELD nodeId, score
RETURN gds.util.asNode(nodeId).title AS page, score
ORDER BY score DESC LIMIT 100
```

### 9.2 Community Detection
```cypher
// Find clusters of related topics
CALL gds.louvain.stream('wiki-graph')
YIELD nodeId, communityId
RETURN communityId, collect(gds.util.asNode(nodeId).title) AS pages
```

### 9.3 N-Hop Neighborhood Queries
```cypher
// Find all pages within 3 hops
MATCH (start:Page {title: $title})-[:LINKS_TO*1..3]-(neighbor:Page)
RETURN DISTINCT neighbor.title
```

### 9.4 Graph Visualization
- Use Neo4j Browser for interactive exploration
- Export subgraphs for D3.js visualization
- Identify interesting patterns (hubs, bridges)

---

## 10. Conclusion

The current SQLite-based approach is architecturally sound for the crawler component but fundamentally limited for graph query operations at scale. The proposed dual-database architecture:

1. **Solves the root problem:** Uses the right tool for each job
2. **Industry-standard:** How companies like LinkedIn, Facebook handle this
3. **Future-proof:** Scales beyond current 162M edges
4. **Maintainable:** Clear separation of concerns
5. **Performant:** 900x startup improvement, 10-50x query improvement

This migration represents a maturation from a monolithic SQLite approach to a polyglot persistence architecture—a pattern used in production systems at scale.

**Recommendation:** Proceed with implementation.

---

## Appendix A: References

- [Neo4j Graph Database](https://neo4j.com/)
- [Cypher Query Language](https://neo4j.com/docs/cypher-manual/current/)
- [Graph Databases vs Relational Databases](https://neo4j.com/blog/why-graph-databases-are-the-future/)
- [Facebook TAO: The Power of the Graph](https://research.facebook.com/publications/tao-the-power-of-the-graph/)
- [LinkedIn's Graph Database](https://engineering.linkedin.com/blog/2016/03/followfeed--linkedin-s-feed-made-faster-and-smarter)

## Appendix B: Code Examples

See implementation in:
- `internal/neo4j/` - Neo4j client wrapper
- `internal/sync/` - SQLite→Neo4j synchronization
- `internal/api/handlers.go` - Updated query handlers
- `cmd/wikigraph/sync.go` - Sync CLI commands

## Appendix C: Benchmarks

[To be populated after implementation with actual measurements]
