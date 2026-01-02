# Phase 3: Embeddings Microservice

## Overview

Phase 3 adds semantic similarity capabilities to WikiGraph. A Python microservice generates text embeddings using sentence-transformers, and a Go client integrates this into the main application. Users will be able to find Wikipedia pages that are semantically similar to a given page.

---

## Table of Contents

- [What We Already Have](#what-we-already-have)
- [What Phase 3 Adds](#what-phase-3-adds)
- [Architecture](#architecture)
- [Components](#components)
  - [Python Embeddings Service](#python-embeddings-service)
  - [Go Embeddings Client](#go-embeddings-client)
  - [SQLite Embeddings Storage](#sqlite-embeddings-storage)
  - [Similar Command](#similar-command)
- [API Reference](#api-reference)
- [Implementation Details](#implementation-details)
- [Testing](#testing)
- [Docker Setup](#docker-setup)
- [Performance](#performance)

---

## What We Already Have

From Phase 1 and Phase 2:

| Feature | Location | Status |
|---------|----------|--------|
| Page fetching and caching | `internal/scraper/`, `internal/cache/` | Done |
| Link storage in SQLite | `internal/cache/` | Done |
| In-memory graph construction | `internal/graph/` | Done |
| BFS and bidirectional pathfinding | `internal/graph/` | Done |
| Python service skeleton | `python/` | Partial |

---

## What Phase 3 Adds

| Feature | Location | Description |
|---------|----------|-------------|
| Embeddings service | `python/` | FastAPI service with sentence-transformers |
| Go embeddings client | `internal/embeddings/` | HTTP client to call Python service |
| Embeddings storage | `migrations/` | New table for embedding vectors |
| `similar` CLI command | `cmd/wikigraph/similar.go` | Find semantically similar pages |
| Docker Compose | `docker-compose.yml` | Run Go + Python services together |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         CLI                                 │
│               wikigraph similar <title>                     │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                 internal/embeddings                          │
│                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │    Client    │───▶│   Service    │───▶│   Storage    │   │
│  │   (HTTP)     │    │   (Lookup)   │    │   (SQLite)   │   │
│  └──────────────┘    └──────────────┘    └──────────────┘   │
│          │                                                   │
└──────────┼───────────────────────────────────────────────────┘
           │ HTTP (localhost:8001)
           ▼
┌─────────────────────────────────────────────────────────────┐
│                  Python Service                              │
│              (FastAPI + sentence-transformers)               │
│                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │   /embed     │    │ /similarity  │    │  /embed/batch│   │
│  └──────────────┘    └──────────────┘    └──────────────┘   │
│          │                                                   │
│          ▼                                                   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         SentenceTransformer (all-MiniLM-L6-v2)       │   │
│  │                   384-dimensional vectors             │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow

1. User requests similar pages for a Wikipedia title
2. Check if embedding exists in SQLite cache
3. If not cached, fetch page content and generate embedding via Python service
4. Compare embedding against all stored embeddings using cosine similarity
5. Return top-N most similar pages ranked by score

---

## Components

### Python Embeddings Service

**Location**: `python/`

**Files**:
- `main.py` - FastAPI application and endpoints
- `requirements.txt` - Python dependencies
- `Dockerfile` - Container configuration

#### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/embed` | POST | Generate embedding for single text |
| `/embed/batch` | POST | Generate embeddings for multiple texts |
| `/similarity` | POST | Calculate similarity between two texts |

#### Endpoint Details

**POST /embed**

Request:
```json
{
  "text": "Albert Einstein was a theoretical physicist"
}
```

Response:
```json
{
  "vector": [0.123, -0.456, ...],
  "dimensions": 384,
  "model": "all-MiniLM-L6-v2"
}
```

**POST /embed/batch**

Request:
```json
{
  "texts": [
    "Albert Einstein",
    "Isaac Newton",
    "Marie Curie"
  ]
}
```

Response:
```json
{
  "embeddings": [
    {"text": "Albert Einstein", "vector": [...]},
    {"text": "Isaac Newton", "vector": [...]},
    {"text": "Marie Curie", "vector": [...]}
  ],
  "dimensions": 384,
  "model": "all-MiniLM-L6-v2"
}
```

**POST /similarity**

Request:
```json
{
  "text1": "Albert Einstein was a physicist",
  "text2": "Isaac Newton discovered gravity"
}
```

Response:
```json
{
  "score": 0.72,
  "text1": "Albert Einstein was a physicist",
  "text2": "Isaac Newton discovered gravity"
}
```

#### Model Selection

Using `all-MiniLM-L6-v2` because:
- Fast inference (~14ms per text on CPU)
- Small model size (~80MB)
- Good quality embeddings (384 dimensions)
- Well-suited for semantic similarity tasks

Alternative models to consider:
| Model | Dimensions | Speed | Quality |
|-------|-----------|-------|---------|
| all-MiniLM-L6-v2 | 384 | Fast | Good |
| all-mpnet-base-v2 | 768 | Medium | Better |
| all-MiniLM-L12-v2 | 384 | Medium | Better |

---

### Go Embeddings Client

**Location**: `internal/embeddings/`

**Files**:
- `client.go` - HTTP client for Python service
- `service.go` - High-level embedding operations
- `storage.go` - SQLite embedding storage
- `embeddings_test.go` - Unit tests

#### Client Interface

```go
package embeddings

// Client communicates with the Python embeddings service
type Client struct {
    baseURL    string
    httpClient *http.Client
    timeout    time.Duration
}

// NewClient creates a new embeddings client
func NewClient(baseURL string, opts ...Option) *Client

// Embed generates an embedding for a single text
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error)

// EmbedBatch generates embeddings for multiple texts
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

// Similarity calculates similarity between two texts
func (c *Client) Similarity(ctx context.Context, text1, text2 string) (float64, error)

// Health checks if the service is available
func (c *Client) Health(ctx context.Context) error
```

#### Service Interface

```go
// Service provides high-level embedding operations
type Service struct {
    client  *Client
    storage *Storage
    cache   *cache.Cache
}

// NewService creates a new embeddings service
func NewService(client *Client, storage *Storage, cache *cache.Cache) *Service

// GetEmbedding returns the embedding for a page, computing if needed
func (s *Service) GetEmbedding(ctx context.Context, title string) ([]float32, error)

// FindSimilar finds pages similar to the given title
func (s *Service) FindSimilar(ctx context.Context, title string, limit int) ([]SimilarPage, error)

// ComputeAllEmbeddings generates embeddings for all cached pages
func (s *Service) ComputeAllEmbeddings(ctx context.Context, batchSize int) error

// SimilarPage represents a page with its similarity score
type SimilarPage struct {
    Title string
    Score float64
}
```

---

### SQLite Embeddings Storage

**Location**: `migrations/002_embeddings.sql`

#### Schema

```sql
-- Embeddings table for storing pre-computed vectors
CREATE TABLE IF NOT EXISTS embeddings (
    id         INTEGER PRIMARY KEY,
    page_id    INTEGER NOT NULL UNIQUE,
    vector     BLOB NOT NULL,
    model      TEXT NOT NULL DEFAULT 'all-MiniLM-L6-v2',
    dimensions INTEGER NOT NULL DEFAULT 384,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    FOREIGN KEY (page_id) REFERENCES pages(id) ON DELETE CASCADE
);

CREATE INDEX idx_embeddings_page_id ON embeddings(page_id);
```

#### Vector Storage

Embeddings stored as binary BLOBs (float32 array):
- 384 dimensions * 4 bytes = 1,536 bytes per embedding
- For 10,000 pages: ~15 MB of embedding data

```go
// EncodeVector converts float32 slice to bytes for storage
func EncodeVector(v []float32) []byte {
    buf := make([]byte, len(v)*4)
    for i, f := range v {
        binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
    }
    return buf
}

// DecodeVector converts bytes back to float32 slice
func DecodeVector(b []byte) []float32 {
    v := make([]float32, len(b)/4)
    for i := range v {
        v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
    }
    return v
}
```

---

### Similar Command

**Location**: `cmd/wikigraph/similar.go`

```go
var similarCmd = &cobra.Command{
    Use:   "similar <title>",
    Short: "Find Wikipedia pages similar to the given page",
    Long: `Find Wikipedia pages that are semantically similar to the given page.

The page must already be in the local database. Uses text embeddings to
find pages with similar meaning, not just similar links.

Requires the embeddings service to be running (python/main.py).

Examples:
  wikigraph similar "Albert Einstein"
  wikigraph similar "World War II" --limit 20
  wikigraph similar "Physics" --threshold 0.7
  wikigraph similar "Go (programming language)" --format json`,
    Args: cobra.ExactArgs(1),
    RunE: runSimilar,
}
```

#### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--limit` | `-n` | 10 | Maximum number of similar pages to return |
| `--threshold` | `-t` | 0.5 | Minimum similarity score (0-1) |
| `--format` | `-f` | text | Output format: text, json |
| `--embeddings-url` | | http://localhost:8001 | Embeddings service URL |

#### Output

**Text format:**
```
Similar to "Albert Einstein":

  1. Isaac Newton          0.847
  2. Niels Bohr            0.823
  3. Richard Feynman       0.812
  4. Werner Heisenberg     0.798
  5. Max Planck            0.791
  ...

Found 10 similar pages (threshold: 0.50)
```

**JSON format:**
```json
{
  "query": "Albert Einstein",
  "similar": [
    {"title": "Isaac Newton", "score": 0.847},
    {"title": "Niels Bohr", "score": 0.823},
    {"title": "Richard Feynman", "score": 0.812}
  ],
  "count": 10,
  "threshold": 0.5
}
```

---

## API Reference

### CLI Commands

```bash
# Find similar pages (default: top 10)
wikigraph similar "Albert Einstein"

# Limit results
wikigraph similar "Physics" --limit 20

# Filter by minimum similarity
wikigraph similar "Mathematics" --threshold 0.7

# JSON output
wikigraph similar "Computer Science" --format json

# Custom embeddings service URL
wikigraph similar "Python" --embeddings-url http://localhost:8001
```

### Programmatic API

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/user/wikigraph/internal/cache"
    "github.com/user/wikigraph/internal/embeddings"
)

func main() {
    // Initialize cache
    c, err := cache.New("wikigraph.db")
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Initialize embeddings client
    client := embeddings.NewClient("http://localhost:8001")
    storage := embeddings.NewStorage(c.DB())
    service := embeddings.NewService(client, storage, c)

    // Find similar pages
    ctx := context.Background()
    similar, err := service.FindSimilar(ctx, "Albert Einstein", 10)
    if err != nil {
        log.Fatal(err)
    }

    for _, page := range similar {
        fmt.Printf("%s: %.3f\n", page.Title, page.Score)
    }
}
```

---

## Implementation Details

### Similarity Calculation

Using cosine similarity for comparing embeddings:

```go
// CosineSimilarity calculates similarity between two vectors
func CosineSimilarity(a, b []float32) float64 {
    if len(a) != len(b) {
        return 0
    }

    var dotProduct, normA, normB float64
    for i := range a {
        dotProduct += float64(a[i]) * float64(b[i])
        normA += float64(a[i]) * float64(a[i])
        normB += float64(b[i]) * float64(b[i])
    }

    if normA == 0 || normB == 0 {
        return 0
    }

    return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

### Text for Embeddings

For each Wikipedia page, we generate an embedding from:
1. **Title only** (fast, simple)
2. **Title + first paragraph** (better quality, requires fetching content)
3. **Title + all outgoing link titles** (graph-aware)

Starting with option 1 (title only) for simplicity. Can upgrade later.

### Batch Processing

For computing embeddings for all cached pages:

```go
func (s *Service) ComputeAllEmbeddings(ctx context.Context, batchSize int) error {
    // Get all pages without embeddings
    pages, err := s.cache.GetPagesWithoutEmbeddings(ctx)
    if err != nil {
        return err
    }

    // Process in batches
    for i := 0; i < len(pages); i += batchSize {
        end := min(i+batchSize, len(pages))
        batch := pages[i:end]

        // Get titles
        titles := make([]string, len(batch))
        for j, p := range batch {
            titles[j] = p.Title
        }

        // Compute embeddings
        vectors, err := s.client.EmbedBatch(ctx, titles)
        if err != nil {
            return err
        }

        // Store embeddings
        for j, p := range batch {
            if err := s.storage.Save(ctx, p.ID, vectors[j]); err != nil {
                return err
            }
        }
    }

    return nil
}
```

---

## Testing

### Python Service Tests

```python
# test_main.py
import pytest
from fastapi.testclient import TestClient
from main import app

client = TestClient(app)

def test_health():
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json()["status"] == "healthy"

def test_embed():
    response = client.post("/embed", json={"text": "Albert Einstein"})
    assert response.status_code == 200
    data = response.json()
    assert "vector" in data
    assert len(data["vector"]) == 384
    assert data["dimensions"] == 384

def test_embed_batch():
    response = client.post("/embed/batch", json={
        "texts": ["Albert Einstein", "Isaac Newton"]
    })
    assert response.status_code == 200
    data = response.json()
    assert len(data["embeddings"]) == 2

def test_similarity():
    response = client.post("/similarity", json={
        "text1": "Albert Einstein",
        "text2": "Isaac Newton"
    })
    assert response.status_code == 200
    data = response.json()
    assert 0 <= data["score"] <= 1
```

### Go Client Tests

```go
func TestClient_Embed(t *testing.T) {
    // Start test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "vector":     make([]float32, 384),
            "dimensions": 384,
            "model":      "all-MiniLM-L6-v2",
        })
    }))
    defer server.Close()

    client := embeddings.NewClient(server.URL)
    vector, err := client.Embed(context.Background(), "test")

    require.NoError(t, err)
    assert.Len(t, vector, 384)
}

func TestCosineSimilarity(t *testing.T) {
    tests := []struct {
        name string
        a, b []float32
        want float64
    }{
        {
            name: "identical vectors",
            a:    []float32{1, 0, 0},
            b:    []float32{1, 0, 0},
            want: 1.0,
        },
        {
            name: "orthogonal vectors",
            a:    []float32{1, 0, 0},
            b:    []float32{0, 1, 0},
            want: 0.0,
        },
        {
            name: "opposite vectors",
            a:    []float32{1, 0, 0},
            b:    []float32{-1, 0, 0},
            want: -1.0,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := embeddings.CosineSimilarity(tt.a, tt.b)
            assert.InDelta(t, tt.want, got, 0.001)
        })
    }
}
```

---

## Docker Setup

### Dockerfile for Python Service

```dockerfile
# python/Dockerfile
FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Download model at build time
RUN python -c "from sentence_transformers import SentenceTransformer; SentenceTransformer('all-MiniLM-L6-v2')"

COPY . .

EXPOSE 8001

CMD ["python", "main.py"]
```

### Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  wikigraph:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - WIKIGRAPH_DB_PATH=/app/data/wikigraph.db
      - WIKIGRAPH_EMBEDDINGS_URL=http://embeddings:8001
    depends_on:
      embeddings:
        condition: service_healthy

  embeddings:
    build: ./python
    ports:
      - "8001:8001"
    environment:
      - MODEL_NAME=all-MiniLM-L6-v2
      - HOST=0.0.0.0
      - PORT=8001
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8001/health"]
      interval: 10s
      timeout: 5s
      retries: 5
```

### Running with Docker

```bash
# Build and start all services
docker-compose up --build

# Run in background
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

---

## Performance

### Embedding Generation

| Operation | Time (CPU) | Time (GPU) |
|-----------|-----------|------------|
| Single embedding | ~15ms | ~3ms |
| Batch of 32 | ~200ms | ~20ms |
| Batch of 100 | ~500ms | ~50ms |

### Similarity Search

For N pages with 384-dimensional embeddings:

| Pages | Comparison Time | Memory |
|-------|-----------------|--------|
| 1,000 | ~5ms | ~1.5 MB |
| 10,000 | ~50ms | ~15 MB |
| 100,000 | ~500ms | ~150 MB |

### Optimization Strategies

1. **Batch embedding computation**: Process multiple pages at once
2. **Precompute embeddings**: Generate during crawl or as background job
3. **Approximate nearest neighbors**: Use FAISS or Annoy for 100k+ pages
4. **GPU acceleration**: Use CUDA for faster inference

---

## Checklist

- [ ] Python FastAPI service running
- [ ] `/embed` endpoint working
- [ ] `/embed/batch` endpoint working
- [ ] `/similarity` endpoint working
- [ ] Go HTTP client implemented
- [ ] Embeddings stored in SQLite
- [ ] Similar page lookup working
- [ ] `similar` CLI command
- [ ] Docker Compose configuration
- [ ] Unit tests passing
- [ ] Integration tests passing
- [ ] Documentation complete

---

## Edge Cases

| Case | Handling |
|------|----------|
| Embeddings service unavailable | Return error with helpful message |
| Page not in database | Error: "page not found: <title>" |
| No embeddings computed yet | Prompt user to run batch compute |
| Very short titles | Still generate embedding (may be low quality) |
| Non-English text | Model handles multilingual reasonably well |
| Identical pages | Return similarity of 1.0 |

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_EMBEDDINGS_URL` | `http://localhost:8001` | Embeddings service URL |
| `WIKIGRAPH_EMBEDDINGS_TIMEOUT` | `30s` | Request timeout |
| `WIKIGRAPH_EMBEDDINGS_BATCH_SIZE` | `32` | Batch size for bulk operations |

### Python Service Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MODEL_NAME` | `all-MiniLM-L6-v2` | Sentence transformer model |
| `HOST` | `0.0.0.0` | Server host |
| `PORT` | `8001` | Server port |
| `WORKERS` | `1` | Number of workers |

---

## Dependencies

### Python

| Package | Version | Purpose |
|---------|---------|---------|
| `fastapi` | ~0.100 | Web framework |
| `uvicorn` | ~0.23 | ASGI server |
| `sentence-transformers` | ~2.2 | Text embeddings |
| `numpy` | ~1.24 | Vector operations |
| `pydantic` | ~2.0 | Data validation |

### Go

| Package | Version | Purpose |
|---------|---------|---------|
| `net/http` | stdlib | HTTP client |
| `encoding/json` | stdlib | JSON encoding |
| `encoding/binary` | stdlib | Binary encoding for vectors |

No new external Go dependencies required.

---

## Next Steps (Phase 4)

Phase 3 provides semantic similarity. Phase 4 will add:

- RESTful API exposing all functionality
- `/page`, `/path`, `/connections`, `/similar` endpoints
- Background crawl jobs via API
- Request validation and error handling

See [Phase 4 Documentation](./phase4-api.md) for details.
