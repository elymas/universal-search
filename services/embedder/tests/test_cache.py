"""LRU cache tests.

REQ-IDX-002-004: Cache hit/miss, key derivation, disabled-when-zero, LRU eviction.
NFR-IDX-003: Cache-hit latency <= 5ms.
NFR-IDX-004: Steady-state hit ratio >= 30%.
"""

from __future__ import annotations

import time

from embedder.cache import EmbedderCache

MODEL = "BAAI/bge-m3"
VERSION = "abc123"


def _key(cache: EmbedderCache, text: str, dense: bool = True, sparse: bool = False, colbert: bool = False) -> str:
    return cache.key_for(text, MODEL, VERSION, dense, sparse, colbert)


class TestEmbedderCache:
    def test_cache_miss_returns_none(self) -> None:
        cache = EmbedderCache(maxsize=100)
        assert cache.get("missing_key") is None

    def test_cache_put_and_get(self) -> None:
        cache = EmbedderCache(maxsize=100)
        value = ([0.1] * 1024, None, None)
        key = _key(cache, "hello")
        cache.put(key, value)
        assert cache.get(key) == value

    def test_cache_disabled_when_zero(self) -> None:
        cache = EmbedderCache(maxsize=0)
        assert cache.disabled is True
        key = _key(cache, "hello")
        cache.put(key, ([0.1] * 1024, None, None))
        assert cache.get(key) is None

    def test_cache_key_includes_mode_flags(self) -> None:
        cache = EmbedderCache(maxsize=100)
        key_dense_only = _key(cache, "foo", dense=True, sparse=False)
        key_dense_sparse = _key(cache, "foo", dense=True, sparse=True)
        assert key_dense_only != key_dense_sparse

    def test_cache_key_strips_whitespace(self) -> None:
        cache = EmbedderCache(maxsize=100)
        key_raw = _key(cache, "  hello  ")
        key_stripped = _key(cache, "hello")
        assert key_raw == key_stripped

    def test_cache_lru_eviction(self) -> None:
        # With maxsize=2, inserting 3 entries evicts the oldest.
        cache = EmbedderCache(maxsize=2)
        k1 = _key(cache, "text1")
        k2 = _key(cache, "text2")
        k3 = _key(cache, "text3")
        val = ([0.1] * 1024, None, None)
        cache.put(k1, val)
        cache.put(k2, val)
        cache.put(k3, val)
        # k1 should be evicted (LRU)
        assert cache.get(k1) is None
        assert cache.get(k2) is not None
        assert cache.get(k3) is not None

    def test_cache_key_different_versions(self) -> None:
        cache = EmbedderCache(maxsize=100)
        key_v1 = cache.key_for("hello", MODEL, "v1", True, False, False)
        key_v2 = cache.key_for("hello", MODEL, "v2", True, False, False)
        assert key_v1 != key_v2

    def test_cache_hit_latency(self) -> None:
        """NFR-IDX-003: cached responses <= 5ms p99."""
        cache = EmbedderCache(maxsize=10000)
        val = ([0.1] * 1024, None, None)
        key = _key(cache, "benchmark_text")
        cache.put(key, val)

        durations = []
        for _ in range(100):
            start = time.perf_counter()
            cache.get(key)
            durations.append(time.perf_counter() - start)

        # All hits (after first put) must be <= 5ms = 0.005s
        max_latency = max(durations)
        assert max_latency <= 0.005, f"Cache hit latency {max_latency:.4f}s exceeds 5ms"

    def test_steady_state_hit_ratio(self) -> None:
        """NFR-IDX-004: >= 30% hit ratio with Zipf distribution over 1000 texts."""
        import numpy as np

        cache = EmbedderCache(maxsize=10000)
        rng = np.random.default_rng(42)
        zipf_indices = rng.zipf(1.5, size=10000)
        zipf_indices = np.clip(zipf_indices, 1, 1000) - 1

        texts = [f"text_{i}" for i in range(1000)]
        val = ([0.1] * 1024, None, None)

        hits = 0
        for idx in zipf_indices:
            text = texts[idx]
            key = _key(cache, text)
            if cache.get(key) is not None:
                hits += 1
            else:
                cache.put(key, val)

        ratio = hits / len(zipf_indices)
        assert ratio >= 0.30, f"Steady-state hit ratio {ratio:.3f} < 0.30"
