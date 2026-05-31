# SPEC Review Report: SPEC-EVAL-002

Iteration: 1/3
Verdict: PASS-WITH-FINDINGS
Overall Score: 0.88

> Reasoning context ignored per M1 Context Isolation. The v0.2.0 amendment
> self-report was treated as unverified claims and confirmed/refuted against
> LIVE code, not trusted as ground truth.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: `REQ-EVAL2-001` … `REQ-EVAL2-010`
  sequential, no gaps, no duplicates, consistent 3-digit zero-pad
  (spec.md:255–284). `NFR-EVAL2-001` … `NFR-EVAL2-005` sequential
  (spec.md:292–296). No defect.

- **[PASS] MP-2 EARS compliance**: All 10 REQs match a valid EARS pattern —
  Ubiquitous (001/003/005/006/009), Event-Driven (002 "WHEN Prometheus
  evaluates" L256, 004 "WHEN `fanout.Dispatch` returns" L263, 008 "WHEN …
  drops below" L277, 010 "WHEN an operator … issues GET" L284), Optional
  (007 "WHERE the operator deploys" L271). REQ-EVAL2-010 decomposes cleanly
  into (a) ubiquitous fill + (b) event-driven endpoint — acceptable. Minor
  EARS-purity nit only (see EARS findings).

- **[PASS (project schema)] MP-3 YAML frontmatter validity**: `id`
  (SPEC-EVAL-002), `version` (0.2.0), `status` (draft), `priority` (P1),
  `created` (2026-05-22, ISO) all present and correctly typed (spec.md:2–17).
  The generic rubric expects `created_at` and `labels`; this project's SPEC
  schema uses `created` + `milestone`/`owner`/`depends_on`/`related` and
  omits `labels` — consistent with sibling SPECs (e.g., SPEC-LSP-CORE-002).
  Judged against the project schema, not the foreign generic template:
  PASS. Divergence documented transparently (D-note below), not scored as FAIL.

- **[N/A] MP-4 Section 22 language neutrality**: This SPEC targets
  Prometheus/Grafana adapter-telemetry, not LSP multi-language tooling. The
  12 adapters are enumerated with equal weight (spec.md:59–64); no language
  server is hardcoded as primary. The 16-language LSP rule does not apply.
  Auto-pass.

---

## Category Scores (rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.85 | 0.75 band | Single unambiguous interpretation for nearly all REQs; cardinality budget framing is muddled (NFR-EVAL2-001 L292 counts existing series inside "three new families"). |
| Completeness | 0.90 | 1.0 band (minor staleness) | HISTORY, Overview, Requirements, NFRs, Exclusions (15 entries L306–403), Acceptance, Dependencies, Files, Open Questions all present. Frontmatter complete per project schema. Deducted for stale "5 panel"/"4 alert" counts surviving the A2 deferral. |
| Testability | 0.85 | 0.75 band | Most ACs binary-testable with concrete tools (`promtool check rules` exits 0, `≤ 0.001` tolerance, HTTP 200/503). One concrete test target is self-contradictory (D1: ≤120 vs ≤132). |
| Traceability | 0.92 | 1.0 band (one count drift) | Coverage Matrix (acceptance.md:351–367) maps every REQ-001..010 + NFR-001..005 to ≥1 AC; no orphan REQ; every REQ maps to a plan phase. Deducted for NFR-EVAL2-002 "5 panels" not reconciled with the 4-panel V1 gate. |

---

## Defects Found

- **D1. spec.md:262 — REQ-EVAL2-003 acceptance cardinality arithmetic is
  inconsistent with the rest of the document.** It states
  "total series … ≤ 12 × 6 + 12 + 12 × 3 = 120 series" (72+12+36=120, the
  `+12` health term is dropped), while NFR-EVAL2-001 (L292), AC-010
  (acceptance.md:234), §5.10 (L425) and HISTORY D3 (L93) all compute
  **132** (72+12+12+36). This is a concrete, self-contradictory test
  assertion target. — Severity: **minor** (both < 500, NFR not violated;
  but the unit-test bound is ambiguous).

- **D2. spec.md:262,292 — "three new metric families" budget conflates the
  EXISTING `usearch_adapter_calls_total` (72 series).** NFR-EVAL2-001 title
  caps "the three new metric families … ≤ 500" yet the math folds in the
  pre-existing calls_total counter. The three genuinely-new families
  (`fanout_partial` 12 + `health_status` 12 + `circuit_state` 36) total
  **60** series, not 120/132. Conservative (safe) over-count, but conceptually
  muddled. — Severity: **minor**.

- **D3. spec.md:293 — NFR-EVAL2-002 still says "render all 5 panels".**
  Post-A2 the V1 dashboard has 4 core panels (REQ-EVAL2-006 L270, AC-004
  acceptance.md:101, plan Phase 6 L301). The amendment did not reconcile
  this NFR. — Severity: **minor** (stale count, not gate-breaking).

- **D4. spec.md:516,518 — §7.1 file table is stale post-deferral.**
  `deploy/prometheus/alerts.yml` listed as "4 alert rules" (V1 = 3);
  `adapter-reliability.json` listed as "5-panel dashboard" (V1 = 4). Body of
  the spec is correct; the create-file table was not updated. — Severity:
  **minor**.

- **D5. plan.md:527,541 — plan §3 file inventory stale.** Same drift:
  alerts.yml "4 alert rules", dashboard "5-panel dashboard". — Severity:
  **minor**.

- **D6. plan.md:406,425 — plan Phase 8 references "4 alert" runbook
  sections.** V1 ships 3 alerts; acceptance DoD (acceptance.md:339) correctly
  says "each V1 alert name" = 3. Runbook section count should be 3 (+ circuit
  post-V1). — Severity: **minor**.

- **D7. spec.md:243 / plan.md:362–364 — admin-reuse telemetry data-source
  plumbing is under-specified (acknowledged).** `Registry.SnapshotForAdmin`
  (registry.go:243) holds no reference to `obs.Metrics`; there is currently
  no wire from the adapters package to the metrics registry to read
  per-adapter success/fail counts. The spec/plan defers the mechanism to run
  phase (plan "Open factoring" #2, L666–669: Collector.Collect vs sync/atomic).
  Acknowledged, but a reviewer should note the fill mechanism is not yet
  designed. — Severity: **minor** (acknowledged run-phase decision, not a
  blocker).

- **D8. research.md / tasks.md — stale `:223` / `:9090` references (out of
  edit scope).** The amendment self-notes these companion artifacts still
  carry the pre-A1/A3 line/port references. Flagged per audit request. —
  Severity: **minor** (documentation hygiene).

- **D-note (not scored as defect). spec.md:2–17 — frontmatter omits generic
  `labels` and uses `created` not `created_at`.** This matches the project's
  established SPEC schema and is NOT penalized; recorded for transparency.

---

## Chain-of-Verification Pass

Second-look findings (re-read sections rather than skim):

- **AdapterAdminView.Status enum conflict — REFUTED as a defect.** Live
  `AdapterAdminView.Status` is `{connected,auth_required,disabled,error}`
  (registry.go:203). The REQ-EVAL2-010 health surface uses
  `{healthy,degraded,unhealthy}` on a SEPARATE sibling handler
  (`/api/admin/adapters/health`) with its own JSON schema, not on
  `AdapterAdminView.Status`. No conflict. Confirmed by re-reading
  registry.go:200–229 and REQ-EVAL2-010 (spec.md:284).

- **A1 reuse plumbing — VERIFIED end-to-end.** `AdaptersHandler.ServeHTTP`
  (handler_adapters.go:37) serializes `SnapshotForAdmin()` directly, so
  populating `SuccessCount`/`FailCount` in the struct WILL flow into
  `GET /api/admin/adapters` automatically. Mount at main.go:71 behind
  `LoopbackOnly` (middleware_loopback.go:20) confirmed.

- **Cited line numbers — VERIFIED EXACT.** success_count int64 @212,
  fail_count int64 @215, AdapterAdminView @200, SnapshotForAdmin @243,
  emit @433, main.go:71, prometheus.yml:9 (`evaluation_interval: 15s`),
  calls_total labels `[adapter, outcome]` @150. Every cited location is
  accurate in live code.

- **REQ numbering & traceability re-checked end-to-end (not spot-checked):**
  all 10 REQs + 5 NFRs verified individually against the Coverage Matrix.
  No orphan, no AC pointing at a non-existent REQ, no V1 AC requiring the
  deferred circuit alert/panel (AC-008 explicitly deferred; EC-001 asserts
  V1 absence). The only traceability weakness is the panel/alert count
  staleness (D3–D6), not a broken link.

- **Exclusions specificity re-checked:** 15 entries, each with a named
  destination SPEC + rationale (spec.md:306–403). Strong.

No NEW must-pass defects discovered on second pass. First-pass defect set
stands.

---

## Recommendation (PASS-WITH-FINDINGS → amend-then-approve)

All five amendments are CONFIRMED-RESOLVED against live code and no must-pass
criterion fails. The findings are mechanical documentation-consistency nits
plus one acknowledged run-phase design gap; none block implementation. Because
two of them (D1, D3) leave a self-contradictory test target / stale gate count
inside the SPEC body, a trivial v0.2.1 micro-edit is advised before
`draft → approved` so the implementer's test bounds are unambiguous.

must_fix_before_implementation (ordered, all small/mechanical):

1. **D1** — Fix REQ-EVAL2-003 acceptance bound: change
   `≤ 12 × 6 + 12 + 12 × 3 = 120` to the document-wide `= 132`
   (or, better per D2, state the three NEW families = 60 series and the
   ≤500 cap covers the full adapter metric surface). (spec.md:262)
2. **D3** — NFR-EVAL2-002: "all 5 panels" → "all 4 core panels". (spec.md:293)
3. **D4** — §7.1 file table: alerts.yml "4 alert rules" → "3 V1 alert rules
   (4th deferred)"; dashboard "5-panel" → "4-panel V1". (spec.md:516,518)
4. **D5** — plan §3 file inventory: same correction. (plan.md:527,541)
5. **D6** — plan Phase 8: "4 alert sections" → "3 V1 alert sections
   (circuit post-V1)". (plan.md:406,425)
6. **D7** — Add one sentence to REQ-EVAL2-010 / plan Phase 7 naming the
   data path from `obs.Metrics.AdapterCalls` into `Registry.SnapshotForAdmin`
   (the adapters package currently has no metrics-registry reference) — even
   if the exact mechanism stays a run-phase choice. (spec.md:284, plan.md:362)
7. **D8** — Sync `:223`/`:9090` references in research.md/tasks.md (low
   priority, can be a sync-phase cleanup).

If the orchestrator prefers a clean gate, items 1–3 are the minimum to fix
before approval; items 4–7 may be deferred to the implementation PR.
