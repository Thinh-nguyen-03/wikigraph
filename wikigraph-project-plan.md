# WikiGraph Project Plan

**One-liner**: A tool that crawls Wikipedia, builds a knowledge graph of connections between people, places, and events, and lets users explore relationships.

**Timeline**: ~12 weeks at 10-15 hours/week

---

## Table of Contents

- [Phase 0: Setup](#phase-0-setup-week-1)
- [Phase 1: Scraper + Cache](#phase-1-scraper--cache-weeks-2-3)
- [Phase 2: Graph Construction](#phase-2-graph-construction-weeks-4-5)
- [Phase 3: Embeddings Microservice](#phase-3-embeddings-microservice-week-6)
- [Phase 4: API Layer](#phase-4-api-layer-weeks-7-8)
- [Phase 5: Frontend](#phase-5-frontend-weeks-9-10)
- [Phase 6: Polish](#phase-6-polish-weeks-11-12)
- [Timeline Summary](#timeline-summary)
- [Progress Tracking](#progress-tracking)
- [Risk Mitigation](#risk-mitigation)

---

## Phase 0: Setup (Week 1)

**Goal**: Dev environment ready, empty project compiles, basic decisions documented.

| Task | Details | Done when |
|------|---------|-----------|
| Initialize Go module | `go mod init github.com/yourname/wikigraph` | Compiles |
| Set up project structure | See below | Directories exist |
| Choose and install dependencies | `colly`, `gin`, `modernc.org/sqlite` | `go build` works |
| Create README with project overview | Problem, approach, planned features | Readable by a stranger |
| Set up basic Makefile or Taskfile | `make run`, `make test`, `make build` | Commands work |
| Initialize SQLite database | Empty schema, migrations folder | Can connect |

### Proposed Directory Structure

```
wikigraph/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── scraper/
│   ├── cache/
│   ├── graph/
│   ├── embeddings/
│   └── api/
├── pkg/
│   └── wikipedia/
├── migrations/
├── scripts/
├── python/              # embedding microservice
│   ├── main.py
│   └── requirements.txt
├── web/                 # frontend (later)
├── go.mod
├── Makefile
└── README.md
```

### Deliverable

Empty project that runs, prints "WikiGraph starting" and exits.

### Checklist

- [ ] Go module initialized
- [ ] Directory structure created
- [ ] Dependencies installed
- [ ] README started
- [ ] Makefile working
- [ ] SQLite connection working

---

## Phase 1: Scraper + Cache (Weeks 2-3)

**Goal**: Fetch a Wikipedia page, parse its links, store in SQLite, don't refetch cached pages.

| Task | Details | Done when |
|------|---------|-----------|
| Implement basic fetcher | Fetch HTML for a given Wikipedia title | Returns HTML string |
| Parse links from page | Extract internal wiki links, ignore citations/external | Returns `[]string` of titles |
| Design SQLite schema | `pages`, `links` tables | Migrations run |
| Implement page cache | Check if page exists before fetching | Cache hits work |
| Store links in database | Insert source→target relationships | Query returns links |
| Add rate limiting | Respect Wikipedia, max 1 req/sec | No 429 errors |
| Handle edge cases | Redirects, disambiguation pages, missing pages | Doesn't crash |
| Write tests | Table-driven tests for parser, cache | `go test` passes |

### SQLite Schema v1

```sql
CREATE TABLE pages (
    id INTEGER PRIMARY KEY,
    title TEXT UNIQUE NOT NULL,
    fetched_at TIMESTAMP,
    content_hash TEXT
);

CREATE TABLE links (
    id INTEGER PRIMARY KEY,
    source_id INTEGER REFERENCES pages(id),
    target_title TEXT NOT NULL,
    anchor_text TEXT
);

CREATE INDEX idx_links_source ON links(source_id);
CREATE INDEX idx_links_target ON links(target_title);
```

### Deliverable

CLI command that takes a Wikipedia title, fetches it, stores links, and prints them.

```bash
./wikigraph fetch "Albert_Einstein"
# Fetched 347 links, cached in 0.8s
```

### Checklist

- [ ] Basic HTTP fetcher working
- [ ] HTML link parser working
- [ ] SQLite schema created
- [ ] Page caching implemented
- [ ] Links stored in database
- [ ] Rate limiting added
- [ ] Edge cases handled
- [ ] Tests written and passing

---

## Phase 2: Graph Construction (Weeks 4-5)

**Goal**: Crawl multiple pages deep, build a navigable graph, implement basic pathfinding.

| Task | Details | Done when |
|------|---------|-----------|
| Implement BFS crawler | Start from page, crawl N levels deep | Populates DB with subgraph |
| Add crawl depth/limit controls | Max pages, max depth, timeout | Configurable via flags |
| Build in-memory graph from DB | Load nodes and edges into Go structs | Graph struct populated |
| Implement BFS pathfinding | Find shortest path between two titles | Returns path or "not found" |
| Add bidirectional search (optional) | Faster for large graphs | Path found quicker |
| Write tests | Pathfinding on known small graphs | Correct paths returned |

### Graph Representation in Go

```go
type Graph struct {
    Nodes map[string]*Node
    mu    sync.RWMutex
}

type Node struct {
    Title    string
    OutLinks []string
    InLinks  []string
}
```

### Deliverable

CLI that crawls from a starting page and finds paths.

```bash
./wikigraph crawl "Albert_Einstein" --depth=2 --max-pages=500
# Crawled 500 pages in 8m23s

./wikigraph path "Albert_Einstein" "Barack_Obama"
# Found path (4 hops): Albert_Einstein → Princeton → United_States → Barack_Obama
```

### Checklist

- [ ] BFS crawler implemented
- [ ] Crawl controls (depth, max pages, timeout)
- [ ] In-memory graph loading
- [ ] BFS pathfinding working
- [ ] Bidirectional search (stretch goal)
- [ ] Tests written and passing

---

## Phase 3: Embeddings Microservice (Week 6)

**Goal**: Python service that computes and returns embeddings, Go client that calls it.

| Task | Details | Done when |
|------|---------|-----------|
| Set up Python service | FastAPI + sentence-transformers | Runs on port 8001 |
| Implement `/embed` endpoint | Takes text, returns vector | Returns 384-dim float array |
| Implement `/similarity` endpoint | Takes two texts, returns score | Returns float 0-1 |
| Write Go client | HTTP calls to Python service | Can fetch embeddings |
| Store embeddings in SQLite | BLOB or separate file | Persisted |
| Implement similar page lookup | Given a page, find semantically similar ones | Returns ranked list |
| Add Docker Compose for both services | Go + Python run together | `docker-compose up` works |

### Python Service (Minimal)

```python
from fastapi import FastAPI
from sentence_transformers import SentenceTransformer

app = FastAPI()
model = SentenceTransformer('all-MiniLM-L6-v2')

@app.post("/embed")
def embed(text: str):
    vector = model.encode(text).tolist()
    return {"vector": vector}

@app.post("/similarity")
def similarity(text1: str, text2: str):
    embeddings = model.encode([text1, text2])
    score = cosine_similarity([embeddings[0]], [embeddings[1]])[0][0]
    return {"score": float(score)}
```

### Deliverable

Can query for pages similar to a given page.

```bash
./wikigraph similar "World_War_II"
# Similar pages: World_War_I (0.89), Nazi_Germany (0.84), Allied_Powers (0.81)
```

### Checklist

- [ ] Python FastAPI service running
- [ ] `/embed` endpoint working
- [ ] `/similarity` endpoint working
- [ ] Go HTTP client implemented
- [ ] Embeddings stored in SQLite
- [ ] Similar page lookup working
- [ ] Docker Compose configuration

---

## Phase 4: API Layer (Weeks 7-8)

**Goal**: RESTful API exposing all functionality.

| Task | Details | Done when |
|------|---------|-----------|
| Set up Gin router | Basic structure, health endpoint | `/health` returns 200 |
| `GET /page/:title` | Fetch/cache page, return links | Returns JSON |
| `GET /path` | Find path between two pages | Returns path array |
| `GET /connections/:title` | Return N-hop neighborhood | Returns subgraph JSON |
| `GET /similar/:title` | Return similar pages via embeddings | Returns ranked list |
| `POST /crawl` | Trigger background crawl | Returns job ID |
| Add request validation | Bad inputs return 400 | Doesn't crash |
| Add basic error handling | Consistent error response format | Clean errors |
| Write API tests | HTTP tests for each endpoint | `go test` passes |

### Example API Responses

```json
// GET /path?from=Albert_Einstein&to=Barack_Obama
{
  "found": true,
  "path": ["Albert_Einstein", "Princeton", "United_States", "Barack_Obama"],
  "hops": 3,
  "computed_in_ms": 45
}

// GET /page/Albert_Einstein
{
  "title": "Albert_Einstein",
  "links": ["Physics", "Germany", "Princeton", "..."],
  "link_count": 347,
  "cached": true
}

// GET /similar/World_War_II
{
  "query": "World_War_II",
  "similar": [
    {"title": "World_War_I", "score": 0.89},
    {"title": "Nazi_Germany", "score": 0.84},
    {"title": "Allied_Powers", "score": 0.81}
  ]
}
```

### Deliverable

Fully functional API, documented with examples in README.

### Checklist

- [ ] Gin router set up
- [ ] `/health` endpoint
- [ ] `GET /page/:title` endpoint
- [ ] `GET /path` endpoint
- [ ] `GET /connections/:title` endpoint
- [ ] `GET /similar/:title` endpoint
- [ ] `POST /crawl` endpoint
- [ ] Request validation
- [ ] Error handling
- [ ] API tests passing

---

## Phase 5: Frontend (Weeks 9-10)

**Goal**: Simple interactive visualization.

| Task | Details | Done when |
|------|---------|-----------|
| Choose framework | Vis.js for speed, D3 if ambitious | Decision made |
| Set up basic HTML/JS | Connects to API | Loads without errors |
| Implement graph visualization | Display nodes and edges | Renders a subgraph |
| Add search box | Enter a title, center graph on it | Works |
| Add pathfinding UI | Enter two titles, highlight path | Path shown |
| Style it minimally | Doesn't look broken | Presentable |
| Serve from Go | Embed static files or serve directory | Single binary serves frontend |

### Deliverable

Can open browser, search for a page, see its connections, find paths.

### Checklist

- [ ] Framework chosen
- [ ] Basic HTML/JS setup
- [ ] Graph visualization working
- [ ] Search functionality
- [ ] Pathfinding UI
- [ ] Basic styling
- [ ] Static files served from Go

---

## Phase 6: Polish (Weeks 11-12)

**Goal**: Production-ready enough to demo confidently.

| Task | Details | Done when |
|------|---------|-----------|
| Write comprehensive README | Setup, usage, architecture, screenshots | A stranger can run it |
| Add architecture diagram | Draw the system | Included in README |
| Dockerize everything | Single `docker-compose up` | Works on fresh machine |
| Deploy somewhere | Fly.io, Railway, or a VPS | Live URL |
| Record demo video or GIF | 30-60 second walkthrough | Embedded in README |
| Clean up code | Remove dead code, consistent formatting | `golangci-lint` passes |
| Add meaningful tests | Cover critical paths | 60%+ coverage on core packages |
| Write "lessons learned" section | What you'd do differently | In README or blog post |

### Deliverable

Deployed, documented project ready for portfolio.

### Checklist

- [ ] Comprehensive README
- [ ] Architecture diagram
- [ ] Docker Compose working
- [ ] Deployed to hosting provider
- [ ] Demo video/GIF recorded
- [ ] Code cleaned up
- [ ] Test coverage adequate
- [ ] Lessons learned documented

---

## Timeline Summary

| Phase | Weeks | Milestone |
|-------|-------|-----------|
| 0: Setup | 1 | Project compiles |
| 1: Scraper + Cache | 2-3 | Can fetch and cache pages |
| 2: Graph | 4-5 | Can crawl and find paths |
| 3: Embeddings | 6 | Semantic similarity works |
| 4: API | 7-8 | Full REST API |
| 5: Frontend | 9-10 | Interactive visualization |
| 6: Polish | 11-12 | Deployed and documented |

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                      Frontend                           │
│              (Vis.js or D3 graph viz)                   │
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
│   (colly)    │ │  (BFS/  │ │   Go Client     │
│              │ │   DFS)  │ │                 │
└──────┬───────┘ └────┬────┘ └────────┬────────┘
       │              │               │
       └──────────────┼───────────────┘
                      ▼
            ┌──────────────────┐
            │     SQLite       │
            │ pages / links /  │
            │   embeddings     │
            └──────────────────┘
                      
                      │ HTTP (internal)
                      ▼
            ┌──────────────────┐
            │  Python Service  │
            │  (FastAPI +      │
            │  sentence-       │
            │  transformers)   │
            └──────────────────┘
```

---

## Progress Tracking

Copy this section to track your progress:

```markdown
## Current Status

**Current Phase**: 0 - Setup
**Week**: 1
**Last Updated**: YYYY-MM-DD

### Phase 0: Setup
- [ ] Go module initialized
- [ ] Directory structure created
- [ ] Dependencies installed
- [ ] README started
- [ ] Makefile working
- [ ] SQLite connection working

### Phase 1: Scraper + Cache
- [ ] Basic HTTP fetcher
- [ ] HTML link parser
- [ ] SQLite schema
- [ ] Page caching
- [ ] Links storage
- [ ] Rate limiting
- [ ] Edge case handling
- [ ] Tests

### Phase 2: Graph Construction
- [ ] BFS crawler
- [ ] Crawl controls
- [ ] In-memory graph
- [ ] BFS pathfinding
- [ ] Bidirectional search
- [ ] Tests

### Phase 3: Embeddings
- [ ] Python service
- [ ] /embed endpoint
- [ ] /similarity endpoint
- [ ] Go client
- [ ] SQLite storage
- [ ] Similar page lookup
- [ ] Docker Compose

### Phase 4: API
- [ ] Gin router
- [ ] /health endpoint
- [ ] /page endpoint
- [ ] /path endpoint
- [ ] /connections endpoint
- [ ] /similar endpoint
- [ ] /crawl endpoint
- [ ] Validation
- [ ] Error handling
- [ ] Tests

### Phase 5: Frontend
- [ ] Framework chosen
- [ ] Basic setup
- [ ] Graph visualization
- [ ] Search
- [ ] Pathfinding UI
- [ ] Styling
- [ ] Static file serving

### Phase 6: Polish
- [ ] README
- [ ] Architecture diagram
- [ ] Docker
- [ ] Deployment
- [ ] Demo video
- [ ] Code cleanup
- [ ] Test coverage
- [ ] Lessons learned
```

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Scope creep | Stick to the plan, add features only after Phase 6 |
| Wikipedia blocks you | Respect rate limits, use proper User-Agent header |
| Embeddings too slow | Cache aggressively, compute in batches |
| Go graph libraries lacking | Roll your own BFS/DFS—it's not hard and shows fundamentals |
| Frontend takes too long | Fall back to Streamlit or basic HTML if stuck |
| Burnout | 10-15 hrs/week is sustainable; don't sprint |

---

## Tech Stack Summary

| Component | Technology | Why |
|-----------|------------|-----|
| Language (main) | Go | Performance, shows systems thinking |
| Language (ML) | Python | Best ecosystem for embeddings |
| Web scraping | Colly | Built for Go, handles rate limiting |
| Database | SQLite | Zero setup, portable, sufficient for this scale |
| API framework | Gin | Fast, minimal, well-documented |
| Embeddings | sentence-transformers | Free, local, good quality |
| ML API | FastAPI | Simple, async, auto-docs |
| Frontend | Vis.js or D3.js | Interactive graph visualization |
| Containerization | Docker Compose | Single command to run everything |

---

## Resources

- [Colly documentation](http://go-colly.org/)
- [Gin documentation](https://gin-gonic.com/docs/)
- [sentence-transformers](https://www.sbert.net/)
- [Vis.js Network](https://visjs.github.io/vis-network/docs/network/)
- [Wikipedia API](https://www.mediawiki.org/wiki/API:Main_page)
- [SQLite in Go (modernc)](https://pkg.go.dev/modernc.org/sqlite)
