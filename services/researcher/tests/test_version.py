"""Version presence test for researcher service (REQ-BOOT-002)."""

import re

import researcher

SEMVER_PATTERN = re.compile(r"^\d+\.\d+\.\d+")


def test_version_present() -> None:
    """Assert __version__ is defined and matches semver format."""
    assert hasattr(researcher, "__version__"), "__version__ is not defined"
    assert SEMVER_PATTERN.match(
        researcher.__version__
    ), f"__version__ {researcher.__version__!r} does not match semver pattern"
