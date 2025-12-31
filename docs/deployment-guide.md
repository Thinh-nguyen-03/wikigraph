# Deployment Guide

This guide covers deploying WikiGraph to various environments.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Local Development](#local-development)
- [Docker Deployment](#docker-deployment)
- [Cloud Deployment](#cloud-deployment)
  - [Fly.io](#flyio)
  - [Railway](#railway)
  - [DigitalOcean](#digitalocean)
- [Production Considerations](#production-considerations)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

- Docker and Docker Compose
- Git
- A cloud account (Fly.io, Railway, or DigitalOcean)

---

## Local Development

### Using Docker Compose

```bash
# Clone the repository
git clone https://github.com/yourusername/wikigraph.git
cd wikigraph

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Manual Setup

```bash
# Terminal 1: Run the Go API
make build
./wikigraph serve

# Terminal 2: Run the Python embeddings service
cd python
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
python main.py
```

---

## Docker Deployment

### Build Images

```bash
# Build Go API image
docker build -t wikigraph:latest .

# Build Python embeddings image
docker build -t wikigraph-embeddings:latest ./python
```

### Run Containers

```bash
# Create network
docker network create wikigraph-network

# Run embeddings service
docker run -d \
  --name wikigraph-embeddings \
  --network wikigraph-network \
  -p 8001:8001 \
  wikigraph-embeddings:latest

# Run API service
docker run -d \
  --name wikigraph-api \
  --network wikigraph-network \
  -p 8080:8080 \
  -e WIKIGRAPH_EMBEDDINGS_URL=http://wikigraph-embeddings:8001 \
  -v wikigraph-data:/app/data \
  wikigraph:latest
```

---

## Cloud Deployment

### Fly.io

Fly.io offers a simple deployment experience with a generous free tier.

#### Setup

```bash
# Install flyctl
curl -L https://fly.io/install.sh | sh

# Login
fly auth login

# Create app
fly launch --name wikigraph
```

#### fly.toml

```toml
app = "wikigraph"
primary_region = "sjc"

[build]
  dockerfile = "Dockerfile"

[env]
  WIKIGRAPH_PORT = "8080"
  WIKIGRAPH_EMBEDDINGS_URL = "http://wikigraph-embeddings.internal:8001"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  min_machines_running = 0

[[vm]]
  cpu_kind = "shared"
  cpus = 1
  memory_mb = 512

[mounts]
  source = "wikigraph_data"
  destination = "/app/data"
```

#### Deploy

```bash
# Create volume for data persistence
fly volumes create wikigraph_data --size 1

# Deploy
fly deploy

# Check status
fly status

# View logs
fly logs
```

#### Deploy Embeddings Service

Create a separate Fly app for the embeddings service:

```bash
cd python
fly launch --name wikigraph-embeddings
```

```toml
# python/fly.toml
app = "wikigraph-embeddings"
primary_region = "sjc"

[build]
  dockerfile = "Dockerfile"

[env]
  PORT = "8001"

[http_service]
  internal_port = 8001
  force_https = true
  auto_stop_machines = false  # Keep running for model loading
  min_machines_running = 1

[[vm]]
  cpu_kind = "shared"
  cpus = 2
  memory_mb = 2048  # Embeddings model needs more memory
```

---

### Railway

Railway provides easy deployment with automatic builds.

#### Setup

1. Create account at [railway.app](https://railway.app)
2. Connect your GitHub repository
3. Create new project from repo

#### Environment Variables

Set these in the Railway dashboard:

```
WIKIGRAPH_PORT=8080
WIKIGRAPH_DB_PATH=/app/data/wikigraph.db
WIKIGRAPH_EMBEDDINGS_URL=http://embeddings.railway.internal:8001
```

#### railway.toml

```toml
[build]
builder = "dockerfile"
dockerfilePath = "Dockerfile"

[deploy]
healthcheckPath = "/health"
healthcheckTimeout = 30
restartPolicyType = "on_failure"
```

---

### DigitalOcean

#### App Platform

1. Create a new App in DigitalOcean App Platform
2. Connect your GitHub repository
3. Configure the app:

```yaml
# .do/app.yaml
name: wikigraph
services:
  - name: api
    dockerfile_path: Dockerfile
    source_dir: /
    http_port: 8080
    instance_size_slug: basic-xxs
    instance_count: 1
    routes:
      - path: /
    envs:
      - key: WIKIGRAPH_EMBEDDINGS_URL
        value: ${embeddings.INTERNAL_URL}
    health_check:
      http_path: /health

  - name: embeddings
    dockerfile_path: python/Dockerfile
    source_dir: /python
    http_port: 8001
    instance_size_slug: basic-xs
    instance_count: 1
    health_check:
      http_path: /health

databases:
  - name: wikigraph-db
    engine: PG
    production: false
```

#### Droplet (VPS)

```bash
# SSH into droplet
ssh root@your-droplet-ip

# Install Docker
curl -fsSL https://get.docker.com | sh

# Clone repository
git clone https://github.com/yourusername/wikigraph.git
cd wikigraph

# Start services
docker-compose up -d
```

---

## Production Considerations

### Security

1. **HTTPS**: Always use HTTPS in production
2. **Rate Limiting**: Enable rate limiting to prevent abuse
3. **CORS**: Configure CORS for your frontend domain
4. **Secrets**: Use environment variables or secret management

```yaml
# docker-compose.prod.yml
services:
  api:
    environment:
      - WIKIGRAPH_RATE_LIMIT=100/minute
      - WIKIGRAPH_CORS_ORIGINS=https://yourdomain.com
```

### Performance

1. **Caching**: Enable aggressive caching
2. **Connection Pooling**: Use connection pooling for SQLite
3. **Embeddings**: Pre-compute embeddings for popular pages

### Persistence

1. **Volumes**: Use persistent volumes for database
2. **Backups**: Schedule regular backups

```bash
# Backup SQLite database
docker exec wikigraph-api sqlite3 /app/data/wikigraph.db ".backup /app/data/backup.db"
docker cp wikigraph-api:/app/data/backup.db ./backups/
```

### Scaling

For high traffic:

1. **Horizontal Scaling**: Run multiple API instances
2. **Load Balancer**: Use a load balancer (nginx, Caddy)
3. **Read Replicas**: Consider PostgreSQL for better scaling

---

## Monitoring

### Health Checks

```bash
# Check API health
curl http://localhost:8080/health

# Check embeddings health
curl http://localhost:8001/health
```

### Logging

```bash
# View Docker logs
docker-compose logs -f api

# View specific service logs
docker logs -f wikigraph-api --tail 100
```

### Metrics

Consider adding Prometheus metrics:

```go
// Add to API
import "github.com/prometheus/client_golang/prometheus/promhttp"

router.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

---

## Troubleshooting

### Common Issues

#### "Connection refused" to embeddings service

```bash
# Check if embeddings service is running
docker ps | grep embeddings

# Check embeddings logs
docker logs wikigraph-embeddings

# Verify network connectivity
docker exec wikigraph-api curl http://wikigraph-embeddings:8001/health
```

#### "Database is locked"

```bash
# Check for multiple connections
docker exec wikigraph-api sqlite3 /app/data/wikigraph.db "PRAGMA busy_timeout = 5000;"

# Enable WAL mode
docker exec wikigraph-api sqlite3 /app/data/wikigraph.db "PRAGMA journal_mode=WAL;"
```

#### Out of memory (embeddings)

```bash
# Increase container memory
docker update --memory=4g wikigraph-embeddings

# Or use a smaller model
MODEL_NAME=all-MiniLM-L6-v2  # 80MB vs paraphrase-multilingual: 1GB
```

#### Slow response times

1. Check cache hit rate
2. Verify embeddings service is warmed up
3. Check network latency between services

```bash
# Test latency
docker exec wikigraph-api time curl http://wikigraph-embeddings:8001/health
```

---

## Rollback

### Docker

```bash
# List images
docker images wikigraph

# Roll back to previous version
docker-compose down
docker tag wikigraph:latest wikigraph:broken
docker tag wikigraph:previous wikigraph:latest
docker-compose up -d
```

### Fly.io

```bash
# List releases
fly releases

# Roll back
fly deploy --image registry.fly.io/wikigraph:v123
```

---

## Checklist

Before deploying to production:

- [ ] All tests passing
- [ ] Environment variables configured
- [ ] HTTPS enabled
- [ ] Rate limiting configured
- [ ] Health checks working
- [ ] Logging configured
- [ ] Backup strategy in place
- [ ] Monitoring set up
- [ ] Rollback plan documented
