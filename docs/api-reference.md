# WikiGraph REST API Reference

## Overview

The WikiGraph API provides programmatic access to Wikipedia knowledge graph functionality including page fetching, pathfinding, and semantic similarity search.

**Base URL**: `http://localhost:8080`

**Content-Type**: `application/json`

---

## Authentication

Currently, the API does not require authentication. This may change in future versions.

---

## Endpoints

### Health Check

Check if the API is running.

```
GET /health
```

#### Response

```json
{
  "status": "ok",
  "version": "0.1.0",
  "uptime": "2h34m12s"
}
```

---

### Fetch Page

Fetch a Wikipedia page and return its links.

```
GET /page/:title
```

#### Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `title` | string | path | yes | Wikipedia page title |
| `max_links` | int | query | no | Maximum links to return (default: all) |
| `bypass_cache` | bool | query | no | Force fresh fetch (default: false) |

#### Example Request

```bash
curl "http://localhost:8080/page/Albert_Einstein?max_links=10"
```

#### Response

```json
{
  "title": "Albert Einstein",
  "link_count": 347,
  "links": [
    {"target_title": "Physics", "anchor_text": "physics"},
    {"target_title": "Germany", "anchor_text": "Germany"},
    {"target_title": "Theoretical physics", "anchor_text": "theoretical physicist"},
    ...
  ],
  "fetched_at": "2024-01-15T10:30:00Z",
  "from_cache": true,
  "fetch_duration_ms": 0
}
```

#### Errors

| Code | Description |
|------|-------------|
| 400 | Invalid title |
| 404 | Page not found |
| 429 | Rate limited |
| 500 | Internal error |

---

### Find Path

Find the shortest path between two Wikipedia pages.

```
GET /path
```

#### Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `from` | string | query | yes | Starting page title |
| `to` | string | query | yes | Target page title |
| `max_depth` | int | query | no | Maximum path length (default: 6) |
| `timeout` | int | query | no | Timeout in seconds (default: 30) |

#### Example Request

```bash
curl "http://localhost:8080/path?from=Albert_Einstein&to=Barack_Obama"
```

#### Response (Success)

```json
{
  "found": true,
  "path": [
    "Albert Einstein",
    "Princeton University",
    "United States",
    "Barack Obama"
  ],
  "hops": 3,
  "pages_visited": 1247,
  "computed_in_ms": 245
}
```

#### Response (Not Found)

```json
{
  "found": false,
  "path": null,
  "hops": 0,
  "pages_visited": 5000,
  "computed_in_ms": 30000,
  "reason": "max_depth_exceeded"
}
```

#### Errors

| Code | Description |
|------|-------------|
| 400 | Missing or invalid parameters |
| 404 | One or both pages not found |
| 408 | Request timeout |
| 500 | Internal error |

---

### Get Connections

Get the N-hop neighborhood of a page.

```
GET /connections/:title
```

#### Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `title` | string | path | yes | Wikipedia page title |
| `depth` | int | query | no | Neighborhood depth (default: 1, max: 3) |
| `max_nodes` | int | query | no | Maximum nodes to return (default: 100) |
| `direction` | string | query | no | `outgoing`, `incoming`, or `both` (default: `outgoing`) |

#### Example Request

```bash
curl "http://localhost:8080/connections/Albert_Einstein?depth=1&max_nodes=20"
```

#### Response

```json
{
  "center": "Albert Einstein",
  "depth": 1,
  "nodes": [
    {"title": "Albert Einstein", "depth": 0},
    {"title": "Physics", "depth": 1},
    {"title": "Germany", "depth": 1},
    {"title": "Nobel Prize in Physics", "depth": 1},
    ...
  ],
  "edges": [
    {"source": "Albert Einstein", "target": "Physics"},
    {"source": "Albert Einstein", "target": "Germany"},
    ...
  ],
  "node_count": 20,
  "edge_count": 19
}
```

---

### Find Similar Pages

Find semantically similar pages using embeddings.

```
GET /similar/:title
```

#### Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `title` | string | path | yes | Wikipedia page title |
| `limit` | int | query | no | Number of results (default: 10, max: 50) |
| `min_score` | float | query | no | Minimum similarity score (default: 0.5) |

#### Example Request

```bash
curl "http://localhost:8080/similar/World_War_II?limit=5"
```

#### Response

```json
{
  "query": "World War II",
  "results": [
    {"title": "World War I", "score": 0.89},
    {"title": "Nazi Germany", "score": 0.84},
    {"title": "Allied Powers", "score": 0.81},
    {"title": "Adolf Hitler", "score": 0.78},
    {"title": "Holocaust", "score": 0.75}
  ],
  "computed_in_ms": 23
}
```

#### Errors

| Code | Description |
|------|-------------|
| 400 | Invalid parameters |
| 404 | Page not found or no embedding |
| 503 | Embeddings service unavailable |

---

### Start Crawl

Start a background crawl job.

```
POST /crawl
```

#### Request Body

```json
{
  "start_page": "Albert Einstein",
  "max_pages": 500,
  "max_depth": 2,
  "respect_rate_limit": true
}
```

#### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `start_page` | string | yes | Starting Wikipedia page |
| `max_pages` | int | no | Maximum pages to crawl (default: 100) |
| `max_depth` | int | no | Maximum crawl depth (default: 2) |
| `respect_rate_limit` | bool | no | Respect Wikipedia rate limits (default: true) |

#### Response

```json
{
  "job_id": "crawl_abc123",
  "status": "started",
  "start_page": "Albert Einstein",
  "max_pages": 500,
  "estimated_duration": "8m20s"
}
```

---

### Get Crawl Status

Get the status of a crawl job.

```
GET /crawl/:job_id
```

#### Response (In Progress)

```json
{
  "job_id": "crawl_abc123",
  "status": "running",
  "progress": {
    "pages_crawled": 234,
    "pages_total": 500,
    "percent_complete": 46.8,
    "elapsed": "3m45s",
    "estimated_remaining": "4m12s"
  }
}
```

#### Response (Complete)

```json
{
  "job_id": "crawl_abc123",
  "status": "complete",
  "result": {
    "pages_crawled": 500,
    "links_found": 47823,
    "duration": "8m12s",
    "errors": 3
  }
}
```

---

### Cache Statistics

Get cache statistics.

```
GET /cache/stats
```

#### Response

```json
{
  "total_pages": 1542,
  "total_links": 523847,
  "database_size_mb": 45.2,
  "oldest_entry": "2024-01-08T14:22:00Z",
  "newest_entry": "2024-01-15T10:30:00Z",
  "cache_hit_rate": 0.78
}
```

---

### Clear Cache

Clear the cache (admin only).

```
DELETE /cache
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `older_than` | string | no | Clear entries older than duration (e.g., "7d") |
| `title` | string | no | Clear specific page |

#### Response

```json
{
  "cleared": true,
  "pages_removed": 142,
  "links_removed": 48293
}
```

---

## Error Responses

All errors follow this format:

```json
{
  "error": {
    "code": "PAGE_NOT_FOUND",
    "message": "The requested Wikipedia page does not exist",
    "details": {
      "title": "NonexistentPage123"
    }
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_TITLE` | 400 | Invalid page title |
| `INVALID_PARAMETER` | 400 | Invalid query parameter |
| `PAGE_NOT_FOUND` | 404 | Wikipedia page not found |
| `PATH_NOT_FOUND` | 404 | No path exists between pages |
| `RATE_LIMITED` | 429 | Too many requests |
| `TIMEOUT` | 408 | Request timed out |
| `EMBEDDINGS_UNAVAILABLE` | 503 | Embeddings service down |
| `INTERNAL_ERROR` | 500 | Internal server error |

---

## Rate Limiting

The API implements rate limiting to prevent abuse:

- **Default**: 60 requests per minute
- **Crawl endpoints**: 10 requests per hour

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1705312800
```

---

## Pagination

Endpoints returning lists support pagination:

```
GET /page/Albert_Einstein?limit=50&offset=100
```

Response includes pagination metadata:

```json
{
  "data": [...],
  "pagination": {
    "limit": 50,
    "offset": 100,
    "total": 347,
    "has_more": true
  }
}
```

---

## Examples

### Python

```python
import requests

BASE_URL = "http://localhost:8080"

# Fetch a page
response = requests.get(f"{BASE_URL}/page/Albert_Einstein")
page = response.json()
print(f"Found {page['link_count']} links")

# Find a path
response = requests.get(f"{BASE_URL}/path", params={
    "from": "Albert_Einstein",
    "to": "Barack_Obama"
})
result = response.json()
if result["found"]:
    print(" → ".join(result["path"]))
```

### JavaScript

```javascript
const BASE_URL = 'http://localhost:8080';

// Fetch a page
const pageResponse = await fetch(`${BASE_URL}/page/Albert_Einstein`);
const page = await pageResponse.json();
console.log(`Found ${page.link_count} links`);

// Find a path
const pathResponse = await fetch(
  `${BASE_URL}/path?from=Albert_Einstein&to=Barack_Obama`
);
const result = await pathResponse.json();
if (result.found) {
  console.log(result.path.join(' → '));
}
```

### cURL

```bash
# Fetch a page
curl -s "http://localhost:8080/page/Albert_Einstein" | jq

# Find a path
curl -s "http://localhost:8080/path?from=Albert_Einstein&to=Barack_Obama" | jq

# Start a crawl
curl -X POST "http://localhost:8080/crawl" \
  -H "Content-Type: application/json" \
  -d '{"start_page": "Physics", "max_pages": 100}'
```

---

## Changelog

### v0.1.0

- Initial API release
- Basic page fetching
- Cache management

### v0.2.0 (Planned)

- Pathfinding endpoints
- Graph connection queries

### v0.3.0 (Planned)

- Semantic similarity search
- Embeddings integration
