"""Uvicorn entrypoint — python -m youtube_extract."""

from __future__ import annotations

import os

import uvicorn

from .app import app


def main() -> None:
    """Start the YouTube extraction service."""
    port = int(os.getenv("YT_EXTRACT_PORT", "8084"))
    log_level = os.getenv("YT_EXTRACT_LOG_LEVEL", "info").lower()
    uvicorn.run(app, host="0.0.0.0", port=port, log_level=log_level, workers=1)


if __name__ == "__main__":
    main()
