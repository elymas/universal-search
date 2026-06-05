---
id: SPEC-SEC-002
version: 0.1.0
title: Adapter Credential Resolution via Configured Secret Backend
milestone: M8 — Eval + polish
status: draft
priority: P1
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-06-04
updated: 2026-06-04
author: limbowl
issue_number: null
labels: [security, secrets, adapters, M8]
depends_on: [SPEC-SEC-001, SPEC-CLI-002, SPEC-ADP-008]
blocks: []
---

# SPEC-SEC-002: Adapter Credential Resolution via Configured Secret Backend

## HISTORY

- 2026-06-04 (initial draft v0.1, limbowl via manager-spec):
  Companion artifact: `research.md` (this same directory). Resolves GSD
  codebase-audit finding **F-07** (`.planning/AUDIT-FINDINGS.md:30`,
  severity medium, class manual, "blocks secure key backends;
  config.toml/koanf does not feed adapter creds"). F-07 is the only audit
  finding without a SPEC as of 2026-06-04 (`AUDIT-FINDINGS.md:15`).

  This SPEC is the call-site refactor that SPEC-SEC-001 REQ-SEC-016
  explicitly anticipated: the `secretstore.Resolver` `@MX:ANCHOR`
  (`internal/security/secretstore/resolver.go:17-22`) predicts
  `fan_in >= 3 once call-site refactors land` and names
  `llm/config, adapters/naver, and future K8s/Vault-deployed secret
  sites` as the callers. SPEC-SEC-002 realizes the adapter half of that
  prediction by threading ONE configured `Resolver` from the CLI into
  every credentialed adapter, replacing the hardcoded env / raw
  `os.Getenv` paths that bypass the backend selection.

  **Two finding overstatements are corrected in this SPEC (verified
  against source):**
  1. F-07 says "keychain/vault/k8s backends exist". There is **NO
     keychain backend.** Only `env.go`, `k8s.go`, and `vault.go` (a stub)
     exist under `internal/security/secretstore/`. Keychain is explicitly
     OUT OF SCOPE here (§2.2) and noted as a possible follow-on.
  2. F-07 groups "github/koreanews use raw `os.Getenv`" as if both bypass
     credential resolution. **koreanews has no credentials.** Its
     `USEARCH_ADP009_*` env vars are feature FLAGS / URLs
     (`RSS_ENABLED`, `RSS_FEEDS`, `DAUM_ENABLED`, `KNC_ENABLED`,
     `KNC_BASE_URL` — `internal/adapters/koreanews/options.go:124-157`),
     and its Capabilities declare `AuthEnvVars: nil`
     (`koreanews.go:91`). The genuine credential-resolution sites are
     exactly TWO: **naver** (`NAVER_CLIENT_ID` + `NAVER_CLIENT_SECRET`)
     and **github** (`USEARCH_GITHUB_TOKEN`), the only two adapters whose
     Capabilities set `RequiresAuth: true` with real credential env vars
     (`naver.go:196-197`, `github.go:146-147`). SPEC-SEC-002 narrows to
     those two; koreanews flags are untouched.

  Verified gap (every claim file:line-checked against current source):
  - `cmd/usearch/query.go` `buildProductionRegistry` constructs NO
    `Resolver`; a grep of `cmd/usearch/` for `secretstore` / `NewResolver`
    returns ZERO matches. The CLI never reads `security.yaml`
    `secrets.backend`, so the entire env|k8s|vault selection is DEAD for
    adapters today.
  - `internal/adapters/naver/naver.go:23` declares a package-global
    `var secretEnv secretstore.Resolver = secretstore.NewEnvResolver()`
    (env-ONLY, ignores configured backend); `naver.go:121,129` resolve
    `NAVER_CLIENT_ID` / `NAVER_CLIENT_SECRET` through it inside `New()`.
  - `internal/adapters/github/github.go` accepts the token via
    `Options.Token` (`github.go:46-48`) and does NOT self-resolve; the
    production wiring at `cmd/usearch/query.go` reads it via raw
    `os.Getenv("USEARCH_GITHUB_TOKEN")` with a `GITHUB_TOKEN` fallback,
    bypassing any Resolver.

  Open question carried to research.md: `internal/security/secrets/`
  (only `resolver.go` + `resolver_test.go`, git-tracked) AND
  `internal/security/secretstore/` (full backend set, git-tracked) BOTH
  exist. SEC-001's HISTORY (`spec.md:29-30`) states it RENAMED
  `secrets/` → `secretstore/` to avoid a repo-root credential-protection
  deny rule, yet `secrets/resolver.go` is still tracked. The `secrets/`
  directory is read-restricted by that same deny rule, so its current
  contents could NOT be inspected via Read; whether it is a stale
  leftover or a live parallel package is UNRESOLVED (§9 Open Question
  OQ-1). naver imports `secretstore`, which is treated as canonical here.

  7 EARS REQs (1 × P0 + 5 × P1 + 1 × P2) + 4 NFRs. Three EARS patterns
  used (Ubiquitous + Event-Driven + Unwanted). Status `draft` pending
  plan-auditor + annotation cycle.

  Frontmatter uses `created` / `updated` (NOT `created_at`): all ~52 SPECs
  in this repo and the direct siblings SPEC-CLI-003 / SPEC-ADP-006-XENABLE
  / SPEC-ADP-010 use `created`/`updated` with no `created_at`. Renaming
  would break house consistency; this is the same accepted firewall
  deviation those siblings carry (plan-audit review-1 MP-3 noted it as a
  project-wide convention).

- 2026-06-04 (revision r1 — plan-audit fixes, limbowl via manager-spec):
  Resolved plan-audit `SPEC-SEC-002-review-1.md` (PASS-WITH-FIXES, 0.62).
  All `file:line` citations and both finding-corrections were verified
  accurate by the auditor; the fixes close a design-soundness gap plus
  labeling/wording defects. Re-verified each against source before writing.
  Changes by defect ID:
  - **D1 (CRITICAL):** the adapter registration gate
    (`internal/adapters/registry.go:151-153`) reads `os.LookupEnv` DIRECTLY
    to decide whether a `RequiresAuth` adapter may register, and the
    admin-view key-presence check (`registry.go:266-274`) does the same.
    So even after adapters resolve credentials through the injected
    Resolver, a `k8s` / `vault` backend (secrets in a mount file / Vault,
    NOT in process env) would REJECT registration with `ErrMissingAuth` —
    defeating SEC-002's core goal for the only non-env backends and making
    the Resolver NOT the "SOLE source" (REQ-SEC2-001) / violating
    NFR-SEC2-004. Fix: added **REQ-SEC2-007** making credentialed-adapter
    registration Resolver-aware — the CLI resolves the key via the Resolver
    FIRST, then registers with `RegisterOptions{SkipAuthCheck: true}` so
    the env-only registry gate is bypassed for credentialed adapters; the
    CLI owns the skip (env-backend, key absent) / loud-fail (vault)
    decision based on the Resolver result. Extended §2.1(b)/§5.3, replaced
    the §2.2 "registration semantics preserved" overclaim with a precise
    statement of the gate bypass, and added a k8s-backend registration
    test (`TestK8sBackendAdapterRegisters`). The `registry.go` source is
    NOT modified — the fix lives in the CLI registration call (the
    auditor's recommended path (1)).
  - **D2 (MAJOR):** `internal/llm/config/config.go:14,45,58` is ALREADY a
    live `secretstore.NewEnvResolver()` caller (same env-only defect as
    naver), so the `Resolver` `@MX:ANCHOR` fan_in was ALREADY 2 (naver +
    llm/config) before SEC-002 — not "predicted". Corrected §6 (llm/config
    is NOT counted as a SEC-002-new caller; the SEC-002 addition is the
    github CLI-site, bringing actual fan_in to 3) and §2.2 / OQ-4
    (llm/config is a CURRENT caller carrying the same defect, to be fixed
    by a separate refactor — not "future").
  - **D3 (MINOR):** EARS labels. HISTORY claimed a State-Driven REQ exists;
    none does — corrected the pattern tally to Ubiquitous + Event-Driven +
    Unwanted. REQ-SEC2-006 was mislabeled "Unwanted" but is a negative
    Ubiquitous (`SHALL NEVER`) — relabeled "Ubiquitous". REQ-SEC2-004
    remains the (correct) Unwanted (`IF…THEN`).
  - **D4 (frontmatter):** KEPT `created`/`updated` per user instruction
    (project-wide convention; same deviation as CLI-003 / ADP-006-XENABLE
    / ADP-010). Rationale recorded above.

  Post-fix tally: 7 EARS REQs (1 × P0 + 5 × P1 + 1 × P2) + 4 NFRs.

---

## 1. Purpose

SPEC-SEC-001 shipped a complete runtime secret-resolution layer
(`internal/security/secretstore/`): a `Resolver` interface, a
`NewResolver(backend, mountPath)` factory selecting env / k8s / vault
backends, and a `security.yaml secrets.backend` configuration key (default
`env`). The intent — documented on the `Resolver` `@MX:ANCHOR`
(`resolver.go:17-22`) — was that credentialed call sites would later be
refactored to resolve through this single seam, so that an operator
choosing `backend: k8s` (Helm) or `backend: vault` (post-V1) would
transparently change how EVERY adapter obtains its secrets.

That refactor never happened for adapters. The CLI's
`buildProductionRegistry` (`cmd/usearch/query.go`) constructs no
`Resolver` and never reads `secrets.backend`. Each credentialed adapter
self-resolves: naver hardcodes a package-global `NewEnvResolver()`
(env-only); github's token is read by the CLI via raw `os.Getenv`. The
net effect is that the entire env|k8s|vault backend selection is **dead
configuration** for adapter credentials — selecting `backend: k8s` has no
effect on whether naver or github can find their keys.

SPEC-SEC-002 closes finding **F-07** by making adapter credential
resolution honor the configured backend:

- **Construct ONE `Resolver`** in the CLI from `security.yaml`
  `secrets.backend` (+ `k8s_mount_path`) via the existing
  `secretstore.NewResolver`, and thread it into `buildProductionRegistry`.
- **Refactor the two true credential sites** (naver, github) to obtain
  their secrets from that injected `Resolver` instead of a hardcoded env
  resolver / raw `os.Getenv`.
- **Preserve backward compatibility**: with `backend: env` (the default),
  observable behavior is unchanged — `EnvResolver` has os.Getenv
  semantics (`env.go:9-27`), so current deployments behave identically.

The animating principle is **one seam for secrets**: after this SPEC,
the answer to "where does an adapter credential come from?" is always
"the configured `Resolver`", regardless of backend. Today that answer
differs per adapter (naver: a private env resolver; github: the CLI's
raw os.Getenv), and the operator's `secrets.backend` choice is ignored
entirely.

### Why the vault stub must surface, not swallow

`secretstore.NewResolver(BackendVault, _)` constructs successfully
(`factory.go:31-32`) so configuration validation passes, but its
`Get` always returns `ErrNotImplemented` (`vault.go:14-17`). If an
operator sets `backend: vault` and the CLI silently treats
`ErrNotImplemented` as a missing credential, naver/github would simply
fail to register with a confusing "not set" message. SPEC-SEC-002
requires `backend: vault` to surface as a CLEAR startup/config error
that names the unimplemented backend (REQ-SEC2-004), so the operator
learns "vault is a stub" rather than "my keys are missing". Implementing
the real Vault client stays OUT of scope (follow-on; §2.2).

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | Construct ONE `secretstore.Resolver` in the CLI from the `security.yaml` `secrets.backend` (`env`/`k8s`/`vault`) and `secrets.k8s_mount_path` values, via the existing `secretstore.NewResolver(backend, mountPath)` (`factory.go:22`). The Resolver is built ONCE per process and passed into `buildProductionRegistry` (`cmd/usearch/query.go`). An unknown backend SHALL fail process startup with the factory's config error (`factory.go:33-34`). |
| b | Thread the Resolver into `buildProductionRegistry` and into the two credentialed adapter constructors. `buildProductionRegistry` SHALL accept the Resolver (signature extension or struct field — exact shape deferred to plan.md) and pass it to naver and github construction. Non-credentialed adapters (reddit, hn, arxiv, youtube, searxng, social, koreanews) are constructed UNCHANGED — they have `AuthEnvVars: nil` and resolve no secrets. Credentialed adapters whose key the Resolver supplied SHALL be registered via `Registry.RegisterWithOptions(a, RegisterOptions{SkipAuthCheck: true})` so the registry's env-only auth gate (`registry.go:151-153`) does NOT re-reject them on a non-env backend (see (i) and REQ-SEC2-007). Non-credentialed adapters keep using plain `Register` (their gate is a no-op since `RequiresAuth` is false). |
| c | Refactor naver credential resolution — remove the package-global `var secretEnv = secretstore.NewEnvResolver()` (`naver.go:23`) and accept an injected `Resolver` via `naver.Options`. `naver.New` SHALL resolve `NAVER_CLIENT_ID` / `NAVER_CLIENT_SECRET` through the injected Resolver (falling back to the existing env resolver only when no Resolver is injected, to preserve test ergonomics). The existing `Options.ClientID` / `Options.ClientSecret` direct-override path (`naver.go:67-72,119-133`) is PRESERVED and takes precedence over Resolver lookup, so existing naver tests that inject literal credentials keep working. |
| d | Refactor github token resolution — the CLI SHALL resolve `USEARCH_GITHUB_TOKEN` through the injected Resolver and pass the resolved value into `github.New` via the existing `Options.Token` field (`github.go:46-48`). The `github` package itself is NOT modified (it already takes the token by injection; only the CLI's raw `os.Getenv` site at `cmd/usearch/query.go` changes). The `GITHUB_TOKEN` fallback alias behavior is preserved (resolve `USEARCH_GITHUB_TOKEN` first, then `GITHUB_TOKEN`), but both lookups go through the Resolver. |
| e | Preserve the env-backend default — when `secrets.backend` is `env` or unset, the constructed Resolver is `EnvResolver` (`factory.go:24-25`), which has identical os.Getenv semantics to today's behavior. Existing deployments with `NAVER_CLIENT_*` / `USEARCH_GITHUB_TOKEN` in the process environment SHALL behave exactly as before (same adapters registered, same skip-on-missing semantics). |
| f | Surface the vault stub gracefully — when `secrets.backend` is `vault`, a credential lookup returns `ErrNotImplemented` (`vault.go:15-16`). The CLI SHALL detect `ErrNotImplemented` (via `errors.Is`) during credential resolution for a credentialed adapter and emit a CLEAR startup/config error naming the vault backend as unimplemented, rather than treating it as a silent missing-credential skip. The real Vault client is NOT implemented here (§2.2). |
| g | Honor the no-leak invariant — neither the Resolver wiring, the refactored adapters, nor the new error paths SHALL log, print, or embed a resolved secret VALUE in any log line, metric, span, or error message. Errors reference env-var NAMES and backend identifiers only. This mirrors the registry's `SecretValue`-always-empty invariant (`registry.go:234-236`) and the `Resolver` godoc contract (`resolver.go:13-15`). |
| h | Test discipline WITHOUT `t.Setenv` under `-race` — all backend-selection and resolution tests SHALL drive credential state by injecting a fake `Resolver` (or fake env-lookup) rather than mutating process env via `os.Setenv` / `t.Setenv`, because `t.Setenv` is goroutine-unsafe under `-race` for parallel tests (same discipline as SPEC-ADP-006 H1). The injected Resolver makes backend behavior deterministic and parallel-safe. |
| i | Make credentialed-adapter registration Resolver-aware (closes audit D1) — the registry's auth gate (`RegisterWithOptions`, `registry.go:151-153`) tests credential PRESENCE via `os.LookupEnv` directly, which would REJECT a `RequiresAuth` adapter on a `k8s` / `vault` backend even when the Resolver successfully supplied the key (the secret lives in a mount file / Vault, not process env). The CLI SHALL therefore resolve each credentialed adapter's key via the Resolver FIRST, decide skip/register/loud-fail from the Resolver result, and register the adapter with `RegisterOptions{SkipAuthCheck: true}` (bypassing the env-only gate). The `registry.go` source is NOT changed — the gate is bypassed from the CLI, not rewritten. This makes the Resolver the genuine SOLE credential source on ALL backends, not just env. |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following. Each is deliberately
out of frame; this list prevents scope creep.

- **NOT adding a keychain backend** — F-07's claim that a keychain
  backend "exists" is FALSE; only `env`/`k8s`/`vault` exist under
  `internal/security/secretstore/`. Adding a macOS Keychain / OS
  credential-store backend is a possible FOLLOW-ON SPEC, not part of
  SPEC-SEC-002. This SPEC wires the EXISTING backends through.
- **NOT implementing the real Vault client** — `vault.go` stays a stub
  returning `ErrNotImplemented`. SPEC-SEC-002 only ensures the stub
  surfaces as a clear error (REQ-SEC2-004), not silent failure. A real
  HashiCorp Vault client is a post-V1 follow-on (the stub's own godoc
  says so, `vault.go:5-8`).
- **NOT touching koreanews** — its `USEARCH_ADP009_*` envs are feature
  flags / URLs, not credentials (`AuthEnvVars: nil`, `koreanews.go:91`).
  Its `OptionsFromEnv` flag-reading (`options.go:124-157`) is untouched.
  Correcting F-07's misclassification of koreanews is documentation,
  not a code change.
- **NOT changing the `secretstore` package** — no signature changes to
  `Resolver`, `NewResolver`, `NewEnvResolver`, `NewK8sResolver`,
  `NewVaultResolver`, or the `Backend*` constants. SPEC-SEC-002 is a
  pure CONSUMER of SEC-001's existing factory and interface.
- **NOT consolidating the `secrets/` vs `secretstore/` packages** — the
  duplication (OQ-1) is FLAGGED as an open question, not resolved here.
  `secrets/` is read-restricted and cannot be safely inspected; deciding
  its fate (delete stale leftover vs keep) is a separate cleanup SPEC.
  SPEC-SEC-002 uses `secretstore` (what naver already imports).
- **NOT migrating `config.toml` / koanf `[auth]` to feed adapter creds**
  — F-07 notes config.toml/koanf does not feed adapter creds. Verified:
  adapter credentials flow through the Resolver/env, NOT through the
  koanf `[auth]` block (SPEC-CLI-002 REQ-CLI2-007). Unifying the koanf
  auth config with the secret backend is a possible follow-on; SPEC-SEC-002
  resolves via the `secretstore` backend only, leaving koanf config as-is.
- **NOT changing the llm/config secret site** — `internal/llm/config/
  config.go:14,45,58` is ALREADY a live `secretstore` caller: it imports
  the package, declares `var secretEnv = secretstore.NewEnvResolver()`,
  and resolves `LITELLM_MASTER_KEY` through it — carrying the SAME env-only
  hardcoded defect as naver (it ignores the configured backend). It is a
  CURRENT (not future) caller. SPEC-SEC-002 scope is ADAPTERS only, so the
  llm/config defect is fixed by a SEPARATE follow-on refactor (OQ-4). Note
  for fan_in accounting: the `Resolver` `@MX:ANCHOR` already has 2 callers
  (naver + llm/config) BEFORE this SPEC; SEC-002 adds the github CLI-site,
  bringing actual fan_in to 3 (see §6).
- **NOT rewriting the registry source** — SPEC-SEC-002 does NOT modify
  `registry.go`. The env-only auth gate at `registry.go:151-153` and the
  admin-view key-presence check at `registry.go:266-274` are left in
  place. The D1 fix is achieved from the CLI side: credentialed adapters
  are registered with `RegisterOptions{SkipAuthCheck: true}` after the CLI
  has already resolved their keys via the Resolver (§2.1(i),
  REQ-SEC2-007), so the gate is BYPASSED for them rather than rewritten.
  (Making the registry itself Resolver-aware — passing a Resolver into
  `RegisterWithOptions` — is a larger SEC-001-owned change and is the
  rejected alternative; the CLI-side bypass is the minimal fix.) The
  admin-view `os.LookupEnv` key-presence display (`registry.go:266-274`)
  is a SEPARATE telemetry concern owned by SPEC-UI-002 and is NOT fixed
  here — it is flagged as OQ-5.
- **NOT GitHub Issue tracking on this SPEC** (`issue_number: null`).

### 2.3 Forward-Looking / Dependency Notes

[HARD] SPEC-SEC-002 `depends_on`:
- **SPEC-SEC-001** (implemented) — the LOAD-BEARING dependency. Owns
  `internal/security/secretstore/` (Resolver, NewResolver, env/k8s/vault
  backends), the `security.yaml secrets.backend` key, and REQ-SEC-016's
  `Resolver` `@MX:ANCHOR` whose predicted `fan_in >= 3` this SPEC
  realizes. SPEC-SEC-002 is a pure consumer of these primitives.
- **SPEC-CLI-002** (implemented) — owns the CLI surface and the
  `config.toml`/koanf config plumbing this SPEC reads `security.yaml`
  alongside. The Resolver construction is wired into the CLI startup
  path CLI-002 established.
- **SPEC-ADP-008** (implemented) — owns the naver adapter being
  refactored (`naver.New`, `naver.Options`, the `secretEnv` global, the
  `NAVER_CLIENT_ID`/`SECRET` resolution). SPEC-SEC-002 modifies this
  adapter's credential path; ADP-008's tests must continue to pass.

The github adapter (SPEC-ADP-004 territory) is NOT modified internally —
only the CLI's token-resolution site changes — so an explicit ADP-004
`depends_on` is not required; the github package is consumed unchanged.

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-SEC2-001 | Ubiquitous | The CLI SHALL construct exactly ONE `secretstore.Resolver` per process by calling `secretstore.NewResolver(backend, mountPath)` (`factory.go:22`) with `backend` from `security.yaml` `secrets.backend` (`env`/`k8s`/`vault`, default `env` when unset) and `mountPath` from `secrets.k8s_mount_path` (default `secretstore.DefaultK8sMountPath` = `/var/run/secrets`). This single Resolver SHALL be passed into `buildProductionRegistry` (`cmd/usearch/query.go`) and forwarded to every credentialed adapter constructor. After this SPEC, `cmd/usearch` SHALL contain at least one call to `secretstore.NewResolver` (today it contains none). The Resolver SHALL be the SOLE source of adapter credentials in the production path — no credentialed adapter SHALL read `os.Getenv` directly for its key. | P0 | `TestCLIBuildsResolverFromConfig` (env/k8s/vault config → correct concrete Resolver type), `TestCLIDefaultBackendIsEnv` (unset backend → `*EnvResolver`), `TestBuildProductionRegistryUsesInjectedResolver` (registry built with a fake Resolver resolves naver/github through it); a grep test `TestCLIHasResolverWiring` asserts `cmd/usearch` references `secretstore.NewResolver` and no credentialed adapter site uses raw `os.Getenv` for its key. |
| REQ-SEC2-002 | Event-Driven | WHEN `naver.New` is invoked with an injected `Resolver` and empty `Options.ClientID` / `Options.ClientSecret`, the adapter SHALL resolve `NAVER_CLIENT_ID` and `NAVER_CLIENT_SECRET` via `resolver.Get(ctx, key)` rather than the removed package-global `secretEnv` (`naver.go:23`). The package-global `secretEnv` SHALL be removed. WHEN `Options.ClientID` / `Options.ClientSecret` ARE provided (non-empty), those literal values SHALL take precedence and NO Resolver lookup SHALL occur (preserving the existing test-injection path at `naver.go:119-133`). WHEN no Resolver is injected (zero-value Options field), `naver.New` SHALL fall back to a default `EnvResolver` so standalone construction and existing tests keep working. | P1 | `TestNaverResolvesViaInjectedResolver` (fake Resolver supplies both keys → adapter constructs), `TestNaverOptionsCredsBeatResolver` (literal Options.ClientID/Secret used; fake Resolver `Get` asserted NOT called), `TestNaverNoResolverFallsBackToEnv` (nil Resolver → default EnvResolver behavior), `TestNaverNoPackageGlobalSecretEnv` (grep: the `var secretEnv` global is gone). |
| REQ-SEC2-003 | Event-Driven | WHEN `buildProductionRegistry` constructs the github adapter, the CLI SHALL resolve the token by calling `resolver.Get(ctx, "USEARCH_GITHUB_TOKEN")` and, only if that yields no value, `resolver.Get(ctx, "GITHUB_TOKEN")`, then pass the resolved token into `github.New` via `Options.Token` (`github.go:46-48`). The raw `os.Getenv("USEARCH_GITHUB_TOKEN")` / `os.Getenv("GITHUB_TOKEN")` site in `cmd/usearch/query.go` SHALL be removed. The `github` package internals SHALL NOT change. As today, when neither key resolves to a non-empty value, the github adapter SHALL be skipped (not registered) without error. | P1 | `TestGithubTokenViaResolver` (fake Resolver returns a token for `USEARCH_GITHUB_TOKEN` → github registered with that token), `TestGithubTokenFallbackAlias` (primary key empty, `GITHUB_TOKEN` set → fallback used), `TestGithubSkippedWhenNoTokenResolves` (both keys unresolved → github absent from registry, no error), `TestCLINoRawGetenvForGithubToken` (grep: query.go no longer calls `os.Getenv` for the github token). |
| REQ-SEC2-004 | Unwanted | IF `secrets.backend` is `vault`, THEN any credentialed-adapter token lookup returns `ErrNotImplemented` (`vault.go:15-16`); the CLI SHALL detect this via `errors.Is(err, secretstore.ErrNotImplemented)` and SHALL fail process startup (or the registry-build step) with a CLEAR error that names the vault backend as unimplemented and points to the env/k8s alternatives — it SHALL NOT silently skip the adapter as if the credential were merely absent. The error SHALL NOT contain any secret value. IF `secrets.backend` is an unrecognized string, THEN `NewResolver` returns the factory config error (`factory.go:33-34`) and the CLI SHALL fail startup with that message. | P1 | `TestVaultBackendReturnsNotImplemented` (Resolver from `vault` backend → `Get` returns `ErrNotImplemented`), `TestCLIVaultBackendFailsLoudly` (vault backend + a credentialed adapter → startup error is `errors.Is(..., ErrNotImplemented)` and message names "vault"), `TestVaultErrorNotTreatedAsMissingKey` (vault error path distinct from the env "not set" skip path), `TestCLIUnknownBackendFailsStartup` (backend "hsm" → factory config error). |
| REQ-SEC2-005 | Ubiquitous | The system SHALL preserve env-backend backward compatibility: WHEN `secrets.backend` is `env` or unset, the constructed Resolver is `*EnvResolver` (`factory.go:24-25`) with os.Getenv semantics (`env.go:21-27`), and the set of adapters registered by `buildProductionRegistry` under a given process environment SHALL be IDENTICAL to the pre-SPEC behavior (same adapters present/absent, same skip-on-missing-credential semantics). No new required configuration SHALL be introduced: a deployment that does not set `secrets.backend` continues to work via the env default. | P1 | `TestEnvBackendBackwardCompatible` (with NAVER_CLIENT_ID/SECRET + USEARCH_GITHUB_TOKEN present via injected EnvResolver-equivalent, naver+github register exactly as before), `TestEnvBackendMissingCredsSkipsAdapter` (missing keys → adapter skipped, no error, parity with pre-SPEC), `TestNoNewRequiredConfig` (registry builds with absent `secrets` block → env default). |
| REQ-SEC2-006 | Ubiquitous | The system SHALL NEVER emit a resolved secret VALUE in any output. No log record, metric label, OTel span attribute, or error message produced by the Resolver wiring, the refactored naver path, or the github token resolution SHALL contain the resolved `NAVER_CLIENT_ID`, `NAVER_CLIENT_SECRET`, or github token value. Errors SHALL reference env-var NAMES (e.g. `"NAVER_CLIENT_ID"`) and backend identifiers (`"env"`/`"k8s"`/`"vault"`) only. This preserves the `Resolver` no-leak contract (`resolver.go:13-15`) and the registry `SecretValue`-always-empty invariant (`registry.go:234-236`). (Pattern: negative Ubiquitous — `SHALL NEVER` — not the `IF…THEN` Unwanted form; REQ-SEC2-004 is the Unwanted REQ.) | P2 | `TestNoSecretInErrorOutput` (force missing/failed resolution for each credentialed adapter; assert no error string contains the secret value), `TestNoSecretInVaultError` (vault path error names "vault" but no value), `TestResolverWiringEmitsNoSecretValue` (capture logs/spans during a successful resolution; assert env-var names may appear, values never do). |
| REQ-SEC2-007 | Event-Driven | WHEN the CLI registers a credentialed adapter (naver or github) into the production registry, it SHALL first resolve the adapter's credential via the injected Resolver, then register the adapter via `Registry.RegisterWithOptions(a, RegisterOptions{SkipAuthCheck: true})` so the registry's env-only auth gate (`registry.go:151-153`, which calls `os.LookupEnv` on `AuthEnvVars`) does NOT re-reject an adapter whose secret was supplied by a non-env backend. The CLI (NOT the registry) SHALL decide the disposition from the Resolver result: credential resolved → register; credential absent under the `env` backend → skip (parity with today); credential lookup returns `ErrNotImplemented` under the `vault` backend → loud startup failure (REQ-SEC2-004). The `registry.go` source SHALL NOT be modified. After this SPEC, with `backend: k8s` and the required secret present in the mount path, naver/github SHALL successfully REGISTER (not be rejected with `ErrMissingAuth`). | P1 | `TestK8sBackendAdapterRegisters` (fake `K8sResolver`-equivalent supplies NAVER_CLIENT_ID/SECRET + USEARCH_GITHUB_TOKEN from a temp mount dir; process env UNSET; naver+github both present in `Registry.List()` — proves the env gate no longer rejects them), `TestCredentialedAdapterRegisteredWithSkipAuthCheck` (asserts the CLI registration call uses `SkipAuthCheck: true` for credentialed adapters), `TestK8sBackendMissingSecretSkipsAdapter` (mount file absent under env-equivalent skip semantics → adapter skipped, no panic), `TestRegistryGateBypassedNotRewritten` (grep: `registry.go` auth-gate source is unchanged). |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-SEC2-001 | No secret in logs/metrics/errors | [HARD] Across ALL code paths touched by this SPEC (Resolver construction, naver/github credential resolution, the vault/unknown-backend error paths), zero resolved secret values SHALL appear in slog records, Prometheus labels, OTel span attributes, or returned error strings. CI SHALL assert this with a fake Resolver that returns a sentinel secret value and a log/span capture that fails if the sentinel value (not its key name) appears anywhere. This is the security-critical NFR; a violation is a release blocker. |
| NFR-SEC2-002 | Test isolation without `t.Setenv` under `-race` | [HARD] All backend-selection and credential-resolution tests SHALL drive secret state by injecting a fake `Resolver` (or fake env-lookup closure), NOT by mutating process env via `os.Setenv` / `t.Setenv`. Rationale: `t.Setenv` is goroutine-unsafe under `-race` for parallel tests (per Go testing docs; same discipline as SPEC-ADP-006 H1). The full SPEC-SEC-002 test set SHALL pass under `go test -race ./...` with `t.Parallel()` enabled where applicable. CI SHALL contain no `t.Setenv` in the new test files. |
| NFR-SEC2-003 | Backend-selection determinism | Given a fixed `security.yaml` `secrets.backend` value and `k8s_mount_path`, `secretstore.NewResolver` SHALL produce a deterministic concrete Resolver type (`env`→`*EnvResolver`, `k8s`→`*K8sResolver`, `vault`→`*VaultResolver`, unknown→config error). The CLI's Resolver construction SHALL be a pure function of the config values (no hidden global state, no order-dependence). A table-driven test SHALL assert the backend-string→Resolver-type mapping including the empty-string→env default and the unknown→error case. |
| NFR-SEC2-004 | Single source of truth for adapter credentials | After this SPEC there SHALL be exactly one mechanism by which a production adapter obtains a credential: the injected `Resolver`. The naver package-global `secretEnv` (`naver.go:23`) SHALL be removed; the CLI's raw `os.Getenv` token site SHALL be removed. A regression test SHALL assert that no credentialed adapter resolves its key by a path other than the injected Resolver (grep-based: no `os.Getenv` of a credential env var inside adapter constructors or the github token site in `cmd/usearch`). Non-credentialed adapters (AuthEnvVars: nil) are exempt — they resolve nothing. |

---

## 4. Acceptance Criteria

Detailed Given/When/Then scenarios live in the companion file
`.moai/specs/SPEC-SEC-002/acceptance.md` (to be authored alongside the
annotation cycle). This section enumerates the acceptance gate per
requirement.

### REQ-SEC2-001 — One Resolver from config, threaded into the registry

- `cmd/usearch` constructs the Resolver via `secretstore.NewResolver`
  using `security.yaml` `secrets.backend` + `k8s_mount_path`.
- The Resolver is passed into `buildProductionRegistry` and forwarded to
  naver + github construction.
- No credentialed adapter reads `os.Getenv` directly for its key.

### REQ-SEC2-002 — naver resolves via injected Resolver

- Package-global `secretEnv` removed.
- `naver.New` resolves `NAVER_CLIENT_ID`/`SECRET` via injected Resolver.
- Literal `Options.ClientID`/`Secret` still take precedence.
- nil Resolver falls back to a default EnvResolver.

### REQ-SEC2-003 — github token via injected Resolver

- CLI resolves `USEARCH_GITHUB_TOKEN` (then `GITHUB_TOKEN`) via Resolver,
  passes into `github.New` Options.Token.
- Raw `os.Getenv` token site in query.go removed; github package unchanged.
- No token resolved → github skipped, no error.

### REQ-SEC2-004 — vault stub surfaces; unknown backend fails

- `vault` backend → `Get` returns `ErrNotImplemented`.
- CLI detects it via `errors.Is` and fails startup with a clear,
  vault-naming, value-free error (NOT a silent skip).
- Unknown backend → factory config error fails startup.

### REQ-SEC2-005 — env backend backward compatibility

- `env`/unset → `*EnvResolver`; identical adapter-registration outcome.
- Missing creds → adapter skipped (parity with pre-SPEC).
- No new required config.

### REQ-SEC2-006 — no secret value leak

- No error/log/metric/span emits a resolved secret value.
- Errors reference env-var NAMES and backend identifiers only.

### REQ-SEC2-007 — Resolver-aware registration (k8s/vault reach adapters)

- Credentialed adapters are registered with `SkipAuthCheck: true` after
  the CLI resolves their key via the Resolver.
- With `backend: k8s` and the secret in the mount path (process env
  UNSET), naver+github successfully REGISTER (no `ErrMissingAuth`).
- Skip (env, key absent) / loud-fail (vault) decided by the CLI from the
  Resolver result, not the env-only registry gate.
- `registry.go` source is unchanged (gate bypassed, not rewritten).

### NFR-SEC2-001 — no secret in observability

- Sentinel-value capture test: secret value never appears; key names may.

### NFR-SEC2-002 — `-race`-safe test isolation

- No `t.Setenv`; fake Resolver injection; `-race` + `t.Parallel()` clean.

### NFR-SEC2-003 — backend-selection determinism

- backend-string → Resolver-type mapping table asserted (incl. default + error).

### NFR-SEC2-004 — single source of truth

- Grep regression: no credentialed adapter resolves its key off-Resolver.

---

## 5. Technical Approach (high-level — full plan in plan.md)

### 5.1 Phasing

1. **CLI Resolver construction** — read `security.yaml` `secrets.backend`
   + `k8s_mount_path`; build one Resolver via `secretstore.NewResolver`;
   fail startup on unknown backend.
2. **Registry threading** — extend `buildProductionRegistry` to accept
   the Resolver; forward it to naver + github construction; non-credentialed
   adapters unchanged.
3. **naver refactor** — drop the `secretEnv` global; add an injected
   Resolver to `naver.Options`; resolve `NAVER_CLIENT_*` via it with the
   literal-Options-override and nil-fallback rules.
4. **github token via CLI** — replace the raw `os.Getenv` token site with
   Resolver lookups (`USEARCH_GITHUB_TOKEN` then `GITHUB_TOKEN`) feeding
   `github.New` Options.Token.
5. **vault/unknown error paths + no-leak hardening** — surface
   `ErrNotImplemented` as a loud startup error; audit all touched paths
   for secret-value leakage.

### 5.2 Files touched (proposed — refinement in plan.md)

```
cmd/usearch/query.go            [MODIFY] construct Resolver from security.yaml;
                                         thread into buildProductionRegistry;
                                         replace raw os.Getenv github-token site
                                         with Resolver lookups.
internal/adapters/naver/naver.go [MODIFY] remove package-global secretEnv;
                                         add Resolver to Options; resolve
                                         NAVER_CLIENT_* via injected Resolver
                                         (literal-override + nil-fallback).
internal/adapters/naver/naver_test.go [MODIFY/EXTEND] inject fake Resolver;
                                         assert override + fallback paths.
cmd/usearch/*_test.go            [NEW/MODIFY] Resolver-wiring + backend-selection
                                         + vault/unknown + backward-compat tests.
```

No `internal/security/secretstore/` changes — SPEC-SEC-002 is a pure
consumer of `NewResolver` / `Resolver` / `ErrNotImplemented`. The
`github` package is consumed unchanged.

### 5.3 Credential resolution logic (normative)

CLI startup (once): `resolver, err := secretstore.NewResolver(cfg.Secrets.Backend, cfg.Secrets.K8sMountPath)`; on `err != nil` → fail startup.

For each credentialed adapter in `buildProductionRegistry(resolver)`:
- **naver**: `naver.New(naver.Options{Resolver: resolver})`. Inside
  `New`: if `Options.ClientID != ""` use it; else `resolver.Get(ctx,
  "NAVER_CLIENT_ID")`; same for secret. On `errors.Is(err,
  ErrNotImplemented)` → propagate the vault error (loud). On other
  resolution error with env backend → existing skip semantics. On
  success → `reg.RegisterWithOptions(a, RegisterOptions{SkipAuthCheck:
  true})` (see registration step below).
- **github**: in the CLI, `tok, err := resolver.Get(ctx,
  "USEARCH_GITHUB_TOKEN")`; if empty, `tok, err = resolver.Get(ctx,
  "GITHUB_TOKEN")`. On `errors.Is(err, ErrNotImplemented)` → loud vault
  error. On empty/not-set with env backend → skip github (parity). Else
  `github.New(github.Options{Token: tok})` → register via
  `RegisterWithOptions(..., SkipAuthCheck: true)`.

**Registration step (closes audit D1, REQ-SEC2-007).** The registry's
auth gate `RegisterWithOptions` (`registry.go:147-157`) calls
`os.LookupEnv` on `AuthEnvVars` UNLESS `opts.SkipAuthCheck` is true
(`registry.go:151`). With a `k8s`/`vault` backend the secret is NOT in
process env, so a plain `Register` (which is `RegisterWithOptions(a,
RegisterOptions{})`, `registry.go:136-138`) would reject the adapter with
`ErrMissingAuth` even though the Resolver supplied the key. Therefore
credentialed adapters that the CLI has already credentialed via the
Resolver SHALL be registered with `RegisterOptions{SkipAuthCheck: true}`,
bypassing the env-only gate. The CLI — having the Resolver result in hand
— is the authority on whether the key is present; the registry gate is
redundant for these adapters and would be actively wrong on non-env
backends. `registry.go` is NOT modified. Non-credentialed adapters
continue to use plain `Register` (their gate is a no-op).

The vault `ErrNotImplemented` check is the one path that converts a
resolution error into a STARTUP failure rather than an adapter skip
(REQ-SEC2-004).

### 5.4 Test strategy

- **Resolver wiring**: build the registry with a fake `Resolver`
  (`Get` scripted per key); assert naver/github resolve through it.
- **Backend selection**: table-driven over `secretstore.NewResolver`
  inputs (`env`/`""`→`*EnvResolver`, `k8s`→`*K8sResolver`,
  `vault`→`*VaultResolver`, `"hsm"`→error) via type assertion.
- **vault path**: Resolver from `vault` backend → `Get` returns
  `ErrNotImplemented`; assert CLI build fails with `errors.Is` and a
  vault-naming message.
- **backward compat**: fake Resolver mimicking EnvResolver (returns
  values for present keys, error for absent) → assert identical
  register/skip outcomes.
- **no-leak**: fake Resolver returns a SENTINEL value; capture
  slog/span output during success and during forced failure; assert the
  sentinel value never appears (key names allowed).
- **`-race` + `t.Parallel()`**: no `t.Setenv`; all state via injection.

### 5.5 Coverage target

85% (matches SEC-001 and the adapter SPECs). The credential-resolution
branches (override / fallback / vault / unknown / skip) are the
highest-value lines to cover.

---

## 6. @MX Tag Targets

The following are the @MX annotation targets for the Run phase (per
`.claude/rules/moai/workflow/mx-tag-protocol.md`):

| Symbol | File | Tag | Rationale |
|--------|------|-----|-----------|
| `secretstore.Resolver` (interface) | `internal/security/secretstore/resolver.go` | `@MX:ANCHOR` (update) | The ANCHOR predicts `fan_in >= 3 once call-site refactors land`. ACTUAL accounting: BEFORE this SPEC the Resolver already has 2 live callers — `internal/adapters/naver/naver.go:23` and `internal/llm/config/config.go:45` (both hardcoded env-only). SEC-002 ADDS the github CLI token-resolution site (and converts naver's lookup to the injected, backend-aware Resolver), bringing fan_in to 3. UPDATE the ANCHOR's caller list to the actual 3 and change "predicted" → "realized (naver backend-aware + github CLI-site; llm/config still pending its own refactor — OQ-4)". The interface is NOT modified — only the annotation is refreshed. |
| `buildProductionRegistry` | `cmd/usearch/query.go` | `@MX:ANCHOR` (if fan_in ≥ 3) + `@MX:NOTE` | Now the single seam that constructs the Resolver, resolves each credentialed adapter's key, and registers credentialed adapters with `RegisterOptions{SkipAuthCheck: true}` (REQ-SEC2-007). Document that adapter credentials flow exclusively through the injected Resolver (single source of truth, F-07 fix intent) and that the registry's env-only auth gate is intentionally bypassed for credentialed adapters because the CLI already validated presence via the Resolver. |
| `naver.New` | `internal/adapters/naver/naver.go` | `@MX:ANCHOR` (update) + `@MX:WARN` + `@MX:REASON` | Already carries an ANCHOR (`naver.go:115-117`). UPDATE to note credentials now come from the injected Resolver. Add `@MX:WARN` on the credential-resolution branch: removing the literal-override precedence or the nil-fallback breaks existing tests / silently changes which backend supplies the secret. `@MX:REASON` SHALL note the no-leak contract. |
| github token-resolution site | `cmd/usearch/query.go` | `@MX:NOTE` | Document that the token is resolved via the injected Resolver (`USEARCH_GITHUB_TOKEN` then `GITHUB_TOKEN`), NOT raw os.Getenv — explains the F-07 fix and the alias-fallback order. |

All tags are `[AUTO]`-prefixed, include `@MX:SPEC: SPEC-SEC-002`, and
follow `code_comments: en`. Per-file limits (3 ANCHOR + 5 WARN) respected.

---

## 7. Dependencies

### 7.1 Upstream SPEC dependencies (depends_on)

- **SPEC-SEC-001** (implemented) — LOAD-BEARING. Owns
  `internal/security/secretstore/` (`Resolver`, `NewResolver`,
  `EnvResolver`/`K8sResolver`/`VaultResolver`, `ErrNotImplemented`,
  `Backend*` constants, `DefaultK8sMountPath`) and the `security.yaml`
  `secrets.backend` key (REQ-SEC-013/016). Consumed unchanged.
- **SPEC-CLI-002** (implemented) — owns the CLI startup + config
  plumbing into which the Resolver construction is wired.
- **SPEC-ADP-008** (implemented) — owns the naver adapter whose
  credential path is refactored; ADP-008 tests must keep passing.

### 7.2 Reused primitives (no change to these — pure consumer)

- `internal/security/secretstore/factory.go:22` `NewResolver(backend,
  mountPath)` — Resolver construction.
- `internal/security/secretstore/resolver.go:23-27` `Resolver.Get` —
  the resolution call.
- `internal/security/secretstore/resolver.go:11` /
  `vault.go:15-16` `ErrNotImplemented` — the vault stub sentinel.
- `internal/security/secretstore/env.go:16` `NewEnvResolver()` — the
  nil-Resolver fallback.
- `internal/adapters/github/github.go:46-48` `Options.Token` — the
  github token injection point (package unchanged).
- `internal/adapters/registry.go:136-138,147-157` `Register` /
  `RegisterWithOptions(a, RegisterOptions{SkipAuthCheck: bool})` — the
  registration entry point. SEC-002 calls it with `SkipAuthCheck: true`
  for credentialed adapters (REQ-SEC2-007). The `RegisterOptions` /
  `SkipAuthCheck` field already exists (`registry.go:55`); no signature
  change. The auth gate at `:151-153` and admin-view `os.LookupEnv` at
  `:266-274` are NOT modified.
- `cmd/usearch/query.go` `buildProductionRegistry` — the wiring seam.

### 7.3 External dependencies

- Zero new Go module dependencies. Uses stdlib `context`, `errors`,
  `os` and the existing `secretstore` package.

---

## 8. Risks (full register in research.md)

| # | Risk | Severity | Mitigation |
|---|------|----------|------------|
| R1 | Refactoring naver's credential path breaks existing ADP-008 tests | Medium | Preserve `Options.ClientID`/`Secret` literal-override precedence and the nil-Resolver→EnvResolver fallback; run ADP-008's suite as a regression gate. |
| R2 | Vault backend silently swallows `ErrNotImplemented` as a missing key | High | REQ-SEC2-004 + `TestVaultErrorNotTreatedAsMissingKey`: `errors.Is(err, ErrNotImplemented)` converts to a LOUD startup error distinct from the env skip path. |
| R3 | A resolved secret value leaks into a log/error during the refactor | High | NFR-SEC2-001 sentinel-capture test; errors reference NAMES + backend ids only; mirrors `SecretValue`-always-empty invariant. |
| R4 | `t.Setenv` reintroduced in new tests breaks `-race` | Medium | NFR-SEC2-002 forbids it; fake-Resolver injection; CI grep for `t.Setenv` in new files. |
| R5 | Changing `buildProductionRegistry` signature ripples to other callers | Medium | Both `buildProductionRegistry` and its callers are in package `main`; signature extension is contained. plan.md to confirm caller set. |
| R6 | Env-backend behavior subtly changes (different skip semantics) | High | REQ-SEC2-005 + `TestEnvBackendBackwardCompatible`/`TestEnvBackendMissingCredsSkipsAdapter` assert parity with pre-SPEC register/skip outcomes. |
| R7 | The `secrets/` vs `secretstore/` duplication causes the wrong package to be wired | Medium | naver already imports `secretstore`; SPEC-SEC-002 uses `secretstore` exclusively. OQ-1 flags the duplication for a separate cleanup; SPEC-SEC-002 does not touch `secrets/`. |
| R8 | github `GITHUB_TOKEN` fallback alias dropped during refactor | Low | REQ-SEC2-003 + `TestGithubTokenFallbackAlias` preserve the two-key resolution order through the Resolver. |
| R9 | Env-only registry auth gate (`registry.go:151-153`) rejects k8s/vault adapters even after Resolver supplies the key — defeating SEC-002's core goal on non-env backends | High | REQ-SEC2-007 + §2.1(i): credentialed adapters registered with `SkipAuthCheck: true`; CLI owns skip/loud-fail from the Resolver result. `TestK8sBackendAdapterRegisters` proves k8s registration succeeds. `registry.go` unchanged. |
| R10 | `SkipAuthCheck: true` accidentally applied to a non-credentialed adapter (or a credentialed one whose key was NOT actually resolved), silently registering an unusable adapter | Medium | The CLI applies `SkipAuthCheck` ONLY after a successful Resolver `Get`; if resolution fails it skips (env) or loud-fails (vault) BEFORE registering. `TestCredentialedAdapterRegisteredWithSkipAuthCheck` + `TestK8sBackendMissingSecretSkipsAdapter` cover both branches. |
| R11 | Admin-view (`registry.go:266-274`) still reports `key_set: false` for a registered k8s/vault adapter (display-only discrepancy) | Low | Out of scope (OQ-5); registration is fixed by REQ-SEC2-007, only the telemetry display lags. Flagged for a SPEC-UI-002 follow-on. |

---

## 9. Open Questions

These are explicitly UNRESOLVED at SPEC-draft time. Each has a
recommended default. They do NOT block SPEC approval.

1. **OQ-1 — `secrets/` vs `secretstore/` package duplication.** BOTH
   `internal/security/secrets/` (only `resolver.go` + `resolver_test.go`,
   git-tracked) AND `internal/security/secretstore/` (full backend set,
   git-tracked) exist. SEC-001 HISTORY (`spec.md:29-30`) claims a RENAME
   `secrets/`→`secretstore/` (v0.2.1) to dodge a repo-root
   credential-protection deny rule, yet `secrets/resolver.go` is still
   tracked. The `secrets/` directory is read-restricted by that deny rule,
   so its CURRENT contents could not be inspected via Read — whether it is
   a stale leftover or a live parallel package is unknown.
   **Recommended default**: SPEC-SEC-002 uses `secretstore` exclusively
   (what naver already imports) and does NOT touch `secrets/`. Resolving
   the duplication (delete the stale leftover vs reconcile) is a separate
   cleanup SPEC after confirming `secrets/` has no live importers.
   **Resolution owner**: a follow-on cleanup SPEC author.

2. **OQ-2 — koanf `[auth]` vs secret-backend unification.** F-07 notes
   `config.toml`/koanf does not feed adapter creds. Verified: it does not.
   Should the koanf `[auth]` block (SPEC-CLI-002 REQ-CLI2-007) be unified
   with the `secretstore` backend so a single config surface governs both?
   **Recommended default**: NO in this SPEC. SPEC-SEC-002 resolves via the
   `secretstore` backend only; koanf auth config is left as-is. Unifying
   the two config surfaces is a follow-on.
   **Resolution owner**: SPEC-CLI-002 / a future config-consolidation SPEC.

3. **OQ-3 — keychain backend.** F-07 wrongly implies a keychain backend
   exists. Should an OS-keychain backend be added?
   **Recommended default**: NOT in this SPEC. SPEC-SEC-002 wires the
   existing env/k8s/vault backends through; a keychain backend (new
   `secretstore` backend type) is a follow-on if dev-machine secret
   storage becomes a requirement.
   **Resolution owner**: a future secretstore-backend SPEC author.

4. **OQ-4 — llm/config Resolver refactor.** `internal/llm/config/
   config.go:14,45,58` is ALREADY a live `secretstore.NewEnvResolver()`
   caller carrying the SAME env-only hardcoded defect as naver (it ignores
   the configured backend when resolving `LITELLM_MASTER_KEY`). Should it
   be refactored in the same SPEC?
   **Recommended default**: NO; SPEC-SEC-002 scope is ADAPTERS. The
   llm/config site is a separate (non-adapter) call-site refactor with the
   identical fix shape (inject the configured Resolver instead of a
   hardcoded `NewEnvResolver()`).
   **Resolution owner**: a future SEC follow-on author.

5. **OQ-5 — admin-view key-presence check uses raw env.** The registry
   admin-view (`registry.go:266-274`, `SnapshotForAdmin`) computes the
   per-adapter `KeySet` boolean via `os.LookupEnv(AuthEnvVars)` directly,
   NOT via the Resolver. On a `k8s`/`vault` backend the admin API would
   therefore report `key_set: false` for a credentialed adapter even when
   the secret IS present in the mount path / Vault and the adapter
   registered successfully. SEC-002 fixes the registration path but leaves
   this DISPLAY discrepancy.
   **Recommended default**: OUT of SEC-002 scope. `SnapshotForAdmin` is
   SPEC-UI-002 telemetry; making its key-presence check Resolver-aware
   requires passing a Resolver into the registry (a SEC-001/UI-002 change)
   and is a follow-on. SEC-002 does not modify `registry.go`.
   **Resolution owner**: a future SPEC-UI-002 / SEC follow-on author.

---

## 10. References

### Internal (code — exact citations)

- `internal/security/secretstore/resolver.go:17-22` — `Resolver`
  `@MX:ANCHOR` "Runtime secret resolution boundary"; predicts
  `fan_in >= 3 once call-site refactors land`; names `adapters/naver`
  as a caller. SPEC-SEC-002 realizes this.
- `internal/security/secretstore/resolver.go:11` /
  `internal/security/secretstore/vault.go:14-17` — `ErrNotImplemented`
  (vault stub sentinel; `Get` always returns it).
- `internal/security/secretstore/factory.go:22-36` — `NewResolver(backend,
  mountPath)`; `BackendEnv`/`BackendK8s`/`BackendVault`; unknown backend
  config error; `DefaultK8sMountPath = "/var/run/secrets"`.
- `internal/security/secretstore/env.go:16,21-27` — `NewEnvResolver`,
  os.Getenv semantics (unset → empty + error).
- `internal/security/secretstore/k8s.go:21-49` — `NewK8sResolver`,
  mounted-file-per-key resolution.
- `cmd/usearch/query.go` `buildProductionRegistry` — constructs NO
  Resolver today; reads `USEARCH_GITHUB_TOKEN`/`GITHUB_TOKEN` raw; lets
  naver self-resolve. (grep of `cmd/usearch/` for
  `secretstore`/`NewResolver` = ZERO matches.)
- `internal/adapters/naver/naver.go:23` — package-global
  `var secretEnv = secretstore.NewEnvResolver()` (env-only; to remove).
- `internal/adapters/naver/naver.go:118-133` — `New` resolves
  `NAVER_CLIENT_ID`/`SECRET` via `secretEnv`; `Options.ClientID`/`Secret`
  override path.
- `internal/adapters/naver/naver.go:196-197` — `RequiresAuth: true`,
  `AuthEnvVars: [NAVER_CLIENT_ID, NAVER_CLIENT_SECRET]`.
- `internal/adapters/github/github.go:46-48` — `Options.Token` injection.
- `internal/adapters/github/github.go:146-147` — `RequiresAuth: true`,
  `AuthEnvVars: [USEARCH_GITHUB_TOKEN]`.
- `internal/adapters/koreanews/koreanews.go:91` — `AuthEnvVars: nil`
  (NOT a credential adapter).
- `internal/adapters/koreanews/options.go:124-157` — `OptionsFromEnv`
  reads `USEARCH_ADP009_*` FLAGS / URLs (not credentials).
- `internal/adapters/registry.go:136-138` — `Register` =
  `RegisterWithOptions(a, RegisterOptions{})`.
- `internal/adapters/registry.go:147-157` — `RegisterWithOptions`; auth
  gate at `:151-153` reads `os.LookupEnv(AuthEnvVars)` unless
  `opts.SkipAuthCheck` (`:151`). This is the env-only gate SEC-002
  bypasses via `SkipAuthCheck: true` (REQ-SEC2-007). `RegisterOptions.
  SkipAuthCheck` field at `:55`.
- `internal/adapters/registry.go:266-274` — admin-view `SnapshotForAdmin`
  computes `KeySet` via `os.LookupEnv(AuthEnvVars)` directly (NOT via the
  Resolver) — the display discrepancy on k8s/vault (OQ-5; out of scope).
- `internal/adapters/registry.go:234-236` — `SecretValue` is ALWAYS empty
  (no-leak invariant; test-only field).
- `internal/llm/config/config.go:14,45,58` — ALREADY a live
  `secretstore.NewEnvResolver()` caller (`var secretEnv = …`, resolves
  `LITELLM_MASTER_KEY`); same env-only defect as naver; OQ-4 follow-on.
- `.moai/config/sections/security.yaml:17-25` — `secrets.backend`
  (env|k8s|vault, default env) + `k8s_mount_path: /var/run/secrets`;
  comment names `internal/security/{secrets,...}` (the package is actually
  `secretstore`).

### Internal (SPEC + project docs)

- `.moai/specs/SPEC-SEC-001/spec.md` — owns `secretstore`; REQ-SEC-016
  Resolver ANCHOR; `spec.md:29-30` renames `secrets/`→`secretstore/`.
- `.moai/specs/SPEC-CLI-002/spec.md` — CLI surface + koanf `[auth]`
  config (REQ-CLI2-007).
- `.moai/specs/SPEC-ADP-008/spec.md` — naver adapter being refactored.
- `.planning/AUDIT-FINDINGS.md:30` — F-07 finding text (with the two
  overstatements corrected here); `:15` notes F-07 is the only finding
  without a SPEC as of 2026-06-04.

---

*End of SPEC-SEC-002 v0.1 (draft).*
