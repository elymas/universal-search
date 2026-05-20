"""Pydantic v2 request/response models for the tokenizer-ko sidecar.

REQ-IDX-003-001: TokenizeRequest and TokenizeResponse with strict validation.
"""

from __future__ import annotations

from pydantic import BaseModel, ConfigDict


class TokenizeRequest(BaseModel):
    """POST /tokenize request body.

    REQ-IDX-003-001: extra='forbid' rejects unknown fields; str_strip_whitespace
    normalises leading/trailing whitespace before validation.
    """

    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    request_id: str
    text: str


class TokenizeResponse(BaseModel):
    """POST /tokenize response body.

    Fields:
    - request_id: echoed back from request.
    - tokens: list of surface-form morphemes in input order.
    - joined: ' '.join(tokens) — convenience field.
    - morpheme_count: len(tokens).
    - latency_ms: end-to-end processing time in milliseconds.
    - dict_version: mecab-ko-dic version string.
    """

    model_config = ConfigDict(extra="forbid")

    request_id: str
    tokens: list[str]
    joined: str
    morpheme_count: int
    latency_ms: float
    dict_version: str
