# WikiGraph

A tool that crawls Wikipedia, builds a knowledge graph of connections between people, places, and events, and lets users explore relationships.

## Status

🚧 Under development

## Tech Stack

- **Backend**: Go (Gin, Colly, SQLite)
- **Embeddings**: Python (FastAPI, sentence-transformers)
- **Frontend**: TBD (Vis.js or D3.js)

## Getting Started

### Building

**Windows (PowerShell):**
```powershell
.\build.ps1 build
```

**Linux/macOS:**
```bash
make build
```

Or use Go directly:
```bash
go build -o wikigraph ./cmd/server
```

### Running

**Windows (PowerShell):**
```powershell
.\build.ps1 run
```

**Linux/macOS:**
```bash
make run
```

Or use Go directly:
```bash
go run ./cmd/server
```

## License

MIT

