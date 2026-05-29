# SPEC-EVAL-001 Progress Log

## 2026-05-29 — Phase 1 (Analysis & Planning) — manager-strategy

Phase 1 of /moai run executed in analysis-only mode (no implementation).

**Outputs:** `tasks.md` written (10 units: 1 gate + 9 impl). Memory entries recorded.

**Harness:** standard (confirmed from `.moai/config/sections/harness.yaml`). P1 feature SPEC, multi-domain (Go + Python + CI), no security/payment keywords, priority != critical → standard. plan_audit.enabled=true, require_must_pass=true, evaluator=final-pass.

**Dependency verification (all 3 declared deps + transitive):**
| SPEC | declared status | actual status | assets exist |
|------|----------------|---------------|--------------|
| SPEC-SYN-002 | implemented | implemented ✓ | structural faithfulness shipped (see note) |
| SPEC-OBS-001 | implemented | implemented ✓ | `internal/obs/metrics/` allowlist present |
| SPEC-CLI-002 | implemented | implemented ✓ | cmd/usearch present |
| SPEC-LLM-001 | implemented | implemented ✓ | `internal/llm/` LiteLLM router present |
| SPEC-SYN-001 | implemented | implemented ✓ | synthesis client/Result present |
| SPEC-CORE-001 | implemented | implemented ✓ | pkg/types/normalized_doc.go present |
| SPEC-REL-001 (blocks) | draft | draft ✓ | — |

**SPEC factual corrections found (C1-style staleness — flagged, not blocking):**
1. SYN-002 faithfulness path `services/researcher/src/researcher/faithfulness.py` cited in HISTORY does NOT exist. Real: `faithfulness_endpoint.py` (Python) + `internal/synthesis/faithfulness.go` (Go), both owned by SPEC-DEEP-002 REQ-DEEP2-006, not SYN-002.
2. Quoted CJK-aware sentence regex does not match real code (`_MARKER_RE=r"\[(\d+)\]"`, split `(?<=[.!?])\s+`, no CJK). Affects 15 KO queries.
3. Go result type is `synthesis.Result` w/ `[]Citation{Marker,DocID,URL,Title}`, not `SynthesizeResponse`.
4. Orphaned `.pyc` cache for `eval_judge`/`test_eval_judge` exists with no source (clean before Phase 2).

**SYN-002 overlap (C1 risk) check:** REFUTED. EVAL-001 adds the *semantic* layer (LLM-judge entailment) that the shipped *structural* gate cannot do, and consumes synthesis output read-only. No reinvention. (Path-attribution is wrong in the SPEC, but the capability boundary is sound.)

**Phase 0 / plan-auditor:** NO report exists at `.moai/reports/plan-audit/SPEC-EVAL-001-*`. Standard harness REQUIRES plan-auditor PASS before implementation. → recommendation: **needs-plan-auditor-first**.

**Acceptance count:** 16 AC + 3 edge cases, full REQ→AC coverage matrix present. acceptance.md is complete.

**Blockers:** none hard. One gate: plan-auditor must run (T-EVAL1-00).

**Acceptance criteria met this iteration:** 0 (analysis phase). **Error delta:** 0.
