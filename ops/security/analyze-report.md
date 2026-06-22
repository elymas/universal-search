# SPEC-SEC-001 — Phase 1 ANALYZE Report

Security-surface inventory and gap analysis for the security-hardening DDD
cycle. Produced during T01 (DDD ANALYZE). All findings verified against the
live codebase at branch `feature/SPEC-SEC-001`.

## 1. Existing security surface (inventory)

### 1.1 SSRF guards (SPEC-CACHE-001 REQ-CACHE-013)

| File                        | Symbol                                          | Visibility | Behavior                                                                                                                                            |
| --------------------------- | ----------------------------------------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/access/ssrf.go`   | `validateScheme(u)`                             | unexported | Allows `http`/`https`; blocks all else with `*FetchError{CategoryBlocked}`                                                                          |
| `internal/access/ssrf.go`   | `validateHost(ctx, u, opts, fopts)`             | unexported | Resolves host; blocks if any resolved IP is private/loopback/link-local. Dual override: `opts.AllowPrivateNetworks \|\| fopts.AllowPrivateNetworks` |
| `internal/access/ssrf.go`   | `validateRedirect(next, opts, fopts, hopCount)` | unexported | Hop cap (default 5) + re-runs scheme + host check                                                                                                   |
| `internal/access/ssrf.go`   | `isPrivateOrLoopback(ip)`                       | unexported | CIDR membership: 10/8, 172.16/12, 192.168/16, 169.254/16, fc00::/7, fe80::/10, ::1/128, 127/8, 100.64/10                                            |
| `internal/access/dialer.go` | `pinnedDialContext(ctx, host, opts, fopts)`     | unexported | Resolves once, pins IP, rejects private pinned IP (DNS-rebind mitigation)                                                                           |
| `internal/access/dialer.go` | `dialContextWithPinnedIP(ip)`                   | unexported | Forces all TCP dials to the pinned IP                                                                                                               |

Non-test call sites (verified via grep):

- `internal/access/cascade.go:60-63` — pre-flight `validateScheme` + `validateHost`
- `internal/access/phase3_get.go:61` — `validateRedirect` in CheckRedirect; `:165,172` — pinned-IP path
- `internal/access/phase4_tls.go:69` — `validateRedirect` in CheckRedirect

Test surface (the de-facto characterization suite, 22 tests):
`ssrf_test.go` (14) + `ssrf_redirect_test.go` (5) + `dialer_test.go` (3).
All 22 verified GREEN against unchanged code at T01.

### 1.2 OIDC discovery SSRF guard (SPEC-AUTH-001 D8)

| File                          | Symbol                                 | Behavior                                                                      |
| ----------------------------- | -------------------------------------- | ----------------------------------------------------------------------------- |
| `internal/auth/private_ip.go` | `checkPrivateIP(host)`                 | Resolves host; rejects private/loopback/link-local via `net/netip` predicates |
| `internal/auth/private_ip.go` | `isPrivateIP(ipStr)`                   | Boolean private-range predicate                                               |
| `internal/auth/discovery.go`  | `validateIssuerURL(url, allowPrivate)` | HTTPS enforcement + private-IP block (bypassable for dev/CI)                  |

This is a **duplicate** of the access private-IP classification using a
different implementation (`net/netip` vs `net.IPNet` CIDR table). DDD IMPROVE
target: dedup onto the shared `internal/security/ssrf/` predicate while
preserving the auth package's exact external behavior (HTTPS + private block).

### 1.3 Audit subsystem (SPEC-AUTH-003) — REUSE, do not rebuild

Confirmed already implemented (the C1 amendment ground truth):

- `internal/audit/types.go` — `AuditEvent` struct with `PrevHash`/`ThisHash`
  fields; `EventType` enum (21 types) with **startup enum lock**
  (`AllEventTypes()` + `allEventTypesSet`; `EmitEvent` rejects unknown types).
- `internal/audit/store.go` — `Emitter.EmitEvent(ctx, AuditEvent)` single
  funnel (fan_in >= 7, already `@MX:ANCHOR`-tagged). `EventStore.Insert`
  interface.
- `internal/audit/chain.go` — `ComputeThisHash`, `CanonicalJSON`,
  `VerifyChain`, `AcquireAdvisoryLock`, `ChainManager` (default OFF).
- `deploy/postgres/migrations/0003_audit_events.sql` — `prev_hash`/`this_hash`
  columns + append-only triggers (confirmed present; **no new migration
  needed**).

Implication for T05: the SEC-001 7-type taxonomy must map onto
`internal/audit.EventType` constants and call the existing `Emitter`. Adding
new constants requires extending **both** the `const` block and
`AllEventTypes()` (the enum lock), in coordination with the AUTH-003 owner.

### 1.4 Observability cardinality allowlist (SPEC-OBS-001)

`internal/obs/metrics/metrics.go` — `Registry` with a static `labelNames`
allowlist (`AllLabelNames()`), guarded by NFR-OBS-002 cardinality tests.
Existing label names include `reason`, `component`-adjacent `adapter`,
`outcome`, `reason_class`, etc. SEC-001 needs to add `severity`, `type`,
`component`, and a security-metric label set without breaching the cap.

### 1.5 CI surface

- `.github/workflows/deps-audit.yml` — govulncheck + pip-audit (matrix) +
  pnpm-audit + hadolint + license-scan + searxng-digest-check. **Leave
  unchanged** (D1). All Dockerfiles already hadolint-scanned recursively.
- `.pre-commit-config.yaml` — hygiene + go-fmt/imports/mod-tidy + ruff +
  prettier + eslint + hadolint + shellcheck + yamllint. **No gitleaks hook.**
- No `.github/workflows/security.yml`, no `.gitleaks.toml`, no `.github/CODEOWNERS`.
- 5 Dockerfiles under `services/*` (koreanews, tokenizer-ko, embedder,
  researcher, storm).

## 2. Identified gaps (SEC-001 closes)

| Gap                                                                                                           | REQ         | Phase/Task                  |
| ------------------------------------------------------------------------------------------------------------- | ----------- | --------------------------- |
| SSRF guards are access-internal (unexported); not reusable by auth/adapters                                   | REQ-SEC-007 | T04                         |
| No cloud-metadata hostname blocklist (only IP 169.254/16 covered)                                             | REQ-SEC-008 | T04                         |
| No SSRF block metric (`ssrf_blocks_total`)                                                                    | REQ-SEC-009 | T04                         |
| Duplicate private-IP logic in access vs auth                                                                  | REQ-SEC-007 | T04                         |
| No secret scanning (gitleaks) — pre-commit or CI                                                              | REQ-SEC-004 | T02                         |
| No container CVE scan (Trivy)                                                                                 | REQ-SEC-001 | T03                         |
| No UNFIXED-CVE exception tracking with deadlines                                                              | REQ-SEC-003 | T03                         |
| 7-type security event taxonomy not emitted into AUTH-003 chain                                                | REQ-SEC-017 | T05                         |
| `secret.scan.finding` / `ssrf.blocked` / `ratelimit.exceeded` / `prompt.sanitized` EventType constants absent | REQ-SEC-017 | T05 (AUTH-003 coordination) |

## 3. PRESERVE safety net (T01 deliverable)

Two new characterization baseline test files pin the observable contract:

- `internal/access/ssrf_baseline_test.go` — asserts `FetchError.Category`
  surfaced to cascade callers (not just non-nil), the dual `AllowPrivateNetworks`
  override, the redirect hop-cap default-of-5, and the IP classification table.
- `internal/auth/private_ip_baseline_test.go` — asserts HTTPS enforcement
  independent of the private bypass, and the private-IP rejection boundary.

These tests are GREEN against unchanged code (verified at T01) and are the
regression gate for the T04 extraction.

## 4. Risk notes

- **R4 (CACHE-001 behavior drift):** the access package keeps its own
  `FetchError`/`Options`/`FetchOptions` types. The extracted generic package
  must use its own error type (cycle: access→ssrf forbids ssrf→access). The
  access wrappers translate the generic error back to `*FetchError` so the
  cascade contract is byte-identical. Mitigation: characterization baseline +
  all 22 CACHE-001 tests run after every refactor commit.
- **R5 (AUTH-003 enum lock):** new EventType constants require coordinated
  edits to the `const` block AND `AllEventTypes()`. Marked with `@MX:NOTE` +
  AUTH-003 owner sign-off gate. Fail-closed lockdown stays default OFF.
- **R12 (hostname blocklist bypass):** suffix-match + case-insensitive +
  resolved-IP cross-check required; IP 169.254.169.254 already covered by the
  private-range table, so the hostname layer is defense-in-depth.

## 5. Drift observed vs spec.md

- spec.md references `internal/cache/access/` throughout — actual path is
  `internal/access/` (documentation drift, confirmed in tasks.md path-correction
  notice). Not a scope change.
- spec.md initially claimed "9" CACHE-013 tests; verified count is 22.
