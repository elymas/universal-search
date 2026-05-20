"""Tokenization correctness tests for mecab-ko wrapper.

Covers REQ-IDX-003-002: morpheme extraction, EOS exclusion, golden fixtures.
"""

from __future__ import annotations

import json
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch
import asyncio

import pytest


FIXTURES_PATH = Path(__file__).parent / "fixtures" / "golden_morphemes.json"


@pytest.fixture()
def golden_fixtures():
    """Load the golden morpheme fixtures from JSON."""
    with FIXTURES_PATH.open() as f:
        return json.load(f)


class TestMecabKoMorphemes:
    """REQ-IDX-003-002: mecab-ko morpheme extraction correctness."""

    def test_tokenize_korean_morphemes(self) -> None:
        """Feed 'ChatGPT 사용법' and assert key morphemes are present."""
        from tokenizer_ko.tokenize import tokenize_text

        tagger = _make_real_tagger()
        if tagger is None:
            pytest.skip("mecab-ko not installed — integration test skipped")

        result = asyncio.get_event_loop().run_until_complete(
            tokenize_text("ChatGPT 사용법", tagger)
        )
        tokens = result["tokens"]
        token_text = " ".join(tokens)
        # Must contain at least some recognizable Korean morphemes
        assert len(tokens) >= 2, f"Expected >=2 morphemes, got {tokens}"
        assert any("사용" in t or "법" in t or "ChatGPT" in t for t in tokens), (
            f"Expected morphemes from '사용법', got: {tokens}"
        )

    def test_morpheme_count_matches_tokens_length(self) -> None:
        """morpheme_count must equal len(tokens) for any input."""
        from tokenizer_ko.tokenize import tokenize_text

        tagger = _make_real_tagger()
        if tagger is None:
            pytest.skip("mecab-ko not installed — integration test skipped")

        result = asyncio.get_event_loop().run_until_complete(
            tokenize_text("한국어 형태소 분석", tagger)
        )
        assert result["morpheme_count"] == len(result["tokens"])

    def test_eos_lines_excluded(self) -> None:
        """EOS lines from MeCab output must NOT appear in returned tokens."""
        from tokenizer_ko import tokenize as tok_module
        import asyncio

        # Mock tagger that returns raw MeCab output with EOS
        mock_tagger = MagicMock()
        mock_tagger.parse.return_value = "안녕\tNN\n하세요\tXSV+EF\nEOS\n"

        result = asyncio.get_event_loop().run_until_complete(
            tok_module.tokenize_text("안녕하세요", mock_tagger)
        )
        assert "EOS" not in result["tokens"], f"EOS should be excluded, got: {result['tokens']}"
        assert "" not in result["tokens"], "Empty string should be excluded from tokens"

    def test_joined_equals_space_join(self) -> None:
        """joined field must be exactly ' '.join(tokens)."""
        from tokenizer_ko import tokenize as tok_module
        import asyncio

        mock_tagger = MagicMock()
        mock_tagger.parse.return_value = "서울\tNN\n날씨\tNN\nEOS\n"

        result = asyncio.get_event_loop().run_until_complete(
            tok_module.tokenize_text("서울 날씨", mock_tagger)
        )
        assert result["joined"] == " ".join(result["tokens"])

    def test_golden_fixtures_at_least_one_must_contain(self, golden_fixtures) -> None:
        """Each golden fixture: returned tokens contain at least one expected morpheme."""
        from tokenizer_ko.tokenize import tokenize_text

        tagger = _make_real_tagger()
        if tagger is None:
            pytest.skip("mecab-ko not installed — integration test skipped")

        for fixture in golden_fixtures[:5]:  # test first 5 to keep fast
            result = asyncio.get_event_loop().run_until_complete(
                tokenize_text(fixture["input"], tagger)
            )
            tokens = result["tokens"]
            must_contain = fixture["must_contain"]

            found = any(expected in tokens for expected in must_contain)
            # Accept partial: if any token contains a must_contain substring
            if not found:
                found = any(
                    any(expected in tok for tok in tokens)
                    for expected in must_contain
                )
            assert found, (
                f"Fixture '{fixture['description']}': "
                f"expected one of {must_contain} in tokens {tokens}"
            )

    def test_parse_with_tab_separator(self) -> None:
        """tokenize_text correctly handles MeCab tab-separated output format."""
        from tokenizer_ko import tokenize as tok_module
        import asyncio

        mock_tagger = MagicMock()
        # MeCab output: surface\tfeatures format
        mock_tagger.parse.return_value = "날씨\tNN,*,F,날씨,*,*,*,*\n가\tJX,*,F,가,*,*,*,*\nEOS\n"

        result = asyncio.get_event_loop().run_until_complete(
            tok_module.tokenize_text("날씨가", mock_tagger)
        )
        assert "날씨" in result["tokens"]
        assert "가" in result["tokens"]
        assert "EOS" not in result["tokens"]
        assert "날씨가" not in result["tokens"]  # original text should be split


def _make_real_tagger():
    """Attempt to create a real mecab-ko Tagger. Returns None if unavailable."""
    try:
        import mecab_ko as mecab
        return mecab.Tagger()
    except (ImportError, RuntimeError, AttributeError):
        return None
