"""
WikiGraph Embeddings Service

A FastAPI microservice for generating text embeddings using sentence-transformers.
"""

import os
import time
from typing import List, Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from sentence_transformers import SentenceTransformer
from sklearn.metrics.pairwise import cosine_similarity
import numpy as np
import uvicorn

# Configuration
MODEL_NAME = os.getenv("MODEL_NAME", "all-MiniLM-L6-v2")
HOST = os.getenv("HOST", "0.0.0.0")
PORT = int(os.getenv("PORT", "8001"))

# Initialize FastAPI app
app = FastAPI(
    title="WikiGraph Embeddings Service",
    description="Text embedding service using sentence-transformers",
    version="0.1.0",
)

# Load model at startup
print(f"Loading model: {MODEL_NAME}")
start_time = time.time()
model = SentenceTransformer(MODEL_NAME)
load_time = time.time() - start_time
print(f"Model loaded in {load_time:.2f}s")


# Request/Response models
class EmbedRequest(BaseModel):
    """Request for single text embedding."""
    text: str = Field(..., min_length=1, max_length=10000)


class EmbedBatchRequest(BaseModel):
    """Request for batch text embeddings."""
    texts: List[str] = Field(..., min_items=1, max_items=100)


class EmbedResponse(BaseModel):
    """Response containing embedding vector."""
    vector: List[float]
    dimensions: int
    model: str


class EmbedBatchResponse(BaseModel):
    """Response containing multiple embedding vectors."""
    vectors: List[List[float]]
    count: int
    dimensions: int
    model: str


class SimilarityRequest(BaseModel):
    """Request for similarity calculation."""
    text1: str = Field(..., min_length=1, max_length=10000)
    text2: str = Field(..., min_length=1, max_length=10000)


class SimilarityResponse(BaseModel):
    """Response containing similarity score."""
    score: float
    text1: str
    text2: str


class SimilarityBatchRequest(BaseModel):
    """Request for finding most similar texts."""
    query: str = Field(..., min_length=1, max_length=10000)
    candidates: List[str] = Field(..., min_items=1, max_items=1000)
    top_k: Optional[int] = Field(default=10, ge=1, le=100)


class SimilarityBatchResponse(BaseModel):
    """Response containing ranked similarity results."""
    query: str
    results: List[dict]


class HealthResponse(BaseModel):
    """Health check response."""
    status: str
    model: str
    model_dimensions: int


# Endpoints
@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    return HealthResponse(
        status="ok",
        model=MODEL_NAME,
        model_dimensions=model.get_sentence_embedding_dimension(),
    )


@app.post("/embed", response_model=EmbedResponse)
async def embed_text(request: EmbedRequest):
    """Generate embedding for a single text."""
    try:
        vector = model.encode(request.text).tolist()
        return EmbedResponse(
            vector=vector,
            dimensions=len(vector),
            model=MODEL_NAME,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/embed/batch", response_model=EmbedBatchResponse)
async def embed_batch(request: EmbedBatchRequest):
    """Generate embeddings for multiple texts."""
    try:
        vectors = model.encode(request.texts).tolist()
        return EmbedBatchResponse(
            vectors=vectors,
            count=len(vectors),
            dimensions=len(vectors[0]) if vectors else 0,
            model=MODEL_NAME,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/similarity", response_model=SimilarityResponse)
async def calculate_similarity(request: SimilarityRequest):
    """Calculate cosine similarity between two texts."""
    try:
        embeddings = model.encode([request.text1, request.text2])
        score = cosine_similarity([embeddings[0]], [embeddings[1]])[0][0]
        return SimilarityResponse(
            score=float(score),
            text1=request.text1,
            text2=request.text2,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/similarity/batch", response_model=SimilarityBatchResponse)
async def find_similar(request: SimilarityBatchRequest):
    """Find most similar texts from candidates."""
    try:
        # Encode query and candidates
        query_embedding = model.encode([request.query])[0]
        candidate_embeddings = model.encode(request.candidates)
        
        # Calculate similarities
        similarities = cosine_similarity([query_embedding], candidate_embeddings)[0]
        
        # Sort by similarity and get top_k
        indices = np.argsort(similarities)[::-1][:request.top_k]
        
        results = [
            {
                "text": request.candidates[i],
                "score": float(similarities[i]),
                "index": int(i),
            }
            for i in indices
        ]
        
        return SimilarityBatchResponse(
            query=request.query,
            results=results,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/")
async def root():
    """Root endpoint with API info."""
    return {
        "service": "WikiGraph Embeddings",
        "version": "0.1.0",
        "model": MODEL_NAME,
        "docs": "/docs",
    }


if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host=HOST,
        port=PORT,
        reload=os.getenv("DEBUG", "false").lower() == "true",
    )
