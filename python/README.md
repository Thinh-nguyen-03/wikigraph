# WikiGraph Embeddings Service

A FastAPI microservice that provides text embedding functionality using sentence-transformers.

## Setup

```bash
# Create virtual environment
python -m venv .venv
source .venv/bin/activate  # On Windows: .venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt

# Run the service
python main.py
```

## API Endpoints

### Health Check

```
GET /health
```

### Generate Embedding

```
POST /embed
Content-Type: application/json

{
  "text": "Albert Einstein"
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

### Batch Embeddings

```
POST /embed/batch
Content-Type: application/json

{
  "texts": ["Albert Einstein", "Isaac Newton", "Marie Curie"]
}
```

### Calculate Similarity

```
POST /similarity
Content-Type: application/json

{
  "text1": "Albert Einstein",
  "text2": "Isaac Newton"
}
```

Response:

```json
{
  "score": 0.847,
  "text1": "Albert Einstein",
  "text2": "Isaac Newton"
}
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MODEL_NAME` | `all-MiniLM-L6-v2` | Sentence transformer model |
| `HOST` | `0.0.0.0` | Server host |
| `PORT` | `8001` | Server port |
| `WORKERS` | `1` | Number of workers |

## Docker

```bash
docker build -t wikigraph-embeddings .
docker run -p 8001:8001 wikigraph-embeddings
```
