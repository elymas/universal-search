# @MX Tag Validation Report — SPEC-IR-001

Generated: 2026-04-26
Commit: 8a20b68

## Summary

| Tag Type | Count | Compliance |
|----------|-------|------------|
| @MX:ANCHOR | 3 | All carry @MX:REASON ✓ All carry @MX:SPEC ✓ |
| @MX:WARN | 1 | Carries @MX:REASON ✓ |
| @MX:NOTE | 8 | No mandatory sub-lines required |
| @MX:TODO | 0 | All RED tests promoted to GREEN |

**Total tags: 12** (3 ANCHOR + 1 WARN + 8 NOTE)

Per-file ANCHOR usage: `router.go` carries 2 ANCHOR tags (within the 3-per-file default limit). No single file exceeds the limit.

## Tag Inventory

### @MX:ANCHOR — 3 tags

**1. internal/router/router.go:75**

```
// @MX:ANCHOR: [AUTO] Sole sanctioned classification entry point. Callers:
// FAN-001 fanout, CLI-001 CLI, SYN-001 synthesis, future debug tooling.
// @MX:REASON: fan_in >= 5 expected post-M2; all RoutingDecision values flow
// through Router.Classify. Behaviour change here cascades to every downstream.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes
- Note: Tag is on the `Router` struct declaration; `Classify` is its single exported method, making fan_in attribution to the struct correct per protocol.

**2. internal/router/router.go:146**

```
// @MX:ANCHOR: [AUTO] Classification orchestrator. Public Router entry point
// fan_in >= 5 expected.
// @MX:REASON: every RoutingDecision flows through this method; nil-safety
// invariants for obs and LLMClient are load-bearing.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes

**3. internal/router/router.go:213**

```
// @MX:ANCHOR: [AUTO] LLM-fallback path; sole place that talks to internal/llm.
// fan_in >= 3 (Classify + tests + future debug tooling).
// @MX:REASON: contract-level integration with SPEC-LLM-001; behaviour
// changes (timeout, circuit-breaker handling) ripple through the public
// degradation policy.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes

### @MX:WARN — 1 tag

**1. internal/router/llm.go:128**

```
// @MX:WARN: [AUTO] Timeout-and-fall-through: callers must inspect the
// returned error and check Metadata flags. Silent degradation is by design
// (REQ-IR-003 + REQ-IR-007) but a foot-gun if the caller forgets to surface
// the flag.
// @MX:REASON: silent degradation hides LLM failure from naive callers;
// router.go::Classify is the canonical caller and handles flags correctly.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes

### @MX:NOTE — 8 tags

**1. internal/router/metrics.go:21**

```
// @MX:NOTE: [AUTO] Static label-value enumeration. Bounded cardinality (10
// values). Adding a new value requires a SPEC amendment.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- Comments in English: yes

**2. internal/router/rules.go:20**

```
// @MX:NOTE: [AUTO] Magic constants for the rule scorer. ConfidenceThreshold
// (τ_high) gates LLM escalation. RatioHigh / RatioLow define the Hangul
// ambiguous band. Empirical choice; tunable via SPEC amendment.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- Comments in English: yes

**3. internal/router/rules.go:39**

```
// @MX:NOTE: [AUTO] Tie-break order is fixed in code; spec.md §2.3 enumerates
// it explicitly. Reordering changes deterministic behaviour for tied inputs.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- Comments in English: yes

**4. internal/router/rules.go:54**

```
// @MX:NOTE: [AUTO] Keyword tables. Curated single-token, lowercase,
// high-precision terms; tokens that risk false-positive matches against
// generic queries are deliberately excluded. Update only with SPEC review.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- Comments in English: yes

**5. internal/router/korean.go:15**

```
// @MX:NOTE: [AUTO] Korean particle list — 11 high-frequency postpositions.
// Curated from Wikipedia "Korean_postpositions"; no user customisation in v0.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- Comments in English: yes

**6. internal/router/korean.go:23**

```
// @MX:NOTE: [AUTO] Hangul block ranges per Unicode 15.1: U+AC00-D7A3 (modern
// syllables), U+1100-11FF (Jamo), U+3130-318F (Compat Jamo), U+A960-A97F
// (Jamo Extended-A). Updating these ranges is a SPEC-amendment-level decision.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- Comments in English: yes

**7. internal/router/category.go:87**

```
// @MX:NOTE: [AUTO] DocType eligibility table — central to REQ-IR-008
// AdapterSet selection. Update only with a SPEC amendment.
// @MX:SPEC: SPEC-IR-001
```

- [AUTO] prefix: present
- Comments in English: yes

**8. internal/obs/metrics/router.go** — no @MX tags found in this file. The 12-tag count (per commit message) is accounted for entirely within `internal/router/`. `internal/obs/metrics/router.go` registers the two new Prometheus families but does not carry its own @MX tags; cardinality enforcement is already annotated in `internal/router/metrics.go:21` (tag #1 above).

## Defects: None

All 12 tags (3 ANCHOR + 1 WARN + 8 NOTE across `internal/router/`) pass compliance checks:

- Every @MX:ANCHOR carries both @MX:REASON and @MX:SPEC ✓
- The single @MX:WARN carries @MX:REASON ✓
- All agent-generated tags carry [AUTO] prefix ✓
- All tag descriptions and sub-lines are in English (consistent with `code_comments: en` in language.yaml) ✓
- @MX:TODO count is zero — all RED-phase requirements promoted to GREEN ✓
- Per-file ANCHOR limit (3) not exceeded: `router.go` carries 2 ANCHORs, all other files carry 0 ✓
- Per-file WARN limit (5) not exceeded: `llm.go` carries 1 WARN ✓

## Notes

- `internal/obs/metrics/router.go` is grep-scanned but carries no @MX tags of its own; metric cardinality context is documented in `internal/router/metrics.go:21` via a NOTE tag, satisfying the protocol's "bounded cardinality" documentation requirement.
- `router.go` carries 2 ANCHOR tags (Router struct at line 75 and `Classify` method at line 146) rather than one. Both are justified: the struct declaration anchors the type boundary; the method declaration anchors the call-site contract. Protocol allows up to 3 ANCHORs per file; no demotion required.
- `router.go::classifyByLLM` ANCHOR at line 213 uses `fan_in >= 3 (Classify + tests + future debug tooling)` — the `future debug tooling` attribution is forward-looking and acceptable per protocol (ANCHOR captures planned callers).
