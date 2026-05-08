"""NFR-IDX-002: Warm p50 latency tests.

Marked @pytest.mark.slow / @pytest.mark.gpu — skipped in default CI.
"""

from __future__ import annotations

import time

import pytest


@pytest.mark.slow
def test_p50_latency_cpu(client) -> None:
    """50 sequential /embed calls; p50 <= 500ms on CPU."""
    payload = {"request_id": "lat", "texts": ["word " * 50]}
    durations = []
    for i in range(50):
        start = time.perf_counter()
        r = client.post("/embed", json={**payload, "request_id": f"lat-{i}"})
        assert r.status_code == 200
        durations.append(time.perf_counter() - start)
    p50 = sorted(durations)[25]
    assert p50 <= 0.5, f"p50 latency {p50:.3f}s > 500ms"


@pytest.mark.slow
@pytest.mark.gpu
def test_p50_latency_gpu(client) -> None:
    """50 sequential /embed calls; p50 <= 100ms on GPU."""
    payload = {"request_id": "lat", "texts": ["word " * 50]}
    durations = []
    for i in range(50):
        start = time.perf_counter()
        r = client.post("/embed", json={**payload, "request_id": f"lat-gpu-{i}"})
        assert r.status_code == 200
        durations.append(time.perf_counter() - start)
    p50 = sorted(durations)[25]
    assert p50 <= 0.1, f"GPU p50 latency {p50:.3f}s > 100ms"
