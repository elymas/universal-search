"""Pydantic v2 request/response models for the embedder sidecar.

REQ-IDX-002-001: EmbedRequest and EmbedResponse with ConfigDict(extra='forbid').
"""

from __future__ import annotations

from typing import Optional

from pydantic import BaseModel, ConfigDict


class EmbedRequest(BaseModel):
    """POST /embed request body.

    ConfigDict(extra='forbid') rejects unknown fields per REQ-IDX-002-001.
    str_strip_whitespace=False: text stripping happens in the cache layer, not here.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=False)

    request_id: str
    texts: list[str]
    return_dense: bool = True
    return_sparse: bool = False
    return_colbert_vecs: bool = False
    batch_size: int = 32


class EmbedResponse(BaseModel):
    """POST /embed response body."""

    model_config = ConfigDict(extra="forbid")

    request_id: str
    # dense: list of 1024-dim float32 vectors, one per input text (when return_dense=true)
    dense: Optional[list[list[float]]] = None
    # sparse: list of token_id -> weight dicts, one per input text (when return_sparse=true)
    sparse: Optional[list[dict[str, float]]] = None
    # colbert: list of [T_i, 1024] matrices, one per input text (when return_colbert_vecs=true)
    colbert: Optional[list[list[list[float]]]] = None
    model: str
    model_version: str
    device: str
    latency_ms: float
    cache_hits: int
    cache_misses: int


class HealthResponse(BaseModel):
    """GET /health response body."""

    status: str
    model: Optional[str] = None
    model_version: Optional[str] = None
    device: Optional[str] = None
    reason: Optional[str] = None
