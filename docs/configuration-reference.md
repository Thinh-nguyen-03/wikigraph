# Configuration Reference

This document describes all configuration options for WikiGraph.

---

## Configuration Methods

WikiGraph supports configuration through:

1. **Environment variables** (highest priority)
2. **Config file** (config.yaml)
3. **Command-line flags**
4. **Default values** (lowest priority)

---

## Environment Variables

### Database

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_DB_PATH` | `./data/wikigraph.db` | Path to SQLite database file |
| `WIKIGRAPH_DB_MAX_CONNECTIONS` | `10` | Maximum database connections |
| `WIKIGRAPH_DB_WAL_MODE` | `true` | Enable WAL mode for better concurrency |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_PORT` | `8080` | HTTP server port |
| `WIKIGRAPH_HOST` | `0.0.0.0` | HTTP server host |
| `WIKIGRAPH_READ_TIMEOUT` | `30s` | HTTP read timeout |
| `WIKIGRAPH_WRITE_TIMEOUT` | `30s` | HTTP write timeout |
| `WIKIGRAPH_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout |

### Scraper

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_RATE_LIMIT` | `1s` | Delay between Wikipedia requests |
| `WIKIGRAPH_REQUEST_TIMEOUT` | `45s` | HTTP request timeout |
| `WIKIGRAPH_USER_AGENT` | `WikiGraph/1.0` | HTTP User-Agent header |
| `WIKIGRAPH_MAX_RETRIES` | `3` | Maximum retry attempts |
| `WIKIGRAPH_RETRY_DELAY` | `5s` | Delay between retries |

### Cache

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_CACHE_TTL` | `168h` | Cache time-to-live (7 days) |
| `WIKIGRAPH_CACHE_CLEANUP_INTERVAL` | `1h` | Interval for cache cleanup |

### Embeddings

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_EMBEDDINGS_URL` | `http://localhost:8001` | Embeddings service URL |
| `WIKIGRAPH_EMBEDDINGS_TIMEOUT` | `30s` | Embeddings request timeout |
| `WIKIGRAPH_EMBEDDINGS_BATCH_SIZE` | `32` | Batch size for embedding requests |

### Graph Caching (Performance Critical)

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_GRAPH_CACHE_PATH` | `""` | Path to graph cache file (empty = same dir as database) |
| `WIKIGRAPH_GRAPH_MAX_CACHE_AGE` | `24h` | Maximum cache age before forced rebuild |
| `WIKIGRAPH_GRAPH_REFRESH_INTERVAL` | `5m` | Interval for checking incremental updates |
| `WIKIGRAPH_GRAPH_FORCE_REBUILD` | `false` | Force rebuild on startup (ignores cache) |

### Graph Algorithms

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_MAX_PATH_DEPTH` | `6` | Maximum pathfinding depth |
| `WIKIGRAPH_PATH_TIMEOUT` | `30s` | Pathfinding timeout |
| `WIKIGRAPH_MAX_CRAWL_PAGES` | `10000` | Maximum pages per crawl job |
| `WIKIGRAPH_MAX_CRAWL_DEPTH` | `3` | Maximum crawl depth |

### Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `WIKIGRAPH_LOG_FORMAT` | `json` | Log format (json, text) |
| `WIKIGRAPH_LOG_FILE` | `` | Log file path (empty = stdout) |

### API

| Variable | Default | Description |
|----------|---------|-------------|
| `WIKIGRAPH_API_RATE_LIMIT` | `60/m` | API rate limit per client |
| `WIKIGRAPH_API_CORS_ORIGINS` | `*` | CORS allowed origins |
| `WIKIGRAPH_API_MAX_PAGE_SIZE` | `100` | Maximum items per page |

---

## Config File

Create `config.yaml` in the working directory or specify with `--config` flag.

```yaml
# config.yaml

# Database settings
database:
  path: ./data/wikigraph.db
  max_connections: 10
  wal_mode: true

# HTTP server settings
server:
  port: 8080
  host: 0.0.0.0
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 10s

# Wikipedia scraper settings
scraper:
  rate_limit: 1s
  request_timeout: 45s
  user_agent: "WikiGraph/1.0 (github.com/yourusername/wikigraph)"
  max_retries: 3
  retry_delay: 5s

# Cache settings
cache:
  ttl: 168h  # 7 days
  cleanup_interval: 1h

# Embeddings service settings
embeddings:
  url: http://localhost:8001
  timeout: 30s
  batch_size: 32

# Graph caching settings (CRITICAL FOR PERFORMANCE)
graph:
  cache_path: ""              # Empty = same directory as database
  max_cache_age: 24h          # Force rebuild after this age
  refresh_interval: 5m        # Check for DB updates every 5 minutes
  force_rebuild: false        # Force rebuild on startup

# Logging settings
logging:
  level: info
  format: json
  file: ""  # Empty = stdout

# API settings
api:
  rate_limit: "60/m"
  cors_origins:
    - "*"
  max_page_size: 100
```

---

## Command-Line Flags

### Global Flags

```bash
wikigraph --help

Flags:
  -c, --config string   Config file path (default "config.yaml")
  -v, --verbose         Enable verbose logging
      --version         Print version and exit
  -h, --help            Help for wikigraph
```

### Serve Command

```bash
wikigraph serve --help

Start the WikiGraph API server.

Usage:
  wikigraph serve [flags]

Flags:
  -p, --port int                Server port (default 8080)
      --host string             Server host (default "0.0.0.0")
      --read-timeout duration   Read timeout (default 30s)
      --write-timeout duration  Write timeout (default 30s)
  -h, --help                    Help for serve
```

### Fetch Command

```bash
wikigraph fetch --help

Fetch a Wikipedia page and display its links.

Usage:
  wikigraph fetch <title> [flags]

Flags:
  -n, --max-links int       Maximum links to display (default: all)
  -f, --format string       Output format: text, json, csv (default "text")
      --bypass-cache        Force fetch from Wikipedia
      --include-anchors     Include anchor text in output
  -h, --help                Help for fetch

Examples:
  wikigraph fetch "Albert Einstein"
  wikigraph fetch "Go_(programming_language)" --max-links=50 --format=json
```

### Crawl Command

```bash
wikigraph crawl --help

Crawl Wikipedia starting from a page.

Usage:
  wikigraph crawl <title> [flags]

Flags:
  -d, --depth int         Maximum crawl depth (default 2)
  -m, --max-pages int     Maximum pages to crawl (default 100)
  -t, --timeout duration  Crawl timeout (default 30m)
      --async             Run crawl in background
  -h, --help              Help for crawl

Examples:
  wikigraph crawl "Physics" --depth=2 --max-pages=500
  wikigraph crawl "Albert Einstein" --async
```

### Path Command

```bash
wikigraph path --help

Find the shortest path between two Wikipedia pages.

Usage:
  wikigraph path <from> <to> [flags]

Flags:
  -d, --max-depth int     Maximum path length (default 6)
  -t, --timeout duration  Search timeout (default 30s)
  -f, --format string     Output format: text, json (default "text")
  -h, --help              Help for path

Examples:
  wikigraph path "Albert Einstein" "Barack Obama"
  wikigraph path "Potato" "Philosophy" --max-depth=10
```

### Similar Command

```bash
wikigraph similar --help

Find semantically similar Wikipedia pages.

Usage:
  wikigraph similar <title> [flags]

Flags:
  -n, --limit int         Number of results (default 10)
      --min-score float   Minimum similarity score (default 0.5)
  -f, --format string     Output format: text, json (default "text")
  -h, --help              Help for similar

Examples:
  wikigraph similar "World War II" --limit=20
  wikigraph similar "Machine learning" --min-score=0.7
```

### Cache Command

```bash
wikigraph cache --help

Manage the page cache.

Usage:
  wikigraph cache <subcommand> [flags]

Subcommands:
  stats       Show cache statistics
  clear       Clear the cache
  invalidate  Invalidate specific pages

Examples:
  wikigraph cache stats
  wikigraph cache clear --older-than=30d
  wikigraph cache invalidate "Albert Einstein"
```

### Migrate Command

```bash
wikigraph migrate --help

Run database migrations.

Usage:
  wikigraph migrate [flags]

Flags:
      --dry-run    Show migrations without applying
      --rollback   Rollback last migration
  -h, --help       Help for migrate
```

---

## Configuration Examples

### Development

```yaml
# config.dev.yaml
database:
  path: ./data/wikigraph-dev.db

server:
  port: 8080

scraper:
  rate_limit: 2s  # Slower to avoid rate limits during testing

logging:
  level: debug
  format: text

api:
  cors_origins:
    - "http://localhost:3000"
    - "http://localhost:5173"
```

### Production

```yaml
# config.prod.yaml
database:
  path: /app/data/wikigraph.db
  max_connections: 25
  wal_mode: true

server:
  port: 8080
  read_timeout: 15s
  write_timeout: 15s

scraper:
  rate_limit: 1s
  request_timeout: 30s
  user_agent: "WikiGraph/1.0 (https://yoursite.com; contact@yoursite.com)"

cache:
  ttl: 336h  # 14 days in production

embeddings:
  url: http://embeddings:8001
  timeout: 10s

logging:
  level: info
  format: json

api:
  rate_limit: "100/m"
  cors_origins:
    - "https://yoursite.com"
```

### Docker Compose Override

```yaml
# docker-compose.override.yml (for local development)
services:
  api:
    environment:
      - WIKIGRAPH_LOG_LEVEL=debug
      - WIKIGRAPH_LOG_FORMAT=text
    volumes:
      - ./data:/app/data
```

---

## Duration Format

Duration values support Go duration format:

- `s` - seconds (e.g., `30s`)
- `m` - minutes (e.g., `5m`)
- `h` - hours (e.g., `2h`)
- `d` - days (e.g., `7d`)

Examples:
- `30s` = 30 seconds
- `5m` = 5 minutes
- `2h30m` = 2 hours 30 minutes
- `168h` = 7 days

---

## Rate Limit Format

Rate limits use the format `<count>/<period>`:

- `60/m` = 60 requests per minute
- `1000/h` = 1000 requests per hour
- `10/s` = 10 requests per second

---

## Validation

WikiGraph validates configuration on startup. Invalid configuration will cause the application to exit with an error message.

```bash
# Test configuration without starting
wikigraph serve --config=config.yaml --dry-run
```

Common validation errors:

```
Error: invalid configuration
  - database.path: must be a valid file path
  - scraper.rate_limit: must be at least 500ms
  - cache.ttl: must be at least 1h
```

---

## Environment-Specific Files

WikiGraph looks for config files in this order:

1. Path specified by `--config` flag
2. `config.local.yaml` (gitignored, for local overrides)
3. `config.yaml`

You can also use environment-specific files:

```bash
# Set environment
export WIKIGRAPH_ENV=production

# WikiGraph will look for config.production.yaml
wikigraph serve
```
