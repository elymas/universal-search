"""Embedder wrapper tests — inference logic, OOM, validation, Korean text.

REQ-IDX-002-002: BGE-M3 inference + order preservation.
REQ-IDX-002-007: Input validation.
REQ-IDX-002-009: OOM recovery.
REQ-IDX-002-010: Korean text verbatim passthrough.
REQ-IDX-002-013: model_version / revision kwarg.
"""

from __future__ import annotations

from typing import Any

import pytest

from embedder.embed import MAX_BATCH_SIZE, Embedder, EmbedValidationError


@pytest.fixture()
def embedder(mock_bgem3) -> Embedder:
    """Embedder with a mocked BGEM3FlagModel."""
    return Embedder(model_name="BAAI/bge-m3", device="cpu")


class TestEmbedderValidation:
    def test_empty_texts_raises(self, embedder: Embedder) -> None:
        with pytest.raises(EmbedValidationError) as exc_info:
            embedder.embed([], return_dense=True)
        assert exc_info.value.code == "empty_input"

    def test_too_many_texts_raises(self, embedder: Embedder) -> None:
        texts = ["x"] * (MAX_BATCH_SIZE + 1)
        with pytest.raises(EmbedValidationError) as exc_info:
            embedder.embed(texts, return_dense=True)
        assert exc_info.value.code == "batch_too_large"
        assert str(MAX_BATCH_SIZE + 1) in exc_info.value.detail

    def test_text_too_long_raises(self, embedder: Embedder) -> None:
        long_text = "a" * 100_001
        with pytest.raises(EmbedValidationError) as exc_info:
            embedder.embed([long_text], return_dense=True)
        assert exc_info.value.code == "text_too_long"
        assert "index 0" in exc_info.value.detail

    def test_no_modes_raises(self, embedder: Embedder) -> None:
        with pytest.raises(EmbedValidationError) as exc_info:
            embedder.embed(["hello"], return_dense=False, return_sparse=False, return_colbert_vecs=False)
        assert exc_info.value.code == "empty_modes"

    def test_validation_does_not_call_model(self, embedder: Embedder, mock_bgem3) -> None:
        with pytest.raises(EmbedValidationError):
            embedder.embed([], return_dense=True)
        assert len(mock_bgem3.encode_calls) == 0


class TestEmbedderInference:
    def test_dense_returns_1024_dim(self, embedder: Embedder, mock_bgem3) -> None:
        result = embedder.embed(["hello"], return_dense=True)
        assert result["dense"] is not None
        assert len(result["dense"][0]) == 1024

    def test_response_order_matches_request_order(self, embedder: Embedder, mock_bgem3) -> None:
        texts = ["foo", "bar", "baz"]
        result = embedder.embed(texts, return_dense=True)
        assert len(result["dense"]) == 3
        # The mock call must have received texts in the same order.
        assert mock_bgem3.encode_calls[-1]["sentences"] == texts

    def test_sparse_returns_dict(self, embedder: Embedder, mock_bgem3) -> None:
        result = embedder.embed(["hello"], return_sparse=True)
        assert result["sparse"] is not None
        assert isinstance(result["sparse"][0], dict)

    def test_all_modes_single_call(self, embedder: Embedder, mock_bgem3) -> None:
        embedder.embed(["hello"], return_dense=True, return_sparse=True, return_colbert_vecs=True)
        # Single forward pass — exactly one encode call.
        assert len(mock_bgem3.encode_calls) == 1
        call_kwargs = mock_bgem3.encode_calls[0]
        assert call_kwargs["return_dense"] is True
        assert call_kwargs["return_sparse"] is True
        assert call_kwargs["return_colbert_vecs"] is True


class TestOOMRecovery:
    def test_oom_raises_memory_error(self, embedder: Embedder, mock_bgem3) -> None:
        mock_bgem3._oom = True

        # Make encode raise MemoryError.

        def oom_encode(*args: Any, **kwargs: Any) -> dict:
            raise MemoryError("out of memory")

        mock_bgem3.encode = oom_encode
        with pytest.raises(MemoryError):
            embedder.embed(["hello"], return_dense=True)

    def test_oom_does_not_crash_process(self, embedder: Embedder, mock_bgem3) -> None:
        call_count = 0
        original_encode = mock_bgem3.encode

        def flaky_encode(*args: Any, **kwargs: Any) -> dict:
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                raise MemoryError("OOM on first call")
            return original_encode(*args, **kwargs)

        mock_bgem3.encode = flaky_encode

        with pytest.raises(MemoryError):
            embedder.embed(["first"], return_dense=True)

        # Second call succeeds.
        result = embedder.embed(["second"], return_dense=True)
        assert result["dense"] is not None


class TestKoreanText:
    def test_korean_text_dense_shape(self, embedder: Embedder, mock_bgem3) -> None:
        result = embedder.embed(["안녕하세요"], return_dense=True)
        assert result["dense"] is not None
        assert len(result["dense"][0]) == 1024

    def test_korean_text_passed_verbatim_to_model(self, embedder: Embedder, mock_bgem3) -> None:
        korean_text = "안녕하세요"
        embedder.embed([korean_text], return_dense=True)
        assert mock_bgem3.encode_calls[-1]["sentences"] == [korean_text]

    def test_mixed_korean_english_succeeds(self, embedder: Embedder, mock_bgem3) -> None:
        result = embedder.embed(["안녕 hello 안녕"], return_dense=True)
        assert result["dense"] is not None
        assert len(result["dense"][0]) == 1024


class TestModelVersion:
    def test_model_version_pinned(self, mock_bgem3) -> None:

        captured_kwargs: dict = {}

        # Patch at the sys.modules level.
        import FlagEmbedding  # type: ignore[import-untyped]

        original_cls = FlagEmbedding.BGEM3FlagModel

        class CapturingModel:
            def __init__(self, model_name: str, **kwargs: Any) -> None:
                captured_kwargs.update(kwargs)
                self.model_name = model_name

            def encode(self, *args: Any, **kwargs: Any) -> dict:
                return {}

        FlagEmbedding.BGEM3FlagModel = CapturingModel
        try:
            Embedder(model_name="BAAI/bge-m3", model_version="abc123def")
            assert captured_kwargs.get("revision") == "abc123def"
        finally:
            FlagEmbedding.BGEM3FlagModel = original_cls

    def test_model_version_latest_no_revision(self, mock_bgem3) -> None:
        # When model_version is None / 'latest', no revision kwarg is passed.
        Embedder(model_name="BAAI/bge-m3", model_version=None)
        assert "revision" not in mock_bgem3.init_kwargs

    def test_model_version_in_embedder(self, mock_bgem3) -> None:
        e = Embedder(model_name="BAAI/bge-m3", model_version="v1.0")
        assert e.model_version == "v1.0"
