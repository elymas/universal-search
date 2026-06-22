"""Pytest configuration and shared fixtures for YouTube extract sidecar tests."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest
from fastapi.testclient import TestClient

# Ensure the package is importable from the test environment.
sys.path.insert(0, str(Path(__file__).parent.parent / "src"))

from youtube_extract.app import app

FIXTURES_DIR = Path(__file__).parent / "fixtures"


def load_fixture(name: str) -> dict | list:
    """Load a JSON fixture from the fixtures directory."""
    path = FIXTURES_DIR / name
    with open(path) as f:
        return json.load(f)


@pytest.fixture
def client() -> TestClient:
    """FastAPI test client with sidecar in ready state."""
    # Force ready state for tests.
    import youtube_extract.app as app_module

    app_module._ready = True
    app_module._ytdlp_version = "2026.03.17"
    return TestClient(app)


@pytest.fixture
def client_not_ready() -> TestClient:
    """FastAPI test client with sidecar in not-ready state."""
    import youtube_extract.app as app_module

    app_module._ready = False
    app_module._ytdlp_version = "unknown"
    return TestClient(app)


@pytest.fixture
def sample_ytdlp_entries() -> list[dict]:
    """Sample yt-dlp JSON entries simulating search results."""
    return [
        {
            "id": "dQw4w9WgXcQ",
            "title": "Never Gonna Give You Up",
            "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
            "webpage_url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
            "description": "The official video for Never Gonna Give You Up by Rick Astley.",
            "channel": "Rick Astley",
            "channel_id": "UCuAXFkgsw1L7xaCfnd5JJOw",
            "channel_url": "https://www.youtube.com/@RickAstleyYT",
            "uploader": "Rick Astley",
            "uploader_id": "RickAstleyYT",
            "duration": 213,
            "view_count": 1600000000,
            "like_count": 16000000,
            "upload_date": "20091025",
            "thumbnail": "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg",
            "tags": ["music", "rick astley", "never gonna give you up"],
        },
        {
            "id": "abcdef12345",
            "title": "Go Generics Tutorial",
            "url": "https://www.youtube.com/watch?v=abcdef12345",
            "webpage_url": "https://www.youtube.com/watch?v=abcdef12345",
            "description": "Learn Go generics step by step.",
            "channel": "Go Academy",
            "channel_id": "UCgoacademy",
            "channel_url": "https://www.youtube.com/@GoAcademy",
            "uploader": "Go Academy",
            "uploader_id": "GoAcademy",
            "duration": 600,
            "view_count": 50000,
            "like_count": 2000,
            "upload_date": "20260115",
            "thumbnail": "https://i.ytimg.com/vi/abcdef12345/hqdefault.jpg",
            "tags": ["golang", "generics", "tutorial"],
        },
    ]


@pytest.fixture
def livestream_entry() -> dict:
    """Sample yt-dlp entry for a livestream-archived video (null view_count)."""
    return {
        "id": "live001",
        "title": "Live Stream Recording",
        "url": "https://www.youtube.com/watch?v=live001",
        "webpage_url": "https://www.youtube.com/watch?v=live001",
        "description": "A past livestream recording.",
        "channel": "LiveChannel",
        "channel_id": "UClivechannel",
        "channel_url": "https://www.youtube.com/@LiveChannel",
        "uploader": "LiveChannel",
        "uploader_id": "LiveChannel",
        "duration": 0,
        "view_count": None,
        "like_count": None,
        "upload_date": "20260301",
        "thumbnail": "https://i.ytimg.com/vi/live001/hqdefault.jpg",
        "tags": [],
    }


@pytest.fixture
def partial_error_entry() -> dict:
    """Sample yt-dlp entry with a per-item extraction error."""
    return {
        "id": "error001",
        "title": "Unavailable Video",
        "url": "https://www.youtube.com/watch?v=error001",
        "webpage_url": "https://www.youtube.com/watch?v=error001",
        "description": "",
        "channel": "",
        "channel_id": "",
        "channel_url": "",
        "uploader": "",
        "uploader_id": "",
        "duration": None,
        "view_count": None,
        "upload_date": "",
        "thumbnail": "",
        "tags": [],
        "_error": "Video unavailable",
    }


def make_mock_process(
    stdout: str = "",
    stderr: str = "",
    returncode: int = 0,
) -> AsyncMock:
    """Create a mock asyncio.subprocess.Process."""
    proc = AsyncMock()
    proc.communicate.return_value = (
        stdout.encode("utf-8"),
        stderr.encode("utf-8"),
    )
    proc.returncode = returncode
    proc.wait = AsyncMock()
    proc.terminate = MagicMock()
    proc.kill = MagicMock()
    return proc
