"""Throughput tests for the tokenizer-ko sidecar.

NFR-IDX-003-001: ≥ 1000 RPS on single worker.
These tests are marked @pytest.mark.slow and skipped by default.
"""

from __future__ import annotations

import asyncio
import time

import pytest
from fastapi.testclient import TestClient

from tokenizer_ko.app import app


@pytest.fixture()
def client():
    """FastAPI TestClient with the tokenizer-ko app."""
    with TestClient(app) as c:
        yield c


@pytest.mark.slow
class TestThroughput:
    """NFR-IDX-003-001: Sidecar throughput ≥ 1000 RPS."""

    def test_throughput_1000rps(self, client: TestClient) -> None:
        """10000 async-batched requests; total time < 10s (≥ 1000 RPS)."""
        text = "한국어 형태소 분석 테스트" * 7  # ~100 chars

        async def run_batched():
            loop = asyncio.get_event_loop()
            tasks = []
            for _ in range(10000):
                future = loop.run_in_executor(
                    None,
                    lambda: client.post(
                        "/tokenize",
                        json={"request_id": "throughput", "text": text},
                    ),
                )
                tasks.append(future)
            return await asyncio.gather(*tasks)

        start = time.perf_counter()
        responses = asyncio.get_event_loop().run_until_complete(run_batched())
        total_seconds = time.perf_counter() - start

        success_count = sum(1 for r in responses if r.status_code == 200)
        assert success_count == 10000, f"Only {success_count}/10000 requests succeeded"
        assert total_seconds < 10.0, (
            f"Throughput {success_count/total_seconds:.0f} RPS "
            f"(total {total_seconds:.2f}s) < 1000 RPS target"
        )
