# Neo4j Setup Guide

This guide helps you set up and test the Neo4j integration for WikiGraph.

## Prerequisites

- Docker and Docker Compose installed
- Go 1.22 or later
- Existing WikiGraph SQLite database

## Quick Start

### 1. Start Neo4j

Start the Neo4j database using Docker Compose:

```bash
docker-compose up -d neo4j
```

This will:
- Pull the Neo4j 5.15 image
- Start Neo4j on ports 7474 (browser) and 7687 (bolt)
- Use credentials: username=`neo4j`, password=`wikigraph`

Verify Neo4j is running:
```bash
docker-compose ps
```

You should see the `wikigraph-neo4j` container running.

### 2. Access Neo4j Browser

Open your browser and navigate to:
```
http://localhost:7474
```

Login with:
- Username: `neo4j`
- Password: `wikigraph`

### 3. Enable Neo4j in Config

Edit [config.yaml](../config.yaml) and set:

```yaml
neo4j:
  enabled: true
  uri: "bolt://localhost:7687"
  username: "neo4j"
  password: "wikigraph"
```

Or use environment variables:
```bash
export WIKIGRAPH_NEO4J_ENABLED=true
export WIKIGRAPH_NEO4J_URI=bolt://localhost:7687
export WIKIGRAPH_NEO4J_USERNAME=neo4j
export WIKIGRAPH_NEO4J_PASSWORD=wikigraph
```

### 4. Run Initial Sync

Sync your SQLite data to Neo4j:

```bash
# Build the latest version
go build ./cmd/wikigraph

# Run sync (this will sync ALL data)
./wikigraph sync --batch-size 10000
```

Options:
- `--batch-size`: Number of nodes/edges per batch (default: 10000)
- `--clear`: Clear Neo4j database before syncing

**Note:** The initial sync with 162M edges will take 5-10 minutes. For testing, use a smaller test database.

### 5. Verify Sync

Check that the sync completed successfully:

```bash
./wikigraph sync verify
```

This compares node and edge counts between SQLite and Neo4j.

## Testing with Small Dataset

To test the Neo4j integration without syncing 162M edges:

### Option 1: Create a Test Database

```bash
# Create a fresh test database
rm -f wikigraph-test.db

# Crawl a small dataset (100 pages, depth 2)
./wikigraph fetch "Python_(programming_language)" \
  --database wikigraph-test.db \
  --max-pages 100 \
  --depth 2

# Update config to use test database
export WIKIGRAPH_DATABASE_PATH=wikigraph-test.db

# Sync to Neo4j
./wikigraph sync --clear
```

### Option 2: Use Existing Subset

If you have the full database but want to test with a subset, you'll need to create a filtered copy. The `--limit` flag in sync is not fully implemented yet.

## Querying Neo4j

Once synced, you can query the graph using Cypher in the Neo4j Browser:

### Count nodes and edges
```cypher
MATCH (n:Page) RETURN count(n) AS nodes
```

```cypher
MATCH ()-[r:LINKS_TO]->() RETURN count(r) AS edges
```

### Find shortest path
```cypher
MATCH (start:Page {title: "Python_(programming_language)"}),
      (end:Page {title: "Computer_science"})
MATCH path = shortestPath((start)-[:LINKS_TO*1..6]-(end))
RETURN [node in nodes(path) | node.title] AS path
```

### Get outgoing links
```cypher
MATCH (p:Page {title: "Python_(programming_language)"})-[:LINKS_TO]->(target)
RETURN target.title
LIMIT 20
```

### N-hop neighborhood
```cypher
MATCH (start:Page {title: "Python_(programming_language)"})-[:LINKS_TO*1..2]-(neighbor)
RETURN DISTINCT neighbor.title
LIMIT 50
```

## Performance Testing

Test query performance:

```cypher
// Warm up the cache
MATCH (n:Page) RETURN count(n)

// Time a pathfinding query
PROFILE MATCH (start:Page {title: "Python_(programming_language)"}),
               (end:Page {title: "Albert_Einstein"})
        MATCH path = shortestPath((start)-[:LINKS_TO*1..6]-(end))
        RETURN [node in nodes(path) | node.title]
```

The query should complete in 5-20ms for typical Wikipedia paths.

## Troubleshooting

### Neo4j won't start
```bash
# Check logs
docker-compose logs neo4j

# Restart
docker-compose restart neo4j
```

### Connection refused
- Verify Neo4j is running: `docker-compose ps`
- Check port 7687 is not blocked by firewall
- Verify credentials in config.yaml

### Sync is slow
- Increase `batch_size` to 20000 or 50000
- Check Docker container has enough memory (4GB heap + 2GB page cache)
- Monitor with: `docker stats wikigraph-neo4j`

### Out of memory during sync
Reduce heap size in [docker-compose.yml](../docker-compose.yml):
```yaml
- NEO4J_dbms_memory_heap_max__size=2G
- NEO4J_dbms_memory_pagecache_size=1G
```

## Next Steps

After successful sync and testing:

1. **Update API handlers** - Modify [internal/api/handlers.go](../internal/api/handlers.go) to query Neo4j
2. **Implement incremental sync** - Add background sync service
3. **Update serve command** - Initialize Neo4j connection in server startup
4. **Add benchmarks** - Compare in-memory vs Neo4j query performance
5. **Production deployment** - Set up separate Neo4j instance

See [graph-database-migration.md](graph-database-migration.md) for the full migration plan.

## Commands Reference

```bash
# Start Neo4j
docker-compose up -d neo4j

# Stop Neo4j
docker-compose stop neo4j

# View Neo4j logs
docker-compose logs -f neo4j

# Remove Neo4j data (start fresh)
docker-compose down -v
docker volume rm wikigraph_neo4j-data

# Sync data
./wikigraph sync

# Verify sync
./wikigraph sync verify

# Clear and re-sync
./wikigraph sync --clear
```
