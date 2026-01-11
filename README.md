# WikiGraph

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A high-performance Wikipedia knowledge graph tool that crawls Wikipedia, builds an in-memory graph of connections, and provides fast pathfinding and exploration capabilities with production-grade performance optimizations.

## Status

**Phase 4 Complete** - API Server implemented with advanced optimizations.

**Current Scale:** Successfully tested with **162M edges** (5.6M pages). At this scale, architectural limitations of the in-memory approach have been identified. See [Graph Database Migration Plan](docs/graph-database-migration.md) for details on the transition to a dual-database architecture using Neo4j for production-scale graph queries.

**Performance at Different Scales:**
- **Small graphs (<10M edges):** Optimized startup in < 2 seconds via gob caching
- **Large graphs (>100M edges):** Migration to Neo4j recommended for sub-second startup and query performance

## Features

**Implemented**:
- **Wikipedia Scraping**: Concurrent fetching (30 workers) with intelligent caching and rate limiting
- **Knowledge Graph**: In-memory graph with persistent disk caching for instant startup
- **Blazing Fast Pathfinding**: BFS and bidirectional search with < 50ms response times
- **Background Loading**: Server starts in < 500ms, graph loads asynchronously
- **Incremental Updates**: Automatic graph refresh every 5 minutes without downtime
- **REST API**: Production-ready HTTP API with health monitoring and 503 handling
- **Interactive Visualization**: Explore N-hop neighborhoods via API
- **Performance Optimized**: Handles 10M+ links with < 2 second startup (600x improvement)

**Planned**:
- **Semantic Similarity**: Discover related pages using word embeddings (Phase 3)
- **Advanced UI**: Web interface with interactive graph visualization (Phase 5)

---

## Quick Start

### Prerequisites

- Go 1.22+
- ~1GB RAM (scales with graph size: ~100MB per 1M links)

### Installation

```bash
# Clone the repository
git clone https://github.com/Thinh-nguyen-03/wikigraph.git
cd wikigraph

# Build
go build -o wikigraph.exe ./cmd/wikigraph
```

### First Run

```bash
# 1. Fetch some Wikipedia pages to build initial graph
wikigraph fetch "Go (programming language)" --depth 1 --max-pages 100

# 2. Start the API server (graph loads in < 2 seconds from cache)
wikigraph serve

# Server output:
# Starting WikiGraph API server on http://0.0.0.0:8080
# Graph loading in background - server is immediately available
# Note: Path queries will return 503 until graph is ready
# ...
# Graph ready (loaded in 1.8s)
```

The server starts instantly (< 500ms) and responds to health checks while the graph loads in the background. Subsequent restarts are even faster thanks to disk caching.

---

## Usage

### CLI Commands

#### Fetch Wikipedia Pages

```bash
# Fetch a single page
wikigraph fetch "Albert Einstein"

# Crawl with depth and limits (concurrent with 30 workers)
wikigraph fetch "Physics" --depth 2 --max-pages 500

# Large crawl
wikigraph fetch "Computer Science" --depth 3 --max-pages 5000
```

#### Find Shortest Path

```bash
# Basic pathfinding (BFS)
wikigraph path "Albert Einstein" "Barack Obama"

# Bidirectional search (faster for distant pages)
wikigraph path "Cat" "Philosophy" --algorithm bidirectional

# Limit search depth
wikigraph path "Go (programming language)" "Python (programming language)" --max-depth 10

# JSON output
wikigraph path "Physics" "Mathematics" --format json
```

#### View Statistics

```bash
# Database and cache statistics
wikigraph stats
```

#### Start API Server

```bash
# Start with default settings (port 8080)
wikigraph serve

# Custom port and host
wikigraph serve --port 3000 --host 127.0.0.1

# Force rebuild graph cache (useful after large imports)
wikigraph serve --rebuild-cache

# Production mode (structured logging, CORS restrictions)
wikigraph serve --production
```

#### Database Maintenance

```bash
# Rebuild query planner statistics (run after large data imports)
# Recommended: weekly via cron for large databases
wikigraph analyze
```

### REST API

The REST API is available at `http://localhost:8080` when running `wikigraph serve`.

#### Core Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check and graph loading status |
| `/api/v1/page/:title` | GET | Get page and its links |
| `/api/v1/path` | GET | Find shortest path between pages |
| `/api/v1/connections/:title` | GET | Get N-hop neighborhood subgraph |
| `/api/v1/crawl` | POST | Start background crawl job |

#### Example Usage

```bash
# Health check (works even during graph loading)
curl http://localhost:8080/health
# {
#   "status": "healthy",
#   "version": "1.0.0",
#   "graph": {"nodes": 10000000, "edges": 100000000},
#   "graph_ready": true
# }

# Get a page with its links
curl http://localhost:8080/api/v1/page/Albert_Einstein

# Find shortest path with query parameters
curl "http://localhost:8080/api/v1/path?from=Albert_Einstein&to=Physics&algorithm=bidirectional"

# Get 2-hop neighborhood (up to 100 nodes)
curl "http://localhost:8080/api/v1/connections/Physics?depth=2&max_nodes=100"

# Start a background crawl job
curl -X POST http://localhost:8080/api/v1/crawl \
  -H "Content-Type: application/json" \
  -d '{"title": "Mathematics", "depth": 2, "max_pages": 1000}'
# {
#   "job_id": "crawl_abc12345",
#   "status": "started",
#   "message": "Crawl job started for 'Mathematics'"
# }
```

Full API documentation: [docs/api-reference.md](docs/api-reference.md)

---

## Architecture

### System Overview

```
┌─────────────────┐
│   CLI / API     │
└────────┬────────┘
         │
    ┌────┴────┐
    │ Scraper │ ──► Wikipedia API (concurrent workers)
    └────┬────┘
         │
    ┌────┴────┐
    │  Cache  │ ──► SQLite Database (optimized indexes)
    └────┬────┘
         │
    ┌────┴────────┐
    │ GraphService│ ──► graph.cache (gob encoding)
    │ (background)│ ──► Periodic refresh (5min)
    └────┬────────┘
         │
    ┌────┴────┐
    │  Graph  │ ──► In-memory pathfinding (< 50ms)
    │(in-RAM) │
    └─────────┘
```

### Key Components

- **Scraper**: Concurrent Wikipedia fetching with 30-worker pool pattern
- **Cache**: SQLite repository with covering indexes and bulk operations
- **GraphService**: Manages graph lifecycle (background load, incremental update, persistence)
- **Graph**: In-memory adjacency list with O(1) node lookup and O(edges) construction
- **Persistence**: Gob-encoded disk cache for sub-second startup

### Performance Features

| Feature | Implementation | Benefit |
|---------|----------------|---------|
| Graph Persistence | Gob encoding to disk | **600x faster startup** (20min → 2s) |
| Background Loading | Async graph load with goroutines | Server starts in **< 500ms** |
| Incremental Updates | Periodic refresh from DB (5min interval) | **Always fresh**, no downtime |
| Concurrent Fetching | Worker pool (30 workers) | **5-10x crawl throughput** |
| Bulk Loading | AddEdgeUnchecked without O(degree) check | **5x faster initial build** |
| Ring Buffer Queue | Custom BFS queue | Better memory efficiency |
| Slice Pre-allocation | Pre-allocated capacity | Zero reallocations |
| Single-Query Loading | JOIN instead of 2 queries | 50% fewer DB round-trips |
| Map-Based Lookup | O(1) namespace exclusion | 12x fewer comparisons |

Full optimization details: [docs/technical-optimizations.md](docs/technical-optimizations.md)

### Scale Challenges and Architectural Evolution

WikiGraph has been tested at production scale (162M edges, 5.6M nodes, ~50GB database). At this scale, fundamental architectural challenges emerged:

**Challenge 1: Startup Bottleneck**
- Database load: 15 minutes for 162M edges
- Gob serialization: 10+ minutes to save cache
- Gob deserialization: 30+ minutes or timeout

**Challenge 2: Memory Footprint**
- 12-15GB RAM required for full graph
- All-or-nothing loading (can't serve queries until 100% loaded)

**Challenge 3: Architectural Mismatch**
- SQLite optimized for OLTP, not graph traversal
- Gob serialization doesn't scale beyond ~10M edges
- Query patterns access <0.1% of graph, yet loads 100%

**Solution: Dual-Database Architecture**

After analysis, we determined that the proper solution is migrating to a specialized graph database (Neo4j) for query operations while retaining SQLite for crawler data persistence. This represents an evolution from a monolithic approach to a polyglot persistence architecture.

**Read the full analysis:** [Graph Database Migration Plan](docs/graph-database-migration.md)

This document demonstrates:
- Root cause analysis with performance measurements
- Evaluation of 5 different solutions
- Production-scale architectural decision making
- Honest assessment of when to use the right tool

---

## Project Structure

```
wikigraph/
├── cmd/
│   └── wikigraph/
│       ├── main.go           # CLI entry point (Cobra)
│       ├── root.go           # Root command and global flags
│       ├── fetch.go          # Fetch command
│       ├── path.go           # Path command
│       ├── serve.go          # API server command (background loading)
│       ├── stats.go          # Stats command
│       └── analyze.go        # ANALYZE command (manual DB optimization)
├── internal/
│   ├── api/                  # HTTP server and handlers
│   │   ├── server.go         # Server setup with GraphService
│   │   ├── handlers.go       # API handlers with 503 support
│   │   ├── graph_service.go  # Background graph management
│   │   └── types.go          # Request/response types
│   ├── cache/                # SQLite repository layer
│   │   └── cache.go          # CRUD operations, bulk inserts
│   ├── config/               # Configuration management (Viper)
│   │   └── config.go         # Config loading with GraphConfig
│   ├── database/             # SQLite wrapper
│   │   ├── database.go       # Connection management, migrations
│   │   └── migrations/       # SQL migrations (003_graph_optimization.sql)
│   ├── fetcher/              # Wikipedia HTTP client
│   │   └── fetcher.go        # Colly-based fetcher with pooling
│   ├── graph/                # Graph algorithms and persistence
│   │   ├── graph.go          # Graph data structure
│   │   ├── loader.go         # Database → Graph with caching
│   │   ├── pathfinder.go     # BFS/bidirectional search
│   │   └── persistence.go    # Disk serialization (gob)
│   ├── parser/               # HTML parsing
│   │   └── parser.go         # Link extraction with map lookups
│   └── scraper/              # Crawl orchestration
│       └── scraper.go        # BFS crawler with worker pool
├── docs/                     # Documentation
│   ├── graph-database-migration.md # Architecture decision doc (Neo4j)
│   ├── api-reference.md      # REST API endpoints
│   ├── configuration-reference.md  # Config options
│   ├── database-schema.md    # Database design
│   ├── deployment-guide.md   # Production deployment
│   └── technical-optimizations.md  # Performance details
├── go.mod
├── go.sum
└── README.md
```

---

## Configuration

WikiGraph supports configuration via:
1. Environment variables (highest priority)
2. Config file (`config.yaml`)
3. Command-line flags
4. Default values (lowest priority)

### Key Configuration Options

```yaml
# Database
database:
  path: ./wikigraph.db

# API Server
api:
  host: 0.0.0.0
  port: 8080
  enable_cors: true
  cors_origins: ["*"]
  production: false
  rate_limit: 100.0
  rate_burst: 200

# Scraper
scraper:
  rate_limit: 100.0       # requests per second
  max_depth: 3
  request_timeout: 30s
  user_agent: "WikiGraph/1.0"

# Graph Caching (Critical for performance)
graph:
  cache_path: ""              # Default: same directory as database
  max_cache_age: 24h          # Force rebuild after this age
  refresh_interval: 5m        # Check for DB updates every 5 minutes
  force_rebuild: false        # Force rebuild on startup (use --rebuild-cache flag)
```

Full configuration reference: [docs/configuration-reference.md](docs/configuration-reference.md)

### Environment Variables

```bash
# Database
export WIKIGRAPH_DATABASE_PATH=./wikigraph.db

# API Server
export WIKIGRAPH_API_PORT=8080
export WIKIGRAPH_API_HOST=0.0.0.0
export WIKIGRAPH_API_PRODUCTION=false

# Graph Cache (Important!)
export WIKIGRAPH_GRAPH_CACHE_PATH=./graph.cache
export WIKIGRAPH_GRAPH_REFRESH_INTERVAL=5m
export WIKIGRAPH_GRAPH_MAX_CACHE_AGE=24h
```

---

## Performance

### Benchmarks

Tested on consumer hardware (16GB RAM) with 10M links database:

| Operation | Time | Notes |
|-----------|------|-------|
| **Server startup (cache hit)** | **< 2s** | First run: ~20min, subsequent: instant |
| **Server first HTTP response** | **< 500ms** | HTTP available immediately |
| **Graph load from disk** | **~2s** | 10M links via gob deserialization |
| **Path search (average)** | **< 50ms** | BFS within in-memory graph |
| **Path search (bidirectional)** | **< 20ms** | For distant pages |
| **Incremental update (1000 pages)** | **< 5s** | No downtime, periodic refresh |
| **ANALYZE command** | **2-5min** | Manual, run after large imports |

### Startup Performance Improvement

| Metric | Before Optimization | After Optimization | Improvement |
|--------|--------------------|--------------------|-------------|
| Cold start | 20 minutes | 20 minutes (one-time) | - |
| Warm start | 20 minutes | **< 2 seconds** | **600x faster** |
| Server availability | After full load | **< 500ms** | **2400x faster** |

### Scalability

| Graph Size | Startup | Memory | Path Search | Cache File |
|------------|---------|--------|-------------|------------|
| 100K links | < 1s | ~50MB | < 10ms | ~5MB |
| 1M links | < 2s | ~200MB | < 30ms | ~50MB |
| 10M links | < 2s | ~1GB | < 50ms | ~500MB |
| 100M links | < 3s | ~8GB | < 100ms | ~5GB |

---

## Development

### Running Tests

```bash
# Unit tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/graph/

# Benchmarks
go test -bench=. ./internal/graph/
go test -bench=. ./internal/scraper/

# Race detection
go test -race ./...
```

### Building

```bash
# Development build
go build -o wikigraph.exe ./cmd/wikigraph

# Production build (optimized, smaller binary)
go build -ldflags="-s -w" -o wikigraph.exe ./cmd/wikigraph

# With version info
go build -ldflags="-s -w -X main.version=1.0.0" -o wikigraph.exe ./cmd/wikigraph
```

### Code Quality

```bash
# Format code
go fmt ./...

# Lint (requires golangci-lint)
golangci-lint run

# Vet
go vet ./...

# Static analysis
staticcheck ./...
```

---

## Tech Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| Language | Go 1.22+ | Core application, high performance |
| CLI Framework | Cobra | Command-line interface |
| Config | Viper | Configuration management |
| HTTP Framework | Gin | REST API server |
| Web Scraping | Colly | Wikipedia fetching with pooling |
| HTML Parsing | goquery | Link extraction |
| Database | SQLite (modernc.org/sqlite) | Persistent storage |
| Serialization | encoding/gob | Graph disk caching |
| Logging | slog (stdlib) | Structured logging |

---

## Documentation

- [**Graph Database Migration**](docs/graph-database-migration.md) - Architectural decision document for Neo4j migration
- [API Reference](docs/api-reference.md) - REST API endpoints and examples
- [Configuration Reference](docs/configuration-reference.md) - All configuration options
- [Technical Optimizations](docs/technical-optimizations.md) - Detailed performance analysis
- [Database Schema](docs/database-schema.md) - Database design and indexes
- [Deployment Guide](docs/deployment-guide.md) - Production deployment guide

---

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Write tests for new features
- Run `go fmt` and `go vet` before committing
- Update documentation for user-facing changes
- Follow existing code style and patterns

---

## License

This project is licensed under the MIT License - see [LICENSE](LICENSE) for details.

---

## Acknowledgments

- Wikipedia for providing free access to knowledge
- Go community for excellent libraries and tools
- [Colly](https://github.com/gocolly/colly) - Web scraping framework
- [Gin](https://github.com/gin-gonic/gin) - HTTP framework
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
