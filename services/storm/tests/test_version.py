"""Version presence test for storm service (REQ-BOOT-002)."""

import re

import storm


SEMVER_PATTERN = re.compile(r"^\d+\.\d+\.\d+")


def test_version_present() -> None:
    """Assert __version__ is defined and matches semver format."""
    assert hasattr(storm, "__version__"), "__version__ is not defined"
    assert SEMVER_PATTERN.match(storm.__version__), (
        f"__version__ {storm.__version__!r} does not match semver pattern"
    )
