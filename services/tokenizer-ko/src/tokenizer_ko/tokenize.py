"""mecab-ko wrapper for Korean morphological segmentation.

REQ-IDX-003-002: Morpheme extraction from MeCab output.

The pymecab-ko Tagger is NOT thread-safe. All calls are serialised through
an asyncio.Lock to prevent data corruption under concurrent asyncio requests.

# @MX:NOTE: [AUTO] asyncio.Lock serialises pymecab-ko Tagger.parse() calls.
# pymecab-ko wraps a C library (mecab-ko) that is not thread-safe.
# asyncio.to_thread avoids blocking the event loop during C call.
# @MX:SPEC: SPEC-IDX-003
"""

from __future__ import annotations

import asyncio
from typing import Any

# MAX_INPUT_BYTES: maximum text size accepted before returning 400.
# REQ-IDX-003-004: texts exceeding this limit are rejected without Tagger call.
MAX_INPUT_BYTES = 65536

# Module-level asyncio.Lock; one lock per process since the Tagger is a
# process-level singleton (created during lifespan startup).
_tagger_lock: asyncio.Lock | None = None


def get_tagger_lock() -> asyncio.Lock:
    """Return the module-level asyncio.Lock for Tagger serialization."""
    global _tagger_lock
    if _tagger_lock is None:
        _tagger_lock = asyncio.Lock()
    return _tagger_lock


def create_tagger() -> Any:
    """Instantiate and return a pymecab-ko Tagger.

    Called once during FastAPI lifespan startup. Raises RuntimeError or
    FileNotFoundError if mecab-ko-dic is not available.

    REQ-IDX-003-003: failure here causes lifespan to raise, which prevents
    the app from accepting requests.
    """
    try:
        import mecab_ko as mecab  # PyPI: mecab-ko
        return mecab.Tagger()
    except ImportError as exc:
        raise RuntimeError(
            f"mecab-ko is not installed (pip install mecab-ko): {exc}"
        ) from exc


def get_dict_version(tagger: Any) -> str:
    """Extract the bundled dictionary version from the Tagger.

    Falls back to the pymecab-ko package version when introspection fails.
    """
    try:
        # pymecab-ko exposes dictionary_info() returning a SWIG DictionaryInfo object.
        dict_info = tagger.dictionary_info()
        if dict_info:
            # Try to extract the filename as version hint.
            filename = getattr(dict_info, "filename", None)
            if filename:
                return str(filename).split("/")[-1] or "unknown"
    except Exception:  # noqa: BLE001
        pass

    # Fallback: use the installed package version.
    try:
        import importlib.metadata
        return importlib.metadata.version("mecab-ko")
    except Exception:  # noqa: BLE001
        return "unknown"


async def tokenize_text(text: str, tagger: Any) -> dict:
    """Tokenize Korean text using the pymecab-ko Tagger.

    Acquires the module-level asyncio.Lock before calling the non-thread-safe
    tagger.parse(). Runs parse() in a thread pool to avoid blocking the
    asyncio event loop during the C call.

    Returns a dict matching TokenizeResponse fields (minus request_id).

    REQ-IDX-003-002: surface forms extracted from MeCab output, EOS excluded.
    """
    lock = get_tagger_lock()

    import time
    started = time.perf_counter()

    async with lock:
        # Run C-blocking parse in thread pool to avoid blocking the event loop.
        raw: str = await asyncio.to_thread(tagger.parse, text)

    latency_ms = (time.perf_counter() - started) * 1000.0

    tokens: list[str] = _parse_mecab_output(raw)

    return {
        "tokens": tokens,
        "joined": " ".join(tokens),
        "morpheme_count": len(tokens),
        "latency_ms": latency_ms,
    }


def _parse_mecab_output(raw: str) -> list[str]:
    """Extract surface forms from raw MeCab text output.

    MeCab output format: one morpheme per line as `surface\\tfeatures`.
    Lines 'EOS' and empty lines are skipped.
    """
    tokens: list[str] = []
    for line in raw.splitlines():
        line = line.strip()
        if not line or line == "EOS":
            continue
        # Split on tab; take only the surface form (first column).
        surface = line.split("\t", 1)[0]
        if surface and surface != "EOS":
            tokens.append(surface)
    return tokens
