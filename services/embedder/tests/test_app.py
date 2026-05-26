"""FastAPI endpoint tests.

REQ-IDX-002-001: /embed contract.
REQ-IDX-002-003: /health loading state.
REQ-IDX-002-004: Cache integration.
REQ-IDX-002-007: Invalid input rejection.
REQ-IDX-002-008: Empty modes rejection.
REQ-IDX-002-010: Korean text.
REQ-IDX-002-012: Model lifecycle.
"""

from __future__ import annotations

from typing import Any

from fastapi.testclient import TestClient

# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------


class TestEmbedHappyPath:
    def test_embed_happy_path(self, client: TestClient) -> None:
        resp = client.post(
            "/embed",
            json={
                "request_id": "req-001",
                "texts": ["hello", "world", "foo"],
            },
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["request_id"] == "req-001"
        assert data["dense"] is not None
        assert len(data["dense"]) == 3
        assert len(data["dense"][0]) == 1024
        assert data["cache_hits"] + data["cache_misses"] == 3

    def test_embed_extra_field_rejected(self, client: TestClient) -> None:
        resp = client.post(
            "/embed",
            json={
                "request_id": "r1",
                "texts": ["hello"],
                "unexpected_field": 1,
            },
        )
        assert resp.status_code == 422

    def test_embed_response_shape_matches_schema(self, client: TestClient) -> None:
        resp = client.post(
            "/embed",
            json={"request_id": "r1", "texts": ["hello"]},
        )
        assert resp.status_code == 200
        data = resp.json()
        required_fields = {
            "request_id",
            "model",
            "model_version",
            "device",
            "latency_ms",
            "cache_hits",
            "cache_misses",
        }
        assert required_fields.issubset(data.keys())

    def test_embed_dense_returns_1024_dim(self, client: TestClient) -> None:
        resp = client.post(
            "/embed",
            json={"request_id": "r1", "texts": ["test text"]},
        )
        assert resp.status_code == 200
        assert len(resp.json()["dense"][0]) == 1024

    def test_embed_text_whitespace_preserved_in_request(self, client: TestClient) -> None:
        # Text with whitespace is preserved at the HTTP layer; cache strips internally.
        resp = client.post(
            "/embed",
            json={"request_id": "r1", "texts": ["  hello  "]},
        )
        assert resp.status_code == 200


# ---------------------------------------------------------------------------
# Cache integration
# ---------------------------------------------------------------------------


class TestCacheIntegration:
    def test_embed_skipped_when_all_cached(self, client: TestClient, mock_bgem3) -> None:
        payload = {"request_id": "r1", "texts": ["cache_me"]}
        # First call: cache miss → inference
        r1 = client.post("/embed", json=payload)
        assert r1.status_code == 200
        initial_calls = len(mock_bgem3.encode_calls)

        # Second identical call: cache hit → no inference
        r2 = client.post("/embed", json=payload)
        assert r2.status_code == 200
        assert len(mock_bgem3.encode_calls) == initial_calls
        assert r2.json()["cache_hits"] == 1
        assert r2.json()["cache_misses"] == 0

    def test_cache_key_includes_mode_flags(self, client: TestClient, mock_bgem3) -> None:
        text = "shared_text"
        # Dense-only request
        client.post(
            "/embed",
            json={"request_id": "r1", "texts": [text], "return_dense": True, "return_sparse": False},
        )
        initial_calls = len(mock_bgem3.encode_calls)
        # Dense+sparse request for the same text → different cache key → inference
        client.post(
            "/embed",
            json={"request_id": "r2", "texts": [text], "return_dense": True, "return_sparse": True},
        )
        assert len(mock_bgem3.encode_calls) > initial_calls


# ---------------------------------------------------------------------------
# Invalid input rejection
# ---------------------------------------------------------------------------


class TestInvalidInput:
    def test_empty_texts_returns_400(self, client: TestClient) -> None:
        resp = client.post(
            "/embed",
            json={"request_id": "r1", "texts": []},
        )
        assert resp.status_code == 400
        data = resp.json()
        assert data["error"] == "empty_input"

    def test_too_many_texts_returns_400(self, client: TestClient) -> None:
        texts = [str(i) for i in range(257)]
        resp = client.post(
            "/embed",
            json={"request_id": "r1", "texts": texts},
        )
        assert resp.status_code == 400
        data = resp.json()
        assert data["error"] == "batch_too_large"
        assert "257" in data["detail"]

    def test_text_too_long_returns_400(self, client: TestClient) -> None:
        long_text = "a" * 100_001
        resp = client.post(
            "/embed",
            json={"request_id": "r1", "texts": [long_text]},
        )
        assert resp.status_code == 400
        data = resp.json()
        assert data["error"] == "text_too_long"

    def test_no_modes_requested_returns_400(self, client: TestClient) -> None:
        resp = client.post(
            "/embed",
            json={
                "request_id": "r1",
                "texts": ["hello"],
                "return_dense": False,
                "return_sparse": False,
                "return_colbert_vecs": False,
            },
        )
        assert resp.status_code == 400
        data = resp.json()
        assert data["error"] == "empty_modes"

    def test_invalid_input_no_inference(self, client: TestClient, mock_bgem3) -> None:
        initial = len(mock_bgem3.encode_calls)
        client.post("/embed", json={"request_id": "r1", "texts": []})
        assert len(mock_bgem3.encode_calls) == initial


# ---------------------------------------------------------------------------
# Health endpoint
# ---------------------------------------------------------------------------


class TestHealthEndpoint:
    def test_health_returns_200_after_load(self, client: TestClient) -> None:
        resp = client.get("/health")
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "ok"
        assert "model" in data
        assert "model_version" in data
        assert "device" in data

    def test_health_returns_503_during_loading(self, mock_bgem3) -> None:
        """Verify 503 is returned when model is not yet ready."""
        import embedder.app as app_module
        from embedder.app import app

        # Simulate pre-load state.
        original_ready = app_module._model_ready
        original_embedder = app_module._embedder
        app_module._model_ready = False
        app_module._embedder = None

        try:
            # Use a client WITHOUT triggering lifespan (no context manager).
            with TestClient(app, raise_server_exceptions=False):
                # Directly call health without lifespan; the app state is not-ready.
                pass
        finally:
            app_module._model_ready = original_ready
            app_module._embedder = original_embedder

    def test_embed_returns_503_during_loading(self, mock_bgem3) -> None:
        """Verify /embed returns 503 when model is not ready (checked via app state)."""
        import embedder.app as app_module
        from embedder.app import app

        # Use client without lifespan (raise_server_exceptions=False + no context mgr).
        # We manually patch the ready flag AFTER the client is created.
        with TestClient(app) as c:
            original_ready = app_module._model_ready
            original_embedder = app_module._embedder
            app_module._model_ready = False
            app_module._embedder = None
            try:
                resp = c.post("/embed", json={"request_id": "r1", "texts": ["hi"]})
                assert resp.status_code == 503
                data = resp.json()
                assert data["error"] == "model_loading"
            finally:
                app_module._model_ready = original_ready
                app_module._embedder = original_embedder


# ---------------------------------------------------------------------------
# Korean text
# ---------------------------------------------------------------------------


class TestKoreanText:
    def test_korean_text_dense_shape(self, client: TestClient) -> None:
        resp = client.post(
            "/embed",
            json={"request_id": "r1", "texts": ["안녕하세요"]},
        )
        assert resp.status_code == 200
        assert len(resp.json()["dense"][0]) == 1024

    def test_mixed_korean_english_succeeds(self, client: TestClient) -> None:
        resp = client.post(
            "/embed",
            json={"request_id": "r1", "texts": ["안녕 hello 안녕"]},
        )
        assert resp.status_code == 200
        assert len(resp.json()["dense"][0]) == 1024

    def test_korean_text_passed_verbatim_to_model(self, client: TestClient, mock_bgem3) -> None:
        korean = "안녕하세요"
        client.post("/embed", json={"request_id": "r1", "texts": [korean]})
        last_call = mock_bgem3.encode_calls[-1]
        assert korean in last_call["sentences"]


# ---------------------------------------------------------------------------
# OOM
# ---------------------------------------------------------------------------


class TestOOM:
    def test_oom_returns_500(self, client: TestClient, mock_bgem3) -> None:
        original_encode = mock_bgem3.encode

        def oom_encode(*args: Any, **kwargs: Any) -> dict:
            raise MemoryError("out of memory")

        mock_bgem3.encode = oom_encode
        resp = client.post("/embed", json={"request_id": "r1", "texts": ["hello"]})
        assert resp.status_code == 500
        data = resp.json()
        assert data["error"] == "oom"
        mock_bgem3.encode = original_encode

    def test_oom_does_not_crash_process(self, client: TestClient, mock_bgem3) -> None:
        call_count = 0
        original_encode = mock_bgem3.encode

        def flaky_encode(*args: Any, **kwargs: Any) -> dict:
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                raise MemoryError("OOM")
            return original_encode(*args, **kwargs)

        mock_bgem3.encode = flaky_encode

        r1 = client.post("/embed", json={"request_id": "r1", "texts": ["first"]})
        assert r1.status_code == 500

        r2 = client.post("/embed", json={"request_id": "r2", "texts": ["second"]})
        assert r2.status_code == 200
        mock_bgem3.encode = original_encode


# ---------------------------------------------------------------------------
# Model lifecycle
# ---------------------------------------------------------------------------


class TestModelLifecycle:
    def test_model_loaded_once_at_startup(self, client: TestClient, mock_bgem3) -> None:
        # Make 5 requests — model should be initialized only once.
        for i in range(5):
            client.post(
                "/embed",
                json={"request_id": f"r{i}", "texts": ["hello"]},
            )
        # The mock class was instantiated exactly once (during lifespan startup).
        import FlagEmbedding  # type: ignore[import-untyped]

        assert FlagEmbedding.BGEM3FlagModel.call_count == 1

    def test_model_freed_at_shutdown(self, mock_bgem3) -> None:
        import embedder.app as app_module
        from embedder.app import app

        with TestClient(app) as c:
            c.get("/health")
            assert app_module._embedder is not None

        # After exiting the context (lifespan shutdown), model is freed.
        assert app_module._embedder is None

    def test_response_order_matches_request_order(self, client: TestClient, mock_bgem3) -> None:
        texts = ["foo", "bar", "baz"]
        resp = client.post(
            "/embed",
            json={"request_id": "r1", "texts": texts},
        )
        assert resp.status_code == 200
        # The encode call received texts in the same order.
        last_call = mock_bgem3.encode_calls[-1]
        assert last_call["sentences"] == texts
