# SPEC-ACC-001 Research

Research artifact for SPEC-ACC-001 — Access Layer WAF Profile Detection
+ TLS Hardening (insane-search v0.5 port). Every current-code claim is
file:line-cited and was verified on 2026-06-17.

---

## 1. Verified Current-Code State (`internal/access/`)

The `internal/access/` package was shipped by SPEC-CACHE-001
(status: implemented) as a strictly-sequential 5-phase content-fetch
cascade. The WAF handling is intentionally minimal. The named gap is
the lack of vendor-aware, confidence-ranked, challenge-aware WAF
handling.

### 1.1 Phase 4 uses STANDARD crypto/tls — NOT utls impersonation

`internal/access/phase4_tls.go`:
- Lines 46-51 construct a STANDARD `crypto/tls.Config`:
  ```go
  tlsCfg := &tls.Config{
      MinVersion: tls.VersionTLS12,
      MaxVersion: tls.VersionTLS13,
      NextProtos: []string{"h2", "http/1.1"},
      ServerName: u.Hostname(),
  }
  ```
  This is stdlib TLS version/ALPN tuning only — there is NO TLS
  fingerprint impersonation (no JA3/JA4 spoofing, no ClientHello byte
  shaping).
- Lines 19-21: a single fixed `browserUserAgent` constant (Chrome 130
  on macOS) is the only "browser shaping". One UA, not a candidate set.
- Lines 23-29: JS-challenge detection is 4 hardcoded substring patterns:
  ```go
  var jsChallengePatterns = []string{
      "cf-please-stand-by",
      "<noscript>",
      "captcha-bypass",
      "checking if the site connection is secure",
  }
  ```
  consumed by `containsJSChallenge(body []byte) bool` (lines 144-153).
- Lines 53-57: `buildTransport(ctx, u, opts, fopts, tlsCfg)` builds the
  transport (see §1.2). No fingerprint candidate set exists to filter.

### 1.2 buildTransport wires STANDARD transport — confirming utls absence

`internal/access/phase3_get.go::buildTransport` (lines 135-184):
- Test mode (`AllowPrivateNetworks`): clones `http.DefaultTransport`
  (a standard `*http.Transport`), optionally overriding `TLSClientConfig`.
- Production: builds a fresh `&http.Transport{DialContext: dialFn, ...}`
  with the pinned-IP dialer, optionally overriding `TLSClientConfig`.
- In BOTH paths the transport is the stdlib `*http.Transport`. There is
  NO utls round-tripper, NO custom ClientHello, NO fingerprint library.

`go.mod` grep (2026-06-17): NO `utls`, NO `refraction-networking`, NO
`cycletls` / `CycleTLS`. Confirmed: the absence of fingerprint
impersonation is part of the gap. The avoid-list in this SPEC therefore
filters the existing stdlib candidate set; a true impersonation library
is the deferred SPEC-CACHE-001b (CACHE-001 OQ §8.8 already reserves it).

### 1.3 WAF detection is binary header-prefix matching

`internal/access/phase3_get.go`:
- Lines 21-25: `wafHeaders = []string{"cf-ray", "x-akamai-",
  "x-served-by"}` — 3 vendor header prefixes (Cloudflare, Akamai,
  Fastly).
- Lines 186-200: `isWAFResponse(resp) bool` returns true ONLY when
  status is 403 OR 503 AND at least one `wafHeaders` prefix matches a
  response header key. No cookie inspection, no body markers, no
  confidence, no vendor identity returned.
- Lines 88-96: on `isWAFResponse(resp)`, phase3 sets
  `attempt := &PhaseAttempt{Phase: 3, isWAF: true, Outcome: "failure"}`
  and returns a `CategoryUnavailable` FetchError. Binary signal only.

### 1.4 PhaseAttempt carries a binary isWAF bool

`internal/access/types.go` lines 56-81: the `PhaseAttempt` struct has
unexported escalation-signal fields:
- `isTLSError bool` (line 75) — set by phase3/phase4 on TLS errors.
- `isJSChallenge bool` (line 78) — set by phase4 on JS-challenge body.
- `isWAF bool` (line 80) — set by phase3 on a WAF-gated response.

There is NO profile, NO vendor ID, NO confidence, NO Verdict. This SPEC
replaces `isWAF bool` with `profileHits []ProfileHit` and adds
`verdict Verdict`.

### 1.5 Escalation consumes isWAF as a single boolean

`internal/access/escalation.go` lines 21-43: `shouldEscalate(prev
*PhaseAttempt) bool`:
- Case 3 (lines 31-33): `return prev.isTLSError || prev.isWAF`. Purely
  binary WAF handling — no profile-aware or Verdict-aware branch.
- Case 4 (lines 34-37): `return prev.isJSChallenge` (Playwright guard
  applied in `cascade.go::shouldEscalatePhase`).

### 1.6 No cascade_waf.go — only a test fixture

`ls internal/access/` (2026-06-17): there is NO `cascade_waf.go` source
file. There IS `cascade_waf_test.go` (integration test fixture):
- `TestFetch_WAFBlocked_EscalatesToPhase4` (lines 13-48): a server
  returns 403 + `Cf-Ray` header on first call (Phase 3 WAF detection),
  then succeeds on the second call (Phase 4).
- `TestFetch_Phase4_DirectSuccess` (lines 50-72): exercises `phase4TLS`
  directly.
- `TestShouldEscalatePhase_Phase4_JSChallenge_NoPlaywright` (lines
  74-84): asserts Phase 4 JS challenge does NOT escalate to Phase 5
  when Playwright disabled.

The new WAF-profile logic in this SPEC lands in two NEW source files
(`wafprofile.go`, `validity.go`), not a `cascade_waf.go`.

### 1.7 Error / signal plumbing (unchanged by this SPEC)

`internal/access/errors.go` lines 59-82: the error type is `*FetchError`
(NOT `*SourceError` — that lives in `pkg/types` for adapters) with an
`ErrorCategory` enum (`blocked / permanent / rate_limited / unavailable
/ timeout`). Escalation signals are carried on FetchError unexported
fields `isTLSSignal / isWAFSignal / isJSChallengeSignal` (lines 69-71),
copied into the `PhaseAttempt` by `runPhase` (`cascade.go` lines
217-222). This SPEC adds a profile/Verdict signal alongside these but
does NOT change the FetchError taxonomy.

`internal/access/cascade.go` lines 251-273: `dispatchPhase` case 3
already threads phase3's escalation signals (`isTLSError`, `isWAF`) into
the returned FetchError. This SPEC extends that threading to carry the
top `ProfileHit` (for the Phase 4 avoid-list) and the `verdict`.

### 1.8 No access.yaml config on disk

CACHE-001 spec reserved `.moai/config/sections/access.yaml`, but the
file does NOT exist on disk (2026-06-17). The defaults are baked into
`internal/access/options.go` (`defaultPerPhaseTimeout`,
`defaultMaxBodyBytes`, etc.). This SPEC introduces NO config file — the
profile table, escalation threshold, and body-size floor are Go
package-level constants/tables (matching the data-driven, code-table
discipline the task brief permits).

---

## 2. Upstream Port Source — fivetaku/insane-search v0.5.0

Source: `https://github.com/fivetaku/insane-search`, v0.5.0, commit
8dededa5 (the prototype→production graduation that CACHE-001 already
cites as its 5-phase pattern source). The upstream has three relevant
concepts; v0.1 ports the first two (§2.1, §2.2). The third — the
per-WAF TLS avoid-list (§2.3) — is DEFERRED to SPEC-CACHE-001b.

### 2.1 WAF product profiles (→ §2.1a, REQ-ACC-010/011/012)

7 vendor-generic profiles: Akamai, Cloudflare, F5, AWS WAF, DataDome,
PerimeterX, unknown. Each declares detectors:
- **Cookie patterns** — WAF sensor cookie names (e.g. Akamai `_abck`,
  `ak_bmsc`, `bm_sz`; Cloudflare `__cf_bm`, `cf_clearance`; DataDome
  `datadome`).
- **Response headers** — vendor header prefixes (e.g. `x-akamai-`,
  `cf-ray`).
- **Body markers** — vendor challenge-page strings.

`detect()` returns a CONFIDENCE-RANKED list `[(profile_id,
confidence)]` rather than a yes/no. The cascade consumes the top
profile. This SPEC replaces CACHE-001's binary `isWAF bool` with the
equivalent `[]ProfileHit`.

### 2.2 4-layer validation gate (→ §2.5, REQ-ACC-020/021/022)

A gate answering "is this really a real page", AND-gating four signals
(NOT single-signal):
1. **Challenge markers** — a challenge page is being served.
2. **Body-size fingerprint** — a sub-threshold body is a stub/error.
3. **Cookie sensor state** — e.g. Akamai `_abck=~-1~` means "client
   still being challenged" (vs `~0~` cleared).
4. **CSS success-selector proof** — a real-content selector rendered.

Verdict enum: `STRONG_OK / WEAK_OK / CHALLENGE / BLOCKED / UNKNOWN`.
The key insight ported here: a HTTP 200 can be a SILENT challenge
(`_abck=~-1~` + tiny body), so the cascade must not count every 200 as
success. CACHE-001 currently has no such gate.

### 2.3 Per-WAF TLS avoid-list (DEFERRED to SPEC-CACHE-001b)

Each profile lists TLS fingerprints known to be BLOCKED by that vendor
(e.g. Akamai avoids chrome107 / chrome123 / firefox). The avoid-list
FILTERS the impersonation candidate set BEFORE a fingerprint is tried,
avoiding wasted handshakes against pre-blocked fingerprints.

In CACHE-001's stdlib-only world there is no impersonation candidate
set yet (§1.1/1.2), so an avoid-list would have nothing real to filter.
This third upstream concept is therefore DEFERRED out of v0.1 to
SPEC-CACHE-001b (the D4 scope decision), where it ships alongside the
impersonation library that gives it a candidate set to act on. v0.1
ports only the first two concepts (WAF profiles, 4-layer validator); it
does NOT add the `TLSAvoidList` field, the candidate-set filter, or the
exhaustion fallback.

### 2.4 No-Site-Name rule (→ §2.3, REQ-ACC-040)

insane-search's discipline: profiles MUST be vendor-generic — NO
site-specific, brand, or domain hardcoding. Detectors key off vendor
infrastructure (WAF cookies/headers/challenge markers/TLS behaviour),
never off a target site's domain. This SPEC enforces it by review + one
unit-test tripwire (REQ-ACC-040). The build-time bias-check LINTER that
insane-search ships is a P2 item and is OUT OF SCOPE here.

### 2.5 Explicitly deferred (P2 in the upstream plan)

- **R7 API-first parallel branch** — try a source's official API in
  parallel before the browser cascade. Cascade-architecture change, not
  WAF detection. OUT OF SCOPE.
- **Bias-check CI linter** — build-time No-Site-Name scanner. OUT OF
  SCOPE (REQ-ACC-040 unit tripwire suffices for the new code).
- **Porting insane-search reference docs** — docs, not code. OUT OF
  SCOPE.

---

## 3. Design Decisions Derived From Research

| # | Decision | Basis |
|---|----------|-------|
| D1 | Replace binary `isWAF bool` with `[]ProfileHit`; cascade reads top hit via `hasWAFProfile()` (≥ 0.3) | §1.4, §1.5, §2.1 |
| D2 | `detectProfiles` + `validatePage` are PURE functions (testable from a fixture alone, deterministic escalation) | §1.3, §2.2 |
| D3 | Additive bounded confidence model (cookie 0.5 + header 0.4 + body 0.3, clamped) | §2.1; deterministic & explainable |
| D4 | `unknown` fallback at 0.2 (below 0.3 threshold): recorded for telemetry, does not by itself force TLS hardening | §1.3, §2.1; intentional (OQ §11.6) |
| D5 | Verdict gate AND-gates 4 layers; a 200 with `VerdictChallenge`/`VerdictBlocked` is NOT success | §2.2; the silent-200 trap |
| D6 | `VerdictWeakOK` is a permissive success path for JSON / minimal pages (no success selector) | §2.2; avoids false Challenge on API bodies |
| D7 | Avoid-list filters the stdlib candidate set + records avoided fingerprints; full value in CACHE-001b | §1.1, §1.2, §2.3 |
| D8 | ZERO new module deps; no config file; profile table + thresholds are Go constants/tables | §1.1, §1.2, §1.8 |
| D9 | No-Site-Name enforced by review + unit tripwire (REQ-ACC-040); linter deferred P2 | §2.4, §2.5 |
| D10 | Two new files (`wafprofile.go`, `validity.go`) + surgical edits to 5 existing files; no `cascade_waf.go` | §1.6 |

---

## 4. Testing Approach

- Table-driven Go tests with captured WAF response fixtures under
  `internal/access/testdata/`: Akamai `_abck=~-1~` sensor state,
  Cloudflare challenge body, DataDome marker, plus a `page_strong_ok`
  real-page fixture.
- Profile-ranking assertions (multi-match ordering, confidence formula,
  unknown fallback, clean-response empty).
- Verdict transition table covering every §6.3 row + AND-gating
  (single positive signal does NOT produce STRONG_OK).
- Avoid-list filtering assertions (exclusion + exhaustion fallback).
- No-Site-Name tripwire over every marker + selector.
- Re-run the existing CACHE-001 `TestFetchConcurrent` under `-race`
  (NFR-ACC-003) to confirm the `PhaseAttempt` field change is safe.
- 85% coverage target; benchmarks excluded from coverage.

---

## 5. References (verified 2026-06-17)

External:
- `https://github.com/fivetaku/insane-search` v0.5.0 commit 8dededa5.
- `https://pkg.go.dev/crypto/tls`.

Internal (file:line):
- `internal/access/phase4_tls.go:19-29, 46-51, 144-153`.
- `internal/access/phase3_get.go:21-25, 88-96, 135-200`.
- `internal/access/types.go:56-81`.
- `internal/access/escalation.go:21-43`.
- `internal/access/cascade.go:217-222, 251-273`.
- `internal/access/errors.go:59-82`.
- `internal/access/cascade_waf_test.go` (no `cascade_waf.go` source).
- `internal/access/options.go` (defaults baked in; no access.yaml).
- `.moai/specs/SPEC-CACHE-001/spec.md` (5-phase contract; OQ §8.8).
- `go.mod` (no utls/CycleTLS/refraction-networking).
- `.moai/config/sections/quality.yaml` (tdd, 85).
- `.moai/config/sections/language.yaml` (en, en).
