"""Long-form citation faithfulness gate.

SPEC-DEEP-001 M4, REQ-DEEP1-003:
per-section sentence gate that strips/rejects/bypasses uncited sentences.
DEEP-001 owns the canonical sentence regex literal on the Python side.

# @MX:NOTE: [AUTO] REQ-DEEP1-003 faithfulness gate
# @MX:SPEC: SPEC-DEEP-001
"""

from __future__ import annotations

import re
from typing import Any

from storm.models import Section, Sentence
from storm.obs import log_report

# ---------------------------------------------------------------------------
# Canonical sentence-splitting regex (DEEP-001 single source of truth)
# ---------------------------------------------------------------------------

# @MX:ANCHOR: [AUTO] regex contract
# @MX:REASON: any future change must be coordinated
#   across the Go-side streamsynth file to avoid divergence
SENTENCE_SPLIT_RE = re.compile(r"[.!?。！？]\s+|[.!?。！？]$")

# ---------------------------------------------------------------------------
# Faithfulness outcomes counter
# ---------------------------------------------------------------------------

_faithfulness_outcomes: dict[str, int] = {
    "accepted": 0,
    "stripped": 0,
    "rejected": 0,
    "off": 0,
}

_COUNTER_NAME = "usearch_storm_faithfulness_outcomes_total"


def reset_faithfulness_outcomes() -> None:
    """Reset all outcome counters to zero (for testing)."""
    for key in _faithfulness_outcomes:
        _faithfulness_outcomes[key] = 0


def get_faithfulness_outcomes() -> dict[str, int]:
    """Return a copy of the current outcome counters."""
    return dict(_faithfulness_outcomes)


def _inc_outcome(outcome: str) -> None:
    """Increment the outcome counter and log."""
    _faithfulness_outcomes[outcome] = _faithfulness_outcomes.get(outcome, 0) + 1
    log_report(
        {
            "metric": _COUNTER_NAME,
            "outcome": outcome,
            "value": _faithfulness_outcomes[outcome],
        }
    )


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def enforce_long_form_faithfulness(
    sections: list[Section],
    docs: list[dict[str, Any]],
    mode: str,
) -> tuple[list[Section], str, int, int]:
    """Enforce citation faithfulness across all sections.

    Args:
        sections: Report sections containing sentences with citation markers.
        docs: Input document corpus (used for reference, not directly here).
        mode: One of "strip", "reject", "off".

    Returns:
        Tuple of:
          - filtered_sections: Sections after applying the gate.
          - outcome: Aggregate outcome string
            ("accepted", "stripped", "rejected", "off").
          - uncited_count: Total number of uncited sentences found.
          - sections_affected: Number of sections that contained uncited sentences.
    """
    if mode == "off":
        _inc_outcome("off")
        return list(sections), "off", 0, 0

    uncited_total = 0
    sections_affected = 0
    result_sections: list[Section] = []

    for section in sections:
        cited_sentences: list[Sentence] = []
        section_uncited = 0

        for sentence in section.sentences:
            if sentence.citations:
                cited_sentences.append(sentence)
            else:
                section_uncited += 1

        if section_uncited > 0:
            uncited_total += section_uncited
            sections_affected += 1

            if mode == "strip":
                # Keep only cited sentences; section may become empty
                if cited_sentences:
                    result_sections.append(
                        Section(
                            heading=section.heading,
                            level=section.level,
                            sentences=cited_sentences,
                        )
                    )
                # Empty section is dropped entirely
            elif mode == "reject":
                # Return original sections unchanged on rejection
                result_sections.append(section)
        else:
            result_sections.append(section)

    # Determine aggregate outcome
    if mode == "reject" and sections_affected > 0:
        _inc_outcome("rejected")
        return result_sections, "rejected", uncited_total, sections_affected

    if mode == "strip" and sections_affected > 0:
        _inc_outcome("stripped")
        return result_sections, "stripped", uncited_total, sections_affected

    # All sentences were cited
    _inc_outcome("accepted")
    return result_sections, "accepted", 0, 0
