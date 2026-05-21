# Configuration Reference

WikiGraph reads configuration from three sources in priority order:

1. Environment variables (highest priority)
2. `config.yaml` (in the working directory or `~/.config/wikigraph/`)
3. Built-in defaults (lowest priority)

---

## Full config.yaml Example

```yaml
database:
  path: wikigraph.db

scraper:
  rate_limit: 100.0          # Wikipedia requests per second
  max_concurrent: 30         # parallel worker goroutines
  max_depth: 3               # default crawl depth
  request_timeout: 30s
  user_agent: "WikiGraph/1.0 (https://github.com/Thinh-nguyen-03/wikigraph)"
  wikipedia_api_url: "https://en.wikipedia.org/api/rest_v1"

api:
  host: 0.0.0.0
  port: 8080
  enable_cors: true
  cors_origins: ["*"]
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 10s
  rate_limit: 100.0          # requests per second per IP
  rate_burst: 200            # token bucket burst size
  production: false          # enables Gin release mode + strict CORS

graph:
  cache_path: ""             # defaults to graph.cache beside the database file
  max_cache_age: 24h         # rebuild cache when older than this
  refresh_interval: 5m       # check for incremental DB updates every N minutes
  force_rebuild: false       # ignore cache and rebuild from DB on startup

neo4j:
  enabled: false             # set true to use Neo4j as query backend
  uri: bolt://localhost:7687
  username: neo4j
  password: wikigraph
  max_connection_pool_size: 50
  connection_acquisition_timeout: 60s
  sync_interval: 5m          # how often to sync new SQLite data to Neo4j
  sync_batch_size: 10000     # nodes/edges per sync batch

log:
  level: info                # debug | info | warn | error
```

---

## Environment Variables

Variables use the prefix `WIKIGRAPH_` with dots replaced by underscores and letters uppercased.

### Database

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_DATABASE_PATH` | `wikigraph.db` | SQLite database file path |

### Scraper

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_SCRAPER_RATE_LIMIT` | `100.0` | Max Wikipedia requests per second |
| `WIKIGRAPH_SCRAPER_MAX_CONCURRENT` | `30` | Parallel worker count |
| `WIKIGRAPH_SCRAPER_MAX_DEPTH` | `3` | Default crawl depth |
| `WIKIGRAPH_SCRAPER_REQUEST_TIMEOUT` | `30s` | Per-request HTTP timeout |
| `WIKIGRAPH_SCRAPER_USER_AGENT` | `WikiGraph/1.0` | HTTP User-Agent |
| `WIKIGRAPH_SCRAPER_WIKIPEDIA_API_URL` | `https://en.wikipedia.org/api/rest_v1` | Wikipedia API base |

### API Server

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_API_HOST` | `localhost` | Bind address |
| `WIKIGRAPH_API_PORT` | `8080` | Listen port |
| `WIKIGRAPH_API_ENABLE_CORS` | `true` | Enable CORS middleware |
| `WIKIGRAPH_API_CORS_ORIGINS` | `["*"]` | Allowed CORS origins |
| `WIKIGRAPH_API_READ_TIMEOUT` | `30s` | HTTP read deadline |
| `WIKIGRAPH_API_WRITE_TIMEOUT` | `30s` | HTTP write deadline |
| `WIKIGRAPH_API_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout |
| `WIKIGRAPH_API_RATE_LIMIT` | `100.0` | Token bucket refill rate (req/s per IP) |
| `WIKIGRAPH_API_RATE_BURST` | `200` | Token bucket burst capacity |
| `WIKIGRAPH_API_PRODUCTION` | `false` | Gin release mode |

### Graph Cache

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_GRAPH_CACHE_PATH` | `""` | Path to gob cache file; empty = auto |
| `WIKIGRAPH_GRAPH_MAX_CACHE_AGE` | `24h` | Age beyond which cache triggers rebuild |
| `WIKIGRAPH_GRAPH_REFRESH_INTERVAL` | `5m` | Incremental DB check interval |
| `WIKIGRAPH_GRAPH_FORCE_REBUILD` | `false` | Bypass cache and rebuild on startup |

### Neo4j

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_NEO4J_ENABLED` | `false` | Enable Neo4j backend |
| `WIKIGRAPH_NEO4J_URI` | `bolt://localhost:7687` | Bolt connection URI |
| `WIKIGRAPH_NEO4J_USERNAME` | `neo4j` | Auth username |
| `WIKIGRAPH_NEO4J_PASSWORD` | `wikigraph` | Auth password |
| `WIKIGRAPH_NEO4J_MAX_CONNECTION_POOL_SIZE` | `50` | Max driver connections |
| `WIKIGRAPH_NEO4J_CONNECTION_ACQUISITION_TIMEOUT` | `60s` | Connection wait timeout |
| `WIKIGRAPH_NEO4J_SYNC_INTERVAL` | `5m` | SQLite → Neo4j sync frequency |
| `WIKIGRAPH_NEO4J_SYNC_BATCH_SIZE` | `10000` | Nodes/edges per sync batch |

### Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |

---

## Command-Line Flags

The `serve` command accepts flags that override both config file and environment:

```
--port int          API server port (default: config value)
--host string       API server host (default: config value)
--rebuild-cache     Force graph cache rebuild on startup
--production        Enable Gin release mode
```

The `fetch` command:

```
--depth int         Crawl depth (overrides scraper.max_depth)
--max-pages int     Maximum pages to crawl
```

The `path` command:

```
--algorithm string  bfs or bidirectional (default: bfs)
--max-depth int     Maximum path length (default: 6)
--format string     text or json (default: text)
```

---

## Common Deployment Configs

### Local development (defaults)

No config file needed. Run `wikigraph serve` directly.

### Docker with Neo4j

```yaml
database:
  path: /data/wikigraph.db

api:
  host: 0.0.0.0
  port: 8080
  production: true

graph:
  cache_path: /data/graph.cache

neo4j:
  enabled: true
  uri: bolt://neo4j:7687
  username: neo4j
  password: ${NEO4J_PASSWORD}
```

Or via environment:

```bash
WIKIGRAPH_DATABASE_PATH=/data/wikigraph.db \
WIKIGRAPH_API_HOST=0.0.0.0 \
WIKIGRAPH_API_PRODUCTION=true \
WIKIGRAPH_NEO4J_ENABLED=true \
WIKIGRAPH_NEO4J_URI=bolt://neo4j:7687 \
WIKIGRAPH_NEO4J_PASSWORD=secret \
wikigraph serve
```

### High-throughput crawling

```yaml
scraper:
  rate_limit: 200.0
  max_concurrent: 50
  request_timeout: 20s

api:
  rate_limit: 500.0
  rate_burst: 1000
```
