"""LRU cache for embedding results.

REQ-IDX-002-004: In-process LRU bounded by EMBEDDER_CACHE_MAX_ENTRIES.
Cache key = sha256(text.strip() + model_name + model_version + mode_flags).
"""

from __future__ import annotations

import hashlib
from typing import Optional

from cachetools import LRUCache

# Type alias for cached embedding tuple: (dense, sparse, colbert)
CachedValue = tuple[
    Optional[list[float]],
    Optional[dict[str, float]],
    Optional[list[list[float]]],
]


def _mode_flags_string(
    return_dense: bool, return_sparse: bool, return_colbert_vecs: bool
) -> str:
    """Encode mode flags as a deterministic string."""
    d = 1 if return_dense else 0
    s = 1 if return_sparse else 0
    c = 1 if return_colbert_vecs else 0
    return f"d={d},s={s},c={c}"


class EmbedderCache:
    """Bounded LRU cache for embedding results.

    # @MX:NOTE: [AUTO] Cache key includes model_version to auto-invalidate on model upgrade.
    """

    def __init__(self, maxsize: int) -> None:
        # maxsize=0 means caching is disabled (every request runs inference)
        self._disabled = maxsize == 0
        self._cache: LRUCache[str, CachedValue] = LRUCache(maxsize=max(maxsize, 1))

    def key_for(
        self,
        text: str,
        model_name: str,
        model_version: str,
        return_dense: bool,
        return_sparse: bool,
        return_colbert_vecs: bool,
    ) -> str:
        """Derive a deterministic SHA-256 cache key.

        text.strip() is applied here — not at validation time — per SPEC §2.4.
        """
        mode_flags = _mode_flags_string(return_dense, return_sparse, return_colbert_vecs)
        key_input = f"{text.strip()}\n{model_name}\n{model_version}\n{mode_flags}"
        return hashlib.sha256(key_input.encode("utf-8")).hexdigest()

    def get(self, key: str) -> Optional[CachedValue]:
        """Return cached value or None if missing or cache is disabled."""
        if self._disabled:
            return None
        return self._cache.get(key)

    def put(self, key: str, value: CachedValue) -> None:
        """Store a value in the cache (no-op when disabled)."""
        if self._disabled:
            return
        self._cache[key] = value

    @property
    def disabled(self) -> bool:
        """True when cache is disabled (EMBEDDER_CACHE_MAX_ENTRIES=0)."""
        return self._disabled
