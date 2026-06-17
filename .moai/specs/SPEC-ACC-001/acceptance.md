# SPEC-ACC-001 Acceptance Criteria

Concrete test scenarios (table-driven Go / Given-When-Then) mapping to
each EARS requirement and NFR. All tests live under `internal/access/`.
Methodology: TDD. Coverage target: 85% (benchmarks excluded).

---

## REQ-ACC-010 — WAF Profile Table (Ubiquitous)

### Scenario 10.1 — Table shape
- **Given** the package-level `wafProfiles` table.
- **When** `TestWAFProfilesTableShape` enumerates it.
- **Then** there are exactly 7 entries; the ID set equals
  `{akamai, cloudflare, f5, aws-waf, datadome, perimeterx, unknown}`;
  every profile has a non-nil `CookiePatterns`/`HeaderMarkers`/
  `BodyMarkers` slice (may be empty but not nil-deref). (No
  `TLSAvoidList` field in v0.1 — deferred to SPEC-CACHE-001b.)

```
File: internal/access/wafprofile_test.go
func TestWAFProfilesTableShape(t *testing.T)
  want IDs: akamai, cloudflare, f5, aws-waf, datadome, perimeterx, unknown
  assert len(wafProfiles) == 7
```

---

## REQ-ACC-011 — Confidence-Ranked Detection (Event-Driven)

### Scenario 11.1 — Multi-match ranking
- **Given** a response with an Akamai `_abck` cookie + `x-akamai-*`
  header AND a weak DataDome `datadome` cookie.
- **When** `detectProfiles(resp, body)` runs.
- **Then** the slice is `[{akamai, 0.9}, {datadome, 0.5}]` (Akamai
  cookie+header = 0.5+0.4 = 0.9; DataDome cookie-only = 0.5), sorted
  descending, confidences within ±0.001.

### Scenario 11.2 — Confidence formula table
- **Given** synthetic responses exercising single / double / triple
  detector-class matches for one profile.
- **When** `detectProfiles` runs.
- **Then** confidence == cookie(0.5) / header(0.4) / body(0.3) sums,
  clamped to ≤ 1.0 (triple = 1.2 → clamped 1.0).

```
File: internal/access/wafprofile_test.go
func TestDetectProfilesRanking(t *testing.T)  -- table over multi-vendor fixtures
func TestDetectProfilesConfidenceFormula(t *testing.T)
  cases: cookie-only=0.5, header-only=0.4, body-only=0.3,
         cookie+header=0.9, all-three=1.0(clamped)
```

---

## REQ-ACC-012 — Unknown Fallback / Clean-Response Empty (Event-Driven)

### Scenario 12.1 — Unknown fallback
- **Given** a 403 response with no vendor cookie/header/body signature.
- **When** `detectProfiles` runs.
- **Then** the slice is exactly `[{unknown, 0.2}]`.

### Scenario 12.2 — Clean response empty
- **Given** a 200 response, normal-size body, no challenge marker.
- **When** `detectProfiles` runs.
- **Then** `len(hits) == 0`.

```
File: internal/access/wafprofile_test.go
func TestDetectProfilesUnknownFallback(t *testing.T)   -- 403, no vendor → [{unknown,0.2}]
func TestDetectProfilesCleanResponseEmpty(t *testing.T) -- 200 clean → len==0
```

---

## REQ-ACC-013 — Profile Hits Replace Binary isWAF (Ubiquitous)

### Scenario 13.1 — topProfile
- **Given** a `PhaseAttempt` with `profileHits = [{akamai,0.9},{datadome,0.5}]`.
- **When** `topProfile()` is called.
- **Then** returns `({akamai,0.9}, true)`. For an empty slice → `(_, false)`.

### Scenario 13.2 — hasWAFProfile threshold
- **Given** a `PhaseAttempt` with top hit 0.5 / 0.2 respectively.
- **When** `hasWAFProfile()` is called.
- **Then** 0.5 → true; 0.2 → false (< `wafEscalateThreshold` 0.3).

### Scenario 13.3 — escalation reads profile
- **Given** a Phase 3 `PhaseAttempt` with a 0.5-confidence hit.
- **When** `shouldEscalate(prev)` runs.
- **Then** returns true (escalate to Phase 4).

```
File: internal/access/escalation_test.go
func TestPhaseAttemptTopProfile(t *testing.T)
func TestPhaseAttemptHasWAFProfile(t *testing.T)
func TestShouldEscalatePhase3OnWAFProfile(t *testing.T)
Compile-time: types.go no longer declares PhaseAttempt.isWAF.
```

---

## REQ-ACC-020 — 4-Layer Verdict Production (Event-Driven)

### Scenario 20.1 — Verdict truth table
- **Given** synthetic (resp, body, hit) tuples exercising every §6.3
  truth-table row.
- **When** `validatePage` runs.
- **Then** each tuple maps to the documented Verdict. AND-gating: a
  single positive signal (e.g. success selector present but `_abck=~-1~`
  also present) does NOT yield `VerdictStrongOK` — Challenge wins.

### Scenario 20.2 — Phase 3 sets Verdict
- **Given** a Phase 3 200 returning `page_strong_ok.html` (real page,
  `<main>` selector, > 512 bytes, no challenge, cleared sensor).
- **When** the cascade runs Phase 3.
- **Then** `attempt.verdict == VerdictStrongOK`.

```
File: internal/access/validity_test.go
func TestValidatePageVerdictTable(t *testing.T)  -- every §6.3 row + Akamai _abck=~-1~ case
File: internal/access/phase3_test.go
func TestPhase3SetsVerdict(t *testing.T)
```

---

## REQ-ACC-021 — Silent-200 Not Counted as Success (State-Driven)

### Scenario 21.1 — Phase 3 silent challenge 200
- **Given** a server returning HTTP 200 with `Set-Cookie: _abck=...~-1~...`
  and a sub-512-byte challenge body.
- **When** the cascade runs Phase 3.
- **Then** `attempt.Outcome != "success"`, no `FetchedContent` returned
  for Phase 3, AND Phase 4 is attempted (verdict-challenge escalation).

### Scenario 21.2 — Phase 4 silent blocked 200
- **Given** Phase 4 returns HTTP 200 with a `VerdictBlocked` body (tiny
  + sensor challenging).
- **When** the cascade runs Phase 4.
- **Then** `attempt.Outcome != "success"`.

```
File: internal/access/phase3_test.go
func TestPhase3Silent200ChallengeNotSuccess(t *testing.T)
File: internal/access/phase4_test.go
func TestPhase4Silent200BlockedNotSuccess(t *testing.T)
```

---

## REQ-ACC-022 — WeakOK Counted as Success (State-Driven)

### Scenario 22.1 — JSON body WeakOK
- **Given** a Phase 3 200 with a JSON body (no challenge marker,
  normal size, no `<main>` selector).
- **When** the cascade runs Phase 3.
- **Then** `attempt.Outcome == "success"`, `attempt.verdict ==
  VerdictWeakOK`, and the body is returned as `FetchedContent`.

```
File: internal/access/phase3_test.go
func TestPhase3WeakOKIsSuccess(t *testing.T)
```

---

## REQ-ACC-040 — No-Site-Name Rule Enforced (Unwanted)

### Scenario 40.1 — No site-name in profiles/selectors
- **Given** every `wafProfiles` detector marker + every `validity.go`
  success selector.
- **When** `TestNoSiteNameRule` iterates them against the tripwire
  deny-list `{".com/", "reddit", "naver", "google", "youtube", "github"}`.
- **Then** no marker or selector contains any tripwire substring (the
  test fails if a brand/domain literal sneaks in).

```
File: internal/access/wafprofile_test.go
func TestNoSiteNameRule(t *testing.T)
  tripwire := []string{".com/", "reddit", "naver", "google", "youtube", "github"}
```

---

## NFR-ACC-001 — Detection Performance

### Scenario N1.1 — Pure-path benchmark
- **Given** the `waf_akamai_abck_challenged.json` + `page_strong_ok.html`
  fixtures.
- **When** `BenchmarkDetectAndValidate` runs
  `-benchtime=100x -count=5` on amd64.
- **Then** the median of 5 per-op means is ≤ 1 ms.

```
File: internal/access/bench_test.go (or wafprofile_test.go)
func BenchmarkDetectAndValidate(b *testing.B)
Run: go test -bench=BenchmarkDetectAndValidate -benchtime=100x -count=5 ./internal/access/...
```

---

## NFR-ACC-002 — No New Network Surface

### Scenario N2.1 — No more requests than the CACHE-001 baseline
- **Given** a Phase 3 → 4 cascade against one stub server that counts
  inbound requests (Phase 3 returns a WAF challenge → escalate;
  Phase 4 returns a real page), and the measured SPEC-CACHE-001
  baseline request count for the SAME fixture/cascade path.
- **When** `TestNoNewNetworkCalls` runs the cascade with the
  profile/Verdict additions.
- **Then** the server observes NO MORE requests than that baseline
  (i.e. detection + validation add zero additional network ops). The
  test captures the baseline (it does not hard-code an absolute count),
  so any incidental robots/HEAD requests the existing cascade already
  makes are folded into the baseline rather than asserted against.

```
File: internal/access/cascade_test.go
func TestNoNewNetworkCalls(t *testing.T)
  baseline := <CACHE-001 request count for this fixture>  // captured, not hard-coded
  assert serverRequestCount <= baseline  // zero new network ops from detection/validation
```

---

## NFR-ACC-003 — Race-Clean Concurrent Detection

### Scenario N3.1 — Existing concurrent workload still race-clean
- **Given** the existing CACHE-001 `TestFetchConcurrent` workload
  (concurrent `Fetch` calls) plus the new `profileHits`/`verdict`
  fields on `PhaseAttempt`.
- **When** run under `go test -race ./internal/access/...`.
- **Then** zero new race-detector alarms attributable to the access
  package; the test passes.

```
File: internal/access/concurrent_test.go (existing, re-run)
Run: go test -race ./internal/access/...
```

---

## Quality Gate Criteria (Definition of Done)

- [ ] All 8 EARS REQs (REQ-ACC-010..013, 020..022, 040) have
      passing tests.
- [ ] All 3 NFRs (NFR-ACC-001/002/003) verified.
- [ ] `detectProfiles` and `validatePage` are pure (no I/O, no time, no
      randomness) — confirmed by deterministic table tests.
- [ ] `PhaseAttempt.isWAF` removed; `profileHits`/`verdict` added; all
      `isWAF` references migrated (including `cascade_waf_test.go` path).
- [ ] No-Site-Name tripwire (REQ-ACC-040) green.
- [ ] Silent-200 trap closed: Akamai `_abck=~-1~` 200 is NOT counted as
      success (REQ-ACC-021).
- [ ] No TLS avoid-list shipped (deferred to SPEC-CACHE-001b); no
      `TLSAvoidList` field on `WAFProfile`.
- [ ] ZERO new Go module dependencies (no utls/CycleTLS); confirm
      `go.mod` unchanged.
- [ ] No new Prometheus metric family; no cardinality allowlist
      amendment.
- [ ] `go vet` + `golangci-lint` clean on `internal/access/...`.
- [ ] `go test -race ./internal/access/...` green (incl. full CACHE-001
      suite).
- [ ] Coverage ≥ 85% for new code (`wafprofile.go`, `validity.go`).
- [ ] MX tags applied (2 ANCHOR: `detectProfiles`, `validatePage`;
      1 NOTE: `wafProfiles`+threshold), `[AUTO]`-
      prefixed with `@MX:SPEC: SPEC-ACC-001`.
- [ ] Pre-submission self-review: no over-engineering; the confidence
      model + Verdict table are the minimal shapes satisfying the REQs.
