# SPEC-SYN-002 Compact Summary

One-page distillation of `.moai/specs/SPEC-SYN-002/spec.md` for
loading-into-context efficiency. Keep under 200 lines.

---

## Identity

- **ID**: SPEC-SYN-002
- **Title**: Citation faithfulness enforcement
- **Status**: draft
- **Priority**: P0
- **Milestone**: M4 — Basic Synthesis Hardening
- **Methodology**: TDD (RED-GREEN-REFACTOR), coverage ≥85%
- **Owner**: expert-backend
- **Depends on**: SPEC-SYN-001, SPEC-CORE-001, SPEC-LLM-001
- **Blocks**: SPEC-EVAL-001
- **Issue**: 0 (no GH tracking)

---

## Purpose (1 paragraph)

SPEC-SYN-001 delivered structural marker→doc mapping (every `[N]`
resolves to a real input doc). SPEC-SYN-002 raises the bar to
*per-sentence* `doc_id` provenance: every sentence in the synthesized
paragraph SHALL carry at least one valid `[N]` marker. LLM output
failing this gate is re-prompted once with stricter system message;
on retry failure, the configured mode (`strip` default | `reject` |
`off`) governs the response. **Structural faithfulness only —
semantic faithfulness (does the cited doc actually support the claim?)
is SPEC-EVAL-001's territory (RAGAS / DeepEval, M4).**

---

## EARS Requirements (5)

| ID | Pattern | One-line |
|----|---------|----------|
| REQ-SYN2-001 | Ubiquitous | Every sentence in `text` SHALL carry ≥1 valid `[N]` marker; SPEC-SYN-001 NFR-SYN-002 invariants preserved |
| REQ-SYN2-002 | Event-Driven | WHEN un-cited sentence detected AND mode != "off", THEN re-prompt once with stricter system message; on retry failure apply configured mode |
| REQ-SYN2-003 | State-Driven | WHILE mode=="off" SHALL bypass entire faithfulness gate (backward-compat / rollback) |
| REQ-SYN2-004 | Unwanted | IF un-cited remains AFTER retry AND mode=="reject" THEN return HTTP 422 `{error:"un_cited_output", uncited_count:N}` |
| REQ-SYN2-005 | Optional | WHERE mode=="strip" (default), un-cited sentences SHALL be removed; notice="N uncited sentence(s) stripped" |

## NFRs (2)

- **NFR-SYN2-001 Performance**: `enforce_faithfulness()` p99 ≤ 50 ms
  (gate alone); end-to-end p95 ≤ 14 s when retry triggers (preserves
  SPEC-SYN-001 NFR-SYN-001 p50 ≤ 8 s on no-retry path)
- **NFR-SYN2-002 Property**: idempotence — accepted/stripped output
  re-enforced returns ACCEPTED + identical text; hypothesis property
  test over arbitrary `(text, docs)` pairs

---

## Architecture Delta

```
synthesis.py:synthesize()
  ├→ build_prompt           [MODIFY: stricter directive in retry path]
  ├→ gateway.complete       [EXISTING; called twice on retry path]
  ├→ _process_markers       [EXISTING; unchanged]
  ├→ ★ enforce_faithfulness [NEW INSERTION]
  │   └→ faithfulness.py    [NEW MODULE; pure regex + string ops]
  ├→ [retry] gateway.complete + _process_markers + enforce
  └→ SynthesizeResponse OR raise UncitedOutputError → 422
```

## Files Touched

**[NEW]**
- `services/researcher/src/researcher/faithfulness.py`
- `services/researcher/tests/test_faithfulness.py`

**[MODIFY]**
- `services/researcher/src/researcher/synthesis.py` (insert gate +
  retry loop + UncitedOutputError raise)
- `services/researcher/src/researcher/app.py` (UncitedOutputError →
  422 handler)
- `services/researcher/src/researcher/obs.py` (3 new log attrs)
- `services/researcher/tests/test_synthesis.py`,
  `test_app.py` (mode tests)
- `internal/obs/metrics/synthesis.go` (2 new collectors)
- `internal/obs/metrics/metrics.go` (register)
- `internal/obs/obs.go` (re-export)
- `.env.example` (RESEARCHER_FAITHFULNESS_MODE)

**[UNCHANGED]**
- `services/researcher/src/researcher/models.py` (Citation.doc_id
  already exists)
- `internal/synthesis/types.go`, `client.go` (Go-side untouched)
- `pkg/types/normalized_doc.go`

## Configuration

- `RESEARCHER_FAITHFULNESS_MODE`: `strip` (default) | `reject` | `off`
- Single retry max (FROZEN, max_retries=1)
- Sentence regex: `[.!?。！？]\s+|[.!?。！？]$` (English + Korean)

## Observability

**New Prometheus collectors** (cardinality allowlist unchanged —
reuses existing `outcome` label name):
- `usearch_synthesis_faithfulness_outcomes_total{outcome=...}`
  values: accepted | stripped | rejected | retry_succeeded |
  retry_failed (5 values; mode=off bypasses the counter entirely
  per REQ-SYN2-003 — see plan.md §10 D2)
- `usearch_synthesis_faithfulness_retries_total` (no labels)

**JSON log extensions** in `log_synthesis()`:
- `uncited_sentences_count: int`
- `faithfulness_action: str ∈ {accepted, stripped, rejected, off}`
  (4 values — final action; retry status captured by
  `retry_attempted: bool` separately)
- `retry_attempted: bool`

## MX Tag Plan

- `faithfulness.py:enforce_faithfulness()` → `@MX:ANCHOR` + `@MX:WARN`
  (LLM-trust boundary, fan_in ≥ 3)
- `synthesis.py:build_prompt()` → `@MX:NOTE` (strict-mode directive)
- `app.py:uncited_output_handler` → `@MX:NOTE`
- `internal/obs/metrics/synthesis.go` new collectors → `@MX:NOTE`

---

## Top 3 Risks (one-line each)

1. **False-positive rejection** under strict gate on Korean prose →
   mitigated via default `mode=strip` + observable retry-rate counter
2. **Multi-claim-single-`[N]` semantic gap** → out-of-scope, deferred
   to SPEC-EVAL-001 (RAGAS)
3. **Hallucinated content under valid `[N]`** → out-of-scope,
   deferred to SPEC-EVAL-001

---

## Exclusions (key)

NOT scoring citation *quality* (RAGAS-style faithfulness ratio →
SPEC-EVAL-001). NOT modifying retrieval/fanout/adapters. NOT char-span
citations. NOT multi-retry (FROZEN at 1). NOT streaming-aware
(SPEC-SYN-004). NOT SynthesizeResponse schema breakage.

---

## Acceptance Quick-Check

6 acceptance scenarios + 10 edge cases in `acceptance.md`. Definition
of Done has 11 binary gates including SPEC-SYN-001 regression
suite green. Edge Cases 1 and 7 (empty post-strip text) resolve to
HTTP 422 per plan.md §10 D1 (no degraded fallback).

---

*Companion: spec.md, plan.md, acceptance.md, research.md*
