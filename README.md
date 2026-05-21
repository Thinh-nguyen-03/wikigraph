# WikiGraph

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A high-performance Wikipedia knowledge graph tool that crawls Wikipedia, builds a graph of page connections, and provides fast pathfinding and exploration via a REST API.

## Status

**Phase 4 Complete** — REST API server with production-grade middleware, background graph loading, and Neo4j integration.

**Scale tested:** 162M edges (5.6M pages). At this size the in-memory approach hits memory and startup limits. Neo4j is integrated as an optional backend that eliminates those limits — see [Graph Database Migration Plan](docs/graph-database-migration.md).

---

## Features

**Implemented**
- **Wikipedia crawling** — Concurrent fetching with 30 workers, intelligent deduplication, rate limiting
- **In-memory graph** — Pointer-based adjacency list with O(1) node lookup and O(1) duplicate detection
- **Disk cache** — Gob-encoded graph cache for sub-2-second warm starts
- **BFS & bidirectional pathfinding** — Context-aware traversal with configurable depth limits
- **Background loading** — Server starts in <500ms; graph loads asynchronously with 503 during warm-up
- **Incremental refresh** — Automatic graph update every 5 minutes without downtime
- **REST API** — Versioned endpoints with health check, per-IP rate limiting, request tracing, CORS, graceful shutdown
- **Neo4j backend** — Optional graph database for path and page queries; selected per-request via `?backend=`

**Planned**
- **Web UI** — Interactive graph visualization (Phase 5)

---

## Quick Start

### Prerequisites

- Go 1.22+
- Neo4j 5.x (optional — only needed for the Neo4j backend)

### Installation

```bash
git clone https://github.com/Thinh-nguyen-03/wikigraph.git
cd wikigraph
go build -o wikigraph.exe ./cmd/wikigraph
```

### First Run

```bash
# 1. Crawl some Wikipedia pages
wikigraph fetch "Go (programming language)" --depth 1 --max-pages 100

# 2. Start the API server (graph loads from disk cache in the background)
wikigraph serve
```

The server accepts requests immediately. Path queries return 503 with a `Retry-After` header until the graph finishes loading. Subsequent starts use the disk cache and are ready in under 2 seconds.

---

## CLI Commands

```bash
# Crawl Wikipedia from a seed page
wikigraph fetch "Albert Einstein" --depth 2 --max-pages 500

# Find shortest path (uses in-memory graph by default)
wikigraph path "Albert Einstein" "Barack Obama"
wikigraph path "Cat" "Philosophy" --algorithm bidirectional
wikigraph path "Physics" "Mathematics" --max-depth 10 --format json

# Show database statistics
wikigraph stats

# Start API server
wikigraph serve
wikigraph serve --port 3000 --host 127.0.0.1
wikigraph serve --rebuild-cache   # force graph rebuild
wikigraph serve --production      # structured logging, strict CORS

# Rebuild query planner statistics (run after large imports)
wikigraph analyze

# Sync SQLite data to Neo4j
wikigraph sync
wikigraph sync --clear            # wipe Neo4j first
```

---

## REST API

Base URL: `http://localhost:8080`

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Server and graph status |
| GET | `/api/v1/page/:title` | Page metadata and links |
| GET | `/api/v1/path` | Shortest path between two pages |
| GET | `/api/v1/connections/:title` | N-hop neighborhood subgraph |
| POST | `/api/v1/crawl` | Start a background crawl job |
| GET | `/api/v1/crawl/:id` | Poll a crawl job's status |

Underscores in `:title` and query params are normalized to spaces, so `Albert_Einstein` and `Albert Einstein` resolve to the same page.

### Example Requests

```bash
# Health check
curl http://localhost:8080/health
# {
#   "status": "healthy",
#   "version": "1.0.0",
#   "graph": {"nodes": 5600000, "edges": 162000000},
#   "graph_ready": true,
#   "neo4j": {"enabled": true, "connected": true, "nodes": 5600000, "edges": 162000000}
# }

# Page info
curl http://localhost:8080/api/v1/page/Albert_Einstein

# Shortest path (auto-selects Neo4j if available, falls back to in-memory)
curl "http://localhost:8080/api/v1/path?from=Albert_Einstein&to=Physics"

# Force a specific backend
curl "http://localhost:8080/api/v1/path?from=Albert_Einstein&to=Physics&backend=neo4j"
curl "http://localhost:8080/api/v1/path?from=Albert_Einstein&to=Physics&backend=memory"

# Bidirectional search
curl "http://localhost:8080/api/v1/path?from=Cat&to=Philosophy&algorithm=bidirectional"

# 2-hop neighborhood
curl "http://localhost:8080/api/v1/connections/Physics?depth=2&max_nodes=100"

# Start crawl job
curl -X POST http://localhost:8080/api/v1/crawl \
  -H "Content-Type: application/json" \
  -d '{"title": "Mathematics", "depth": 2, "max_pages": 1000}'
# {"job_id": "crawl_abc12345", "status": "started", "message": "..."}

# Poll job status
curl http://localhost:8080/api/v1/crawl/crawl_abc12345
```

Full API documentation: [docs/api-reference.md](docs/api-reference.md)

---

## Architecture

```
                      Wikipedia API
                           ^
                           |
  wikigraph fetch    ┌─────┴──────┐
  ─────────────────► │  Scraper   │ (30 concurrent workers)
                     └─────┬──────┘
                           │
                     ┌─────▼──────┐
                     │   Cache    │ ──► SQLite (pages, links)
                     └─────┬──────┘
                           │
               ┌───────────┴───────────┐
               │                       │
        ┌──────▼──────┐        ┌───────▼──────┐
        │ GraphService │        │   neostore   │ (optional)
        │  (in-memory) │        │   Neo4j DB   │
        └──────┬──────┘        └───────┬──────┘
               │                       │
               └───────────┬───────────┘
                           │
                     ┌─────▼──────┐
                     │  REST API  │ ──► ?backend=auto|neo4j|memory
                     └────────────┘
```

### Key Components

| Package | Purpose |
|---------|---------|
| `internal/scraper` | BFS crawl orchestration with worker pool |
| `internal/cache` | SQLite repository — covering indexes, bulk inserts, atomic link replacement |
| `internal/graph` | In-memory adjacency list; O(1) AddEdge, BFS/bidirectional pathfinder |
| `internal/graph/loader` | Bulk-loads graph from SQLite; manages disk cache (gob) |
| `internal/api` | Gin HTTP server — middleware, handlers, GraphService lifecycle |
| `internal/neostore` | Neo4j client — pathfinding, page queries, SQLite→Neo4j sync |
| `internal/config` | Viper-based configuration with environment variable overrides |

---

## Configuration

Priority order: environment variables > `config.yaml` > defaults.

```yaml
database:
  path: wikigraph.db

scraper:
  rate_limit: 100.0        # requests/sec to Wikipedia
  max_concurrent: 30       # parallel worker count
  max_depth: 3
  request_timeout: 30s
  user_agent: "WikiGraph/1.0"

api:
  host: 0.0.0.0
  port: 8080
  enable_cors: true
  cors_origins: ["*"]
  rate_limit: 100.0        # requests/sec per IP
  rate_burst: 200
  read_timeout: 30s
  production: false

graph:
  cache_path: ""           # defaults to graph.cache beside the database
  max_cache_age: 24h
  refresh_interval: 5m
  force_rebuild: false

neo4j:
  enabled: false           # set to true to use Neo4j as query backend
  uri: bolt://localhost:7687
  username: neo4j
  password: wikigraph
  max_connection_pool_size: 50
  sync_interval: 5m
  sync_batch_size: 10000

log:
  level: info
```

Environment variables use the prefix `WIKIGRAPH_` with dots replaced by underscores:

```bash
export WIKIGRAPH_DATABASE_PATH=./wikigraph.db
export WIKIGRAPH_API_PORT=8080
export WIKIGRAPH_NEO4J_ENABLED=true
export WIKIGRAPH_NEO4J_URI=bolt://neo4j:7687
export WIKIGRAPH_NEO4J_PASSWORD=secret
```

Full reference: [docs/configuration-reference.md](docs/configuration-reference.md)

---

## Performance

Benchmarks on consumer hardware (16 GB RAM) with a 10M-link database:

| Operation | Time |
|-----------|------|
| Warm server start (cache hit) | < 2s |
| First HTTP response | < 500ms |
| In-memory path search | < 50ms |
| In-memory bidirectional search | < 20ms |
| Neo4j path search | < 20ms |

### Scalability (in-memory backend)

| Graph size | Startup | RAM | Path search |
|------------|---------|-----|-------------|
| 1M links | < 1s | ~200 MB | < 10ms |
| 10M links | < 2s | ~1 GB | < 50ms |
| 100M links | 15 min load / 2s from cache | ~8 GB | < 100ms |

For graphs over ~10M edges, Neo4j provides lower memory usage and instant startup.

---

## Development

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Single package
go test ./tests/graph/
go test ./tests/api/

# Benchmarks
go test -bench=. ./tests/graph/

# Race detector
go test -race ./...
```

Tests live in `tests/<pkg>/` as external `package <pkg>_test` packages (black-box style).

### Building

```bash
# Development
go build -o wikigraph.exe ./cmd/wikigraph

# Production (stripped symbols)
go build -ldflags="-s -w" -o wikigraph.exe ./cmd/wikigraph
```

### Code Quality

```bash
go fmt ./...
go vet ./...
golangci-lint run
```

---

## Project Structure

```
wikigraph/
├── cmd/wikigraph/
│   ├── main.go           # CLI entry point (Cobra)
│   ├── fetch.go          # wikigraph fetch
│   ├── path.go           # wikigraph path
│   ├── serve.go          # wikigraph serve
│   ├── stats.go          # wikigraph stats
│   ├── sync.go           # wikigraph sync (SQLite → Neo4j)
│   └── analyze.go        # wikigraph analyze
├── internal/
│   ├── api/
│   │   ├── server.go         # Server setup and lifecycle
│   │   ├── handlers.go       # Route handlers
│   │   ├── graph_service.go  # Background graph management
│   │   ├── router.go         # Route registration
│   │   ├── types.go          # Request/response types
│   │   ├── errors.go         # Error helpers
│   │   ├── config.go         # API config defaults
│   │   └── middleware/       # Recovery, logging, rate limit, CORS, timeout, request ID
│   ├── cache/                # SQLite repository (CRUD, bulk inserts)
│   ├── config/               # Viper config with Neo4j, Graph, API sections
│   ├── database/
│   │   ├── database.go       # Connection, pragmas, migrations
│   │   └── migrations/       # SQL migrations 001–005
│   ├── fetcher/              # Colly-based Wikipedia HTTP client
│   ├── graph/
│   │   ├── graph.go          # Adjacency list, O(1) AddEdge/dedup
│   │   ├── loader.go         # SQLite → Graph bulk loader + disk cache
│   │   ├── pathfinder.go     # BFS and bidirectional search (context-aware)
│   │   └── persistence.go    # Gob serialization
│   ├── neostore/
│   │   ├── client.go         # Neo4j driver wrapper and health
│   │   ├── queries.go        # Cypher pathfinding and page queries
│   │   └── sync.go           # SQLite → Neo4j batch sync
│   ├── parser/               # Wikipedia HTML link extraction
│   └── scraper/              # BFS crawl orchestration
├── tests/                    # External black-box tests (package <pkg>_test)
│   ├── api/                  # Handler and middleware tests
│   ├── cache/
│   ├── config/
│   ├── database/
│   ├── fetcher/
│   ├── graph/                # Graph, pathfinder, and loader tests
│   ├── parser/
│   └── scraper/
├── scripts/
│   ├── setup/                # Neo4j index setup script
│   ├── perf/                 # In-memory performance benchmarks
│   └── neo4j_perf/           # Neo4j performance benchmarks
├── docs/                     # Documentation
├── config.yaml               # Default configuration
├── go.mod
└── go.sum
```

---

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.22+ |
| CLI | Cobra |
| Configuration | Viper |
| HTTP | Gin |
| Web scraping | Colly |
| HTML parsing | goquery |
| Primary store | SQLite (modernc.org/sqlite) |
| Graph backend | Neo4j 5.x (optional, via neo4j-go-driver) |
| Graph cache | encoding/gob |
| Logging | log/slog (stdlib) |

---

## Documentation

- [API Reference](docs/api-reference.md)
- [Configuration Reference](docs/configuration-reference.md)
- [Database Schema](docs/database-schema.md)
- [Technical Optimizations](docs/technical-optimizations.md)
- [Neo4j Setup](docs/neo4j-setup.md)
- [Graph Database Migration Plan](docs/graph-database-migration.md)
- [Deployment Guide](docs/deployment-guide.md)

---

## License

MIT — see [LICENSE](LICENSE).
