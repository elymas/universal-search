# SPEC-SEC-002 Research — Adapter Credential Resolution via Configured Secret Backend

Companion research artifact for `spec.md`. Resolves GSD audit finding
**F-07**. Every code claim below was verified file:line against current
source on 2026-06-04 before being written. This repo has a documented
history of SPECs citing nonexistent paths/types
(`.moai/.../spec-stale-code-assumptions`), so each citation here was
opened and read, not assumed.

---

## 1. The finding (verbatim) and its two overstatements

`.planning/AUDIT-FINDINGS.md:30`:

> F-07 | Adapter auth keys bypass the `secretstore` factory (naver
> hardcodes `NewEnvResolver()`; github/koreanews use raw `os.Getenv`).
> keychain/vault/k8s backends unreachable from CLI; vault is a stub.
> | medium | manual | blocks secure key backends;
> config.toml/koanf does not feed adapter creds. **No SPEC authored
> yet.**

`AUDIT-FINDINGS.md:15` confirms F-07 is the only finding without a SPEC
as of 2026-06-04.

### Overstatement 1 — there is NO keychain backend

The finding lists "keychain/vault/k8s backends". The actual backends
under `internal/security/secretstore/` are:

- `env.go` — `EnvResolver` (os.Getenv).
- `k8s.go` — `K8sResolver` (mounted file per key).
- `vault.go` — `VaultResolver` (STUB; `Get` → `ErrNotImplemented`).

There is **no keychain.go and no keychain backend constant.** The
factory (`factory.go:6-10`) defines exactly `BackendEnv`, `BackendK8s`,
`BackendVault`. Adding a keychain backend is a possible follow-on, not
part of F-07's actual surface. The SPEC corrects this (§2.2, OQ-3).

### Overstatement 2 — koreanews has no credentials

The finding groups "github/koreanews use raw `os.Getenv`" as if both
bypass credential resolution. koreanews resolves NO credentials:

- `internal/adapters/koreanews/koreanews.go:91` — `AuthEnvVars: nil`
  (and it does not set `RequiresAuth: true`).
- `internal/adapters/koreanews/options.go:124-157` `OptionsFromEnv`
  reads `USEARCH_ADP009_RSS_ENABLED`, `USEARCH_ADP009_RSS_FEEDS`,
  `USEARCH_ADP009_DAUM_ENABLED`, `USEARCH_ADP009_KNC_ENABLED`,
  `USEARCH_ADP009_KNC_BASE_URL` — these are feature FLAGS and URLs, not
  secrets.

So koreanews's `os.Getenv` usage is correct (it is reading config flags,
which have no business going through a secret resolver). The SPEC narrows
F-07 to the genuine credential sites.

---

## 2. The genuine credential sites (verified) — exactly TWO

A grep of every adapter's Capabilities for `RequiresAuth: true` +
real credential `AuthEnvVars`:

| Adapter | RequiresAuth | AuthEnvVars | Source |
|---------|--------------|-------------|--------|
| naver | true | `NAVER_CLIENT_ID`, `NAVER_CLIENT_SECRET` | `naver.go:196-197` |
| github | true | `USEARCH_GITHUB_TOKEN` | `github.go:146-147` |
| reddit | (false) | nil | `reddit.go:109` |
| hn | (false) | nil | `hn.go:109` |
| arxiv | (false) | nil | `arxiv.go:121` |
| youtube | (false) | nil | `youtube.go:104` |
| searxng | (false) | nil | `searxng.go:149` |
| social (bluesky/x) | (false) | nil | `social.go:152,172` |
| koreanews | (false) | nil | `koreanews.go:91` |

Only **naver** and **github** carry real credentials. SPEC-SEC-002's
refactor is therefore narrow: two adapter credential paths, one CLI
Resolver.

---

## 3. The infrastructure SEC-001 already built (verified)

### 3.1 `Resolver` interface — the anchor that predicted this SPEC

`internal/security/secretstore/resolver.go`:

```go
// @MX:ANCHOR: [AUTO] Runtime secret resolution boundary; callers: llm/config,
// adapters/naver, and future K8s/Vault-deployed secret sites.
// @MX:REASON: fan_in >= 3 once call-site refactors land; this is the single
// seam between deployment-specific secret backends and consuming code — a
// regression here either leaks a secret or breaks every credentialed adapter.
// @MX:SPEC: SPEC-SEC-001 (REQ-SEC-016)
type Resolver interface {
    Get(ctx context.Context, key string) (string, error)
}
```

The anchor names `llm/config` and `adapters/naver` and predicts
`fan_in >= 3 once call-site refactors land`. **Actual fan_in accounting
(corrected per plan-audit D2):** the ANCHOR is ALREADY at fan_in 2 BEFORE
SEC-002 — `internal/adapters/naver/naver.go:23` AND
`internal/llm/config/config.go:45` are BOTH live `secretstore` callers
today (each hardcodes `NewEnvResolver()`, ignoring the configured
backend). So "predicted" is slightly inaccurate: two of the three callers
already exist. SEC-002 ADDS the github CLI token-resolution site (and
converts naver's lookup from the hardcoded global to the injected,
backend-aware Resolver), bringing fan_in to 3. llm/config is NOT in
SEC-002 scope (adapters only) — it carries the identical env-only defect
and is a separate follow-on (spec.md OQ-4). The godoc also states
implementations "MUST NOT log, print, or otherwise surface the resolved
value" — the no-leak contract SPEC-SEC-002 must honor (REQ-SEC2-006 /
NFR-SEC2-001).

### 3.2 `NewResolver` factory

`internal/security/secretstore/factory.go`:

```go
const (
    BackendEnv   = "env"
    BackendK8s   = "k8s"
    BackendVault = "vault"
)
const DefaultK8sMountPath = "/var/run/secrets"

func NewResolver(backend, mountPath string) (Resolver, error) {
    switch backend {
    case BackendEnv, "":
        return NewEnvResolver(), nil
    case BackendK8s:
        if mountPath == "" { mountPath = DefaultK8sMountPath }
        return NewK8sResolver(mountPath), nil
    case BackendVault:
        return NewVaultResolver(), nil
    default:
        return nil, fmt.Errorf("secretstore: unknown backend %q (want env|k8s|vault)", backend)
    }
}
```

- `env` and `""` → `*EnvResolver` (the backward-compat default).
- `vault` → constructs successfully (so config validation passes) but
  `Get` always fails — see 3.3.
- unknown → config error (REQ-SEC2-004 second clause).

### 3.3 Vault is a stub

`internal/security/secretstore/vault.go`:

```go
func (r *VaultResolver) Get(_ context.Context, _ string) (string, error) {
    return "", ErrNotImplemented
}
```

`ErrNotImplemented` is defined in `resolver.go:11`. The stub's godoc says
the real client is post-V1. SPEC-SEC-002 keeps it a stub but makes the
CLI surface `ErrNotImplemented` as a LOUD startup error
(REQ-SEC2-004), not a silent missing-key skip.

### 3.4 Env backend has os.Getenv semantics (backward-compat key)

`internal/security/secretstore/env.go:21-27`: `Get` returns
`os.LookupEnv(key)`; unset/empty → empty + error. This is identical to
the direct `os.Getenv` semantics the call sites use today, which is what
makes REQ-SEC2-005 (backward compat) achievable: with `backend: env`,
nothing observable changes.

### 3.5 security.yaml selection — currently DEAD for adapters

`.moai/config/sections/security.yaml:17-25`:

```yaml
secrets:
    backend: env          # env | k8s | vault
    k8s_mount_path: /var/run/secrets
```

The comment (`:14`) says this is "Consumed by
`internal/security/{secrets,ratelimit,ssrf}`". But for ADAPTER
credentials it is consumed by NOTHING — the CLI never calls
`NewResolver` (see §4). Selecting `backend: k8s` has zero effect on
whether naver or github find their keys today.

---

## 4. The gap (verified): CLI never wires a Resolver

`grep secretstore|NewResolver|NewEnvResolver` in `cmd/usearch/` →
**NO MATCHES.** The CLI does not construct a Resolver and does not read
`secrets.backend`.

### 4.1 naver self-resolves with a hardcoded env resolver

`internal/adapters/naver/naver.go:23`:

```go
var secretEnv secretstore.Resolver = secretstore.NewEnvResolver()
```

`naver.go:118-133` (`New`): resolves `NAVER_CLIENT_ID` /
`NAVER_CLIENT_SECRET` through this package-global `secretEnv` (env-only),
unless `Options.ClientID` / `Options.ClientSecret` are provided. The
configured backend is ignored.

### 4.2 github token read raw by the CLI

`internal/adapters/github/github.go:46-48`: the adapter takes the token
via `Options.Token` (it does NOT self-resolve — clean injection point).
The production wiring at `cmd/usearch/query.go` `buildProductionRegistry`
reads it raw:

```go
token := os.Getenv("USEARCH_GITHUB_TOKEN")
if token == "" { token = os.Getenv("GITHUB_TOKEN") }
if token != "" { github.New(github.Options{Token: token, ...}) }
```

No Resolver involved. The backend selection is bypassed.

### 4.3 The wiring seam

`cmd/usearch/query.go` `buildProductionRegistry` constructs the registry
and registers each adapter (skipping credentialed ones whose keys are
absent). It carries `@MX:NOTE` "Production adapter wiring per
SPEC-CLI-001". This is the function SPEC-SEC-002 extends to accept the
Resolver. naver is constructed via `naver.New(naver.Options{})` (relies
on the global resolver); github via the raw-os.Getenv block above.

### 4.4 The registration gate ALSO reads env directly (audit D1 — critical)

Wiring a Resolver into the adapter constructors is necessary but NOT
sufficient. The registry's registration gate reads process env directly:

`internal/adapters/registry.go:147-157` `RegisterWithOptions`:

```go
if !opts.SkipAuthCheck && caps.RequiresAuth {
    for _, ev := range caps.AuthEnvVars {
        if _, ok := os.LookupEnv(ev); !ok {
            return &RegistryError{Op: "register", Name: name, Cause: ErrMissingAuth}
        }
    }
}
```

`Register` (`registry.go:136-138`) is `RegisterWithOptions(a,
RegisterOptions{})` — i.e. `SkipAuthCheck: false`. So on a `k8s`/`vault`
backend, where the secret is in a mount file / Vault and NOT in process
env, a credentialed adapter would be REJECTED with `ErrMissingAuth` even
after the Resolver successfully supplied its key. This moves F-07's
"backend unreachable from adapters" defect to the registration step
rather than fixing it — the Resolver would not be the SOLE credential
source (the env gate is a second, env-only source).

The SAME `os.LookupEnv` pattern appears in the admin-view at
`registry.go:266-274` (`SnapshotForAdmin` computes `KeySet`), so the
admin API would also report `key_set: false` for a k8s/vault adapter that
is actually present.

**Fix (spec.md REQ-SEC2-007, §2.1(i)):** the CLI resolves each
credentialed adapter's key via the Resolver FIRST, then registers it with
`RegisterOptions{SkipAuthCheck: true}` — the `SkipAuthCheck` field already
exists (`registry.go:55`), so the gate is BYPASSED for credentialed
adapters without modifying `registry.go`. The CLI (holding the Resolver
result) is the authority on key presence; it decides skip (env, absent) /
register (resolved) / loud-fail (vault). A k8s-backend registration test
(`TestK8sBackendAdapterRegisters`) proves naver/github register with the
secret in the mount path and process env unset. The admin-view display
discrepancy (`:266-274`) is a separate SPEC-UI-002 telemetry concern,
left as spec.md OQ-5 (out of SEC-002 scope).

---

## 5. The package duplication (OQ-1, unresolved)

`git ls-files` shows BOTH directories tracked:

```
internal/security/secrets/resolver.go
internal/security/secrets/resolver_test.go
internal/security/secretstore/doc.go
internal/security/secretstore/env.go
internal/security/secretstore/factory.go
internal/security/secretstore/k8s.go
internal/security/secretstore/resolver.go
internal/security/secretstore/resolver_test.go
internal/security/secretstore/vault.go
```

`secrets/` has ONLY `resolver.go` + `resolver_test.go`; `secretstore/`
has the full backend set. SEC-001's HISTORY (`spec.md:29-30`) states it
RENAMED `secrets/`→`secretstore/` (v0.2.1) "to avoid repo-root
credential-protection deny rule", yet `secrets/resolver.go` is still
git-tracked.

**Could not Read `secrets/resolver.go`** — the directory is
read-restricted by the same credential-protection deny rule that
triggered the rename, so its current contents are unverifiable here.
Whether `secrets/` is a stale leftover (rename incomplete) or a live
parallel package is UNKNOWN.

What IS verifiable: naver imports `secretstore` (`naver.go:15`), and the
full factory/backends live in `secretstore`. SPEC-SEC-002 therefore
treats `secretstore` as canonical and does NOT touch `secrets/`. The
duplication is flagged as OQ-1 for a separate cleanup SPEC (after
confirming `secrets/` has no live importers — which itself requires
lifting the read restriction or a grep of importers).

---

## 6. config.toml / koanf does not feed adapter creds (verified claim)

F-07's tail says "config.toml/koanf does not feed adapter creds".
Verified: adapter credentials flow through the env / Resolver path, not
through the koanf `[auth]` config block (SPEC-CLI-002 REQ-CLI2-007).
Unifying koanf auth config with the secret backend is a possible
follow-on (OQ-2); SPEC-SEC-002 resolves via the `secretstore` backend
only and leaves koanf config untouched.

---

## 7. Design decisions baked into the SPEC

- **D1 — one Resolver per process, injected.** Build once in the CLI from
  `security.yaml`; thread into `buildProductionRegistry` and the two
  credentialed constructors. This is exactly the "single seam" the
  Resolver anchor describes.
- **D2 — naver takes the Resolver via Options; literal override wins;
  nil → EnvResolver fallback.** Preserves ADP-008's test ergonomics
  (literal `Options.ClientID`/`Secret`) and standalone construction.
- **D3 — github unchanged; only the CLI's token site changes.** The
  github package already injects via `Options.Token`; moving the lookup
  from raw os.Getenv to the Resolver is a CLI-only change. Preserve the
  `USEARCH_GITHUB_TOKEN`→`GITHUB_TOKEN` alias order, both through the
  Resolver.
- **D4 — vault surfaces loudly.** `errors.Is(err, ErrNotImplemented)`
  converts to a startup failure naming "vault", distinct from the env
  skip path. Real Vault client stays out of scope.
- **D5 — env backend is the no-op default.** With `backend: env`,
  `EnvResolver` reproduces today's os.Getenv semantics → zero observable
  change → backward compatible.
- **D6 — no `t.Setenv`.** Inject a fake Resolver; tests are `-race`-safe
  and parallel (same discipline as SPEC-ADP-006 H1).
- **D7 — no secret value ever logged.** Errors name env-vars and
  backends only; mirrors `SecretValue`-always-empty and the Resolver
  godoc no-leak contract.
- **D8 — registration gate bypassed via `SkipAuthCheck: true`, not
  rewritten (added per plan-audit D1).** The registry auth gate
  (`registry.go:151-153`) and admin-view (`:266-274`) read env directly;
  on k8s/vault that would reject a Resolver-credentialed adapter. The CLI
  resolves keys via the Resolver FIRST and registers credentialed adapters
  with `RegisterOptions{SkipAuthCheck: true}` (the field already exists,
  `:55`), making the CLI the credential-presence authority and the
  Resolver the genuine SOLE source on ALL backends. `registry.go` is NOT
  modified (the rejected alternative — passing a Resolver into
  `RegisterWithOptions` — is a larger SEC-001/UI-002 change). spec.md
  REQ-SEC2-007.
  (NOTE: these research D-numbers are this doc's design-decision IDs; they
  are distinct from the plan-audit defect IDs D1-D4. The audit's D1 is
  addressed by research-D8 here + spec.md REQ-SEC2-007.)

---

## 8. Risks

See `spec.md` §8 for the full register (R1-R11). Highest-severity:

- R9 (env-only registry gate rejects k8s/vault adapters even after the
  Resolver supplies the key — the audit-D1 design gap) → REQ-SEC2-007
  `SkipAuthCheck: true` registration + `TestK8sBackendAdapterRegisters`.
- R2 (vault swallows error as missing key) → REQ-SEC2-004 loud-fail.
- R3 (secret value leaks during refactor) → NFR-SEC2-001 sentinel test.
- R6 (env-backend behavior drifts) → REQ-SEC2-005 parity tests.

---

## 9. Open Questions

See `spec.md` §9. Summary: OQ-1 (`secrets/` vs `secretstore/`
duplication — unverifiable, flagged), OQ-2 (koanf `[auth]` unification —
deferred), OQ-3 (keychain backend — out of scope), OQ-4 (llm/config
Resolver refactor — separate non-adapter site; ALREADY a live env-only
caller), OQ-5 (admin-view `registry.go:266-274` key-presence still
env-only — SPEC-UI-002 telemetry follow-on, out of SEC-002 scope).

---

## 10. References

All file:line citations in `spec.md` §10 were verified by direct Read /
Grep on 2026-06-04 (and re-verified for the r1 plan-audit fixes). Key
infra: `internal/security/secretstore/{resolver,factory,env,k8s,vault}.go`.
Key gap: `cmd/usearch/query.go` `buildProductionRegistry`,
`internal/adapters/naver/naver.go:23`,
`internal/adapters/github/github.go:46-48`. Audit-D1 evidence:
`internal/adapters/registry.go:147-157` (auth gate), `:266-274`
(admin-view), `:55` (`SkipAuthCheck` field). Audit-D2 evidence:
`internal/llm/config/config.go:14,45,58` (already a live env-only
secretstore caller). Finding: `.planning/AUDIT-FINDINGS.md:30`.

---

*End of SPEC-SEC-002 research.md.*
