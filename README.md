# WikiGraph

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A tool that crawls Wikipedia, builds a knowledge graph of connections between people, places, and events, and lets users explore relationships through pathfinding and semantic similarity.

## Status

Under development - Currently in Phase 0 (Setup)

## Features

- **Wikipedia Scraping**: Fetch and parse Wikipedia pages with intelligent caching
- **Knowledge Graph**: Build a navigable graph of Wikipedia's internal links
- **Pathfinding**: Find the shortest path between any two Wikipedia articles
- **Semantic Similarity**: Discover related pages using word embeddings
- **Interactive Visualization**: Explore the graph through a web interface
- **REST API**: Programmatic access to all functionality

---

## Quick Start

### Prerequisites

- Go 1.22+
- Python 3.10+ (for embeddings service, coming in Phase 3)
- Docker & Docker Compose (optional, for containerized setup - coming later)

### Building

**Windows (PowerShell):**
```powershell
.\scripts\build.ps1 build
```

**Linux/macOS:**
```bash
make -f scripts/Makefile build
```

Or use Go directly:
```bash
go build -o wikigraph ./cmd/server
```

### Running

**Windows (PowerShell):**
```powershell
.\scripts\build.ps1 run
```

**Linux/macOS:**
```bash
make -f scripts/Makefile run
```

Or use Go directly:
```bash
go run ./cmd/server
```

Currently, the application prints "WikiGraph starting..." and exits. Full functionality will be added in subsequent phases.

---

## Usage

### CLI Commands (Planned)

```bash
# Fetch a Wikipedia page and display its links
wikigraph fetch "Albert Einstein"

# Crawl Wikipedia starting from a page
wikigraph crawl "Albert Einstein" --depth=2 --max-pages=500

# Find the shortest path between two pages
wikigraph path "Albert Einstein" "Barack Obama"

# Find semantically similar pages
wikigraph similar "World War II"

# Start the API server
wikigraph serve --port=8080

# View cache statistics
wikigraph cache stats

# Clear the cache
wikigraph cache clear
```

### API Endpoints (Planned)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/page/:title` | GET | Fetch a page and its links |
| `/path` | GET | Find shortest path between pages |
| `/connections/:title` | GET | Get N-hop neighborhood |
| `/similar/:title` | GET | Find semantically similar pages |
| `/crawl` | POST | Trigger a background crawl |

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      Frontend                           │
│           (Cytoscape.js graph visualization)            │
└─────────────────────┬───────────────────────────────────┘
                      │ HTTP
┌─────────────────────▼───────────────────────────────────┐
│                    Go API (Gin)                         │
│   /page  /path  /connections  /similar  /crawl          │
└─────────────────────┬───────────────────────────────────┘
                      │
        ┌─────────────┼─────────────┐
        ▼             ▼             ▼
┌──────────────┐ ┌─────────┐ ┌─────────────────┐
│   Scraper    │ │  Graph  │ │   Embeddings    │
│   (Colly)    │ │ (BFS/   │ │   Go Client     │
│              │ │  DFS)   │ │                 │
└──────┬───────┘ └────┬────┘ └────────┬────────┘
       │              │               │
       └──────────────┼───────────────┘
                      ▼
            ┌──────────────────┐
            │     SQLite       │
            │  pages / links / │
            │   embeddings     │
            └──────────────────┘
                      │
                      ▼
            ┌──────────────────┐
            │  Python Service  │
            │  (FastAPI +      │
            │  sentence-       │
            │  transformers)   │
            └──────────────────┘
```

---

## Project Structure

```
wikigraph/
├── cmd/
│   └── wikigraph/
│       └── main.go           # CLI entry point (Cobra)
├── internal/
│   ├── api/                  # HTTP handlers and routing (Gin)
│   ├── cache/                # SQLite caching layer
│   ├── config/               # Configuration management (Viper)
│   ├── scraper/              # Wikipedia scraping and parsing (Colly)
│   ├── graph/                # Graph construction and algorithms
│   └── embeddings/           # Embeddings client
├── pkg/
│   └── wikipedia/            # Wikipedia-specific utilities
├── python/
│   ├── main.py               # Embeddings microservice
│   └── requirements.txt
├── migrations/               # Database migrations
├── web/                      # Frontend assets
├── scripts/                  # Build scripts (Makefile, build.ps1)
├── wikigraph-project-plan.md # Development roadmap
├── go.mod
└── README.md
```

---

## Configuration

### Environment Variables (Planned)

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_DB_PATH` | `./data/wikigraph.db` | SQLite database path |
| `WIKIGRAPH_PORT` | `8080` | API server port |
| `WIKIGRAPH_RATE_LIMIT` | `1s` | Delay between Wikipedia requests |
| `WIKIGRAPH_CACHE_TTL` | `168h` | Cache time-to-live (7 days) |
| `WIKIGRAPH_EMBEDDINGS_URL` | `http://localhost:8001` | Embeddings service URL |

---

## Development

### Prerequisites

```bash
# Install development tools (optional)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/air-verse/air@latest
```

### Commands

**Windows (PowerShell):**
```powershell
# Build
.\scripts\build.ps1 build

# Run
.\scripts\build.ps1 run

# Test
.\scripts\build.ps1 test

# Clean
.\scripts\build.ps1 clean
```

**Linux/macOS:**
```bash
# Build
make -f scripts/Makefile build

# Run
make -f scripts/Makefile run

# Test
make -f scripts/Makefile test

# Clean
make -f scripts/Makefile clean
```

### Running Tests

```bash
# Unit tests
go test ./...

# With coverage
go test -cover ./...

# Integration tests (requires network)
go test -tags=integration ./...

# Benchmarks
go test -bench=. ./internal/scraper/
```

---

## Roadmap

- [x] **Phase 0**: Project Setup
- [ ] **Phase 1**: Scraper & Cache
- [ ] **Phase 2**: Graph Construction & Pathfinding
- [ ] **Phase 3**: Embeddings Microservice
- [ ] **Phase 4**: REST API
- [ ] **Phase 5**: Frontend Visualization
- [ ] **Phase 6**: Polish & Deployment

See [wikigraph-project-plan.md](wikigraph-project-plan.md) for detailed timeline and progress tracking.

---

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.22+ |
| CLI Framework | Cobra |
| Configuration | Viper |
| Logging | slog (stdlib) |
| Web Framework | Gin |
| Web Scraping | Colly |
| Database | SQLite (modernc.org/sqlite) |
| Embeddings | Python, FastAPI, sentence-transformers |
| Frontend | Cytoscape.js |
| Containerization | Docker |

---

## License

This project is licensed under the MIT License - see [LICENSE](LICENSE) for details.
