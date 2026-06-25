"""Tests for YTItem field coercion."""

from __future__ import annotations

from youtube_extract.models import YTItem


def test_none_string_fields_coerced_to_empty() -> None:
    """yt-dlp sends explicit None for keys like description/upload_date; the
    model must coerce them to "" instead of raising a string_type error."""
    item = YTItem(
        id="abc",
        title=None,
        description=None,
        channel_id=None,
        channel_url=None,
        uploader_id=None,
        upload_date=None,
        thumbnail_url=None,
    )
    assert item.title == ""
    assert item.description == ""
    assert item.channel_id == ""
    assert item.upload_date == ""
    assert item.thumbnail_url == ""
    # Non-str optionals stay None.
    assert item.view_count is None
    assert item.error is None
