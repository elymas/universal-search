"""NFR-IDX-007: Memory ceiling tests.

Marked @pytest.mark.slow — skipped in default CI.
"""

from __future__ import annotations

import os
import time

import pytest


def _get_vmrss_mb() -> float:
    """Read VmRSS from /proc/self/status in MB."""
    try:
        with open("/proc/self/status") as f:
            for line in f:
                if line.startswith("VmRSS:"):
                    return int(line.split()[1]) / 1024.0
    except FileNotFoundError:
        pass
    return 0.0


@pytest.mark.slow
def test_soak_memory_cpu(client) -> None:
    """1-hour soak test; VmRSS <= 4GB. Runs in a shorter window in CI."""
    # In the actual production soak: 3600 seconds at 5 RPS.
    # For CI feasibility the test runs for 60 seconds (pytest.mark.slow).
    duration_s = int(os.getenv("SOAK_DURATION_SECONDS", "60"))
    target_rps = 5
    interval = 1.0 / target_rps
    payload = {
        "request_id": "soak",
        "texts": ["word " * 20] * 32,
        "return_dense": True,
        "return_sparse": True,
        "return_colbert_vecs": True,
    }
    peak_mb = []
    start = time.perf_counter()
    while time.perf_counter() - start < duration_s:
        r = client.post("/embed", json=payload)
        assert r.status_code == 200
        peak_mb.append(_get_vmrss_mb())
        time.sleep(interval)

    if peak_mb:
        assert max(peak_mb) <= 4096, f"Peak VmRSS {max(peak_mb):.0f} MB > 4 GB ceiling"
