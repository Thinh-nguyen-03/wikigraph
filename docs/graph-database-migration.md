# Graph Database Migration Notes

## Problem Summary

At 162 million edges and 5.6 million nodes, WikiGraph's SQLite-based architecture has hit a wall. Server startup takes 15+ minutes, and the gob-based cache serialization system completely fails at this scale (7GB files, 30+ minute load times with frequent timeouts).

**Current metrics:**
- Startup time: 15 minutes
- Cache save: 10+ minutes
- Cache load: 30+ minutes or timeout
- Memory usage: 12-15GB
- Database size: 28GB (after removing anchor_text)

---

## Root Causes

### Issue 1: Full Graph Loading

The current architecture requires loading the entire 162M-edge graph into memory before serving any requests. This is fundamentally wasteful:

```go
// Current approach - loads everything
func (l *Loader) Load() (*Graph, error) {
    data, err := l.cache.GetGraphData()  // 162M edges queried
    // ... build in-memory graph
}
```

**The problem:** Wikipedia queries typically touch <100 nodes, yet we're loading 5,600,000 nodes. That's 60,000x more data than needed per query.

### Issue 2: SQLite Isn't Designed for Graph Traversal

SQLite excels at:
- Point queries (`WHERE id = ?`)
- Small result sets
- OLTP workloads

SQLite struggles with:
- Bulk scans (162M rows)
- Recursive traversal
- Graph algorithms

Our pathfinding queries need to do recursive neighbor lookups, which means:
```sql
-- Each hop requires a join through the pages table
SELECT l.target_title
FROM links l
JOIN pages p ON p.id = l.source_id
WHERE p.title = ?
```

With an average branching factor of ~29, this explodes quickly:
- Depth 1: 29 queries
- Depth 2: 841 queries
- Depth 3: 24,389 queries

### Issue 3: Gob Serialization Breakdown

The cache system tries to serialize the entire graph to a gob file:

```go
type SerializableGraph struct {
    Nodes map[string]*SerializableNode  // 5.6M entries
}
```

**Why it fails:**
1. String duplication: Each of 162M edges stores the target title as a string (not a pointer)
   - 162M edges × 20 bytes average = 3.2GB
   - Plus reverse edges = another 3.2GB
   - Total: 6.4GB minimum

2. Gob overhead: Type info, map metadata, slice headers
   - Results in 7GB file

3. Deserialization cost:
   - Allocate 5.6M map entries
   - Allocate 11.2M slices (in + out links)
   - Reconstruct 162M string references
   - Massive GC pressure

### Issue 4: All-or-Nothing Architecture

The fundamental issue: the system is architected around having the full graph in memory. This worked fine up to ~10M edges but completely breaks down at scale.

---

## Solution: Dual-Database Architecture

### Overview

Split the system into two databases, each doing what it's good at:

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
 Crawler API              Query API
```

**SQLite responsibilities:**
- Store crawler state (pages, fetch_status, timestamps)
- Manage link deduplication
- Source of truth for all data
- Handle crawler writes

**Neo4j responsibilities:**
- Serve graph queries (pathfinding, connections)
- Handle graph traversal with native algorithms
- Optimized read-only access
- Derived data (synced from SQLite)

### Why Neo4j

Neo4j is purpose-built for graph operations:

1. **Index-free adjacency:** Neighbors accessed via pointers in O(1), not joins
2. **Native graph algorithms:** BFS, Dijkstra, etc. built-in
3. **Cypher query language:** Designed for graph traversal
4. **Lazy loading:** Only loads subgraphs needed for the query
5. **Proven scale:** Handles billions of edges

**Query comparison:**

Before (SQLite + in-memory):
```go
graph := loader.Load()  // 15 minutes, blocks startup
path := graph.FindPath(from, to)
```

After (Neo4j):
```go
session := neo4j.NewSession()
result := session.Run(`
    MATCH path = shortestPath(
        (start:Page {title: $from})-[:LINKS_TO*1..6]-(end:Page {title: $to})
    )
    RETURN [node in nodes(path) | node.title] AS titles
`, params)
// Returns in ~5-20ms, no startup delay
```

### Schema Design

Neo4j schema is intentionally minimal:

```cypher
// Node constraint
CREATE CONSTRAINT page_title_unique ON (p:Page) ASSERT p.title IS UNIQUE;

// Node structure
(:Page {title: "Python_(programming_language)"})

// Edge structure (no properties needed)
(:Page)-[:LINKS_TO]->(:Page)
```

Why so simple? Because:
- Index-free adjacency makes traversal O(1)
- We don't query edge metadata (no created_at needed)
- Source of truth remains in SQLite
- Simpler = faster sync

### Synchronization Strategy

**Initial sync (one-time):**
```go
func (s *Syncer) InitialSync() error {
    // 1. Bulk load all successful pages as nodes
    pages := db.Query("SELECT title FROM pages WHERE fetch_status = 'success'")
    neo4j.BulkCreateNodes(pages)  // ~5.6M nodes

    // 2. Bulk load all links as edges
    links := db.Query("SELECT p.title, l.target_title FROM links l JOIN pages p...")
    neo4j.BulkCreateEdges(links)  // ~162M edges

    // Uses Neo4j's batch API
    // Expected time: 5-10 minutes
}
```

**Incremental sync (ongoing):**
```go
func (s *Syncer) IncrementalSync(since time.Time) error {
    // Sync only new/updated data
    newPages := db.Query("SELECT title FROM pages WHERE created_at > ?", since)
    neo4j.CreateNodes(newPages)

    newLinks := db.Query("SELECT ... FROM links WHERE created_at > ?", since)
    neo4j.CreateEdges(newLinks)

    // Runs every 5 minutes
    // Typical sync time: <1 second
}
```

**Sync approach:** Batched every 5 minutes. Balances data freshness with performance. Real-time sync adds unnecessary overhead; 5-minute lag is acceptable for this use case.

### Deployment

**Development (docker-compose.yml):**
```yaml
services:
  neo4j:
    image: neo4j:5.15
    ports:
      - "7474:7474"  # Browser
      - "7687:7687"  # Bolt
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
    volumes:
      - ./wikigraph.db:/app/wikigraph.db
```

**Production considerations:**
- Separate Neo4j instance from app server
- Monitor Neo4j metrics + query logging
- Backup both databases independently
- Consider Neo4j Enterprise for HA if needed

---

## Expected Improvements

| Metric | Before (SQLite) | After (Neo4j) | Improvement |
|--------|----------------|---------------|-------------|
| Startup time | 15 minutes | <1 second | 900x |
| Path query (cold) | 100-500ms | 5-20ms | 10-50x |
| Path query (warm) | 50-100ms | 1-5ms | 20-50x |
| Memory usage | 12-15GB | 2-4GB | 3-4x |
| Scalability limit | ~200M edges | Billions | Unlimited |

---

## Implementation Checklist

### Phase 1: Setup & POC
- [ ] Set up Neo4j with Docker
- [ ] Create Go Neo4j client wrapper
- [ ] Test bulk import with small dataset (1K pages)
- [ ] Verify query performance matches expectations

### Phase 2: Bulk Sync
- [ ] Implement initial bulk sync from SQLite
- [ ] Add sync progress tracking
- [ ] Test with full 162M edge dataset
- [ ] Measure and optimize sync time

### Phase 3: API Integration
- [ ] Update path query handler to use Neo4j
- [ ] Update connections handler to use Neo4j
- [ ] Add fallback logic if Neo4j unavailable
- [ ] Update health check to include Neo4j status

### Phase 4: Incremental Sync
- [ ] Implement change tracking in SQLite
- [ ] Create incremental sync service
- [ ] Add sync scheduling (every 5 minutes)
- [ ] Test data consistency

### Phase 5: Cleanup
- [ ] Update documentation
- [ ] Remove old in-memory graph loading code
- [ ] Add performance benchmarks
- [ ] Document recovery procedures

---

## Risks & Mitigations

**Data consistency risk:**
- SQLite remains source of truth
- Neo4j can be rebuilt from SQLite anytime
- Add `wikigraph verify-sync` command for consistency checks

**Operational complexity:**
- Docker makes deployment straightforward
- Fallback to 503 if Neo4j unavailable
- Document recovery procedures clearly

**Sync performance:**
- Use Neo4j's UNWIND batch API (handles 10K+ items/query)
- Consider parallel sync workers if needed
- Monitor sync lag metrics

---

## Future Capabilities

Once Neo4j is integrated, we can add:

**PageRank analysis:**
```cypher
CALL gds.pageRank.stream('wiki-graph')
YIELD nodeId, score
RETURN gds.util.asNode(nodeId).title AS page, score
ORDER BY score DESC LIMIT 100
```

**Community detection:**
```cypher
CALL gds.louvain.stream('wiki-graph')
YIELD nodeId, communityId
RETURN communityId, collect(gds.util.asNode(nodeId).title) AS pages
```

**N-hop neighborhood:**
```cypher
MATCH (start:Page {title: $title})-[:LINKS_TO*1..3]-(neighbor:Page)
RETURN DISTINCT neighbor.title
```

**Graph visualization:**
- Use Neo4j Browser for interactive exploration
- Export subgraphs for visualization tools
- Identify interesting patterns (hubs, bridges)

---

## References

- Neo4j: https://neo4j.com/
- Cypher docs: https://neo4j.com/docs/cypher-manual/current/
- Facebook TAO paper: https://research.facebook.com/publications/tao-the-power-of-the-graph/
- LinkedIn's graph architecture: https://engineering.linkedin.com/blog/2016/03/followfeed--linkedin-s-feed-made-faster-and-smarter

## Implementation

Code locations:
- `internal/neo4j/` - Neo4j client wrapper (to be added)
- `internal/sync/` - SQLite→Neo4j sync (to be added)
- `internal/api/handlers.go` - Query handlers (to be updated)
- `cmd/wikigraph/sync.go` - Sync commands (to be added)
