"""Embedder service — BGE-M3 embedding sidecar for Universal Search.

SPEC-IDX-002: Exposes POST /embed and GET /health via FastAPI.
Loads BAAI/bge-m3 once at startup via FlagEmbedding.BGEM3FlagModel.
"""

__version__ = "0.1.0"
