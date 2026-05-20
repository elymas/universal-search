"""Uvicorn entrypoint for the storm service.

Run with: python -m storm
"""

from __future__ import annotations

import os

import uvicorn


def main() -> None:
    """Start the uvicorn server."""
    port = int(os.environ.get("STORM_PORT", "8001"))
    uvicorn.run(
        "storm.app:app",
        host="0.0.0.0",
        port=port,
        log_config=None,  # We manage our own JSON logging
    )


if __name__ == "__main__":
    main()
