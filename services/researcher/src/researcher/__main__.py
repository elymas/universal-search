"""Uvicorn entrypoint for the researcher service.

Run with: python -m researcher
"""

from __future__ import annotations

import os

import uvicorn


def main() -> None:
    """Start the uvicorn server."""
    port = int(os.environ.get("RESEARCHER_PORT", "8081"))
    uvicorn.run(
        "researcher.app:app",
        host="0.0.0.0",
        port=port,
        log_config=None,  # We manage our own JSON logging
    )


if __name__ == "__main__":
    main()
