"""Entrypoint: python -m tokenizer_ko runs the FastAPI app via uvicorn.

Usage (from the services/tokenizer-ko/ directory):
    python -m tokenizer_ko

Environment variables:
    TOKENIZER_KO_PORT      listening port (default 8083)
    TOKENIZER_KO_LOG_LEVEL log level (default INFO)
"""

from __future__ import annotations

import os

import uvicorn

from tokenizer_ko.app import app

if __name__ == "__main__":
    port = int(os.getenv("TOKENIZER_KO_PORT", "8083"))
    log_level = os.getenv("TOKENIZER_KO_LOG_LEVEL", "info").lower()

    uvicorn.run(
        app,
        host="0.0.0.0",  # noqa: S104 — compose-internal only, not public
        port=port,
        log_level=log_level,
        workers=1,  # single worker: asyncio.Lock-based serialisation
    )
