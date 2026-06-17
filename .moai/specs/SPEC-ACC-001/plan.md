# SPEC-ACC-001 Implementation Plan

File-level implementation plan for SPEC-ACC-001 — Access Layer WAF
Profile Detection + TLS Hardening. Methodology: TDD (RED-GREEN-REFACTOR).
No time estimates — priority labels and ordering only.

---

## Technical Approach

Port three insane-search v0.5 concepts into the existing
`internal/access/` 5-phase cascade with TWO new pure-function files and
surgical edits to FIVE existing files. Zero new module dependencies; no
config file; no new network surface. The work is data-driven WAF
classification + an AND-gated page-validity gate + a candidate-set
filter — it does not touch the SSRF guards, the dialer, robots.txt, or
Phase 5.

---

## Milestones (priority-ordered, no dependencies between unless noted)

### Milestone 1 (P1) — WAF profile detection (new, self-contained)

New file, no edits to existing code yet. Pure functions, fully testable
in isolation.

1. Create `internal/access/wafprofile.go`:
   - `WAFProfile` struct (`ID`, `DisplayName`, `CookiePatterns`,
     `HeaderMarkers`, `BodyMarkers`). No `TLSAvoidList` field in v0.1 —
     deferred to SPEC-CACHE-001b.
   - `wafProfiles` table — 7 vendor-generic entries (`akamai`,
     `cloudflare`, `f5`, `aws-waf`, `datadome`, `perimeterx`,
     `unknown`). No-Site-Name rule (§2.3).
   - `ProfileHit` struct (`ProfileID`, `Confidence`).
   - `wafEscalateThreshold = 0.3` constant.
   - `detectProfiles(resp *http.Response, body []byte) []ProfileHit` —
     additive confidence model (§2.4), sorted descending, ties by ID;
     `unknown@0.2` fallback for 403/503/challenge with no vendor match;
     empty slice for clean 200.
2. Create `internal/access/wafprofile_test.go` (RED first):
   - `TestWAFProfilesTableShape` (REQ-ACC-010)
   - `TestDetectProfilesRanking`, `TestDetectProfilesConfidenceFormula`
     (REQ-ACC-011)
   - `TestDetectProfilesUnknownFallback`,
     `TestDetectProfilesCleanResponseEmpty` (REQ-ACC-012)
   - `TestNoSiteNameRule` (REQ-ACC-040)
3. Create testdata fixtures: `waf_akamai_abck_challenged.json`,
   `waf_cloudflare_challenge.html`, `waf_datadome.json`,
   `page_strong_ok.html`.
4. Apply `@MX:ANCHOR` to `detectProfiles`, `@MX:NOTE` to `wafProfiles`
   + `wafEscalateThreshold`.

### Milestone 2 (P1) — 4-layer page-validity validator (new, self-contained)

New file, depends on Milestone 1's `ProfileHit` type only.

1. Create `internal/access/validity.go`:
   - `Verdict` string-enum + 5 constants.
   - `minRealPageBytes = 512` constant + success-selector set
     (`<main`, `<article`, `id="content"`, `class="content"`).
   - `validatePage(resp *http.Response, body []byte, hit *ProfileHit)
     Verdict` — AND-gated 4-layer logic per the §6.3 truth table.
2. Create `internal/access/validity_test.go` (RED first):
   - `TestValidatePageVerdictTable` (REQ-ACC-020) — every §6.3 row +
     AND-gating + Akamai `_abck=~-1~` sensor case.
3. Apply `@MX:ANCHOR` to `validatePage`, `@MX:NOTE` to
   `minRealPageBytes`.

### Milestone 3 (P1) — PhaseAttempt field migration (edit; ripples)

Edit `types.go` and the escalation logic together (they are coupled).

1. Edit `internal/access/types.go`:
   - Remove `isWAF bool` from `PhaseAttempt`.
   - Add `profileHits []ProfileHit` and `verdict Verdict`.
   - Add `(*PhaseAttempt).topProfile() (ProfileHit, bool)` and
     `(*PhaseAttempt).hasWAFProfile() bool`.
2. Edit `internal/access/escalation.go`:
   - Case 3: `prev.isTLSError || prev.hasWAFProfile() ||
     prev.verdict == VerdictChallenge || prev.verdict == VerdictBlocked`.
3. Edit `internal/access/escalation_test.go` (RED first):
   - `TestPhaseAttemptTopProfile`, `TestPhaseAttemptHasWAFProfile`,
     `TestShouldEscalatePhase3OnWAFProfile` (REQ-ACC-013).
   - Extend `TestShouldEscalateTable` with profile + Verdict rows.

### Milestone 4 (P1) — Phase 3 integration (edit)

Wire detection + validation into the standard GET path.

1. Edit `internal/access/phase3_get.go`:
   - Replace `isWAFResponse(resp)` usage with `detectProfiles(resp,
     body)`; set `attempt.profileHits`.
   - Run `validatePage(resp, body, topHit)` on the 200 path; set
     `attempt.verdict`.
   - Verdict-gated success: a 200 whose Verdict is `VerdictChallenge`/
     `VerdictBlocked` is NOT success and signals escalation.
   - Keep `isWAFResponse`/`wafHeaders` only if still referenced;
     otherwise remove (the profile table's Akamai/Cloudflare/Fastly
     header markers subsume them).
2. Edit `internal/access/phase3_test.go` (RED first):
   - `TestPhase3SetsVerdict` (REQ-ACC-020)
   - `TestPhase3Silent200ChallengeNotSuccess` (REQ-ACC-021)
   - `TestPhase3WeakOKIsSuccess` (REQ-ACC-022)

### Milestone 5 (P1) — Phase 4 integration + validation (edit)

Wire validation into the TLS pass. (The TLS avoid-list filter is
DEFERRED to SPEC-CACHE-001b — Phase 4's transport construction is
unchanged from CACHE-001 in v0.1.)

1. Edit `internal/access/cascade.go`:
   - Thread the Phase 3 attempt's `topProfile()` into the Phase 4
     dispatch (so Phase 4 can pass it to `validatePage` — the L3
     cookie-sensor layer is profile-aware).
   - Carry `profileHits`/`verdict` through `runPhase`/`dispatchPhase`
     the same way `isTLSError`/`isJSChallenge` are carried.
   - Update the success gate to be Verdict-aware (challenge/blocked 200
     is not success).
2. Edit `internal/access/phase4_tls.go`:
   - Run `validatePage` on the 200 path; set `attempt.verdict`.
   - Leave the TLS transport construction unchanged (no candidate-set
     filtering in v0.1; the avoid-list is deferred to SPEC-CACHE-001b).
3. Edit `internal/access/phase4_test.go` (RED first):
   - `TestPhase4Silent200BlockedNotSuccess` (REQ-ACC-021)

### Milestone 6 (P1) — NFR verification + regression

1. Add `TestNoNewNetworkCalls` to `cascade_test.go` (NFR-ACC-002).
2. Add `BenchmarkDetectAndValidate` (NFR-ACC-001) — exclude from
   coverage.
3. Re-run the existing `TestFetchConcurrent` under `-race`
   (NFR-ACC-003) to confirm the field migration is race-clean.
4. Run the full `internal/access` suite + `go vet` + `golangci-lint`;
   confirm CACHE-001 tests still pass (the `cascade_waf_test.go`
   fixture exercises the migrated escalation path).

---

## File Change Summary

| File | Action | Milestone | REQ coverage |
|------|--------|-----------|--------------|
| `internal/access/wafprofile.go` | CREATE | M1 | REQ-ACC-010/011/012, §2.4 |
| `internal/access/wafprofile_test.go` | CREATE | M1 | REQ-ACC-010/011/012/040 |
| `internal/access/validity.go` | CREATE | M2 | REQ-ACC-020, §2.5/§6.3 |
| `internal/access/validity_test.go` | CREATE | M2 | REQ-ACC-020 |
| `internal/access/testdata/waf_*.{json,html}` + `page_strong_ok.html` | CREATE | M1/M2 | fixtures |
| `internal/access/types.go` | EDIT | M3 | REQ-ACC-013 |
| `internal/access/escalation.go` | EDIT | M3 | REQ-ACC-013/020 |
| `internal/access/escalation_test.go` | EDIT | M3 | REQ-ACC-013 |
| `internal/access/phase3_get.go` | EDIT | M4 | REQ-ACC-013/020/021/022 |
| `internal/access/phase3_test.go` | EDIT | M4 | REQ-ACC-020/021/022 |
| `internal/access/cascade.go` | EDIT | M5 | REQ-ACC-013/020/021 |
| `internal/access/phase4_tls.go` | EDIT | M5 | REQ-ACC-020 |
| `internal/access/phase4_test.go` | EDIT | M5 | REQ-ACC-021 |
| `internal/access/cascade_test.go` | EDIT | M6 | NFR-ACC-002 |
| `internal/access/bench_test.go` (or wafprofile_test) | EDIT/CREATE | M6 | NFR-ACC-001 |

ZERO changes to: `ssrf.go`, `dialer.go`, `robots.go`, `phase1_index.go`,
`phase2_probe.go`, `phase5_browser.go`, `cache_writethrough.go`,
`observability.go`, `options.go`, `errors.go`, `internal/obs/metrics/*`,
`go.mod`, `go.sum`.

---

## Sequencing Rationale

- M1 and M2 are pure-function islands (no edits to existing code) — they
  can be built and fully green-tested before touching the cascade.
- M3 is the coupling point: changing the `PhaseAttempt` field forces the
  escalation edit; doing both in one milestone keeps the package
  compilable.
- M4 → M5 follows the cascade order (Phase 3 detects + sets the top
  `ProfileHit`; Phase 4 passes it to `validatePage` for the profile-aware
  L3 cookie-sensor layer), so M5 depends on M4's `attempt.profileHits`
  being set.
- M6 is verification: it confirms no network/cardinality/race regression
  against the implemented CACHE-001 baseline.

---

## Risks To Watch During Implementation

- The `PhaseAttempt.isWAF` removal (M3) must update every reference —
  grep `isWAF` across the package before editing; the
  `cascade_waf_test.go` fixture and `dispatchPhase` (cascade.go
  251-273) both touch the WAF signal path.
- The Verdict-gated success change (M4/M5) alters when a 200 counts as
  success — re-run all existing phase3/phase4 happy-path tests to
  confirm legitimate 200s (real pages → StrongOK, JSON → WeakOK) still
  succeed.
- Phase 4's transport construction is unchanged from CACHE-001 in v0.1
  (no avoid-list filtering) — the only Phase 4 edit is running
  `validatePage` on the 200 path, so confirm the existing Phase 4
  happy-path TLS behaviour is not regressed by the validation hook.

---

## Quality Gates

- `go vet ./internal/access/...` clean.
- `golangci-lint run ./internal/access/...` clean.
- `go test -race ./internal/access/...` green (incl. CACHE-001 suite).
- Coverage ≥ 85% for new code (`wafprofile.go`, `validity.go`).
- MX tags applied per §6.6 (2 ANCHOR + 1 NOTE), `[AUTO]`-prefixed,
  `@MX:SPEC: SPEC-ACC-001`.
- Pre-submission self-review: confirm no over-engineering (the
  confidence model and Verdict table are the minimal shapes that
  satisfy the REQs; no flexibility hooks beyond the data-driven table).
