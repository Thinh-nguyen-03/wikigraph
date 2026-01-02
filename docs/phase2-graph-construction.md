# Phase 2: Graph Construction & Pathfinding

> **Status: COMPLETED**
>
> This phase has been fully implemented with:
> - In-memory graph loaded from SQLite cache
> - BFS pathfinding with depth limits
> - Bidirectional search for faster pathfinding on large graphs
> - CLI command: `wikigraph path <from> <to>` with `--bidirectional`, `--max-depth`, `--format` flags
>
> See `internal/graph/` for implementation details.

## Overview

Phase 2 builds on the scraper/cache foundation to construct an in-memory graph and implement pathfinding algorithms. Users will be able to find the shortest path between any two Wikipedia articles that have been crawled.

---

## Table of Contents

- [What We Already Have](#what-we-already-have)
- [What Phase 2 Adds](#what-phase-2-adds)
- [Architecture](#architecture)
- [Components](#components)
  - [Graph Package](#graph-package)
  - [Path Command](#path-command)
- [API Reference](#api-reference)
- [Algorithms](#algorithms)
- [Testing](#testing)
- [Performance](#performance)

---

## What We Already Have

From Phase 1, the following is already implemented:

| Feature | Location | Status |
|---------|----------|--------|
| BFS crawling | `internal/scraper/` | Done |
| Depth control (`--depth`) | `cmd/wikigraph/fetch.go` | Done |
| Max pages control (`--max-pages`) | `cmd/wikigraph/fetch.go` | Done |
| Batch size control (`--batch`) | `cmd/wikigraph/fetch.go` | Done |
| Pages stored in SQLite | `internal/cache/` | Done |
| Links stored in SQLite | `internal/cache/` | Done |

---

## What Phase 2 Adds

| Feature | Location | Description |
|---------|----------|-------------|
| In-memory graph | `internal/graph/` | Load pages/links into navigable structure |
| BFS pathfinding | `internal/graph/` | Find shortest path between two pages |
| Bidirectional search | `internal/graph/` | Faster pathfinding for large graphs |
| `path` CLI command | `cmd/wikigraph/path.go` | User-facing path discovery |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         CLI                                 │
│               wikigraph path <from> <to>                    │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   internal/graph                            │
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │    Graph     │───▶│  Pathfinder  │───▶│    Result    │  │
│  │   (nodes,    │    │  (BFS/Bidi)  │    │   (path,     │  │
│  │    edges)    │    │              │    │    hops)     │  │
│  └──────────────┘    └──────────────┘    └──────────────┘  │
│          ▲                                                  │
│          │ Load                                             │
└──────────┼──────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────┐
│                   internal/cache                            │
│              (pages + links from SQLite)                    │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow

1. User requests path between two Wikipedia titles
2. Graph loader reads pages and links from SQLite
3. Graph is constructed in memory (nodes + edges)
4. Pathfinder runs BFS or bidirectional search
5. Path is returned (or "not found")

---

## Components

### Graph Package

**Location**: `internal/graph/`

**Files**:
- `graph.go` - Graph data structure and loading
- `pathfinder.go` - Pathfinding algorithms
- `graph_test.go` - Unit tests

#### Graph Structure

```go
package graph

import "sync"

type Graph struct {
    nodes map[string]*Node
    mu    sync.RWMutex
}

type Node struct {
    Title    string
    OutLinks []string  // Pages this node links to
    InLinks  []string  // Pages that link to this node
}

type PathResult struct {
    Found    bool
    Path     []string
    Hops     int
    Explored int       // Nodes examined during search
}
```

#### Graph Interface

```go
// New creates an empty graph
func New() *Graph

// AddNode adds a node if it doesn't exist
func (g *Graph) AddNode(title string) *Node

// AddEdge adds a directed edge from source to target
func (g *Graph) AddEdge(source, target string)

// GetNode returns a node by title, or nil if not found
func (g *Graph) GetNode(title string) *Node

// NodeCount returns the number of nodes
func (g *Graph) NodeCount() int

// EdgeCount returns the number of edges
func (g *Graph) EdgeCount() int
```

#### Graph Loader

```go
// Loader loads a graph from the cache
type Loader struct {
    cache *cache.Cache
}

// Load reads all successfully fetched pages and their links
func (l *Loader) Load(ctx context.Context) (*Graph, error)

// LoadSubgraph loads only pages within N hops of a starting page
func (l *Loader) LoadSubgraph(ctx context.Context, start string, maxDepth int) (*Graph, error)
```

#### Pathfinder Interface

```go
// FindPath finds the shortest path between two nodes using BFS
func (g *Graph) FindPath(from, to string) PathResult

// FindPathBidirectional uses bidirectional BFS for faster results
func (g *Graph) FindPathBidirectional(from, to string) PathResult

// FindPathWithLimit adds a maximum depth constraint
func (g *Graph) FindPathWithLimit(from, to string, maxDepth int) PathResult
```

---

### Path Command

**Location**: `cmd/wikigraph/path.go`

```go
var pathCmd = &cobra.Command{
    Use:   "path <from> <to>",
    Short: "Find the shortest path between two Wikipedia pages",
    Long: `Find the shortest path between two Wikipedia pages.

The pages must already be in the local database. Use 'wikigraph fetch'
to crawl pages first.

Examples:
  wikigraph path "Albert Einstein" "Barack Obama"
  wikigraph path "Physics" "Philosophy" --max-depth 10
  wikigraph path "Go (programming language)" "Python" --bidirectional`,
    Args: cobra.ExactArgs(2),
    RunE: runPath,
}
```

#### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--max-depth` | `-d` | 6 | Maximum path length to search |
| `--bidirectional` | `-b` | false | Use bidirectional search |
| `--format` | `-f` | text | Output format: text, json |

---

## API Reference

### CLI Commands

```bash
# Find shortest path (default BFS)
wikigraph path "Albert Einstein" "Barack Obama"
# Output:
# Path found (4 hops):
#   Albert Einstein
#   → Princeton University
#   → New Jersey
#   → Barack Obama
#
# Explored 1,247 nodes in 45ms

# With bidirectional search (faster for large graphs)
wikigraph path "Physics" "Philosophy" --bidirectional

# Limit search depth
wikigraph path "Cat" "Dog" --max-depth 3

# JSON output
wikigraph path "A" "B" --format json
```

### JSON Output

```json
{
  "found": true,
  "from": "Albert Einstein",
  "to": "Barack Obama",
  "path": [
    "Albert Einstein",
    "Princeton University",
    "New Jersey",
    "Barack Obama"
  ],
  "hops": 3,
  "explored": 1247,
  "duration_ms": 45,
  "algorithm": "bfs"
}
```

---

## Algorithms

### BFS (Breadth-First Search)

Standard single-source BFS from the start node.

```go
func (g *Graph) FindPath(from, to string) PathResult {
    if from == to {
        return PathResult{Found: true, Path: []string{from}, Hops: 0}
    }

    visited := make(map[string]bool)
    parent := make(map[string]string)
    queue := []string{from}
    visited[from] = true
    explored := 0

    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]
        explored++

        node := g.GetNode(current)
        if node == nil {
            continue
        }

        for _, neighbor := range node.OutLinks {
            if visited[neighbor] {
                continue
            }

            parent[neighbor] = current

            if neighbor == to {
                return PathResult{
                    Found:    true,
                    Path:     reconstructPath(parent, from, to),
                    Hops:     len(reconstructPath(parent, from, to)) - 1,
                    Explored: explored,
                }
            }

            visited[neighbor] = true
            queue = append(queue, neighbor)
        }
    }

    return PathResult{Found: false, Explored: explored}
}
```

**Time Complexity**: O(V + E) where V = nodes, E = edges

**When to use**: Small to medium graphs, or when the target is expected to be close.

---

### Bidirectional BFS

Search from both ends simultaneously, meeting in the middle.

```go
func (g *Graph) FindPathBidirectional(from, to string) PathResult {
    if from == to {
        return PathResult{Found: true, Path: []string{from}, Hops: 0}
    }

    // Forward search state
    visitedF := map[string]bool{from: true}
    parentF := map[string]string{}
    queueF := []string{from}

    // Backward search state
    visitedB := map[string]bool{to: true}
    parentB := map[string]string{}
    queueB := []string{to}

    explored := 0

    for len(queueF) > 0 && len(queueB) > 0 {
        // Expand forward frontier
        if meeting := expandFrontier(g, queueF, visitedF, parentF, visitedB, true); meeting != "" {
            return buildBidirectionalPath(parentF, parentB, from, to, meeting, explored)
        }

        // Expand backward frontier
        if meeting := expandFrontier(g, queueB, visitedB, parentB, visitedF, false); meeting != "" {
            return buildBidirectionalPath(parentF, parentB, from, to, meeting, explored)
        }
    }

    return PathResult{Found: false, Explored: explored}
}
```

**Time Complexity**: O(b^(d/2)) where b = branching factor, d = path length

**Speedup**: For a graph with branching factor 100 and path length 6:
- BFS: 100^6 = 1 trillion nodes (worst case)
- Bidirectional: 2 * 100^3 = 2 million nodes (worst case)

**When to use**: Large graphs, or when path length is expected to be > 4.

---

### Algorithm Selection

```go
func (g *Graph) FindPathSmart(from, to string, maxDepth int) PathResult {
    nodeCount := g.NodeCount()

    // For small graphs, simple BFS is fine
    if nodeCount < 10000 {
        return g.FindPathWithLimit(from, to, maxDepth)
    }

    // For large graphs, use bidirectional
    return g.FindPathBidirectional(from, to)
}
```

---

## Testing

### Test Cases

```go
func TestGraph_FindPath(t *testing.T) {
    tests := []struct {
        name     string
        edges    [][2]string // [from, to] pairs
        from     string
        to       string
        wantPath []string
        wantHops int
    }{
        {
            name:     "direct link",
            edges:    [][2]string{{"A", "B"}},
            from:     "A",
            to:       "B",
            wantPath: []string{"A", "B"},
            wantHops: 1,
        },
        {
            name:     "two hops",
            edges:    [][2]string{{"A", "B"}, {"B", "C"}},
            from:     "A",
            to:       "C",
            wantPath: []string{"A", "B", "C"},
            wantHops: 2,
        },
        {
            name:     "same node",
            edges:    [][2]string{{"A", "B"}},
            from:     "A",
            to:       "A",
            wantPath: []string{"A"},
            wantHops: 0,
        },
        {
            name:     "no path",
            edges:    [][2]string{{"A", "B"}, {"C", "D"}},
            from:     "A",
            to:       "D",
            wantPath: nil,
            wantHops: 0,
        },
        {
            name:     "shortest of multiple paths",
            edges:    [][2]string{{"A", "B"}, {"B", "C"}, {"A", "C"}},
            from:     "A",
            to:       "C",
            wantPath: []string{"A", "C"}, // Direct path, not via B
            wantHops: 1,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            g := graph.New()
            for _, edge := range tt.edges {
                g.AddEdge(edge[0], edge[1])
            }

            result := g.FindPath(tt.from, tt.to)

            if tt.wantPath == nil {
                assert.False(t, result.Found)
            } else {
                assert.True(t, result.Found)
                assert.Equal(t, tt.wantPath, result.Path)
                assert.Equal(t, tt.wantHops, result.Hops)
            }
        })
    }
}

func TestGraph_FindPathBidirectional(t *testing.T) {
    // Same test cases as above, verifying bidirectional gives same results
}

func TestGraph_LoadFromCache(t *testing.T) {
    // Test loading graph from SQLite cache
}
```

### Benchmarks

```go
func BenchmarkFindPath_SmallGraph(b *testing.B) {
    g := buildTestGraph(100, 500) // 100 nodes, 500 edges
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        g.FindPath("node_0", "node_99")
    }
}

func BenchmarkFindPath_LargeGraph(b *testing.B) {
    g := buildTestGraph(10000, 100000)
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        g.FindPath("node_0", "node_9999")
    }
}

func BenchmarkFindPathBidirectional_LargeGraph(b *testing.B) {
    g := buildTestGraph(10000, 100000)
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        g.FindPathBidirectional("node_0", "node_9999")
    }
}
```

---

## Performance

### Memory Estimates

For a graph with N pages and E links:

| Component | Memory |
|-----------|--------|
| Node map | ~50 bytes per node |
| Node struct | ~80 bytes per node |
| Edge storage | ~16 bytes per edge (slice pointers) |
| Title strings | ~30 bytes average per title |

**Example**: 10,000 pages with 500,000 links
- Nodes: 10,000 * 130 bytes = 1.3 MB
- Edges: 500,000 * 16 bytes = 8 MB
- Titles: 10,000 * 30 bytes = 0.3 MB
- **Total**: ~10 MB

### Load Time

Loading from SQLite is I/O bound:

| Pages | Links | Load Time (SSD) |
|-------|-------|-----------------|
| 1,000 | 50,000 | ~100ms |
| 10,000 | 500,000 | ~1s |
| 100,000 | 5,000,000 | ~10s |

### Pathfinding Time

| Graph Size | BFS (worst) | Bidirectional (worst) |
|------------|-------------|----------------------|
| 1,000 nodes | 5ms | 3ms |
| 10,000 nodes | 50ms | 15ms |
| 100,000 nodes | 500ms | 50ms |

---

## Implementation Order

1. **Graph data structure** (`graph.go`)
   - Node and Graph types
   - AddNode, AddEdge, GetNode methods
   - Thread-safe with RWMutex

2. **Graph loader** (`loader.go`)
   - Load from cache
   - Handle redirects (resolve to canonical titles)
   - Skip error/not_found pages

3. **BFS pathfinder** (`pathfinder.go`)
   - Basic BFS implementation
   - Path reconstruction
   - Depth limit support

4. **Path CLI command** (`cmd/wikigraph/path.go`)
   - Parse arguments
   - Load graph
   - Run pathfinder
   - Format output

5. **Bidirectional search** (stretch goal)
   - Forward and backward frontiers
   - Meeting point detection
   - Path reconstruction from both directions

6. **Tests and benchmarks**
   - Unit tests for all components
   - Integration test with real data
   - Performance benchmarks

---

## Edge Cases

| Case | Handling |
|------|----------|
| Same source and target | Return single-node path with 0 hops |
| Source not in database | Error: "page not found: <title>" |
| Target not in database | Error: "page not found: <title>" |
| No path exists | Return `{Found: false}` |
| Redirect pages | Resolve to canonical title before search |
| Cycle in path | BFS naturally handles (visited set) |
| Very long paths | Respect `--max-depth` limit |

---

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `internal/cache` | - | Read pages and links from SQLite |
| `internal/database` | - | Database connection |

No new external dependencies required.

---

## Checklist

- [x] Graph data structure implemented
- [x] Graph loader from cache
- [x] BFS pathfinding working
- [x] Path CLI command
- [x] Bidirectional search (stretch goal)
- [x] Unit tests passing
- [x] Benchmarks written
- [x] Documentation updated

---

## Next Steps (Phase 3)

Phase 2 provides graph navigation. Phase 3 will add:

- Python embeddings microservice
- Semantic similarity search
- `similar` CLI command

See [Phase 3 Documentation](./phase3-embeddings.md) for details.
