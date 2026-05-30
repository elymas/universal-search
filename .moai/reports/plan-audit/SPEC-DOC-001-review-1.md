# SPEC Review Report: SPEC-DOC-001

Iteration: 1/3
Verdict: PASS-WITH-FINDINGS
Overall Score: 0.90

> Reasoning context ignored per M1 Context Isolation. Audit performed against spec.md + live code only.

## Phase 0 Gate Summary (Korean)

3개 stale assumption(B1/B2/B3) 모두 LIVE 코드 대조로 CONFIRMED-RESOLVED.
Proportionality 축소(KO Tier-1, a11y/Lighthouse/Playwright→V1.1, Docker
publish deferred)는 모든 V1 acceptance criterion과 coherent — deferred
항목에 gating하는 V1 AC 없음. link-check gate는 SEC-001 unmerged 상태에서
fail하지 않음(forward-ref + lychee allowlist). EARS/traceability 양호.
status draft → **approve** 권고 (minor finding은 run phase에서 자연 해소).

---

## Must-Pass Results

- [PASS] MP-1 REQ number consistency: REQ-DOC-001..018 sequential, no gaps,
  no duplicates, consistent zero-padding (spec.md:489-531). 18 REQs = HISTORY
  claim "18 EARS REQs" (spec.md:351) matches.
- [PASS] MP-2 EARS format compliance: every REQ tagged with explicit pattern
  column (Ubiquitous/Event-Driven/State-Driven/Optional/Unwanted) and uses
  SHALL normative verb. Spot: REQ-DOC-004 Event-Driven "WHEN a user...lands...
  the docs SHALL present" (L497); REQ-DOC-016 State-Driven "IF the bilingual-
  coverage CI job runs, THEN the script SHALL..." (L524); REQ-DOC-018 Unwanted
  "SHALL NOT include marketing copy" (L531); REQ-DOC-008 Optional "WHERE
  SPEC-DOC-002...has shipped" (L506). All five patterns represented correctly.
- [PASS] MP-3 YAML frontmatter validity: id/version/status/created/priority
  present (L2-8). Note: `created` not `created_at` and no `labels` field — see
  Findings D2 (non-blocking; project house style uses milestone/owner/methodology
  fields, frontmatter is internally consistent and complete for its schema).
- [N/A] MP-4 Section 22 language neutrality: not a 16-language tooling SPEC.
  Docs-site SPEC; the bilingual EN+KO scope is intentional product scope, not
  LSP/tool-language neutrality. Auto-pass.

## Category Scores (0.0-1.0, rubric-anchored)

| Dimension | Score | Rubric Band | Evidence |
|-----------|-------|-------------|----------|
| Clarity | 0.90 | 0.75-1.0 | REQs single-interpretation; forward-ref rule stated [HARD] (L498) |
| Completeness | 1.0 | 1.0 | HISTORY/Overview/REQs/NFRs/Exclusions(28 entries)/Deps/Files/OpenQ/Refs all present (L22-1024) |
| Testability | 0.90 | 0.75-1.0 | ACs binary; weasel-free; one soft spot REQ-DOC-004 manual-trace (L497) |
| Traceability | 0.95 | 0.75-1.0 | 17 scenarios map REQ→AC (L664-683); REQ-DOC-008/017/018 covered |

## Amendment Verification (LIVE code)

- **B1 (docs/ not greenfield, Next ^16.2.6)** → CONFIRMED-RESOLVED.
  `web/package.json`: `"next": "^16.2.6"` (live grep). REQ-DOC-001 pins
  Next `^16.2.6`, requires v3-artifact cleanup FIRST + preserves git-tracked
  `docs/dependencies.md`/`_deps-*`/`licenses/` (spec.md:489, 885). §1 inventory
  documents stale v3 artifacts vs git-tracked docs distinctly (L105-119).
- **B2 (SEC-001 ops/security forward-ref)** → CONFIRMED-RESOLVED.
  `git ls-files ops/security/` returns EMPTY on main (live). REQ-DOC-005
  makes the 3 security pages FORWARD-REFERENCE placeholders, [HARD] "No docs
  page SHALL emit a link to a nonexistent ops/security/* file" (spec.md:498).
  REQ-DOC-013 adds lychee exclude allowlist for deferred refs (L521). §6.1
  + §7.3 + §1.3 all consistently mark SEC-001 PR #42 unmerged (L725, L902, L462).
- **B3 (7 CLI subcommands)** → CONFIRMED-RESOLVED.
  `cmd/usearch/root.go` `registerSubcommands()` (L61-85) registers exactly 7:
  query(63), config(66), history(69), repl(72), deep(75), sources(78),
  login(81). REQ-DOC-007 enumerates the same 7 set verbatim AND requires
  "no hardcoded subcommand list" auto-track (spec.md:505). Match exact.

## Reductions Coherence

**clean.** No V1 acceptance criterion gates on a deferred item:
- a11y → REQ-DOC-017 P1, AC explicitly "manual axe-core at V1.0.0 freeze";
  automated Pa11y/axe CI excluded (spec.md:530, 623-625).
- Lighthouse → NFR-DOC-003 manual at freeze; automated CI excluded (L541, 627).
- Playwright auto-screenshot → excluded; REQ-DOC-014 uses manual + freshness
  gate only (L643-646).
- full-KO → REQ-DOC-016 gate scoped to Tier-1 only; `reference/` subtree
  EXCLUDED from V1 gate, AC asserts "removal of a reference/ KO page does NOT
  affect the gate" (spec.md:524). REQ-DOC-010 Tier-1 list matches gate (L513).
- Docker publish → REQ-DOC-015 gh-pages is V1 gate; container job is
  build-and-verify-only, push DEFERRED/conditional, AC "does NOT block V1
  when <org> unresolved" (spec.md:523). Exclusions L637-641 consistent.

## Link-Check Safety (SEC unmerged won't break gate?)

**SAFE.** Three independent layers: (1) REQ-DOC-005 [HARD] forbids emitting
`ops/security/*` links pre-merge (L498); (2) security pages are in-site
placeholders linking SPEC-SEC-001 status only; (3) REQ-DOC-013 provides a
`docs/lychee.toml` deferred-reference exclude allowlist with unblocking-SPEC
comment, "link-check job MUST NOT fail on these deferred references" (L521).
acceptance.md AC-012 cross-check below.

## EARS Findings

- All 18 REQs carry an explicit EARS pattern label and SHALL verb. No informal
  "should/may" in normative REQ text. (NFRs use SHALL appropriately.)
- Minor: REQ-DOC-004 acceptance leans on "manual trace test" (subjective edge)
  but is bounded by concrete sub-page existence + Next-button checks (L497) —
  acceptable, not a weasel-word failure.

## Traceability Gaps

- §5 scenario index covers REQ-DOC-001..018 + NFR-001/002. NFR-DOC-003/004/005/
  006/007 lack a dedicated scenario row (NFR-004 partially via §5.12). Minor;
  NFRs commonly verified outside GWT scenarios. Non-blocking.

## Defects Found

D1. spec.md:524 / REQ-DOC-016 — minor — coverage gate uses "≥ 90%" Tier-1
    parity but the Tier-1 EN page set is enumerated by prose reference to
    REQ-DOC-010 rather than a frozen page-count; run phase must derive the
    exact denominator so the 90% is deterministic. Severity: minor.
D2. spec.md:2-8 — minor — frontmatter uses `created` (not `created_at`) and
    omits a `labels`/`tags` field. Internally consistent project schema, but
    flag for cross-SPEC frontmatter-lint consistency. Severity: minor.
D3. spec.md:923-931 (Open Q §3/§4) — minor — `<org>` and registry path
    unresolved; spec correctly gates Docker publish behind this (REQ-DOC-015)
    and the gh-pages canonical URL also contains `<org>` placeholder. gh-pages
    V1 gate (L523) technically depends on `<org>` resolution at deploy time —
    confirm REL-001 resolves `<org>` before the gh-pages deploy AC can pass.
    Severity: minor (flagged as known Open Question, non-blocking for Phase 0).

## Chain-of-Verification Pass

Second-look findings:
- Re-read REQ sequence end-to-end (001-018): confirmed no gap/dup, 18 total.
- Re-checked all 5 EARS patterns present: Ubiquitous (001/002/003/005/006/007/
  009/010/011/017), Event-Driven (004/013/014), State-Driven (016), Optional
  (008), Unwanted (018). All five covered — no pattern mislabeled.
- Re-verified B1/B2/B3 against LIVE outputs (not spec self-claim): next
  ^16.2.6 confirmed, ops/security empty confirmed, root.go 7 subcommands
  confirmed by reading L61-85.
- Re-read Exclusions (28 entries): every deferred item (a11y CI, Lighthouse CI,
  full-KO ref, Docker publish, Playwright) has destination + rationale; none
  contradict an included REQ. Verified Docker-publish exclusion (L637) vs
  REQ-DOC-015 (L523) — coherent (build-verify kept, push deferred).
- New defect surfaced in 2nd pass: D3 (gh-pages `<org>` dependency on the V1
  gate itself, not just the deferred container push) — added above.

## Recommendation

PASS-WITH-FINDINGS. The amendment landed cleanly and is verifiable against
live code; reductions are internally coherent with zero V1-AC dependency on a
deferred item; link-check gate is safe under SEC-001 unmerged.

Run-phase carry-forward (non-blocking):
1. REQ-DOC-016: freeze the Tier-1 EN page denominator so 90% is deterministic.
2. Resolve `<org>` (Open Q §3) before the gh-pages deploy AC is evaluated —
   coordinate with SPEC-REL-001.
3. Optional: align frontmatter field naming (`created_at`, `labels`) with any
   cross-SPEC frontmatter lint.

status_transition_recommendation: **approve** (no must-fix blockers).
