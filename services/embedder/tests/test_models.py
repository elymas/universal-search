"""Pydantic v2 model validation tests.

REQ-IDX-002-001: EmbedRequest and EmbedResponse shape + extra='forbid'.
"""

from __future__ import annotations

import pytest
from pydantic import ValidationError

from embedder.models import EmbedRequest, EmbedResponse


class TestEmbedRequest:
    def test_defaults(self) -> None:
        req = EmbedRequest(request_id="r1", texts=["hello"])
        assert req.return_dense is True
        assert req.return_sparse is False
        assert req.return_colbert_vecs is False
        assert req.batch_size == 32

    def test_extra_field_rejected(self) -> None:
        with pytest.raises(ValidationError):
            EmbedRequest(request_id="r1", texts=["a"], unexpected_field=1)

    def test_whitespace_not_stripped_at_validation(self) -> None:
        # str_strip_whitespace=False: text with leading/trailing space is preserved.
        req = EmbedRequest(request_id="r1", texts=["  hello  "])
        assert req.texts[0] == "  hello  "

    def test_all_fields(self) -> None:
        req = EmbedRequest(
            request_id="r2",
            texts=["a", "b"],
            return_dense=False,
            return_sparse=True,
            return_colbert_vecs=True,
            batch_size=16,
        )
        assert req.return_dense is False
        assert req.return_sparse is True
        assert req.return_colbert_vecs is True
        assert req.batch_size == 16

    def test_empty_texts_allowed_by_model(self) -> None:
        # Pydantic allows empty list; business validation happens in embed.py.
        req = EmbedRequest(request_id="r1", texts=[])
        assert req.texts == []


class TestEmbedResponse:
    def test_minimal(self) -> None:
        resp = EmbedResponse(
            request_id="r1",
            model="BAAI/bge-m3",
            model_version="latest",
            device="cpu",
            latency_ms=10.0,
            cache_hits=0,
            cache_misses=1,
        )
        assert resp.dense is None
        assert resp.sparse is None
        assert resp.colbert is None

    def test_with_dense(self) -> None:
        resp = EmbedResponse(
            request_id="r1",
            dense=[[0.1] * 1024],
            model="BAAI/bge-m3",
            model_version="latest",
            device="cpu",
            latency_ms=5.0,
            cache_hits=0,
            cache_misses=1,
        )
        assert len(resp.dense[0]) == 1024

    def test_json_schema_shape(self) -> None:
        schema = EmbedResponse.model_json_schema()
        props = schema["properties"]
        required_keys = {
            "request_id",
            "model",
            "model_version",
            "device",
            "latency_ms",
            "cache_hits",
            "cache_misses",
        }
        assert required_keys.issubset(props.keys())
