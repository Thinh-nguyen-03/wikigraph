# Phase 4: API Layer & Server Mode

> **Status: NOT STARTED**
>
> This phase will expose all WikiGraph functionality through a RESTful API using Gin.
> The server keeps the graph in memory for fast pathfinding, avoiding repeated database loads.
>
> Phase 3 (Embeddings) is optional - the API can be implemented without it.

## Overview

Phase 4 wraps the existing scraper, graph, and embeddings functionality in a REST API. The server loads the graph into memory once on startup, then serves path queries instantly without hitting the database repeatedly. This also prepares the foundation for the frontend in Phase 5.

---

## Table of Contents

- [What We Already Have](#what-we-already-have)
- [What Phase 4 Adds](#what-phase-4-adds)
- [Architecture](#architecture)
- [Components](#components)
  - [Server Package](#server-package)
  - [API Handlers](#api-handlers)
  - [Middleware](#middleware)
  - [Server Command](#server-command)
- [API Reference](#api-reference)
- [Implementation Details](#implementation-details)
- [Testing](#testing)
- [Server Mode Benefits](#server-mode-benefits)
- [Performance](#performance)

---

## What We Already Have

From Phases 1, 2, and (optionally) 3:

| Feature | Location | Status |
|---------|----------|--------|
| Page fetching and caching | `internal/scraper/`, `internal/cache/` | Done |
| Link storage in SQLite | `internal/cache/` | Done |
| In-memory graph construction | `internal/graph/` | Done |
| BFS and bidirectional pathfinding | `internal/graph/` | Done |
| Embeddings service (optional) | `internal/embeddings/`, `python/` | Phase 3 |

---

## What Phase 4 Adds

| Feature | Location | Description |
|---------|----------|-------------|
| HTTP server | `internal/api/` | Gin router and middleware |
| Health endpoint | `internal/api/health.go` | Service health check |
| Page endpoint | `internal/api/page.go` | Fetch page and return links |
| Path endpoint | `internal/api/path.go` | Find shortest path between pages |
| Connections endpoint | `internal/api/connections.go` | Get N-hop neighborhood |
| Similar endpoint | `internal/api/similar.go` | Semantic similarity (requires Phase 3) |
| Crawl endpoint | `internal/api/crawl.go` | Trigger background crawl |
| Request validation | `internal/api/validation.go` | Input validation middleware |
| Error handling | `internal/api/errors.go` | Consistent error responses |
| `server` CLI command | `cmd/server/main.go` | Start the API server |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP Clients                           │
│         (curl, browser, frontend, scripts)                  │
└─────────────────────────┬───────────────────────────────────┘
                          │ HTTP
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   Gin HTTP Server                           │
│                  (localhost:8080)                           │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                   Middleware                          │  │
│  │  • CORS  • Logging  • Recovery  • Validation         │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                  API Endpoints                        │  │
│  │                                                       │  │
│  │  GET  /health                                         │  │
│  │  GET  /api/v1/page/:title                            │  │
│  │  GET  /api/v1/path                                   │  │
│  │  GET  /api/v1/connections/:title                     │  │
│  │  GET  /api/v1/similar/:title      (Phase 3)          │  │
│  │  POST /api/v1/crawl                                  │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────┬───────────────────────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│   Graph      │  │   Scraper    │  │  Embeddings  │
│ (in-memory)  │  │              │  │  (optional)  │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │
       └─────────────────┼─────────────────┘
                         ▼
              ┌──────────────────┐
              │      Cache       │
              │    (SQLite)      │
              └──────────────────┘
```

### Data Flow

**Server Startup:**
1. Load configuration
2. Connect to SQLite database
3. Load graph into memory from cache
4. Initialize embeddings client (if Phase 3 completed)
5. Start HTTP server on configured port

**Request Processing:**
1. HTTP request arrives
2. Middleware validates, logs, handles CORS
3. Router dispatches to appropriate handler
4. Handler processes request using in-memory data
5. Response formatted as JSON
6. Middleware logs response

---

## Components

### Server Package

**Location**: `internal/api/`

**Files**:
- `server.go` - Server initialization and lifecycle
- `router.go` - Route definitions and middleware setup
- `health.go` - Health check handler
- `page.go` - Page fetch handler
- `path.go` - Pathfinding handler
- `connections.go` - Neighborhood handler
- `similar.go` - Similarity handler (Phase 3)
- `crawl.go` - Crawl job handler
- `validation.go` - Request validation
- `errors.go` - Error handling utilities
- `middleware.go` - Custom middleware
- `api_test.go` - Integration tests

#### Server Interface

```go
package api

import (
    "context"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/user/wikigraph/internal/cache"
    "github.com/user/wikigraph/internal/graph"
    "github.com/user/wikigraph/internal/embeddings"
)

// Server represents the HTTP API server
type Server struct {
    router      *gin.Engine
    httpServer  *http.Server
    graph       *graph.Graph
    cache       *cache.Cache
    embeddings  *embeddings.Service // nil if Phase 3 not complete
    config      Config
}

// Config holds server configuration
type Config struct {
    Host            string
    Port            int
    EnableCORS      bool
    EnableEmbeddings bool
    ReadTimeout     time.Duration
    WriteTimeout    time.Duration
}

// New creates a new API server
func New(g *graph.Graph, c *cache.Cache, emb *embeddings.Service, cfg Config) *Server

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error

// Router returns the Gin router for testing
func (s *Server) Router() *gin.Engine
```

#### Server Lifecycle

```go
// main.go or cmd/server/main.go
func main() {
    // Load configuration
    cfg := loadConfig()

    // Initialize database and cache
    db, err := database.Open(cfg.DBPath)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    cache := cache.New(db)

    // Load graph into memory
    log.Println("Loading graph into memory...")
    loader := graph.NewLoader(cache)
    g, err := loader.Load(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Loaded %d nodes and %d edges", g.NodeCount(), g.EdgeCount())

    // Initialize embeddings (if enabled)
    var embService *embeddings.Service
    if cfg.EnableEmbeddings {
        embClient := embeddings.NewClient(cfg.EmbeddingsURL)
        embStorage := embeddings.NewStorage(db)
        embService = embeddings.NewService(embClient, embStorage, cache)
    }

    // Create and start server
    server := api.New(g, cache, embService, cfg.API)

    // Handle graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
        <-sigChan

        log.Println("Shutting down server...")
        shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer shutdownCancel()

        if err := server.Shutdown(shutdownCtx); err != nil {
            log.Printf("Error during shutdown: %v", err)
        }
        cancel()
    }()

    log.Printf("Starting server on %s:%d", cfg.API.Host, cfg.API.Port)
    if err := server.Start(ctx); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}
```

---

### API Handlers

#### Health Handler

**File**: `internal/api/health.go`

```go
// GET /health
func (s *Server) handleHealth(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status": "healthy",
        "version": "1.0.0",
        "graph": gin.H{
            "nodes": s.graph.NodeCount(),
            "edges": s.graph.EdgeCount(),
        },
        "embeddings_enabled": s.embeddings != nil,
    })
}
```

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "graph": {
    "nodes": 10234,
    "edges": 523941
  },
  "embeddings_enabled": false
}
```

---

#### Page Handler

**File**: `internal/api/page.go`

```go
// GET /api/v1/page/:title
func (s *Server) handleGetPage(c *gin.Context) {
    title := c.Param("title")

    // Get from graph (in-memory)
    node := s.graph.GetNode(title)
    if node == nil {
        c.JSON(http.StatusNotFound, ErrorResponse{
            Error: "page not found",
            Message: fmt.Sprintf("Page '%s' not in database. Try fetching it first.", title),
        })
        return
    }

    // Get cached metadata
    page, err := s.cache.GetPageByTitle(c.Request.Context(), title)
    if err != nil {
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error: "database error",
            Message: err.Error(),
        })
        return
    }

    c.JSON(http.StatusOK, PageResponse{
        Title:      node.Title,
        Links:      node.OutLinks,
        LinkCount:  len(node.OutLinks),
        InLinks:    node.InLinks,
        InLinkCount: len(node.InLinks),
        FetchedAt:  page.FetchedAt,
        Cached:     true,
    })
}

type PageResponse struct {
    Title        string    `json:"title"`
    Links        []string  `json:"links"`
    LinkCount    int       `json:"link_count"`
    InLinks      []string  `json:"in_links,omitempty"`
    InLinkCount  int       `json:"in_link_count"`
    FetchedAt    time.Time `json:"fetched_at"`
    Cached       bool      `json:"cached"`
}
```

**Response:**
```json
{
  "title": "Albert Einstein",
  "links": ["Physics", "Germany", "Nobel Prize in Physics", "..."],
  "link_count": 347,
  "in_links": ["Theory of relativity", "E=mc²", "..."],
  "in_link_count": 1523,
  "fetched_at": "2024-01-15T10:30:00Z",
  "cached": true
}
```

---

#### Path Handler

**File**: `internal/api/path.go`

```go
// GET /api/v1/path?from=<title>&to=<title>&algorithm=<bfs|bidirectional>&max_depth=<n>
func (s *Server) handleFindPath(c *gin.Context) {
    from := c.Query("from")
    to := c.Query("to")
    algorithm := c.DefaultQuery("algorithm", "bfs")
    maxDepth := parseIntQuery(c, "max_depth", 6)

    if from == "" || to == "" {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error: "missing parameters",
            Message: "Both 'from' and 'to' query parameters are required",
        })
        return
    }

    start := time.Now()

    var result graph.PathResult
    switch algorithm {
    case "bidirectional":
        result = s.graph.FindPathBidirectional(from, to)
    case "bfs":
        result = s.graph.FindPathWithLimit(from, to, maxDepth)
    default:
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error: "invalid algorithm",
            Message: "Algorithm must be 'bfs' or 'bidirectional'",
        })
        return
    }

    duration := time.Since(start)

    c.JSON(http.StatusOK, PathResponse{
        Found:       result.Found,
        From:        from,
        To:          to,
        Path:        result.Path,
        Hops:        result.Hops,
        Explored:    result.Explored,
        Algorithm:   algorithm,
        DurationMs:  duration.Milliseconds(),
    })
}

type PathResponse struct {
    Found      bool     `json:"found"`
    From       string   `json:"from"`
    To         string   `json:"to"`
    Path       []string `json:"path,omitempty"`
    Hops       int      `json:"hops"`
    Explored   int      `json:"explored"`
    Algorithm  string   `json:"algorithm"`
    DurationMs int64    `json:"duration_ms"`
}
```

**Response:**
```json
{
  "found": true,
  "from": "Albert Einstein",
  "to": "Barack Obama",
  "path": ["Albert Einstein", "Princeton University", "New Jersey", "Barack Obama"],
  "hops": 3,
  "explored": 1247,
  "algorithm": "bfs",
  "duration_ms": 12
}
```

---

#### Connections Handler

**File**: `internal/api/connections.go`

```go
// GET /api/v1/connections/:title?depth=<n>&max_nodes=<n>
func (s *Server) handleGetConnections(c *gin.Context) {
    title := c.Param("title")
    depth := parseIntQuery(c, "depth", 2)
    maxNodes := parseIntQuery(c, "max_nodes", 1000)

    node := s.graph.GetNode(title)
    if node == nil {
        c.JSON(http.StatusNotFound, ErrorResponse{
            Error: "page not found",
            Message: fmt.Sprintf("Page '%s' not in database", title),
        })
        return
    }

    // BFS to find N-hop neighborhood
    subgraph := s.graph.GetNeighborhood(title, depth, maxNodes)

    c.JSON(http.StatusOK, ConnectionsResponse{
        Center:    title,
        Depth:     depth,
        Nodes:     subgraph.Nodes,
        Edges:     subgraph.Edges,
        NodeCount: len(subgraph.Nodes),
        EdgeCount: len(subgraph.Edges),
    })
}

type ConnectionsResponse struct {
    Center    string      `json:"center"`
    Depth     int         `json:"depth"`
    Nodes     []GraphNode `json:"nodes"`
    Edges     []GraphEdge `json:"edges"`
    NodeCount int         `json:"node_count"`
    EdgeCount int         `json:"edge_count"`
}

type GraphNode struct {
    ID    string `json:"id"`
    Title string `json:"title"`
    Hops  int    `json:"hops"` // Distance from center
}

type GraphEdge struct {
    Source string `json:"source"`
    Target string `json:"target"`
}
```

**Response:**
```json
{
  "center": "Albert Einstein",
  "depth": 2,
  "nodes": [
    {"id": "Albert Einstein", "title": "Albert Einstein", "hops": 0},
    {"id": "Physics", "title": "Physics", "hops": 1},
    {"id": "Isaac Newton", "title": "Isaac Newton", "hops": 2}
  ],
  "edges": [
    {"source": "Albert Einstein", "target": "Physics"},
    {"source": "Physics", "target": "Isaac Newton"}
  ],
  "node_count": 247,
  "edge_count": 1834
}
```

---

#### Similar Handler (Phase 3)

**File**: `internal/api/similar.go`

```go
// GET /api/v1/similar/:title?limit=<n>&threshold=<0-1>
func (s *Server) handleFindSimilar(c *gin.Context) {
    if s.embeddings == nil {
        c.JSON(http.StatusServiceUnavailable, ErrorResponse{
            Error: "embeddings not enabled",
            Message: "Embeddings service not configured. Complete Phase 3 first.",
        })
        return
    }

    title := c.Param("title")
    limit := parseIntQuery(c, "limit", 10)
    threshold := parseFloatQuery(c, "threshold", 0.5)

    similar, err := s.embeddings.FindSimilar(c.Request.Context(), title, limit)
    if err != nil {
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error: "similarity search failed",
            Message: err.Error(),
        })
        return
    }

    // Filter by threshold
    filtered := []embeddings.SimilarPage{}
    for _, page := range similar {
        if page.Score >= threshold {
            filtered = append(filtered, page)
        }
    }

    c.JSON(http.StatusOK, SimilarResponse{
        Query:     title,
        Similar:   filtered,
        Count:     len(filtered),
        Threshold: threshold,
    })
}

type SimilarResponse struct {
    Query     string                     `json:"query"`
    Similar   []embeddings.SimilarPage   `json:"similar"`
    Count     int                        `json:"count"`
    Threshold float64                    `json:"threshold"`
}
```

**Response:**
```json
{
  "query": "Albert Einstein",
  "similar": [
    {"title": "Isaac Newton", "score": 0.847},
    {"title": "Niels Bohr", "score": 0.823},
    {"title": "Richard Feynman", "score": 0.812}
  ],
  "count": 3,
  "threshold": 0.5
}
```

---

#### Crawl Handler

**File**: `internal/api/crawl.go`

```go
// POST /api/v1/crawl
// Body: {"title": "...", "depth": 2, "max_pages": 100}
func (s *Server) handleCrawl(c *gin.Context) {
    var req CrawlRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error: "invalid request",
            Message: err.Error(),
        })
        return
    }

    // Start crawl in background
    jobID := generateJobID()

    go func() {
        ctx := context.Background()
        scraper := scraper.New(s.cache)

        opts := scraper.CrawlOptions{
            StartPages: []string{req.Title},
            MaxDepth:   req.Depth,
            MaxPages:   req.MaxPages,
        }

        if err := scraper.Crawl(ctx, opts); err != nil {
            log.Printf("Crawl job %s failed: %v", jobID, err)
            // TODO: Store job status in database
        }

        // Reload graph after crawl
        s.reloadGraph()
    }()

    c.JSON(http.StatusAccepted, CrawlResponse{
        JobID:   jobID,
        Status:  "started",
        Message: fmt.Sprintf("Crawl started for '%s'", req.Title),
    })
}

type CrawlRequest struct {
    Title    string `json:"title" binding:"required"`
    Depth    int    `json:"depth" binding:"min=1,max=10"`
    MaxPages int    `json:"max_pages" binding:"min=1,max=10000"`
}

type CrawlResponse struct {
    JobID   string `json:"job_id"`
    Status  string `json:"status"`
    Message string `json:"message"`
}
```

**Response:**
```json
{
  "job_id": "crawl_abc123",
  "status": "started",
  "message": "Crawl started for 'Albert Einstein'"
}
```

---

### Middleware

**File**: `internal/api/middleware.go`

```go
// LoggingMiddleware logs all requests
func LoggingMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path
        query := c.Request.URL.RawQuery

        c.Next()

        duration := time.Since(start)

        log.Printf(
            "%s %s %s %d %s",
            c.Request.Method,
            path,
            query,
            c.Writer.Status(),
            duration,
        )
    }
}

// CORSMiddleware handles CORS headers
func CORSMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
        c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(http.StatusNoContent)
            return
        }

        c.Next()
    }
}

// ErrorHandlingMiddleware recovers from panics
func ErrorHandlingMiddleware() gin.HandlerFunc {
    return gin.Recovery()
}
```

---

### Server Command

**File**: `cmd/server/main.go`

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/user/wikigraph/internal/api"
    "github.com/user/wikigraph/internal/cache"
    "github.com/user/wikigraph/internal/config"
    "github.com/user/wikigraph/internal/database"
    "github.com/user/wikigraph/internal/graph"
)

func main() {
    // Load config
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Initialize database
    db, err := database.Open(cfg.Database.Path)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer db.Close()

    cache := cache.New(db)

    // Load graph into memory
    log.Println("Loading graph into memory...")
    loader := graph.NewLoader(cache)
    g, err := loader.Load(context.Background())
    if err != nil {
        log.Fatalf("Failed to load graph: %v", err)
    }
    log.Printf("Loaded %d nodes and %d edges", g.NodeCount(), g.EdgeCount())

    // Create server
    server := api.New(g, cache, nil, cfg.API)

    // Graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go handleShutdown(server, cancel)

    // Start server
    log.Printf("Starting server on %s:%d", cfg.API.Host, cfg.API.Port)
    if err := server.Start(ctx); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}

func handleShutdown(server *api.Server, cancel context.CancelFunc) {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    log.Println("Shutting down server...")
    if err := server.Shutdown(context.Background()); err != nil {
        log.Printf("Error during shutdown: %v", err)
    }
    cancel()
}
```

**CLI Integration** (alternative to standalone server):

```go
// cmd/wikigraph/serve.go
var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start the WikiGraph API server",
    Long: `Start the WikiGraph API server with in-memory graph.

The server loads the graph once on startup and keeps it in memory for fast queries.

Examples:
  wikigraph serve
  wikigraph serve --port 8080
  wikigraph serve --host 0.0.0.0 --port 3000`,
    RunE: runServe,
}

func init() {
    serveCmd.Flags().String("host", "localhost", "Server host")
    serveCmd.Flags().Int("port", 8080, "Server port")
    serveCmd.Flags().Bool("cors", true, "Enable CORS")
    serveCmd.Flags().Bool("embeddings", false, "Enable embeddings endpoints")
}
```

---

## API Reference

### Base URL

```
http://localhost:8080
```

### Endpoints

| Endpoint | Method | Description | Query Params |
|----------|--------|-------------|--------------|
| `/health` | GET | Health check | - |
| `/api/v1/page/:title` | GET | Get page and its links | - |
| `/api/v1/path` | GET | Find shortest path | `from`, `to`, `algorithm`, `max_depth` |
| `/api/v1/connections/:title` | GET | Get N-hop neighborhood | `depth`, `max_nodes` |
| `/api/v1/similar/:title` | GET | Find similar pages | `limit`, `threshold` |
| `/api/v1/crawl` | POST | Start background crawl | - (JSON body) |

### Error Responses

All errors follow this format:

```json
{
  "error": "error_code",
  "message": "Human-readable error message"
}
```

**Common Error Codes:**

| Status | Error Code | Description |
|--------|-----------|-------------|
| 400 | `invalid_request` | Malformed request or missing parameters |
| 404 | `not_found` | Resource not found |
| 500 | `internal_error` | Server error |
| 503 | `service_unavailable` | Service temporarily unavailable |

---

## Implementation Details

### Router Setup

```go
func (s *Server) setupRouter() *gin.Engine {
    // Set Gin mode
    if s.config.Production {
        gin.SetMode(gin.ReleaseMode)
    }

    router := gin.New()

    // Global middleware
    router.Use(gin.Recovery())
    router.Use(LoggingMiddleware())

    if s.config.EnableCORS {
        router.Use(CORSMiddleware())
    }

    // Health endpoint (no versioning)
    router.GET("/health", s.handleHealth)

    // API v1 routes
    v1 := router.Group("/api/v1")
    {
        v1.GET("/page/:title", s.handleGetPage)
        v1.GET("/path", s.handleFindPath)
        v1.GET("/connections/:title", s.handleGetConnections)

        // Phase 3 endpoints (if embeddings enabled)
        if s.embeddings != nil {
            v1.GET("/similar/:title", s.handleFindSimilar)
        }

        v1.POST("/crawl", s.handleCrawl)
    }

    return router
}
```

### Graph Reloading

After background crawls, reload the graph:

```go
func (s *Server) reloadGraph() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    log.Println("Reloading graph...")
    loader := graph.NewLoader(s.cache)
    newGraph, err := loader.Load(context.Background())
    if err != nil {
        return err
    }

    s.graph = newGraph
    log.Printf("Graph reloaded: %d nodes, %d edges",
        newGraph.NodeCount(), newGraph.EdgeCount())

    return nil
}
```

### Request Validation

```go
func parseIntQuery(c *gin.Context, key string, defaultVal int) int {
    val := c.Query(key)
    if val == "" {
        return defaultVal
    }

    n, err := strconv.Atoi(val)
    if err != nil {
        return defaultVal
    }

    return n
}

func parseFloatQuery(c *gin.Context, key string, defaultVal float64) float64 {
    val := c.Query(key)
    if val == "" {
        return defaultVal
    }

    f, err := strconv.ParseFloat(val, 64)
    if err != nil {
        return defaultVal
    }

    return f
}
```

---

## Testing

### Integration Tests

```go
// internal/api/api_test.go
func TestAPI_Health(t *testing.T) {
    server := setupTestServer(t)

    w := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/health", nil)

    server.Router().ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    var resp map[string]interface{}
    err := json.Unmarshal(w.Body.Bytes(), &resp)
    require.NoError(t, err)

    assert.Equal(t, "healthy", resp["status"])
}

func TestAPI_GetPage(t *testing.T) {
    server := setupTestServer(t)

    // Add test page to graph
    server.graph.AddNode("Test Page")
    server.graph.AddEdge("Test Page", "Link 1")
    server.graph.AddEdge("Test Page", "Link 2")

    w := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/api/v1/page/Test%20Page", nil)

    server.Router().ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    var resp PageResponse
    err := json.Unmarshal(w.Body.Bytes(), &resp)
    require.NoError(t, err)

    assert.Equal(t, "Test Page", resp.Title)
    assert.Equal(t, 2, resp.LinkCount)
    assert.Contains(t, resp.Links, "Link 1")
}

func TestAPI_FindPath(t *testing.T) {
    server := setupTestServer(t)

    // Build test graph: A -> B -> C
    server.graph.AddEdge("A", "B")
    server.graph.AddEdge("B", "C")

    w := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/api/v1/path?from=A&to=C", nil)

    server.Router().ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    var resp PathResponse
    err := json.Unmarshal(w.Body.Bytes(), &resp)
    require.NoError(t, err)

    assert.True(t, resp.Found)
    assert.Equal(t, 2, resp.Hops)
    assert.Equal(t, []string{"A", "B", "C"}, resp.Path)
}

func setupTestServer(t *testing.T) *Server {
    g := graph.New()
    cache := setupTestCache(t)

    cfg := Config{
        Host:         "localhost",
        Port:         8080,
        EnableCORS:   false,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
    }

    return New(g, cache, nil, cfg)
}
```

### Manual Testing with curl

```bash
# Health check
curl http://localhost:8080/health

# Get page
curl http://localhost:8080/api/v1/page/Albert%20Einstein

# Find path
curl "http://localhost:8080/api/v1/path?from=Albert%20Einstein&to=Barack%20Obama"

# Get connections
curl "http://localhost:8080/api/v1/connections/Physics?depth=2&max_nodes=100"

# Start crawl
curl -X POST http://localhost:8080/api/v1/crawl \
  -H "Content-Type: application/json" \
  -d '{"title": "Albert Einstein", "depth": 2, "max_pages": 100}'

# Similar pages (Phase 3)
curl "http://localhost:8080/api/v1/similar/Albert%20Einstein?limit=10"
```

---

## Server Mode Benefits

### Why Server Mode?

**Problem with CLI Mode:**
```bash
# Every path query reloads the entire graph from SQLite
wikigraph path "A" "B"  # Load graph (2s) + search (10ms)
wikigraph path "C" "D"  # Load graph (2s) + search (10ms)
wikigraph path "E" "F"  # Load graph (2s) + search (10ms)
```

**Solution with Server Mode:**
```bash
# Start server once
wikigraph serve  # Load graph once (2s)

# Instant queries
curl ".../path?from=A&to=B"  # 10ms
curl ".../path?from=C&to=D"  # 10ms
curl ".../path?from=E&to=F"  # 10ms
```

### Performance Comparison

| Scenario | CLI Mode | Server Mode |
|----------|----------|-------------|
| Single query | 2.01s (2s load + 10ms search) | 10ms (graph already loaded) |
| 10 queries | 20.1s | 100ms |
| 100 queries | 201s | 1s |

### Memory Trade-off

**Server Mode:**
- **Memory**: ~10-50 MB for graph (stays resident)
- **Startup time**: 2-5 seconds (one-time cost)
- **Query time**: 10-100ms

**CLI Mode:**
- **Memory**: Released after each command
- **Startup time**: 2-5 seconds (every command)
- **Query time**: 2.01-5.10s

---

## Performance

### Startup Time

| Pages | Links | Load Time |
|-------|-------|-----------|
| 1,000 | 50,000 | ~200ms |
| 10,000 | 500,000 | ~2s |
| 100,000 | 5,000,000 | ~20s |

### Query Performance

| Endpoint | Average | P95 | P99 |
|----------|---------|-----|-----|
| `/health` | <1ms | 1ms | 2ms |
| `/page/:title` | 2ms | 5ms | 10ms |
| `/path` (BFS) | 15ms | 50ms | 150ms |
| `/path` (bidirectional) | 8ms | 25ms | 80ms |
| `/connections` (depth=2) | 10ms | 30ms | 100ms |
| `/similar` | 50ms | 150ms | 300ms |

### Throughput

With default configuration (single-threaded Gin):
- **Simple queries** (`/health`, `/page`): ~5,000 req/s
- **Pathfinding**: ~1,000 req/s
- **Connections**: ~500 req/s

### Optimization Strategies

1. **Connection pooling**: Reuse database connections
2. **Response caching**: Cache common path queries
3. **Partial graph loading**: Load subgraphs on demand
4. **Horizontal scaling**: Multiple server instances (read-only graph)

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_API_HOST` | `localhost` | Server host |
| `WIKIGRAPH_API_PORT` | `8080` | Server port |
| `WIKIGRAPH_API_CORS` | `true` | Enable CORS |
| `WIKIGRAPH_API_READ_TIMEOUT` | `30s` | Read timeout |
| `WIKIGRAPH_API_WRITE_TIMEOUT` | `30s` | Write timeout |

### Config File

```yaml
# config.yaml
api:
  host: localhost
  port: 8080
  enable_cors: true
  enable_embeddings: false
  read_timeout: 30s
  write_timeout: 30s

database:
  path: ./data/wikigraph.db

graph:
  preload: true  # Load graph on startup
  auto_reload: false  # Auto-reload after crawls
```

---

## Dependencies

### New Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/gin-gonic/gin` | v1.9+ | HTTP framework |
| `github.com/google/uuid` | v1.3+ | Job ID generation |

### Existing Dependencies

| Package | Location | Purpose |
|---------|----------|---------|
| `internal/graph` | - | Graph operations |
| `internal/cache` | - | Database access |
| `internal/scraper` | - | Background crawling |
| `internal/embeddings` | - | Similarity (Phase 3) |

---

## Checklist

- [ ] Gin router set up
- [ ] `/health` endpoint
- [ ] `GET /api/v1/page/:title` endpoint
- [ ] `GET /api/v1/path` endpoint
- [ ] `GET /api/v1/connections/:title` endpoint
- [ ] `GET /api/v1/similar/:title` endpoint (Phase 3)
- [ ] `POST /api/v1/crawl` endpoint
- [ ] CORS middleware
- [ ] Logging middleware
- [ ] Error handling middleware
- [ ] Request validation
- [ ] Graceful shutdown
- [ ] `serve` CLI command
- [ ] Integration tests
- [ ] Documentation
- [ ] Example curl commands

---

## Edge Cases

| Case | Handling |
|------|----------|
| Page not in graph | Return 404 with helpful error |
| Invalid query parameters | Return 400 with validation error |
| No path exists | Return 200 with `found: false` |
| Server still loading | Return 503 "Service Unavailable" |
| Concurrent crawl jobs | Queue or reject with error |
| Graph reload during query | Use RWMutex for safe concurrent access |
| Embeddings service down | Return 503 for `/similar`, other endpoints work |

---

## Next Steps (Phase 5)

Phase 4 provides the API backend. Phase 5 will add:

- Interactive web UI with Cytoscape.js
- Graph visualization
- Search interface
- Pathfinding UI
- Static file serving from Go

See [Phase 5 Documentation](./phase5-frontend.md) for details.
