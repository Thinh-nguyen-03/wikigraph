# WikiGraph API Reference

**Base URL:** `http://localhost:8080`  
**Content-Type:** `application/json`

All endpoints under `/api/v1/` require the graph to be ready. If the graph is still loading the server returns `503 Service Unavailable` with a `Retry-After: 30` header.

Title parameters accept both spaces and underscores — `Albert_Einstein` and `Albert Einstein` resolve to the same page.

---

## GET /health

Health check. Always returns `200` even while the graph is loading.

**Response**

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "graph": {
    "nodes": 5600000,
    "edges": 162000000
  },
  "graph_ready": true,
  "neo4j": {
    "enabled": true,
    "connected": true,
    "nodes": 5600000,
    "edges": 162000000
  }
}
```

`neo4j` is omitted when `neo4j.enabled = false` in config. `status` is `"degraded"` when the graph is not yet ready.

---

## GET /api/v1/page/:title

Returns a page and its outgoing and incoming links.

**Path parameters**

| Parameter | Description |
|-----------|-------------|
| `title` | Wikipedia page title (spaces or underscores) |

**Response `200`**

```json
{
  "title": "Albert Einstein",
  "links": ["Physics", "Germany", "Nobel Prize in Physics"],
  "link_count": 347,
  "in_links": ["Physicist", "German-American"],
  "in_link_count": 89
}
```

`in_links` and `in_link_count` are populated from the in-memory graph. When the Neo4j backend is active, up to 1000 outgoing and 1000 incoming links are returned.

**Errors**

| Status | `error` field | Condition |
|--------|---------------|-----------|
| 404 | `not_found` | Page does not exist in the graph |
| 503 | `graph_loading` | Graph not yet ready (in-memory backend only) |

---

## GET /api/v1/path

Find the shortest path between two pages.

**Query parameters**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `from` | string | required | Source page title |
| `to` | string | required | Target page title |
| `algorithm` | string | `bfs` | `bfs` or `bidirectional` |
| `backend` | string | `auto` | `auto`, `neo4j`, or `memory` |
| `max_depth` | int | `6` | Maximum path length (1–20) |

**Backend selection**

- `auto` — uses Neo4j if connected, otherwise in-memory graph
- `neo4j` — forces Neo4j; returns `503` if unavailable
- `memory` — forces in-memory graph; returns `503` if graph not ready

**Response `200` — path found**

```json
{
  "found": true,
  "from": "Albert Einstein",
  "to": "Barack Obama",
  "path": ["Albert Einstein", "Princeton University", "United States", "Barack Obama"],
  "hops": 3,
  "explored": 1247,
  "algorithm": "memory-bfs",
  "duration_ms": 12
}
```

**Response `200` — no path**

```json
{
  "found": false,
  "from": "Page A",
  "to": "Page B",
  "hops": 0,
  "explored": 5000,
  "algorithm": "memory-bfs",
  "duration_ms": 340
}
```

`algorithm` reflects the backend and algorithm used: `memory-bfs`, `memory-bidirectional`, or `neo4j-shortestpath`.

**Errors**

| Status | `error` field | Condition |
|--------|---------------|-----------|
| 400 | `missing_parameter` | `from` or `to` not provided |
| 400 | `invalid_parameter` | Unknown `algorithm` or `backend`; `max_depth` out of range |
| 503 | `neo4j_unavailable` | `backend=neo4j` but Neo4j not connected |
| 503 | `graph_loading` | `backend=memory` and graph not ready |

---

## GET /api/v1/connections/:title

Return the N-hop neighborhood subgraph centered on a page. Uses BFS from the in-memory graph.

**Path parameters**

| Parameter | Description |
|-----------|-------------|
| `title` | Center page title |

**Query parameters**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `depth` | int | `1` | Hop depth (1–5) |
| `max_nodes` | int | `100` | Node cap (1–10000) |

**Response `200`**

```json
{
  "center": "Physics",
  "depth": 2,
  "nodes": [
    {"id": "Physics", "title": "Physics", "hops": 0},
    {"id": "Mathematics", "title": "Mathematics", "hops": 1}
  ],
  "edges": [
    {"source": "Physics", "target": "Mathematics"}
  ],
  "node_count": 48,
  "edge_count": 51
}
```

**Errors**

| Status | `error` field | Condition |
|--------|---------------|-----------|
| 400 | `invalid_parameter` | `depth` or `max_nodes` out of range |
| 404 | `not_found` | Page not in graph |
| 503 | `graph_loading` | Graph not ready |

---

## POST /api/v1/crawl

Start a background crawl job. Returns immediately with a job ID.

**Request body**

```json
{
  "title": "Mathematics",
  "depth": 2,
  "max_pages": 1000
}
```

| Field | Type | Required | Constraints |
|-------|------|----------|-------------|
| `title` | string | yes | Non-empty Wikipedia page title |
| `depth` | int | no | 1–50 (default: scraper config) |
| `max_pages` | int | no | 1–500000 (default: scraper config) |

**Response `202`**

```json
{
  "job_id": "crawl_a1b2c3d4",
  "status": "started",
  "message": "Crawl job started for 'Mathematics'"
}
```

**Errors**

| Status | `error` field | Condition |
|--------|---------------|-----------|
| 400 | `missing_parameter` | `title` not provided |
| 400 | `invalid_request` | Malformed JSON |

---

## GET /api/v1/crawl/:id

Poll the status of a crawl job.

**Response `200`**

```json
{
  "job_id": "crawl_a1b2c3d4",
  "status": "crawling",
  "title": "Mathematics",
  "started_at": "2024-01-15T10:30:00Z",
  "completed_at": null
}
```

`status` values: `started` | `crawling` | `syncing` | `done` | `failed`

`completed_at` is set (non-null) when `status` is `done` or `failed`. `error` is present on `failed`.

**Errors**

| Status | `error` field | Condition |
|--------|---------------|-----------|
| 404 | `not_found` | Unknown job ID |

---

## Error Response Format

All errors use this shape:

```json
{
  "error": "not_found",
  "message": "Page 'Nonexistent' not found",
  "request_id": "req_abc123"
}
```

`request_id` matches the `X-Request-ID` response header.

---

## Rate Limiting

Requests are rate-limited per source IP using a token bucket. Defaults: 100 req/s with a burst of 200.

When the limit is exceeded the server returns `429 Too Many Requests`. Configure with `api.rate_limit` and `api.rate_burst` in `config.yaml`.

---

## Request Tracing

Every request gets an `X-Request-ID` header. If the client sends one it is echoed back; otherwise the server generates a UUID. The ID appears in server logs and in error response bodies.

---

## Code Examples

### curl

```bash
# Health
curl http://localhost:8080/health

# Page info
curl http://localhost:8080/api/v1/page/Albert_Einstein

# Shortest path (auto backend)
curl "http://localhost:8080/api/v1/path?from=Albert_Einstein&to=Barack_Obama"

# Force Neo4j backend
curl "http://localhost:8080/api/v1/path?from=Albert_Einstein&to=Barack_Obama&backend=neo4j"

# Bidirectional search with depth limit
curl "http://localhost:8080/api/v1/path?from=Cat&to=Philosophy&algorithm=bidirectional&max_depth=8"

# 2-hop neighborhood, max 50 nodes
curl "http://localhost:8080/api/v1/connections/Physics?depth=2&max_nodes=50"

# Start crawl
curl -X POST http://localhost:8080/api/v1/crawl \
  -H "Content-Type: application/json" \
  -d '{"title": "Mathematics", "depth": 2, "max_pages": 1000}'

# Poll crawl job
curl http://localhost:8080/api/v1/crawl/crawl_a1b2c3d4
```

### Python

```python
import requests

BASE = "http://localhost:8080"

# Path search
r = requests.get(f"{BASE}/api/v1/path", params={"from": "Albert Einstein", "to": "Barack Obama"})
result = r.json()
if result["found"]:
    print(" → ".join(result["path"]))

# Neighborhood graph
r = requests.get(f"{BASE}/api/v1/connections/Physics", params={"depth": 2, "max_nodes": 100})
data = r.json()
print(f"{data['node_count']} nodes, {data['edge_count']} edges")
```

### JavaScript

```javascript
const BASE = 'http://localhost:8080';

// Path search
const r = await fetch(`${BASE}/api/v1/path?from=Albert+Einstein&to=Barack+Obama`);
const result = await r.json();
if (result.found) console.log(result.path.join(' → '));

// Start a crawl and poll until done
const start = await fetch(`${BASE}/api/v1/crawl`, {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({title: 'Mathematics', depth: 2, max_pages: 500})
});
const {job_id} = await start.json();

let status;
do {
  await new Promise(r => setTimeout(r, 2000));
  status = await (await fetch(`${BASE}/api/v1/crawl/${job_id}`)).json();
} while (status.status !== 'done' && status.status !== 'failed');
console.log('Crawl finished:', status.status);
```
