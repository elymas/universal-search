"""Shared pytest fixtures for the embedder test suite.

Uses a mocked BGEM3FlagModel so tests do NOT download the real model.
"""

from __future__ import annotations

import sys
from typing import Any
from unittest.mock import MagicMock, patch

import pytest
from fastapi.testclient import TestClient

# ---------------------------------------------------------------------------
# Mock BGEM3FlagModel (no network download during tests)
# ---------------------------------------------------------------------------

FAKE_DENSE_DIM = 1024


def _make_dense(n: int) -> list:
    """Return n fake 1024-dim vectors as numpy-like objects."""
    import numpy as np
    return np.random.rand(n, FAKE_DENSE_DIM).astype("float32")


def _make_sparse(n: int) -> list:
    """Return n fake sparse weight dicts."""
    return [{"1": 0.5, "2": 0.3} for _ in range(n)]


def _make_colbert(n: int, tokens: int = 8) -> list:
    """Return n fake ColBERT matrices of shape [tokens, 1024]."""
    import numpy as np
    return [np.random.rand(tokens, FAKE_DENSE_DIM).astype("float32") for _ in range(n)]


class MockBGEM3FlagModel:
    """Minimal mock that records calls and returns fake vectors."""

    def __init__(self, model_name: str, **kwargs: Any) -> None:
        self.model_name = model_name
        self.init_kwargs = kwargs
        self.encode_calls: list[dict] = []

    def encode(
        self,
        sentences: list[str],
        *,
        batch_size: int = 32,
        max_length: int = 8192,
        return_dense: bool = True,
        return_sparse: bool = False,
        return_colbert_vecs: bool = False,
        **kwargs: Any,
    ) -> dict:
        self.encode_calls.append({
            "sentences": sentences,
            "batch_size": batch_size,
            "max_length": max_length,
            "return_dense": return_dense,
            "return_sparse": return_sparse,
            "return_colbert_vecs": return_colbert_vecs,
        })
        n = len(sentences)
        result: dict = {}
        if return_dense:
            result["dense_vecs"] = _make_dense(n)
        if return_sparse:
            result["lexical_weights"] = _make_sparse(n)
        if return_colbert_vecs:
            result["colbert_vecs"] = _make_colbert(n)
        return result


@pytest.fixture()
def mock_bgem3(monkeypatch: pytest.MonkeyPatch) -> MockBGEM3FlagModel:
    """Patch BGEM3FlagModel globally before the embedder app imports it."""
    mock_instance = MockBGEM3FlagModel("BAAI/bge-m3")

    mock_module = MagicMock()
    mock_module.BGEM3FlagModel = MagicMock(return_value=mock_instance)
    monkeypatch.setitem(sys.modules, "FlagEmbedding", mock_module)

    return mock_instance


@pytest.fixture()
def client(mock_bgem3: MockBGEM3FlagModel) -> TestClient:
    """FastAPI TestClient with the real app but mocked BGEM3FlagModel."""
    # Reset app state between test runs.
    import embedder.app as app_module
    app_module._embedder = None
    app_module._cache = None
    app_module._model_ready = False

    from embedder.app import app
    with TestClient(app) as c:
        yield c
