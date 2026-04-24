"""Version presence test for embedder service (REQ-BOOT-002)."""

import re

import embedder

SEMVER_PATTERN = re.compile(r"^\d+\.\d+\.\d+")


def test_version_present() -> None:
    """Assert __version__ is defined and matches semver format."""
    assert hasattr(embedder, "__version__"), "__version__ is not defined"
    assert SEMVER_PATTERN.match(embedder.__version__), (
        f"__version__ {embedder.__version__!r} does not match semver pattern"
    )
