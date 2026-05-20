"""NFR-IDX-001: CPU throughput >= 30 docs/sec.

Marked @pytest.mark.slow — skipped in default CI.
"""

from __future__ import annotations

import time

import pytest


@pytest.mark.slow
def test_throughput_cpu_dense_only(client) -> None:
    """1000 sequential /embed calls; assert >= 30 docs/sec on 4-vCPU CPU."""
    payload = {"request_id": "bench", "texts": ["word " * 50]}  # ~256 tokens
    n = 1000
    start = time.perf_counter()
    for i in range(n):
        r = client.post("/embed", json={**payload, "request_id": f"bench-{i}"})
        assert r.status_code == 200
    elapsed = time.perf_counter() - start
    throughput = n / elapsed
    assert throughput >= 30, f"Throughput {throughput:.1f} docs/sec < 30"
