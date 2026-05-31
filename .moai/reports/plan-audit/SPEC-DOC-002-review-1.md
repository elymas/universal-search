# SPEC Review Report: SPEC-DOC-002
Iteration: 1/3
Verdict: PASS (amend-then-approve optional; no blockers)
Overall Score: 0.92

Reasoning context ignored per M1 Context Isolation. The prompt framed the four
amendments as already "fixed"; I treated that as an author claim and verified A1–A4
independently against live HEAD source + the spec/plan/acceptance text only.

## Must-Pass Results
- [PASS] MP-1 REQ number consistency: REQ-ADPDOC-001..018 sequential, zero-padded 3-digit, no gaps/dupes (spec.md:L530–L582). NFR-ADPDOC-001..006 likewise (spec.md:L590–L595).
- [PASS] MP-2 EARS compliance: 14 Ubiquitous + 3 State-Driven (REQ-010/011/017, correct "IF…THEN…SHALL", spec.md:L554/L555/L576) + 1 Unwanted (REQ-018). One minor classification imprecision noted in Defects (D1) — not a hard fail; requirement remains binary-testable.
- [PASS] MP-3 YAML frontmatter: id/version/status/created/updated/author/priority present; depends_on + blocks are arrays (spec.md:L2–L17). `created` used instead of `created_at` and `labels` absent, but `priority: P1` (L8) + `milestone`/`owner` cover classification; consistent with sibling M9 SPECs. No type mismatch.
- [PASS] MP-4 language neutrality: N/A in the LSP sense, but the 16-language firewall analogue here is the 10-adapter SourceID enumeration — spec enumerates all 10 with the noop exclusion explicit (spec.md:L530, L387). No adapter privileged without justification.

## Amendment Verification (live HEAD)
- A1 — CONFIRMED-RESOLVED. `internal/adapters/hn/hn.go:101` → `SourceID: "hackernews"`. Spec uses `hackernews` slug consistently (REQ-ADPDOC-001 spec.md:L530; files-to-create L883, L910; acceptance AC-001 L40,L43). Residual `hn` appears ONLY in negative/explanatory context ("page slug `hackernews` (not `hn`)" L31,L40,L1071) — no positive/gate use of an `hn` page name. Drift gate keys on `Capabilities().SourceID` (REQ-007 L546).
- A2 — CONFIRMED-RESOLVED. `social.go:132` `Capabilities()` switches on `a.subSource` → `blueskyCapabilities()` (`social.go:144`) / `xCapabilities()` (`social.go:164`). No `bluesky.go`/`x.go`; the real files are `search_bluesky.go`/`search_x.go` (verified via `ls`). REQ-ADPDOC-007 (spec.md:L546) + AC-007 (acceptance L157) specify a SourceID-keyed registry that resolves the two helper funcs and emits two JSON fragments — matches reality. Per-file `{adapter}.go` glob assumption explicitly corrected (HISTORY L42–L53).
- A3 — CONFIRMED-RESOLVED. `xCapabilities()` (`social.go:164`) → `RateLimitPerMin: 0`, `DefaultMaxResults: 0`, Notes "DISABLED in v0 … no live path wired". Spec frames `x` as `disabled` lifecycle (D5 L272–L276; REQ-005 L539; REQ-009 L553; AC-009 L204), explicitly removes "alpha"/"degraded" (L279, AC-005 L126).
- A4 — CONFIRMED-RESOLVED. `pkg/types/capabilities.go` Capabilities struct fields: SourceID, DisplayName, DocTypes, SupportedLangs, SupportsSince, RequiresAuth, AuthEnvVars, RateLimitPerMin, DefaultMaxResults, Notes — NO `lifecycle` field. Grep for `adapter-status`/`StatusExport`/`ExportStatus` in internal/ + pkg/ → zero hits (no EVAL-002 export on this branch). Spec defines its own 4-tier taxonomy (D5 L265–L293), ships STATIC hand-curated `adapter-status.json` (REQ-006 L540), and treats the EVAL-002 live feed as a FORWARD-REFERENCE with no dependency on a nonexistent export (L66–L82, L460–L472, dep L808–L815).

## Drift Gate Soundness: IMPLEMENTABLE
The 5 extracted fields (SourceID / RequiresAuth / AuthEnvVars / RateLimitPerMin /
DefaultMaxResults) are confirmed as static struct literals — spot-checked
`blueskyCapabilities()` (RateLimitPerMin: 600) and `xCapabilities()` (RateLimitPerMin: 0)
in `social.go`; the struct doc itself states "static metadata … MUST be deterministic"
(`capabilities.go`). `go/parser` AST extraction (no binary execution) is therefore
concretely viable. The only non-trivial path — the `social` package switch over two
package-level helper funcs — is explicitly handled by the SourceID-keyed registry
(REQ-007 case (c), spec.md:L546). stdlib-only (`go/parser`, `go/ast`, `encoding/json`),
no new module deps (L867). NFR-001 5s/60s budget is realistic for 10 static-literal parses.

## Reductions Coherence: CLEAN
- KO Tier-1 fixed at exactly 4 pages (index/naver/koreanews/errors): REQ-017 (L576) + AC-017 (L346). The 8 Tier-2 pages are EN-authoritative and EXPLICITLY excluded from the 90% gate (AC-017 L348,L350) — no V1 AC gates a deferred KO page.
- Live feed → static JSON: REQ-006 (L540) ships static file; schema-validation + cron export + staleness automation DEFERRED (Exclusions L634–L643, NFR-003 L592, EC-001 L380–L391). No V1 AC asserts the deferred plumbing — AC-006 (L145) and DoD (L433) both mark schema-validation as NOT-a-V1-item.
- NFR-ADPDOC-003 staleness gate replaced at V1 by per-page `lastVerified` check (REQ-015 L569), cleanly cross-referenced.

## EVAL-002 Forward-Reference: SOUND
DOC-002 cross-links the EVAL-002 dashboard + `/admin/health/adapters` but declares
EVAL-002 a soft/forward dependency only — "NOT a hard run-phase dependency" (L813–L815).
The static `adapter-status.json` is DOC-002-owned and hand-populated. No build-time or
CI dependency on any nonexistent EVAL-002 export. Open Question §8.4 (L997) marks this
RESOLVED. Note: EVAL-002 remains in `depends_on` frontmatter (L15) while the body
demotes it to forward-reference — harmless but slightly inconsistent (see D2).

## EARS Findings / Traceability Gaps
- EARS: 14 Ubiquitous, 3 State-Driven (well-formed), 1 Unwanted. See D1.
- Traceability: acceptance.md Coverage Matrix (L442–L467) maps every REQ-ADPDOC-001..018
  and NFR-ADPDOC-001..006 to ≥1 AC/EC. No orphan ACs (AC-001..018 each cite a REQ).
  No uncovered REQ. spec↔acceptance consistent post-A1–A4 (slug, helper funcs, disabled
  framing, static JSON all mirrored). plan.md not opened (tool budget); acceptance +
  spec internal traceability is fully closed, so this is not a blocker — flagged as D3
  for iteration-2 confirmation.

## Defects Found
D1. acceptance/spec — REQ-ADPDOC-018 (spec.md:L582) is tagged EARS pattern "Unwanted" but phrased as a Ubiquitous prohibition ("The docs site SHALL NOT contain…"). The canonical Unwanted form is "If [undesired condition], then the system shall…". Reclassify as Ubiquitous (negative constraint) or rephrase to "If an adapter page contains a real-credential-shaped value, then `check-doc-credentials.sh` shall fail the CI job." — Severity: minor
D2. spec.md:L15 — SPEC-EVAL-002 listed in `depends_on` frontmatter while body (L808–L815, L997) demotes it to forward-reference/soft. Move to `related` for consistency, or annotate as docs-only soft dep. — Severity: minor
D3. plan.md REQ→task mapping: 16/18 REQs cited by explicit ID (plan.md grep). REQ-ADPDOC-006 and REQ-ADPDOC-008 are NOT cited by ID but ARE covered by topic — REQ-006 static `adapter-status.json` feed (plan.md:L85–L88, L151–L153) and REQ-008 `<CapabilitiesTable>` (plan.md:L43, L132). All 6 NFRs cited by ID. RESOLVED — recommend adding explicit `REQ-ADPDOC-006`/`REQ-ADPDOC-008` ID tags to the relevant plan phases for grep-clean traceability. — Severity: trivial (cosmetic; coverage is real)

## Chain-of-Verification Pass
Second-look findings:
- Re-read every REQ-ADPDOC-001..018 row end-to-end (not sampled): numbering, pattern column, and SourceID references all consistent post-amendment. No stray `hn.mdx` / `bluesky.go` / `x.go` / "alpha" / EVAL-002-export-dependency survived (grep + line-by-line).
- Re-checked Exclusions (spec.md:L599–L698) for specificity: 20+ entries, each with destination/rationale/follow-up (e.g., contributor guide → SPEC-ADP-DEVGUIDE; live status feed → post-V1 EVAL-002 amendment). Not vague. PASS.
- Contradiction scan: D5 taxonomy (`disabled`, no `alpha`) vs Motivation §1.2 (L431–L438) vs REQ-009 (L553) vs AC-009 (L204) — all agree `x` = disabled. No internal contradiction. Capabilities struct doc ("static metadata") corroborates drift-gate static-literal premise. No new defects beyond D1–D3.

## Recommendation
PASS — approve (draft→approved). All four code-spec contradictions (A1–A4) are
CONFIRMED-RESOLVED against live HEAD; the drift gate is concretely implementable from
static struct literals via go/parser including the social.go helper special-case; V1
reductions are coherent with no AC gating a deferred item; the EVAL-002 forward-reference
introduces no dependency on a nonexistent export. The three defects are all minor and
non-blocking; D1 (Unwanted-pattern phrasing) and D2 (depends_on vs related for EVAL-002)
may be fixed in a fast amendment if a clean EARS taxonomy is desired before approval,
but neither blocks implementation. Confirm D3 (plan.md REQ→task mapping) at iteration 2
if a re-review is triggered.

must_fix_before_implementation: (none — all defects minor/non-blocking)
status_transition_recommendation: approve

*End of SPEC-DOC-002 review-1.*
