"""Tests for the long-form citation faithfulness gate.

SPEC-DEEP-001 M4, REQ-DEEP1-003:
per-section sentence gate that strips/rejects/bypasses uncited sentences.
"""

from __future__ import annotations

import pytest

from storm.faithfulness import (
    SENTENCE_SPLIT_RE,
    enforce_long_form_faithfulness,
    get_faithfulness_outcomes,
    reset_faithfulness_outcomes,
)
from storm.models import Section, Sentence

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_section(
    heading: str = "Test",
    level: int = 1,
    sentences: list[tuple[str, list[int]]] | None = None,
) -> Section:
    """Build a Section from (text, citations) pairs."""
    sents = [Sentence(text=text, citations=cites) for text, cites in (sentences or [])]
    return Section(heading=heading, level=level, sentences=sents)


# ---------------------------------------------------------------------------
# Test 1: strip mode removes uncited sentences
# ---------------------------------------------------------------------------


class TestFaithfulnessStripRemovesUncitedSentences:
    """REQ-DEEP1-003: mode=strip removes uncited sentences from sections."""

    def test_strip_removes_uncited(self) -> None:
        reset_faithfulness_outcomes()
        section = _make_section(
            heading="Intro",
            sentences=[
                ("A cited sentence. [1]", [1]),
                ("An uncited sentence.", []),
                ("Another cited. [2]", [2]),
            ],
        )
        filtered, outcome, uncited, affected = enforce_long_form_faithfulness(
            sections=[section],
            docs=[{"url": "http://example.com", "doc_id": "d1"}],
            mode="strip",
        )
        assert len(filtered) == 1
        assert len(filtered[0].sentences) == 2
        assert filtered[0].sentences[0].text == "A cited sentence. [1]"
        assert filtered[0].sentences[1].text == "Another cited. [2]"
        assert outcome == "stripped"
        assert uncited == 1
        assert affected == 1

    def test_strip_all_cited_passes(self) -> None:
        reset_faithfulness_outcomes()
        section = _make_section(
            sentences=[
                ("All cited. [1]", [1]),
                ("Also cited. [2]", [2]),
            ],
        )
        filtered, outcome, uncited, affected = enforce_long_form_faithfulness(
            sections=[section],
            docs=[],
            mode="strip",
        )
        assert len(filtered) == 1
        assert len(filtered[0].sentences) == 2
        assert outcome == "accepted"
        assert uncited == 0
        assert affected == 0


# ---------------------------------------------------------------------------
# Test 2: reject mode returns rejection
# ---------------------------------------------------------------------------


class TestFaithfulnessRejectReturnsRejection:
    """REQ-DEEP1-003: mode=reject rejects if any uncited sentence found."""

    def test_reject_with_uncited(self) -> None:
        reset_faithfulness_outcomes()
        section = _make_section(
            sentences=[
                ("Cited. [1]", [1]),
                ("Uncited.", []),
            ],
        )
        filtered, outcome, uncited, affected = enforce_long_form_faithfulness(
            sections=[section],
            docs=[],
            mode="reject",
        )
        assert outcome == "rejected"
        assert uncited == 1
        assert affected == 1
        # In reject mode, the original sections are returned unchanged
        # (caller decides what to do with rejection info)
        assert len(filtered) == 1
        assert len(filtered[0].sentences) == 2

    def test_reject_all_cited_passes(self) -> None:
        reset_faithfulness_outcomes()
        section = _make_section(
            sentences=[
                ("Cited. [1]", [1]),
            ],
        )
        filtered, outcome, uncited, affected = enforce_long_form_faithfulness(
            sections=[section],
            docs=[],
            mode="reject",
        )
        assert outcome == "accepted"
        assert uncited == 0
        assert affected == 0


# ---------------------------------------------------------------------------
# Test 3: off mode bypasses gate
# ---------------------------------------------------------------------------


class TestFaithfulnessOffBypassesGate:
    """REQ-DEEP1-003: mode=off bypasses the gate entirely."""

    def test_off_passes_everything_through(self) -> None:
        reset_faithfulness_outcomes()
        section = _make_section(
            sentences=[
                ("Uncited one.", []),
                ("Uncited two.", []),
            ],
        )
        filtered, outcome, uncited, affected = enforce_long_form_faithfulness(
            sections=[section],
            docs=[],
            mode="off",
        )
        assert outcome == "off"
        assert len(filtered) == 1
        assert len(filtered[0].sentences) == 2
        assert uncited == 0
        assert affected == 0


# ---------------------------------------------------------------------------
# Test 4: empty section removed from response
# ---------------------------------------------------------------------------


class TestEmptySectionRemovedFromResponse:
    """REQ-DEEP1-003: sections that become empty after strip are removed."""

    def test_empty_section_removed(self) -> None:
        reset_faithfulness_outcomes()
        section_all_uncited = _make_section(
            heading="AllUncited",
            sentences=[
                ("No citation here.", []),
                ("Also none.", []),
            ],
        )
        section_cited = _make_section(
            heading="Cited",
            sentences=[
                ("Has citation. [1]", [1]),
            ],
        )
        filtered, outcome, uncited, affected = enforce_long_form_faithfulness(
            sections=[section_all_uncited, section_cited],
            docs=[],
            mode="strip",
        )
        # The all-uncited section should be removed entirely
        assert len(filtered) == 1
        assert filtered[0].heading == "Cited"
        assert outcome == "stripped"
        assert uncited == 2
        assert affected == 1  # only the all-uncited section was affected

    def test_all_sections_empty_after_strip(self) -> None:
        reset_faithfulness_outcomes()
        section = _make_section(
            sentences=[
                ("No citations.", []),
            ],
        )
        filtered, outcome, uncited, affected = enforce_long_form_faithfulness(
            sections=[section],
            docs=[],
            mode="strip",
        )
        assert len(filtered) == 0
        assert outcome == "stripped"
        assert uncited == 1
        assert affected == 1


# ---------------------------------------------------------------------------
# Test 5: faithfulness outcomes counter increments
# ---------------------------------------------------------------------------


class TestFaithfulnessOutcomesCounterIncrements:
    """Counter usearch_storm_faithfulness_outcomes_total{outcome} with 4 values."""

    @pytest.mark.parametrize(
        "mode, sentences, expected_outcome",
        [
            (
                "strip",
                [("Cited. [1]", [1])],
                "accepted",
            ),
            (
                "strip",
                [("Cited. [1]", [1]), ("Uncited.", [])],
                "stripped",
            ),
            (
                "reject",
                [("Uncited.", [])],
                "rejected",
            ),
            (
                "off",
                [("Uncited.", [])],
                "off",
            ),
        ],
        ids=["accepted", "stripped", "rejected", "off"],
    )
    def test_counter_increments(
        self,
        mode: str,
        sentences: list[tuple[str, list[int]]],
        expected_outcome: str,
    ) -> None:
        reset_faithfulness_outcomes()
        section = _make_section(sentences=sentences)
        enforce_long_form_faithfulness(
            sections=[section],
            docs=[],
            mode=mode,
        )
        outcomes = get_faithfulness_outcomes()
        assert outcomes.get(expected_outcome, 0) >= 1


# ---------------------------------------------------------------------------
# Test 6: sentence segmentation regex exact
# ---------------------------------------------------------------------------


class TestSentenceSegmentationRegexExact:
    """REQ-DEEP1-003: DEEP-001 owns the Python-side sentence regex literal."""

    def test_regex_literal_is_exact(self) -> None:
        expected = r"[.!?。！？]\s+|[.!?。！？]$"
        assert SENTENCE_SPLIT_RE.pattern == expected

    def test_regex_splits_on_latin_punctuation(self) -> None:
        parts = SENTENCE_SPLIT_RE.split("First. Second! Third?")
        # split keeps text between delimiters
        assert any("First" in p for p in parts if p)
        assert any("Second" in p for p in parts if p)
        assert any("Third" in p for p in parts if p)

    def test_regex_splits_on_cjk_punctuation(self) -> None:
        parts = SENTENCE_SPLIT_RE.split("First。 Second！")
        assert any("First" in p for p in parts if p)
        assert any("Second" in p for p in parts if p)
