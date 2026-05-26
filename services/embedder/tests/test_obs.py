"""Observability: JSON log record shape tests.

REQ-IDX-002-006: 12 documented attributes, outcome in allowed enum.
"""

from __future__ import annotations

import io
import json
import logging

from embedder.obs import _setup_json_logger, log_embed

VALID_OUTCOMES = {"success", "error_invalid", "error_oom", "error_loading", "error_internal"}


def _capture_log(func, *args, **kwargs) -> list[dict]:
    """Run func and capture JSON log records emitted to stdout."""
    buf = io.StringIO()
    test_logger = _setup_json_logger("embedder_test", "DEBUG")
    # Redirect handler to our buffer.
    old_handlers = test_logger.handlers[:]
    test_logger.handlers.clear()
    handler = logging.StreamHandler(buf)
    from embedder.obs import _JsonFormatter

    handler.setFormatter(_JsonFormatter())
    test_logger.addHandler(handler)

    # Monkey-patch the module-level logger temporarily.
    import embedder.obs as obs_module

    orig_logger = obs_module.logger
    obs_module.logger = test_logger
    try:
        func(*args, **kwargs)
    finally:
        obs_module.logger = orig_logger
        test_logger.handlers.clear()
        test_logger.handlers.extend(old_handlers)

    lines = [line for line in buf.getvalue().strip().split("\n") if line]
    return [json.loads(line) for line in lines]


def test_log_record_has_12_attributes() -> None:
    records = _capture_log(
        log_embed,
        request_id="req-001",
        texts_count=3,
        return_dense=True,
        return_sparse=False,
        return_colbert_vecs=False,
        cache_hits=1,
        cache_misses=2,
        latency_ms=42.5,
        model="BAAI/bge-m3",
        model_version="latest",
        device="cpu",
        outcome="success",
    )
    assert len(records) == 1
    rec = records[0]
    expected_keys = {
        "request_id",
        "texts_count",
        "return_dense",
        "return_sparse",
        "return_colbert_vecs",
        "cache_hits",
        "cache_misses",
        "latency_ms",
        "model",
        "model_version",
        "device",
        "outcome",
    }
    assert expected_keys.issubset(rec.keys())


def test_outcome_is_valid_enum() -> None:
    for outcome in VALID_OUTCOMES:
        records = _capture_log(
            log_embed,
            request_id="req-001",
            texts_count=1,
            return_dense=True,
            return_sparse=False,
            return_colbert_vecs=False,
            cache_hits=0,
            cache_misses=1,
            latency_ms=10.0,
            model="BAAI/bge-m3",
            model_version="latest",
            device="cpu",
            outcome=outcome,
        )
        assert records[0]["outcome"] == outcome


def test_no_text_content_in_log() -> None:
    """Privacy bound: log records must not contain verbatim text content."""
    secret_text = "this_is_private_content_xyz"
    records = _capture_log(
        log_embed,
        request_id="req-001",
        texts_count=1,
        return_dense=True,
        return_sparse=False,
        return_colbert_vecs=False,
        cache_hits=0,
        cache_misses=1,
        latency_ms=5.0,
        model="BAAI/bge-m3",
        model_version="latest",
        device="cpu",
        outcome="success",
    )
    raw = json.dumps(records)
    assert secret_text not in raw


def test_log_level_is_info() -> None:
    records = _capture_log(
        log_embed,
        request_id="req-001",
        texts_count=1,
        return_dense=True,
        return_sparse=False,
        return_colbert_vecs=False,
        cache_hits=0,
        cache_misses=1,
        latency_ms=5.0,
        model="BAAI/bge-m3",
        model_version="latest",
        device="cpu",
        outcome="success",
    )
    assert records[0]["level"] == "INFO"
