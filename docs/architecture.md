# Architecture Documentation

This document describes the architecture of WikiGraph, including system design, data flow, and component interactions.

---

## System Overview

WikiGraph is a distributed system consisting of:

1. **Go API Service** - Main backend handling HTTP requests, scraping, graph operations
2. **Python Embeddings Service** - ML microservice for semantic similarity
3. **SQLite Database** - Persistent storage for pages, links, and embeddings
4. **Frontend** - Web-based visualization interface

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Clients                                     │
│                    (Web Browser, CLI, API Consumers)                     │
└─────────────────────────────────────┬───────────────────────────────────┘
                                      │ HTTPS
                                      ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           Load Balancer                                  │
│                        (nginx / Caddy / Cloud)                          │
└─────────────────────────────────────┬───────────────────────────────────┘
                                      │
                    ┌─────────────────┴─────────────────┐
                    │                                   │
                    ▼                                   ▼
┌───────────────────────────────┐     ┌───────────────────────────────────┐
│        Go API Service         │     │         Static Frontend           │
│         (Port 8080)           │     │          (Port 3000)              │
│                               │     │                                   │
│  ┌─────────────────────────┐  │     │  ┌─────────────────────────────┐  │
│  │      HTTP Router        │  │     │  │       Cytoscape.js          │  │
│  │        (Gin)            │  │     │  │    Graph Visualization      │  │
│  └───────────┬─────────────┘  │     │  └─────────────────────────────┘  │
│              │                │     │                                   │
│  ┌───────────┴─────────────┐  │     └───────────────────────────────────┘
│  │      API Handlers       │  │
│  │  /page /path /similar   │  │
│  └───────────┬─────────────┘  │
│              │                │
│  ┌───────────┴─────────────┐  │
│  │    Business Logic       │  │
│  │  Scraper, Graph, Cache  │  │
│  └───────────┬─────────────┘  │
│              │                │
└──────────────┼────────────────┘
               │
    ┌──────────┴──────────┐
    │                     │
    ▼                     ▼
┌─────────────┐    ┌─────────────────────┐
│   SQLite    │    │  Embeddings Service │
│  Database   │    │     (Port 8001)     │
│             │    │                     │
│ ┌─────────┐ │    │ ┌─────────────────┐ │
│ │  pages  │ │    │ │    FastAPI      │ │
│ ├─────────┤ │    │ ├─────────────────┤ │
│ │  links  │ │    │ │   sentence-     │ │
│ ├─────────┤ │    │ │  transformers   │ │
│ │embeddings│ │    │ └─────────────────┘ │
│ └─────────┘ │    │                     │
└─────────────┘    └─────────────────────┘
```

---

## Component Architecture

### Go API Service

```
cmd/wikigraph/
└── main.go                 # CLI entry point (Cobra)

internal/
├── api/                    # HTTP layer (Gin)
│   ├── router.go          # Route definitions
│   ├── handlers.go        # Request handlers
│   ├── middleware.go      # Auth, logging, rate limiting
│   └── responses.go       # Response formatting
│
├── scraper/               # Wikipedia scraping (Colly)
│   ├── scraper.go        # Main scraper logic
│   ├── fetcher.go        # HTTP client
│   ├── parser.go         # HTML parsing
│   └── ratelimit.go      # Rate limiting
│
├── cache/                 # Caching layer
│   ├── cache.go          # Cache interface
│   ├── sqlite.go         # SQLite implementation (modernc.org/sqlite)
│   └── memory.go         # In-memory cache (optional)
│
├── graph/                 # Graph algorithms
│   ├── graph.go          # Graph data structure
│   ├── bfs.go            # Breadth-first search
│   ├── pathfinding.go    # Path finding algorithms
│   └── crawler.go        # Multi-page crawler
│
├── embeddings/           # Embeddings client
│   ├── client.go         # HTTP client for embeddings service
│   └── similarity.go     # Similarity calculations
│
└── config/               # Configuration (Viper)
    ├── config.go         # Config struct
    └── loader.go         # Config loading
```

### Dependency Injection

```go
// main.go (using Cobra + Viper + slog)
func main() {
    // Load configuration (Viper)
    cfg := config.Load()
    
    // Initialize database
    db := database.New(cfg.Database)
    
    // Initialize cache
    cache := cache.New(db, cfg.Cache)
    
    // Initialize scraper
    scraper := scraper.New(
        scraper.WithCache(cache),
        scraper.WithRateLimit(cfg.Scraper.RateLimit),
    )
    
    // Initialize graph
    graph := graph.New(db)
    
    // Initialize embeddings client
    embeddings := embeddings.NewClient(cfg.Embeddings.URL)
    
    // Initialize API
    api := api.New(
        api.WithScraper(scraper),
        api.WithGraph(graph),
        api.WithEmbeddings(embeddings),
    )
    
    // Start server
    api.Serve(cfg.Server.Port)
}
```

---

## Data Flow

### Page Fetch Flow

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│  Client  │────▶│   API    │────▶│  Cache   │────▶│  SQLite  │
└──────────┘     └──────────┘     └──────────┘     └──────────┘
                      │                 │
                      │ Cache Miss      │
                      ▼                 │
                ┌──────────┐            │
                │ Scraper  │            │
                └──────────┘            │
                      │                 │
                      │ Fetch           │
                      ▼                 │
                ┌──────────┐            │
                │Wikipedia │            │
                └──────────┘            │
                      │                 │
                      │ HTML            │
                      ▼                 │
                ┌──────────┐            │
                │  Parser  │────────────┘
                └──────────┘      Store
```

**Sequence:**

1. Client requests `/page/Albert_Einstein`
2. API handler calls Cache
3. Cache checks SQLite for existing entry
4. **Cache Hit**: Return cached data
5. **Cache Miss**: 
   - Scraper fetches from Wikipedia
   - Parser extracts links from HTML
   - Cache stores result in SQLite
   - Return data to client

### Pathfinding Flow

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│  Client  │────▶│   API    │────▶│  Graph   │
└──────────┘     └──────────┘     └──────────┘
                                       │
                      ┌────────────────┼────────────────┐
                      │                │                │
                      ▼                ▼                ▼
                ┌──────────┐    ┌──────────┐    ┌──────────┐
                │   BFS    │    │  Cache   │    │ Scraper  │
                │  Queue   │◀──▶│  Lookup  │◀──▶│ (on miss)│
                └──────────┘    └──────────┘    └──────────┘
```

**Algorithm:**

1. Initialize BFS from source page
2. For each page in queue:
   - Check if target found
   - Get outgoing links (from cache or fetch)
   - Add unvisited links to queue
3. Return path when target found
4. Return "not found" if max depth exceeded

### Similarity Search Flow

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│  Client  │────▶│   API    │────▶│Embeddings│────▶│  Python  │
└──────────┘     └──────────┘     │  Client  │     │ Service  │
                                  └──────────┘     └──────────┘
                                       │                │
                                       │                │
                                       ▼                ▼
                                  ┌──────────┐    ┌──────────┐
                                  │  SQLite  │    │  Model   │
                                  │(vectors) │    │(MiniLM)  │
                                  └──────────┘    └──────────┘
```

---

## Database Schema

### Entity Relationship Diagram

```
┌─────────────────────────────────┐
│            pages                │
├─────────────────────────────────┤
│ id          INTEGER PK          │
│ title       TEXT UNIQUE         │
│ content_hash TEXT               │
│ fetched_at  TIMESTAMP           │
│ created_at  TIMESTAMP           │
└───────────────┬─────────────────┘
                │
                │ 1:N
                │
┌───────────────┴─────────────────┐
│            links                │
├─────────────────────────────────┤
│ id           INTEGER PK         │
│ source_id    INTEGER FK         │
│ target_title TEXT               │
│ anchor_text  TEXT               │
│ created_at   TIMESTAMP          │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│          embeddings             │
├─────────────────────────────────┤
│ id          INTEGER PK          │
│ page_id     INTEGER FK UNIQUE   │
│ vector      BLOB                │
│ model       TEXT                │
│ created_at  TIMESTAMP           │
└─────────────────────────────────┘
```

---

## Caching Strategy

### Multi-Layer Cache

```
┌─────────────────────────────────────────────────────────────┐
│                     Request                                  │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                  L1: In-Memory Cache                         │
│                  (LRU, 1000 entries)                         │
│                  TTL: 5 minutes                              │
└─────────────────────────┬───────────────────────────────────┘
                          │ Miss
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                  L2: SQLite Cache                            │
│                  (Persistent)                                │
│                  TTL: 7 days                                 │
└─────────────────────────┬───────────────────────────────────┘
                          │ Miss
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                  Origin: Wikipedia                           │
└─────────────────────────────────────────────────────────────┘
```

### Cache Invalidation

- **Time-based**: Entries expire after TTL
- **On-demand**: Manual invalidation via API
- **Content-based**: Invalidate if content hash changes

---

## Concurrency Model

### Scraper Concurrency

```go
// Worker pool for concurrent fetching
type Crawler struct {
    workers    int
    queue      chan string
    results    chan *PageResult
    rateLimit  *rate.Limiter
}

func (c *Crawler) Start(ctx context.Context) {
    for i := 0; i < c.workers; i++ {
        go c.worker(ctx)
    }
}

func (c *Crawler) worker(ctx context.Context) {
    for title := range c.queue {
        // Respect rate limit
        c.rateLimit.Wait(ctx)
        
        // Fetch and process
        result := c.fetch(ctx, title)
        c.results <- result
    }
}
```

### Graph Concurrency

```go
// Concurrent BFS with visited set
type BFS struct {
    visited sync.Map  // Thread-safe visited set
    queue   chan string
    found   chan []string
}
```

---

## Error Handling

### Error Types

```go
// Domain errors
var (
    ErrPageNotFound    = errors.New("page not found")
    ErrPathNotFound    = errors.New("path not found")
    ErrRateLimited     = errors.New("rate limited")
    ErrTimeout         = errors.New("operation timeout")
    ErrInvalidInput    = errors.New("invalid input")
)

// Wrap errors with context
func (s *Scraper) Fetch(ctx context.Context, title string) (*Page, error) {
    page, err := s.fetcher.Fetch(ctx, title)
    if err != nil {
        return nil, fmt.Errorf("fetching %q: %w", title, err)
    }
    return page, nil
}
```

### Error Response Format

```json
{
  "error": {
    "code": "PAGE_NOT_FOUND",
    "message": "The requested page does not exist",
    "details": {
      "title": "NonexistentPage"
    },
    "request_id": "abc123"
  }
}
```

---

## Security Considerations

### Input Validation

```go
func ValidateTitle(title string) error {
    if title == "" {
        return ErrInvalidInput
    }
    if len(title) > 256 {
        return ErrInvalidInput
    }
    if containsSQLInjection(title) {
        return ErrInvalidInput
    }
    return nil
}
```

### Rate Limiting

- Per-client rate limiting (IP-based)
- Global rate limiting for Wikipedia requests
- Exponential backoff on 429 responses

### CORS

```go
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
    return cors.New(cors.Config{
        AllowOrigins:     allowedOrigins,
        AllowMethods:     []string{"GET", "POST", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    })
}
```

---

## Scalability

### Horizontal Scaling

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    └────────┬────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│   API Node 1    │ │   API Node 2    │ │   API Node 3    │
└────────┬────────┘ └────────┬────────┘ └────────┬────────┘
         │                   │                   │
         └───────────────────┼───────────────────┘
                             │
                    ┌────────┴────────┐
                    │  Shared SQLite  │
                    │  (or PostgreSQL │
                    │   for scaling)  │
                    └─────────────────┘
```

### Database Scaling Path

1. **Current**: SQLite (single node)
2. **Medium**: SQLite with read replicas
3. **Large**: PostgreSQL with connection pooling
4. **Graph-heavy**: Neo4j for graph operations

---

## Monitoring Points

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `wikigraph_requests_total` | Counter | Total HTTP requests |
| `wikigraph_request_duration_seconds` | Histogram | Request latency |
| `wikigraph_cache_hits_total` | Counter | Cache hit count |
| `wikigraph_cache_misses_total` | Counter | Cache miss count |
| `wikigraph_pages_crawled_total` | Counter | Pages crawled |
| `wikigraph_active_crawl_jobs` | Gauge | Active crawl jobs |
| `wikigraph_db_connections` | Gauge | Database connections |

### Health Checks

```go
func HealthCheck() gin.HandlerFunc {
    return func(c *gin.Context) {
        checks := map[string]string{
            "database":   checkDatabase(),
            "embeddings": checkEmbeddings(),
        }
        
        healthy := true
        for _, status := range checks {
            if status != "ok" {
                healthy = false
            }
        }
        
        status := http.StatusOK
        if !healthy {
            status = http.StatusServiceUnavailable
        }
        
        c.JSON(status, gin.H{
            "status": checks,
            "healthy": healthy,
        })
    }
}
```

---

## Future Considerations

### Planned Improvements

1. **GraphQL API** - More flexible queries
2. **WebSocket** - Real-time crawl updates
3. **Distributed Crawling** - Multiple crawl workers
4. **ML Enhancements** - Better similarity with fine-tuned models
5. **Graph Database** - Neo4j for complex queries
