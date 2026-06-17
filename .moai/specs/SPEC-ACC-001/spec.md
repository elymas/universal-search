---
id: SPEC-ACC-001
title: Access Layer WAF Profile Detection + Page-Validity Gate (insane-search v0.5 port)
version: 0.1.0
milestone: M3 — Fanout, adapters, index
status: draft
priority: P1
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-06-17
updated: 2026-06-17
author: limbowl
issue_number: null
labels: [access, waf, security, M3]
depends_on: [SPEC-CACHE-001]
---

# SPEC-ACC-001: Access Layer WAF Profile Detection + Page-Validity Gate (insane-search v0.5 port)

## HISTORY

- 2026-06-17 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for the M3 access-layer WAF-profile upgrade.
  Drafted after verifying the current `internal/access/` state against
  the named gap (`.moai/specs/SPEC-ACC-001/research.md`, every claim
  file:line-cited). Ports TWO upstream concepts from
  `fivetaku/insane-search` v0.5.0 (prototype→production, commit
  8dededa5): (1) vendor-generic WAF product profiles with
  confidence-ranked detection, and (2) a 4-layer page-validity validator
  producing a Verdict enum. The port replaces CACHE-001's binary
  `isWAF bool` escalation signal with a confidence-ranked
  `[]ProfileHit`, and adds an AND-gated page-validity Verdict that the
  cascade consumes to decide whether a "200 OK" response is actually a
  real page or a silent challenge.

  The upstream's third concept — per-WAF TLS fingerprint avoid-lists
  (the `TLSAvoidList` field, the candidate-set filter, the exhaustion
  fallback) — is DEFERRED out of v0.1 to SPEC-CACHE-001b. Rationale: no
  TLS-impersonation library (utls/CycleTLS) is wired into the codebase
  yet, so there is no real candidate set to filter; the avoid-list
  machinery would be a no-op coupling to deferred work. It lands in
  CACHE-001b alongside the impersonation library that gives it a
  candidate set to act on (see §7 and Open Questions §11).

  Verified current-code state (research.md §1):
  - `internal/access/phase4_tls.go` uses STANDARD `crypto/tls`
    (`tls.Config{MinVersion: TLS12, MaxVersion: TLS13}`) plus a single
    `browserUserAgent` Chrome-130-macOS constant — NOT utls fingerprint
    impersonation. JS-challenge detection is 4 hardcoded substring
    patterns (`var jsChallengePatterns`) via `containsJSChallenge()`.
  - `internal/access/phase3_get.go` `buildTransport()` builds a STANDARD
    `*http.Transport` (pinned-IP dialer or default transport in test
    mode); there is NO utls / refraction-networking / CycleTLS
    dependency in `go.mod`. WAF detection is `isWAFResponse()` =
    (403 OR 503) AND one of 3 header prefixes (`cf-ray`, `x-akamai-`,
    `x-served-by`).
  - `internal/access/types.go` `PhaseAttempt` carries a BINARY `isWAF
    bool` (set by phase3) plus `isTLSError bool` and `isJSChallenge
    bool` — no profile, no vendor, no confidence.
  - `internal/access/escalation.go` `shouldEscalate()` escalates Phase
    3 → 4 on `prev.isTLSError || prev.isWAF` (binary).
  - There is NO `cascade_waf.go` on disk — only a `cascade_waf_test.go`
    integration test fixture.
  - The error type is `*FetchError` (not `*SourceError`) with an
    `ErrorCategory` enum; escalation signals are carried via
    `FetchError.isTLSSignal / isWAFSignal / isJSChallengeSignal`
    unexported fields, copied into `PhaseAttempt` by `runPhase`.

  Scope discipline (HARD): this SPEC covers ONLY the two named v0.1 gaps
  (WAF product profiles, 4-layer validator). The TLS avoid-list, the
  insane-search v0.5 R7 API-first parallel branch, the bias-check CI
  linter, and the porting of insane-search reference docs are deferred
  and explicitly OUT OF SCOPE here (see §7 and Open Questions §11). No
  utls/CycleTLS dependency is introduced; a true fingerprint-
  impersonation library AND its companion avoid-list are deferred to a
  future SPEC-CACHE-001b (already reserved in CACHE-001 OQ §8.8).

  8 EARS REQs (covering four EARS patterns — Ubiquitous, Event-Driven,
  State-Driven, Unwanted) + 3 NFRs grouped in the 10/20/40 numbering
  scheme. The 03x decade (previously the TLS avoid-list concern) is now
  intentionally vacant — that concern is deferred to SPEC-CACHE-001b;
  the No-Site-Name rule keeps its own 04x concern slot. Two-file new
  code surface (`internal/access/wafprofile.go`,
  `internal/access/validity.go`) plus surgical edits to `types.go`,
  `phase3_get.go`, `phase4_tls.go`, `escalation.go`, `cascade.go`
  (Phase 4 runs the validator over its 200-path body; it no longer
  filters a TLS candidate set). Harness level: standard (single
  package; WAF-detection logic is data-driven, not a new network
  surface; no new module deps). Sprint Contract optional. Ready for
  plan-auditor review and annotation cycle.

---

## 1. Purpose

SPEC-CACHE-001 (status: implemented) shipped the 5-phase content-fetch
cascade in `internal/access/`. Its WAF handling is intentionally
minimal: a binary `isWAF bool` flag set by `phase3Get` when a 403/503
response carries one of three known WAF header prefixes
(`internal/access/phase3_get.go::isWAFResponse`), consumed by
`shouldEscalate` (`internal/access/escalation.go:33`) as a single
escalate-or-not signal. The Phase 4 TLS-hardening pass uses standard
`crypto/tls` (TLS 1.2–1.3) with one fixed browser User-Agent and a
4-substring JS-challenge body detector
(`internal/access/phase4_tls.go::containsJSChallenge`). There is no
notion of WHICH WAF vendor is in front of the URL, no confidence
ranking, and no AND-gated proof that a "200 OK" is a real page rather
than a silently-served challenge.

The upstream `fivetaku/insane-search` v0.5.0 (the same project CACHE-001
ported the 5-phase pattern from) graduated three production-hardening
concepts; v0.1 ports the FIRST TWO, which close the vendor-awareness
and silent-200 gaps without expanding the network surface:

1. **WAF product profiles** — a vendor-generic table of 7 profiles
   (Akamai, Cloudflare, F5, AWS WAF, DataDome, PerimeterX, unknown).
   Each profile declares detectors (cookie patterns, response headers,
   body markers). Detection returns a CONFIDENCE-RANKED list rather
   than a yes/no, so the cascade can pick the most-likely vendor and
   apply vendor-specific handling.
2. **4-layer page-validity validator** — a gate that answers "is this
   really a real page" using four AND-gated signals: (1) challenge
   markers, (2) body-size fingerprint, (3) cookie sensor state
   (e.g. Akamai `_abck=~-1~` means "still being challenged"), (4) a CSS
   success-selector proof. It produces a Verdict enum
   (`STRONG_OK / WEAK_OK / CHALLENGE / BLOCKED / UNKNOWN`) so the
   cascade no longer treats every HTTP 200 as a win.

The upstream's third concept — a **per-WAF TLS avoid-list** (each
profile listing TLS fingerprints known to be pre-blocked by that
vendor, so the impersonation candidate set is filtered before a
fingerprint is tried) — is DEFERRED to SPEC-CACHE-001b. An avoid-list
only has value once there is a real fingerprint candidate set to
filter, and that candidate set is born with the impersonation library
(utls/CycleTLS) that CACHE-001b introduces. Wiring the avoid-list in
v0.1 — with only the stdlib-`crypto/tls` path and no impersonation —
would be a no-op coupling to deferred work, so it is shipped together
with its consumer in CACHE-001b (CACHE-001 OQ §8.8).

SPEC-ACC-001 ports exactly these two v0.1 concepts into
`internal/access/`. It replaces the binary `isWAF bool` with a
confidence-ranked `[]ProfileHit`; the cascade escalates based on the
top hit. It adds the 4-layer validator that runs over every Phase 3/4
response, producing a Verdict the cascade consumes to decide success
vs. silent-challenge.

The SPEC does NOT introduce a new fingerprint-impersonation library
(no utls / CycleTLS) and does NOT add the companion TLS avoid-list —
both are deferred to SPEC-CACHE-001b. It does NOT add per-host rate
limiting (CACHE-001 OQ §8.3), does NOT add the insane-search R7
API-first parallel branch, does NOT add the bias-check CI linter, and
does NOT port insane-search reference documentation — these are P2 in
the upstream port plan and are deferred (§7, §11).

Completion makes the access layer's WAF handling vendor-aware and
challenge-aware without expanding its network surface. The
confidence-ranked profile signal and the Verdict enum are consumed by
the existing cascade escalation logic; SPEC-EVAL-002 (M8 reliability
dashboard) can later read the per-profile detection telemetry.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `internal/access/wafprofile.go`: `WAFProfile` value type (`ID string`, `DisplayName string`, `CookiePatterns []string`, `HeaderMarkers []string`, `BodyMarkers []string`); a package-level vendor-generic profile table `wafProfiles` with 7 entries (`akamai`, `cloudflare`, `f5`, `aws-waf`, `datadome`, `perimeterx`, `unknown`); `ProfileHit` value type (`ProfileID string`, `Confidence float64`); `detectProfiles(resp *http.Response, body []byte) []ProfileHit` — a pure function returning a confidence-ranked (descending) slice of hits, one per profile whose detectors matched, with `unknown` as the sole fallback hit when no vendor profile matched a 403/503/challenge response. (The per-WAF `TLSAvoidList` field is DEFERRED to SPEC-CACHE-001b; see §7.) |
| b | `internal/access/validity.go`: `Verdict` string-enum type with constants `VerdictStrongOK`, `VerdictWeakOK`, `VerdictChallenge`, `VerdictBlocked`, `VerdictUnknown`; `validatePage(resp *http.Response, body []byte, hit *ProfileHit) Verdict` — the 4-layer AND-gated validator: (1) challenge-marker check (reuses the body markers in scope item a + the existing `jsChallengePatterns`), (2) body-size fingerprint (sub-threshold body length signals a challenge/error stub), (3) cookie sensor state (e.g. Akamai `_abck=~-1~` / `~0~` distinction, DataDome cookie presence), (4) CSS success-selector proof (presence of a real-content selector such as `<main`, `<article`, or `id="content"`). The function maps the AND-gated signal combination to a Verdict per the table in §6.3. |
| c | `internal/access/types.go` (MODIFIED): replace the binary `isWAF bool` field on `PhaseAttempt` with `profileHits []ProfileHit` and add `verdict Verdict`. Add `isWAFSignal`-equivalent derivation as a helper `(*PhaseAttempt).topProfile() (ProfileHit, bool)` so escalation reads the top hit without re-ranking. |
| d | `internal/access/phase3_get.go` (MODIFIED): replace `isWAFResponse(resp) bool` usage with `detectProfiles(resp, body)`; on a non-empty hit list, set `attempt.profileHits` and (when the top hit's confidence ≥ the escalation threshold) signal escalation to Phase 4. Run `validatePage` over the 200-path body and set `attempt.verdict`; a 200 whose Verdict is `VerdictChallenge` or `VerdictBlocked` is NOT treated as success and triggers escalation. |
| e | `internal/access/phase4_tls.go` (MODIFIED): run `validatePage` over the Phase 4 200-path body and set the Verdict the same way as Phase 3. (No TLS candidate-set filtering in v0.1 — the avoid-list filter is deferred to SPEC-CACHE-001b; see §7.) |
| f | `internal/access/escalation.go` (MODIFIED): replace the `prev.isTLSError \|\| prev.isWAF` predicate with `prev.isTLSError \|\| prev.hasWAFProfile()` where `hasWAFProfile` returns true when the top `profileHits` entry meets the confidence threshold. Add a Verdict-driven escalation branch: a Phase 3 attempt whose Verdict is `VerdictChallenge` escalates to Phase 4 even on HTTP 200. |
| g | `internal/access/cascade.go` (MODIFIED): thread the top `ProfileHit` from the Phase 3 attempt into the Phase 4 dispatch so Phase 4 can pass it to `validatePage` (the L3 cookie-sensor layer is profile-aware); carry `profileHits` and `verdict` through `runPhase`/`dispatchPhase` the same way `isTLSError`/`isJSChallenge` are carried today. |
| h | `internal/access/wafprofile_test.go`: table-driven tests for `detectProfiles` over captured WAF response fixtures (Akamai `_abck` cookie state, Cloudflare challenge body, DataDome marker), confidence-ranking assertions (multi-match ordering), the `unknown` fallback, and the No-Site-Name rule (assert no profile entry contains a brand/domain literal). |
| i | `internal/access/validity_test.go`: table-driven tests for `validatePage` covering each Verdict transition (`STRONG_OK`, `WEAK_OK`, `CHALLENGE`, `BLOCKED`, `UNKNOWN`), AND-gating (a single positive signal does NOT produce `STRONG_OK`), and the Akamai `_abck=~-1~` sensor-state case. |
| j | `internal/access/escalation_test.go` (MODIFIED): extend `TestShouldEscalateTable` with profile-hit-driven and Verdict-driven Phase 3 → 4 escalation rows. |
| k | `internal/access/testdata/`: NEW captured-response fixtures — `waf_akamai_abck_challenged.json` (response with `_abck=~-1~` sensor cookie + Akamai headers), `waf_cloudflare_challenge.html` (Cloudflare challenge body), `waf_datadome.json` (DataDome `datadome` cookie + marker), `page_strong_ok.html` (real page with `<main>` content selector and full body). |

### 2.2 Out-of-Scope

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination; this list prevents scope creep beyond the two
named v0.1 gaps. (Full rationale in §7.)

- **Per-WAF TLS avoid-list** (the `TLSAvoidList` field, the Phase 4
  candidate-set filter, and the exhaustion fallback) → future
  SPEC-CACHE-001b (already reserved in CACHE-001 OQ §8.8). Deferred
  because it only has value once an impersonation library supplies a
  real candidate set to filter; in v0.1 it would be a no-op coupling to
  deferred work. (Full rationale in §7, OQ §11.7.)
- **TLS fingerprint IMPERSONATION** (utls / CycleTLS / refraction-
  networking — emitting a specific Chrome/Firefox TLS handshake byte
  sequence) → future SPEC-CACHE-001b (already reserved in CACHE-001
  OQ §8.8). v0.1 uses the existing stdlib-`crypto/tls` path unchanged;
  it does NOT spoof fingerprints and does NOT filter a candidate set.
- **insane-search R7 API-first parallel branch** (try a source's
  official API in parallel before the browser cascade) → P2 in the
  upstream port plan; deferred (OQ §11.1).
- **Bias-check CI linter** (the insane-search No-Site-Name enforcement
  tool that scans for brand/domain literals at build time) → P2;
  deferred (OQ §11.2). This SPEC enforces No-Site-Name by REVIEW and a
  single unit-test assertion (scope item h), not a CI linter.
- **Porting insane-search reference documentation** → P2; deferred
  (OQ §11.3).
- **Per-host rate limiting** → CACHE-001 OQ §8.3 / future SPEC-CACHE-001a.
- **Per-host circuit breaker / auto-disable** → SPEC-EVAL-002 (M8).
- **New WAF vendors beyond the 7 named profiles** → additive future
  work; the table is data-driven so new entries do not change the
  detection algorithm.
- **PDF / binary content validity scoring** — `validatePage` runs on
  HTML/text response bodies only; binary content is passed through
  unchanged (CACHE-001 D10).
- **New Prometheus metric families** — per-profile detection telemetry
  reuses the existing `AccessPhaseAttempts{phase, outcome}` family;
  no cardinality allowlist amendment. A profile-labeled metric is
  deferred to SPEC-EVAL-002.
- **Playwright (Phase 5) fingerprint randomisation** → CACHE-001
  OQ / future SPEC-CACHE-001c.

### 2.3 No-Site-Name Rule (Architecture)

[HARD] WAF profiles MUST be VENDOR-GENERIC and DATA-DRIVEN. The
profile table (`wafProfiles` in `wafprofile.go`) and the validator
selectors (`validity.go`) MUST NOT contain any site-specific, brand,
or domain literal (the insane-search No-Site-Name rule). Detectors key
off vendor infrastructure signatures (WAF cookie names, response
header prefixes, vendor challenge-page markers, vendor TLS-block
behaviour) — never off a target site's domain or brand string.

Rationale: site-specific hardcoding makes the access layer brittle and
unmaintainable; vendor-generic profiles transfer across every site
fronted by the same WAF. The rule is enforced in this SPEC by code
review and one unit-test assertion (REQ-ACC-040 acceptance), NOT by a
build-time linter (the linter is the deferred P2 item in §7).

### 2.4 Profile Detection and Ranking (Architecture)

[HARD] `detectProfiles(resp, body) []ProfileHit` is a PURE function
(no I/O, no time, no randomness) so that golden tests can compute the
expected ranked hit list from a captured fixture alone, and so the
cascade's escalation decision is deterministic.

**Confidence model** (additive, bounded to `[0.0, 1.0]`):

```
confidence(profile) = clamp(
    0.5 * I(any cookie pattern matched)
  + 0.4 * I(any header marker matched)
  + 0.3 * I(any body marker matched),
  0.0, 1.0)
```

where `I(.)` is the indicator (1.0 if the detector class matched at
least once, else 0.0). A profile with zero matching detector classes
produces NO hit (it is omitted from the slice). The returned slice is
sorted by `Confidence` descending; ties break by `ProfileID`
lexicographic ascending for determinism. When a 403/503 or
challenge-marked response produces no vendor hit, the slice contains
exactly one `unknown` hit at confidence `0.2` (a "WAF is present but
unidentified" floor) so the cascade still escalates.

**Escalation threshold**: a `ProfileHit` participates in Phase 3 → 4
escalation when `Confidence ≥ 0.3` (`wafEscalateThreshold`,
package-level constant annotated `@MX:NOTE`). This threshold is locked
in v0.1; OQ §11.5 documents revisit triggers.

### 2.5 4-Layer Validity Gate (Architecture)

[HARD] `validatePage(resp, body, hit) Verdict` AND-gates four signals
to avoid single-signal false positives. The four layers:

| Layer | Signal | Positive meaning |
|-------|--------|------------------|
| L1 Challenge markers | body contains a profile body-marker OR a `jsChallengePatterns` substring | a challenge page is being served |
| L2 Body-size fingerprint | `len(body) < minRealPageBytes` (default 512) | body is too small to be a real page (stub/error) |
| L3 Cookie sensor state | profile cookie sensor indicates "still challenging" (e.g. Akamai `_abck` value contains `~-1~`; DataDome cookie present on a 403) | the WAF has NOT cleared the client |
| L4 Success-selector proof | body contains a real-content CSS selector (`<main`, `<article`, `id="content"`, `class="content"`) | a real page rendered |

**Verdict mapping.** §6.3 is AUTHORITATIVE; the bullets below restate
the table in prose and are intuition only — on any apparent conflict,
§6.3 wins. The mapping is a complete, mutually-exclusive partition:
first split on whether a challenge signal is present (L1 or L3), then
within each branch disambiguate on body size (L2, NOT L4) and finally on
the success selector (L4). Note that `NOT L2` (normal-size body) is a
precondition for both OK verdicts AND for `VerdictChallenge` — a
challenge signal on a sub-threshold body yields `VerdictBlocked`, not
`VerdictChallenge`.

- `VerdictChallenge` — (L1 present OR L3 present) AND NOT L2 (a
  challenge is in flight and the body is NOT sub-threshold). This is the
  silent-200 trap the gate exists to catch.
- `VerdictBlocked` — (L1 present OR L3 present) AND L2 (a tiny
  challenge/block stub — a challenge signal on a sub-threshold body).
- `VerdictUnknown` — NOT L1 AND NOT L3 AND L2 (no challenge signal but a
  sub-threshold body — an ambiguous tiny response; the L4 success
  selector does NOT override this because a sub-threshold body cannot be
  a trusted real page).
- `VerdictStrongOK` — NOT L1 AND NOT L3 AND NOT L2 AND L4 (real page, no
  challenge, sensor cleared, normal size, success selector present).
- `VerdictWeakOK` — NOT L1 AND NOT L3 AND NOT L2 AND L4 absent
  (plausible page, no challenge, normal size, but no success selector —
  e.g. a JSON API body or a minimal valid page).

Note that `VerdictChallenge` and `VerdictBlocked` take precedence over
the OK verdicts: a detected challenge (L1/L3) always beats a present
success selector, which is why L2 — not L4 — is the disambiguator
between Challenge and Blocked.

The gate is consumed by the cascade: a Phase 3/4 attempt whose Verdict
is `VerdictChallenge` or `VerdictBlocked` is NOT counted as success
even on HTTP 200.

---

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-ACC-010 | Ubiquitous | The package `internal/access` SHALL expose a `WAFProfile` value type and a package-level vendor-generic profile table `wafProfiles` containing exactly 7 entries with IDs `{akamai, cloudflare, f5, aws-waf, datadome, perimeterx, unknown}`. Each profile SHALL declare `CookiePatterns`, `HeaderMarkers`, and `BodyMarkers` slices. The table SHALL be deterministic and constant after package init. | P1 | `TestWAFProfilesTableShape` (assert exactly 7 entries; assert each required ID present; assert each profile has a non-nil detector slice). In `wafprofile_test.go`. |
| REQ-ACC-011 | Event-Driven | WHEN `detectProfiles(resp, body)` is invoked with a response that matches one or more profiles' detectors, the function SHALL return a `[]ProfileHit` sorted by `Confidence` descending (ties broken by `ProfileID` ascending), with one hit per matched profile, where each `Confidence` is computed by the additive model in §2.4 and clamped to `[0.0, 1.0]`. | P1 | `TestDetectProfilesRanking` (fixture matching both Akamai cookie+header and a weaker DataDome cookie → Akamai ranked first; confidences match the §2.4 formula within ±0.001); `TestDetectProfilesConfidenceFormula` (table over single/double/triple detector-class matches → expected clamped confidence). In `wafprofile_test.go`. |
| REQ-ACC-012 | Event-Driven | WHEN `detectProfiles` is invoked with a 403/503 OR challenge-marked response that matches NO vendor profile, the function SHALL return a slice containing exactly one `ProfileHit{ProfileID: "unknown", Confidence: 0.2}`. WHEN invoked with a clean 200 response that matches no profile and carries no challenge marker, the function SHALL return an empty (`len == 0`) slice. | P1 | `TestDetectProfilesUnknownFallback` (403 with no vendor signature → single `unknown` hit at 0.2); `TestDetectProfilesCleanResponseEmpty` (200, normal body, no markers → empty slice). In `wafprofile_test.go`. |
| REQ-ACC-013 | Ubiquitous | The package SHALL replace the binary `PhaseAttempt.isWAF bool` signal with `PhaseAttempt.profileHits []ProfileHit`, and SHALL expose `(*PhaseAttempt).topProfile() (ProfileHit, bool)` returning the highest-confidence hit (false when empty) and `(*PhaseAttempt).hasWAFProfile() bool` returning true when the top hit's `Confidence ≥ wafEscalateThreshold` (0.3). The escalation predicate in `shouldEscalate` SHALL consult `hasWAFProfile()` in place of the removed `isWAF` field. | P1 | `TestPhaseAttemptTopProfile` (empty → false; multi-hit → highest confidence returned); `TestPhaseAttemptHasWAFProfile` (top hit 0.5 → true; top hit 0.2 → false); `TestShouldEscalatePhase3OnWAFProfile` (Phase 3 attempt with a 0.5-confidence hit → escalate to Phase 4). In `escalation_test.go`. |
| REQ-ACC-020 | Event-Driven | WHEN a Phase 3 or Phase 4 response body is received, the cascade SHALL classify it through `validatePage(resp, body, hit)` producing exactly one `Verdict` from `{VerdictStrongOK, VerdictWeakOK, VerdictChallenge, VerdictBlocked, VerdictUnknown}` per the AND-gated truth table in §6.3, and SHALL record it on the `PhaseAttempt.verdict` field. | P1 | `TestValidatePageVerdictTable` (table over the §6.3 truth-table rows; each input → expected Verdict); `TestPhase3SetsVerdict` (Phase 3 200 with a real-page fixture → `attempt.verdict == VerdictStrongOK`). In `validity_test.go` + `phase3_test.go`. |
| REQ-ACC-021 | State-Driven | WHILE a Phase 3 or Phase 4 response has HTTP status 200 but its `validatePage` Verdict is `VerdictChallenge` OR `VerdictBlocked`, the cascade SHALL NOT treat the attempt as success (SHALL NOT set `attempt.Outcome = "success"`, SHALL NOT return the body as `FetchedContent`), and SHALL escalate to the next phase per the escalation predicates. | P1 | `TestPhase3Silent200ChallengeNotSuccess` (server returns 200 with an Akamai `_abck=~-1~` cookie + tiny challenge body; assert `attempt.Outcome != "success"` AND Phase 4 attempted); `TestPhase4Silent200BlockedNotSuccess` (Phase 4 200 with `VerdictBlocked`; assert not success). In `phase3_test.go` + `phase4_test.go`. |
| REQ-ACC-022 | State-Driven | WHILE a Phase 3/4 response Verdict is `VerdictWeakOK` (plausible page, no success selector — e.g. a JSON API body or a minimal valid page), the cascade SHALL treat the attempt as success and return the body, recording `attempt.verdict == VerdictWeakOK` for diagnostics. | P1 | `TestPhase3WeakOKIsSuccess` (Phase 3 200 with a JSON body, no challenge markers, normal size, no `<main>` selector → `attempt.Outcome == "success"`, `attempt.verdict == VerdictWeakOK`, body returned). In `phase3_test.go`. |
| REQ-ACC-040 | Unwanted | IF any entry in the `wafProfiles` table OR any selector in `validity.go` contains a site-specific brand or domain literal (the insane-search No-Site-Name rule), THEN the test suite SHALL fail. Profiles SHALL key only off vendor infrastructure signatures (WAF cookie names, header prefixes, vendor challenge markers, vendor TLS behaviour), never off a target site's domain or brand. | P1 | `TestNoSiteNameRule` (iterate every profile's cookie/header/body markers + every validity selector; assert none contains a known-site domain substring from a small deny-list `{".com/", "reddit", "naver", "google", "youtube", "github"}` used as a tripwire). In `wafprofile_test.go`. |

---

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-ACC-001 | Detection performance (pure path) | `detectProfiles(resp, body)` AND `validatePage(resp, body, hit)` SHALL together execute with mean wall-clock ≤ 1 ms per op over `go test -bench=BenchmarkDetectAndValidate -benchtime=100x -count=5 ./internal/access/...` against the `waf_akamai_abck_challenged.json` + `page_strong_ok.html` fixtures (median of 5 runs is the assertion value). The 1 ms ceiling is platform-agnostic — both functions are pure (no I/O) and the cost is body-substring scanning bounded by `MaxBodyBytes`, so the threshold holds on every supported host architecture. Benchmarks do not count toward coverage. |
| NFR-ACC-002 | No new network surface | The SPEC SHALL NOT add any new outbound network operation. `detectProfiles` and `validatePage` operate on the response + body already fetched by Phase 3/4 — they add zero network ops. Verified by `TestNoNewNetworkCalls` (a Phase 3 → 4 cascade against one stub server observes NO MORE requests than the measured SPEC-CACHE-001 baseline for the same fixture; the test captures that baseline and asserts the profile/Verdict additions issue zero additional requests). |
| NFR-ACC-003 | Race-clean concurrent detection | The new pure functions and the modified `PhaseAttempt` fields SHALL introduce zero shared mutable state. `internal/access/concurrent_test.go::TestFetchConcurrent` (the existing CACHE-001 workload) SHALL continue to pass under `go test -race ./internal/access/...` with the profile/Verdict additions, with zero new race-detector alarms attributable to the access package. |

---

## 5. Acceptance Criteria

### REQ-ACC-010 — WAF Profile Table

- File `internal/access/wafprofile.go` declares `WAFProfile` with the
  documented fields (`CookiePatterns`, `HeaderMarkers`, `BodyMarkers`)
  and a `wafProfiles` table of exactly 7 entries.
- `TestWAFProfilesTableShape` asserts the 7 IDs are present and each
  profile has non-nil detector slices.

### REQ-ACC-011 — Confidence-Ranked Detection

- `TestDetectProfilesRanking`: a fixture matching Akamai (cookie +
  header) and DataDome (cookie only) returns Akamai first; the
  computed confidences match §2.4 within ±0.001.
- `TestDetectProfilesConfidenceFormula`: single-class match → 0.5/0.4/
  0.3 depending on class; double-class → sum clamped; triple-class →
  1.0 (capped from 1.2).

### REQ-ACC-012 — Unknown Fallback / Clean-Response Empty

- `TestDetectProfilesUnknownFallback`: 403 with no vendor signature →
  single `unknown` hit at 0.2.
- `TestDetectProfilesCleanResponseEmpty`: 200, normal body, no markers
  → `len(hits) == 0`.

### REQ-ACC-013 — Profile Hits Replace Binary isWAF

- `internal/access/types.go` no longer declares `isWAF bool` on
  `PhaseAttempt`; it declares `profileHits []ProfileHit` and `verdict
  Verdict`.
- `TestPhaseAttemptTopProfile`, `TestPhaseAttemptHasWAFProfile`,
  `TestShouldEscalatePhase3OnWAFProfile` all pass.
- `internal/access/escalation.go::shouldEscalate` case 3 reads
  `prev.isTLSError || prev.hasWAFProfile()`.

### REQ-ACC-020 — 4-Layer Verdict Production

- `TestValidatePageVerdictTable` covers every §6.3 truth-table row;
  each input maps to the documented Verdict.
- `TestPhase3SetsVerdict`: Phase 3 200 with `page_strong_ok.html` →
  `attempt.verdict == VerdictStrongOK`.

### REQ-ACC-021 — Silent-200 Challenge Not Counted as Success

- `TestPhase3Silent200ChallengeNotSuccess`: server returns HTTP 200
  with Akamai `_abck=~-1~` cookie + a sub-512-byte challenge body;
  assert `attempt.Outcome != "success"` AND Phase 4 attempted.
- `TestPhase4Silent200BlockedNotSuccess`: Phase 4 returns HTTP 200
  with a `VerdictBlocked` body; assert not counted as success.

### REQ-ACC-022 — WeakOK Counted as Success

- `TestPhase3WeakOKIsSuccess`: Phase 3 200 JSON body (no markers,
  normal size, no success selector) → `attempt.Outcome == "success"`,
  `attempt.verdict == VerdictWeakOK`, body returned.

### REQ-ACC-040 — No-Site-Name Rule Enforced

- `TestNoSiteNameRule`: iterates every profile detector marker and
  every validity selector; asserts none contains a tripwire site-name
  substring from `{".com/", "reddit", "naver", "google", "youtube",
  "github"}`.

### NFR-ACC-001 — Detection Performance

- `BenchmarkDetectAndValidate` invoked as `go test
  -bench=BenchmarkDetectAndValidate -benchtime=100x -count=5
  ./internal/access/...`; median of 5 runs ≤ 1 ms per op.

### NFR-ACC-002 — No New Network Surface

- `TestNoNewNetworkCalls`: a Phase 3 → 4 cascade against one stub
  server observes NO MORE requests than the measured SPEC-CACHE-001
  baseline for the same fixture (the test captures that baseline and
  asserts detection + validation add zero additional network ops).

### NFR-ACC-003 — Race-Clean

- `TestFetchConcurrent` (CACHE-001 workload) passes under `go test
  -race ./internal/access/...` with the profile/Verdict additions;
  zero new alarms.

---

## 6. Technical Approach

### 6.1 Files to Modify

**Created (4 files)**:
- `internal/access/wafprofile.go` — `WAFProfile`, `wafProfiles` table,
  `ProfileHit`, `detectProfiles`, `wafEscalateThreshold`.
- `internal/access/wafprofile_test.go` — REQ-ACC-010/011/012/040.
- `internal/access/validity.go` — `Verdict` enum, `validatePage`,
  `minRealPageBytes`, success-selector set.
- `internal/access/validity_test.go` — REQ-ACC-020.
- `internal/access/testdata/waf_akamai_abck_challenged.json`,
  `waf_cloudflare_challenge.html`, `waf_datadome.json`,
  `page_strong_ok.html` (NEW fixtures).

**Modified (5 files)**:
- `internal/access/types.go` — `PhaseAttempt`: remove `isWAF bool`,
  add `profileHits []ProfileHit` + `verdict Verdict`; add
  `topProfile()` + `hasWAFProfile()` helpers.
- `internal/access/phase3_get.go` — replace `isWAFResponse` usage with
  `detectProfiles`; run `validatePage`; set `profileHits` + `verdict`;
  Verdict-gated success.
- `internal/access/phase4_tls.go` — run `validatePage` on the 200 path;
  set `verdict` (no TLS candidate-set filtering in v0.1; the avoid-list
  is deferred to SPEC-CACHE-001b).
- `internal/access/escalation.go` — `shouldEscalate` case 3 reads
  `hasWAFProfile()`; add Verdict-challenge escalation branch.
- `internal/access/cascade.go` — thread top `ProfileHit` from Phase 3
  into Phase 4 dispatch (so Phase 4 can pass it to `validatePage`);
  carry `profileHits`/`verdict` through `runPhase`/`dispatchPhase`.
- `internal/access/escalation_test.go` — extend `TestShouldEscalateTable`.
- `internal/access/phase3_test.go` / `phase4_test.go` — extend with
  REQ-ACC-021/022 cases.

**Unchanged (by design)**:
- `internal/access/ssrf.go`, `dialer.go`, `robots.go`,
  `phase1_index.go`, `phase2_probe.go`, `phase5_browser.go`,
  `cache_writethrough.go`, `observability.go`, `options.go`,
  `errors.go` — no contract change required.
- `internal/obs/metrics/*` — no new metric family.
- `go.mod` / `go.sum` — ZERO new module dependencies (no utls/CycleTLS;
  the TLS avoid-list that would consume them is deferred to
  SPEC-CACHE-001b).

### 6.2 Profile Table Sketch (illustrative; final shapes in run phase)

```go
// internal/access/wafprofile.go
package access

type WAFProfile struct {
    ID             string
    DisplayName    string
    CookiePatterns []string // e.g. "_abck=", "datadome="
    HeaderMarkers  []string // e.g. "cf-ray", "x-akamai-"
    BodyMarkers    []string // e.g. "cf-please-stand-by"
    // NOTE: a per-WAF TLSAvoidList field is DEFERRED to SPEC-CACHE-001b
    // (it only has value once an impersonation library supplies a real
    // candidate set to filter). See §7.
}

type ProfileHit struct {
    ProfileID  string
    Confidence float64
}

const wafEscalateThreshold = 0.3 // @MX:NOTE: §2.4 escalation floor

// vendor-generic; No-Site-Name rule (§2.3).
var wafProfiles = []WAFProfile{
    {ID: "akamai", DisplayName: "Akamai",
        CookiePatterns: []string{"_abck=", "ak_bmsc=", "bm_sz="},
        HeaderMarkers:  []string{"x-akamai-"},
        BodyMarkers:    []string{"reference #", "access denied"}},
    {ID: "cloudflare", DisplayName: "Cloudflare",
        CookiePatterns: []string{"__cf_bm=", "cf_clearance="},
        HeaderMarkers:  []string{"cf-ray", "cf-mitigated"},
        BodyMarkers:    []string{"cf-please-stand-by",
            "checking if the site connection is secure"}},
    // f5, aws-waf, datadome, perimeterx, unknown ...
}
```

### 6.3 Verdict Truth Table (REQ-ACC-020)

`L1`=challenge marker, `L2`=sub-threshold body, `L3`=cookie sensor
"still challenging", `L4`=success selector present. `-` = either.

| L1 | L2 | L3 | L4 | Verdict |
|----|----|----|----|---------|
| no | no | no | yes | `VerdictStrongOK` |
| no | no | no | no | `VerdictWeakOK` |
| yes | no | - | - | `VerdictChallenge` |
| - | no | yes | - | `VerdictChallenge` |
| yes | yes | - | - | `VerdictBlocked` |
| - | yes | yes | - | `VerdictBlocked` |
| no | yes | no | yes | `VerdictUnknown` (tiny but real-selectored — ambiguous) |
| no | yes | no | no | `VerdictUnknown` |

Precedence (top-down): the first matching row wins. `VerdictChallenge`
and `VerdictBlocked` take precedence over OK verdicts (the gate is
conservative — a detected challenge always beats a present success
selector).

### 6.4 Escalation Integration (REQ-ACC-013/021)

`shouldEscalate` (escalation.go) case 3 becomes:

```go
case 3:
    // Phase 3 escalates on TLS error, a confident WAF profile, or a
    // challenge/blocked Verdict on an otherwise-200 response.
    return prev.isTLSError ||
        prev.hasWAFProfile() ||
        prev.verdict == VerdictChallenge ||
        prev.verdict == VerdictBlocked
```

The cascade (`cascade.go`) success gate becomes Verdict-aware: an
attempt is success only when content is non-nil AND its Verdict is NOT
`VerdictChallenge`/`VerdictBlocked` (a silent-200 challenge is demoted
to a failure outcome that escalates).

### 6.5 Profile-Hit Threading (REQ-ACC-020) + Deferred Avoid-List

The Phase 3 attempt's `topProfile()` hit is carried by the cascade into
the Phase 4 dispatch so Phase 4 can pass it to `validatePage` — the L3
cookie-sensor layer is profile-aware (it reads the detected vendor's
sensor-cookie convention, e.g. Akamai `_abck=~-1~`). Phase 4 in v0.1
constructs its TLS transport exactly as CACHE-001 does today (stdlib
`crypto/tls` default plus the existing single `browserUserAgent`
shaping); it does NOT filter a candidate fingerprint set.

The per-WAF TLS avoid-list (the `TLSAvoidList` field, the candidate-set
subtraction, and the exhaustion fallback) is DEFERRED to
SPEC-CACHE-001b. An avoid-list only earns its keep once an
impersonation library (utls/CycleTLS) supplies a real, multi-entry
candidate set to prune; with only the stdlib path in v0.1 there is no
candidate set to filter, so the machinery would be a no-op coupling to
deferred work. It ships in CACHE-001b together with the impersonation
library that gives it a candidate set to act on (see §7, OQ §11.7).

### 6.6 MX Tag Plan

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `wafprofile.go::detectProfiles` | `@MX:ANCHOR` | Sole WAF-classification entry; fan_in = 2 (phase3, phase4). `@MX:REASON: confidence model + No-Site-Name invariant; changing the formula shifts every escalation decision`. `@MX:SPEC: SPEC-ACC-001`. |
| `validity.go::validatePage` | `@MX:ANCHOR` | The silent-200 gate; every Phase 3/4 body passes through it. `@MX:REASON: AND-gating prevents counting a challenge as success; a bug here re-opens the silent-200 trap`. `@MX:SPEC: SPEC-ACC-001`. |
| `wafprofile.go::wafProfiles` table + `wafEscalateThreshold` | `@MX:NOTE` | Vendor-generic data table + escalation floor. Documents the No-Site-Name rule (§2.3) and the §2.4 threshold. |

All tags `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-ACC-001`, follow
`code_comments: en`. Per-file limits (3 ANCHOR / 5 WARN) respected.

### 6.7 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 8 EARS REQs
(all P1) + 3 NFRs touching 1 package (`internal/access/`, 4 new + 5
modified source files) + zero cross-package edits + zero new module
dependencies + no new network surface. WAF-detection is data-driven
classification logic, not a new security boundary (the SSRF guards from
CACHE-001 are unchanged) → **standard** harness level. Sprint Contract
OPTIONAL. Evaluator profile `default` applies.

---

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following. Each has a known
destination; this list prevents scope creep beyond the two named v0.1
gaps (WAF product profiles, 4-layer validator).

- **Per-WAF TLS avoid-list** (the `TLSAvoidList` field on `WAFProfile`,
  the candidate-set filter in Phase 4, and the exhaustion fallback) →
  future SPEC-CACHE-001b (CACHE-001 OQ §8.8). DEFERRED because it only
  has value once an impersonation library supplies a real candidate set
  to filter; in v0.1, with only the stdlib-`crypto/tls` path, it would
  be a no-op coupling to deferred work. It ships in CACHE-001b alongside
  its consumer.
- **TLS fingerprint IMPERSONATION library** (utls / CycleTLS /
  refraction-networking) → future SPEC-CACHE-001b (CACHE-001 OQ §8.8).
  v0.1 uses the existing stdlib-`crypto/tls` path unchanged; no
  candidate-set spoofing.
- **insane-search R7 API-first parallel branch** → P2 in the upstream
  port plan; deferred (OQ §11.1).
- **Bias-check CI linter** (build-time No-Site-Name scanner) → P2;
  deferred (OQ §11.2). No-Site-Name is enforced here by REVIEW + one
  unit-test tripwire (REQ-ACC-040).
- **Porting insane-search reference documentation** → P2; deferred
  (OQ §11.3).
- **Per-host rate limiting** → CACHE-001 OQ §8.3 / SPEC-CACHE-001a.
- **Per-host circuit breaker / auto-disable** → SPEC-EVAL-002 (M8).
- **New WAF vendors beyond the 7 named profiles** → additive future
  work; the data-driven table makes this a config-only change.
- **PDF / binary validity scoring** → `validatePage` is HTML/text only.
- **New Prometheus metric families / per-profile labels** → reuse the
  existing `AccessPhaseAttempts` family; per-profile telemetry is
  SPEC-EVAL-002.
- **Phase 5 (Playwright) fingerprint randomisation** → CACHE-001
  OQ / future SPEC-CACHE-001c.

---

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per `quality.development_mode:
tdd`. Representative RED-phase tests, written before implementation,
grouped by REQ. Coverage target: 85%. Benchmarks do not count toward
coverage. Greenfield-within-brownfield: the two new files
(`wafprofile.go`, `validity.go`) are written test-first; the edits to
`types.go`/`phase3_get.go`/`phase4_tls.go`/`escalation.go`/`cascade.go`
follow the brownfield-enhanced RED phase (read existing behaviour, then
write a failing test informed by it). Phase 4's edit is limited to
running the validator over its 200-path body — the TLS avoid-list
filter is deferred to SPEC-CACHE-001b.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestWAFProfilesTableShape` | `wafprofile_test.go` | REQ-ACC-010 | 7 entries; required IDs present; non-nil slices |
| 2 | `TestDetectProfilesRanking` | `wafprofile_test.go` | REQ-ACC-011 | Akamai > DataDome; confidences ±0.001 |
| 3 | `TestDetectProfilesConfidenceFormula` | `wafprofile_test.go` | REQ-ACC-011 | Single/double/triple class → clamped confidence |
| 4 | `TestDetectProfilesUnknownFallback` | `wafprofile_test.go` | REQ-ACC-012 | 403 no-vendor → single unknown@0.2 |
| 5 | `TestDetectProfilesCleanResponseEmpty` | `wafprofile_test.go` | REQ-ACC-012 | clean 200 → empty slice |
| 6 | `TestNoSiteNameRule` | `wafprofile_test.go` | REQ-ACC-040 | No tripwire site-name in markers/selectors |
| 7 | `TestPhaseAttemptTopProfile` | `escalation_test.go` | REQ-ACC-013 | empty → false; multi → highest |
| 8 | `TestPhaseAttemptHasWAFProfile` | `escalation_test.go` | REQ-ACC-013 | 0.5 → true; 0.2 → false |
| 9 | `TestShouldEscalatePhase3OnWAFProfile` | `escalation_test.go` | REQ-ACC-013 | 0.5 hit → escalate |
| 10 | `TestValidatePageVerdictTable` | `validity_test.go` | REQ-ACC-020 | Every §6.3 row → expected Verdict |
| 11 | `TestPhase3SetsVerdict` | `phase3_test.go` | REQ-ACC-020 | real page → VerdictStrongOK |
| 12 | `TestPhase3Silent200ChallengeNotSuccess` | `phase3_test.go` | REQ-ACC-021 | _abck=~-1~ 200 → not success, Phase 4 attempted |
| 13 | `TestPhase4Silent200BlockedNotSuccess` | `phase4_test.go` | REQ-ACC-021 | Blocked 200 → not success |
| 14 | `TestPhase3WeakOKIsSuccess` | `phase3_test.go` | REQ-ACC-022 | JSON 200 → success, WeakOK, body returned |
| 15 | `TestShouldEscalateTable` (extended) | `escalation_test.go` | REQ-ACC-013/020 | profile + Verdict escalation rows |
| 16 | `TestNoNewNetworkCalls` | `cascade_test.go` | NFR-ACC-002 | no more than the CACHE-001 baseline request count |
| 17 | `TestFetchConcurrent` (existing, re-run) | `concurrent_test.go` | NFR-ACC-003 | race-clean with additions |
| 18 | `BenchmarkDetectAndValidate` | `bench_test.go` (or wafprofile_test) | NFR-ACC-001 | median of 5 ≤ 1 ms |

RED-GREEN-REFACTOR per requirement:
1. RED: write failing test for REQ-ACC-N.
2. GREEN: minimal code to pass.
3. REFACTOR: tidy; keep each new `.go` file < 200 LoC excluding tests.

---

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-CACHE-001 (implemented)**: provides the 5-phase cascade
  (`internal/access/`), the `PhaseAttempt` type (this SPEC modifies it),
  `phase3Get`/`phase4TLS`/`shouldEscalate`/`cascade.go` (this SPEC
  edits them), and the `FetchError`/`ErrorCategory` taxonomy (unchanged).
  HARD dep — ACC-001 modifies CACHE-001's package in place.

### 9.2 Parallelizable

- **SPEC-EVAL-002 (M8)**: the reliability dashboard can later read
  per-profile detection telemetry; plan phase independent of ACC-001.

### 9.3 Downstream

- **SPEC-CACHE-001b (future)**: introduces the TLS-impersonation library
  AND the per-WAF TLS avoid-list deferred out of this SPEC. The
  avoid-list will key off the same `WAFProfile` IDs this SPEC
  establishes (akamai/cloudflare/… ) by adding a `TLSAvoidList` field to
  the existing table, so ACC-001's profile detection is the foundation
  the deferred avoid-list builds on.

### 9.4 External Dependencies

**ZERO new Go module dependencies.** ACC-001 uses only:
- Go stdlib: `net/http`, `crypto/tls`, `bytes`, `strings`, `sort`,
  `math` (clamp).
- Existing `internal/access` package internals.
- No utls, no CycleTLS, no refraction-networking (confirmed absent from
  `go.mod`; introducing one — together with the per-WAF TLS avoid-list
  that consumes it — is the deferred SPEC-CACHE-001b).

---

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Confidence formula mis-ranks a vendor, picking the wrong escalation/validation profile | Medium | Medium | §2.4 formula is locked + table-tested (REQ-ACC-011); ties break deterministically by ID; OQ §11.5 tracks revisit. |
| 4-layer validator false-positive: a real page flagged as Challenge | Medium | High | AND-gating (§2.5) requires L1 OR L3 (an actual challenge signal), not a single weak signal; `VerdictWeakOK` provides a permissive path for JSON/minimal pages (REQ-ACC-022). |
| 4-layer validator false-negative: a silent challenge counted as success | Medium | High | The whole gate exists to catch this; REQ-ACC-021 + the Akamai `_abck=~-1~` sensor case (REQ-ACC-013/020) are the canonical regression tests. |
| No-Site-Name violation slips in via a new profile | Medium | Medium | REQ-ACC-040 tripwire test fails on a deny-list site substring; code review enforces the rule (the build-time linter is deferred P2). |
| Modifying `PhaseAttempt` breaks existing CACHE-001 tests | Medium | Medium | Replace `isWAF` field surgically; provide `hasWAFProfile()` shim so escalation logic reads the same boolean shape; re-run full CACHE-001 suite (NFR-ACC-003). |
| Body-size threshold (512) mis-tuned for some sites | Low | Low | `minRealPageBytes` is a package constant annotated `@MX:NOTE`; OQ §11.4 tracks revisit. AND-gating with other layers bounds the impact. |

---

## 11. Open Questions

These are explicitly UNRESOLVED at SPEC-approval time. Each has a
recommended default and a resolution owner. They do NOT block approval.

1. **insane-search R7 API-first parallel branch** (P2, deferred):
   should a source's official API be tried in parallel before the
   browser cascade? **Recommended default**: NOT in ACC-001; this is a
   cascade-architecture change, not a WAF-detection change. Resolution
   owner: future SPEC-RETRIEVE-001 / SPEC-ACC-002 author.

2. **Bias-check CI linter** (P2, deferred): a build-time scanner
   enforcing No-Site-Name across the whole repo. **Recommended
   default**: NOT in ACC-001; the unit-test tripwire (REQ-ACC-040)
   covers the new profile/validity code. Resolution owner: future
   SPEC-SEC-001 (M8) author.

3. **Porting insane-search reference docs** (P2, deferred):
   **Recommended default**: NOT in ACC-001 (docs are not code).
   Resolution owner: docs-sync agent if reference value emerges.

4. **`minRealPageBytes` threshold** (default 512): is 512 the right
   body-size floor for "real page"? **Recommended default**: 512 in
   v0.1; revisit after M3 traffic shows false Blocked/Challenge
   verdicts on small-but-real pages. Resolution owner: SPEC-EVAL-002
   author with production telemetry.

5. **`wafEscalateThreshold` (default 0.3)**: the confidence floor for
   profile-driven escalation. **Recommended default**: 0.3 in v0.1
   (escalates on a single header-or-cookie match). Resolution owner:
   future SPEC-ACC-002 author if false escalations appear.

6. **`unknown` fallback confidence (0.2)**: should an unidentified
   403/503 still escalate? **Recommended default**: yes, at 0.2 — but
   0.2 < 0.3 means `unknown` alone does NOT meet `hasWAFProfile()`;
   the existing `isTLSError`/5xx escalation paths still cover it.
   **This is an intentional design choice**: `unknown` is recorded for
   telemetry but does not force a TLS-hardening pass by itself.
   Resolution owner: SPEC-EVAL-002 author should confirm against
   production data whether `unknown` should be promoted to ≥ 0.3.

7. **Per-WAF TLS avoid-list** (DEFERRED to CACHE-001b): the upstream's
   third concept — a `TLSAvoidList` field per profile, a candidate-set
   filter in Phase 4, and an exhaustion fallback. **Decision (made at
   draft v0.1, D4)**: deferred OUT of ACC-001. Rationale: an avoid-list
   only earns its keep once an impersonation library supplies a real
   candidate set to prune; in v0.1, with only the stdlib-`crypto/tls`
   path, it would be a no-op coupling to deferred work. It ships in
   CACHE-001b together with its consumer, keying off the `WAFProfile`
   IDs ACC-001 establishes. Resolution owner: SPEC-CACHE-001b author.

8. **TLS impersonation library selection** (deferred to CACHE-001b):
   when a real fingerprint library is added, which one (utls vs
   CycleTLS)? **Recommended default**: out of ACC-001 scope.
   Resolution owner: SPEC-CACHE-001b author.

---

## 12. References

### External

- `https://github.com/fivetaku/insane-search` — upstream port source;
  v0.5.0 commit 8dededa5 (prototype→production). WAF profiles, 4-layer
  validator, per-WAF TLS avoid-list, No-Site-Name rule.
- `https://pkg.go.dev/crypto/tls` — stdlib TLS Config (Phase 4 uses it
  unchanged in v0.1; no impersonation library; the avoid-list that would
  filter a candidate set is deferred to SPEC-CACHE-001b).
- RFC 9309 — Robots Exclusion Protocol (CACHE-001 context).

### Internal (file:line cited; verified 2026-06-17)

- `.moai/specs/SPEC-ACC-001/research.md` — full current-code + upstream
  evidence artifact (this SPEC's research sibling).
- `.moai/specs/SPEC-CACHE-001/spec.md` — 5-phase cascade contract;
  OQ §8.8 (TLS impersonation deferral).
- `internal/access/phase4_tls.go:19-29` — `browserUserAgent` constant
  + `jsChallengePatterns` (4 substrings) + `containsJSChallenge`.
- `internal/access/phase4_tls.go:46-51` — standard `crypto/tls.Config`
  (TLS12/TLS13), NOT utls.
- `internal/access/phase3_get.go:21-25` — `wafHeaders` (3 prefixes).
- `internal/access/phase3_get.go:88-96` — `isWAFResponse` usage +
  binary `isWAF` escalation signal.
- `internal/access/phase3_get.go:135-184` — `buildTransport` (standard
  `*http.Transport`, pinned IP or default).
- `internal/access/phase3_get.go:186-200` — `isWAFResponse` (403/503 +
  header prefix).
- `internal/access/types.go:79-80` — binary `isWAF bool` on
  `PhaseAttempt` (this SPEC replaces it).
- `internal/access/escalation.go:31-33` — `shouldEscalate` case 3
  (`prev.isTLSError || prev.isWAF`).
- `internal/access/cascade.go:251-273` — Phase 3 escalation-signal
  threading (this SPEC extends it for `profileHits`/`verdict`).
- `internal/access/errors.go:59-82` — `FetchError` + `ErrorCategory`
  (unchanged; signals carried via `isWAFSignal` etc.).
- `internal/access/cascade_waf_test.go` — existing WAF integration test
  fixture (no `cascade_waf.go` exists).
- `go.mod` — no utls / CycleTLS / refraction-networking (verified
  2026-06-17).
- `.moai/config/sections/quality.yaml` — `development_mode: tdd`,
  `test_coverage_target: 85`.
- `.moai/config/sections/language.yaml` — `documentation: en`,
  `code_comments: en`.

---

*End of SPEC-ACC-001 v0.1 (status: draft; pending plan-auditor cycle)*
