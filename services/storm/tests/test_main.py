"""Tests for storm __main__ module entry point."""

from __future__ import annotations

import os
from unittest.mock import patch


class TestMainModule:
    """__main__.py provides the uvicorn entry point."""

    def test_main_reads_storm_port_env(self) -> None:
        """main() reads STORM_PORT from environment."""
        with patch("storm.__main__.uvicorn.run") as mock_run:
            with patch.dict(os.environ, {"STORM_PORT": "9001"}, clear=False):
                from storm.__main__ import main

                main()
                mock_run.assert_called_once_with(
                    "storm.app:app",
                    host="0.0.0.0",
                    port=9001,
                    log_config=None,
                )

    def test_main_default_port(self) -> None:
        """main() defaults to port 8001."""
        with patch("storm.__main__.uvicorn.run") as mock_run:
            with patch.dict(os.environ, {}, clear=False):
                # Remove STORM_PORT if set
                os.environ.pop("STORM_PORT", None)
                from storm.__main__ import main

                main()
                args = mock_run.call_args
                assert args[1]["port"] == 8001
