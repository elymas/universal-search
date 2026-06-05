---
id: SPEC-CLI-003
title: usearch sources — Live Health Status & Registry-Backed Listing
version: 0.1.0
milestone: M7 — Surfaces
status: draft
priority: P1
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-06-04
updated: 2026-06-04
author: limbowl
issue_number: null
labels: [cli, sources, health, M7]
depends_on: [SPEC-CLI-002, SPEC-UI-002, SPEC-EVAL-002]
blocks: []
---

# SPEC-CLI-003: `usearch sources` — Live Health Status & Registry-Backed Listing

## HISTORY

- 2026-06-04 (initial draft v0.1, limbowl via manager-spec):
  Companion artifact: `research.md` (this same directory). Resolves
  GSD codebase-audit finding **F-06**. The user-facing symptom is
  documented verbatim in `USAGE.md:~261` (the "현재 상태" note: `sources
  status` shows every source as `unknown`, and `sources list`'s static
  list omits the registered `bluesky` adapter). F-06 is the GSD audit's
  assigned finding ID for this gap; `.planning/codebase/CONCERNS.md` is
  the audit's concerns register. SPEC-CLI-002 REQ-CLI2-009 shipped `usearch
  sources list` but explicitly deferred `sources status` and `sources
  show` as "gated on SPEC-EVAL-002 adapter health endpoint per §6
  _TBD_". SPEC-CLI-003 is the **resolution of that deferral**: it
  implements real `sources status` health checking AND converts
  `sources list` / `show` from a hardcoded static slice to a
  registry-backed listing reflecting only truly-registered adapters.

  F-06 has three concrete defects, each grounded in shipped code:
  1. `cmd/usearch/sources_cmd.go:71-86` (`newSourcesStatusCmd`) is a
     stub that always prints `"Source health check not yet
     implemented."` and lists every adapter as the literal string
     `unknown`. No health check runs.
  2. `cmd/usearch/sources_cmd.go:16-25` defines a package-level
     `knownAdapters` static slice that diverges from the real
     registry built at `cmd/usearch/query.go:458`
     (`buildProductionRegistry`). Consequences: `bluesky` is
     registered in production (`query.go:499-503`) but MISSING from
     the static list; `github`/`youtube`/`naver` appear in the static
     list even when env-gated OFF and never registered.
  3. The shipped `list` output uses a `NAME\tCATEGORY\tDESCRIPTION`
     column shape (`sources_cmd.go:58`), which **diverges from what
     SPEC-CLI-002 REQ-CLI2-009 specified** (`<name>\t<category>\t<lang>\t<auth_required:y/n>`).
     SPEC-CLI-003 reconciles to the REQ-CLI2-009 spec shape (see §2.4).

  This SPEC is INTENTIONALLY ADDITIVE to the CLI surface: it does not
  change `query`, `deep`, `history`, `config`, or `login`. It rewires
  the existing `sources` command tree (owned by SPEC-CLI-002) to the
  real registry and implements the deferred health surface.

  6 EARS REQs (1 × P0 + 4 × P1 + 1 × P2) + 4 NFRs. Four EARS patterns
  used (Ubiquitous + Event-Driven + State-Driven + Unwanted). Status
  `draft` pending plan-auditor + annotation cycle.

- 2026-06-04 (revision r1 — plan-audit fixes, limbowl via manager-spec):
  Resolved plan-audit `SPEC-CLI-003-review-1.md` (PASS-WITH-FIXES, 0.74).
  All `file:line` citations had been verified accurate; the fixes correct
  **semantic claims** that contradicted the real types/signatures, plus
  schema/completeness gaps. Re-verified each against source before
  writing. Changes by defect ID:
  - **D1 (CRITICAL):** `Capabilities` (`pkg/types/capabilities.go:38-62`)
    has NO scalar `Category`/`Lang` — only `DocTypes []DocType` and
    `SupportedLangs []string`; the adapter interface
    (`pkg/types/adapter.go:28-45`) has no `Category()` accessor. Defined
    the precise `DocTypes`→`category` mapping and `SupportedLangs`→`lang`
    folding rules in §2.5 and made them an explicit NFR-CLI3-004 exception
    (the mapping table is the ONE permitted derived constant). Removed
    every false "from `Capabilities()` (category, language)" claim.
  - **D2 (MAJOR):** no public getter for the private `r.disabled` map;
    §2.2 forbids registry changes. The `disabled` set is now obtained via
    the existing public `Registry.SnapshotForAdmin()` (`registry.go:251`,
    whose `Status` field already encodes `disabled`), pre-fetched once
    before the live probe — no registry-layer change needed.
  - **D3 (MAJOR):** `Registry.Resync` only ever sets `connected`/`disabled`
    and early-returns `(nil, *UpstreamError)` on Healthcheck failure
    (`registry.go:400-402`), so it cannot alone yield the 4-state result.
    REQ-CLI3-002 now specifies calling `adapter.Healthcheck(ctx)`
    **directly** per adapter and mapping the result; `Resync` is dropped
    as the recommended primitive.
  - **D4 (MINOR):** documented that `not-configured` is reproducible only
    via `SkipAuthCheck`-registered fakes or a registered-then-key-lost
    scenario (production registration enforces auth at
    `registry.go:151-152`).
  - **D5 (MAJOR):** `--format` is a per-command local flag
    (`query.go:301`, `root.go:181`), absent from `sources`; reframed as a
    NEW deliverable — register `--format`+`--timeout` on the `sources`
    subcommands and centralize a shared validation helper with the
    canonical message.
  - **D6 (MINOR):** pinned `schema_version` to the STRING `"1"` (matching
    `output_json.go:23,37`, not `repl.go:216`'s int).
  - **D8 (MAJOR/MP-3):** added `labels`. Kept `created`/`updated` (NOT
    `created_at`): all 51 SPECs in this repo and the direct sibling
    CLI-002 use `created`/`updated` with no `created_at`; renaming would
    break house consistency. The auditor noted this is a project-wide
    convention (review L33-35). Honors the user instruction "match the
    exact key set used by passing SPECs."
  - **D9 (MAJOR):** added `SPEC-UI-002` to `depends_on` — it owns the
    actually-used primitives (`Resync`/`SnapshotForAdmin`, REQ-AS-002).
    EVAL-002 retained as a parity reference dependency.
  - **D10 (MAJOR):** created the promised `acceptance.md`.
  - **D11/D12/D13 (MINOR):** added the Healthcheck ctx-compliance
    precondition to NFR-CLI3-001; specified the non-cancelling collection
    pattern (`errgroup` WITHOUT `WithContext`, or `sync.WaitGroup`) so one
    failure does not cancel siblings; defined `recover()` per probe and
    `--timeout <= 0` rejection.

---

## 1. Purpose

SPEC-CLI-002 grew the `usearch` CLI into the M7 v1 surface, including a
`sources` subcommand tree (`list`, `status`, `show`). At ship time, only
`list` was wired — and even that was wired to a **hardcoded static
slice** rather than the live adapter registry. `status` was left a stub,
explicitly deferred to "the SPEC-EVAL-002 adapter health endpoint"
(REQ-CLI2-009, §6 _TBD_). The registry primitives this CLI actually
needs are now available: SPEC-UI-002 shipped `Registry.SnapshotForAdmin`
(the public read path for the `disabled` set + key-set) and the live
`Adapter.Healthcheck` probe is part of the core adapter contract;
SPEC-EVAL-002 separately added telemetry-only `Registry.HealthSnapshot`
+ `/api/admin/adapters/health` (which this CLI mirrors but does not call,
see below).

SPEC-CLI-003 closes finding **F-06** by making the `sources` surface
tell the truth about what is actually registered and reachable:

- **`usearch sources status`** — currently a stub. SPEC-CLI-003 makes
  it run each registered adapter's live `Healthcheck(ctx)` concurrently
  with a per-adapter timeout, and classify each adapter into one of
  `connected` / `unhealthy` / `disabled` / `not-configured`, plus a
  key-configured flag derived from `Capabilities().AuthEnvVars`. This
  achieves CLI parity with the admin HTTP endpoint
  `/api/admin/adapters/health`.
- **`usearch sources list` / `show`** — currently backed by a static
  `knownAdapters` slice. SPEC-CLI-003 rebinds them to the actual
  registry (`buildProductionRegistry` + `Registry.List`/`Get`) so the
  output reflects only adapters that are truly registered in the
  current environment, and reconciles the column shape to the
  REQ-CLI2-009 spec (`<name>\t<category>\t<lang>\t<auth_required:y/n>`).
  The `category` and `lang` columns are DERIVED from `Capabilities()`
  via the explicit mapping defined in §2.5 (`Capabilities.DocTypes` →
  category; `Capabilities.SupportedLangs` → lang) — there is no scalar
  `Category`/`Lang` field on `Capabilities` to read directly.

The animating principle is **single source of truth**: `sources`
commands MUST build the SAME registry as the `query` path. Today they
do not — `query` calls `buildProductionRegistry`, while `sources`
reads a string slice maintained by hand. That divergence is the root
cause of F-06.

### Why `HealthSnapshot` alone is insufficient (key design constraint)

`Registry.HealthSnapshot()` (`internal/adapters/registry.go:363`)
derives health from **in-process call telemetry** (success/fail
counters). For a short-lived CLI process that has made zero adapter
calls, every counter is empty, so `classifyHealth`
(`registry.go:340-353`) returns `healthy` for all adapters by the
"absence of evidence is not evidence of failure" rule
(`registry.go:341-343`). That is correct for a long-running server with
live traffic, but **meaningless for a one-shot CLI invocation** — it
would report every adapter `healthy` without ever touching the network.

Therefore `sources status` MUST run each adapter's live probe
`Adapter.Healthcheck(ctx)` (`pkg/types/adapter.go:38-40`) **directly**.

Note on `Registry.Resync` (`registry.go:394-432`): it does run
`Healthcheck`, but it is NOT sufficient as the sole classification
primitive — (1) its returned `AdapterAdminView.Status` is only ever
`connected` or `disabled` (`registry.go:420-423`), never `unhealthy`/
`not-configured`; and (2) on a Healthcheck failure it early-returns
`(nil, *UpstreamError)` (`registry.go:400-402`) rather than a view, and
does not distinguish timeout from generic failure. SPEC-CLI-003
therefore calls `Healthcheck(ctx)` directly per adapter and maps the
result (nil → `connected`; error/deadline → `unhealthy`) in the CLI,
and obtains the `disabled` set separately (see §2.6). `Resync` is not
used.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | Implement real `usearch sources status` — build the production registry, run each registered adapter's `Adapter.Healthcheck(ctx)` (`pkg/types/adapter.go:38-40`) **directly** and concurrently (via `golang.org/x/sync/errgroup` WITHOUT `WithContext`, or `sync.WaitGroup` — see §2.7 for why `WithContext` is wrong here), each bounded by a per-adapter `context.WithTimeout`, and classify each into one of `connected` / `unhealthy` / `disabled` / `not-configured`. Replace the stub at `cmd/usearch/sources_cmd.go:71-86`. |
| b | Surface key-configured state per adapter — derive whether the adapter's required env vars are set from `Capabilities().RequiresAuth` + `Capabilities().AuthEnvVars` (the same computation `SnapshotForAdmin` uses at `registry.go:266-277`). Report it as a `key_set` field (human column `KEYS`). |
| c | Convert `usearch sources list` to be registry-backed — replace the static `knownAdapters` slice (`sources_cmd.go:16-25`) with enumeration over the real registry (`Registry.List` + `Registry.Get`). `list` reflects ONLY adapters registered in the current environment. |
| d | Reconcile the `list` output shape to SPEC-CLI-002 REQ-CLI2-009 — emit `<name>\t<category>\t<lang>\t<auth_required:y/n>` (human/text), an array of objects with the same fields (json), or a Markdown table (markdown), replacing the shipped `NAME\tCATEGORY\tDESCRIPTION` shape (`sources_cmd.go:58`). The `category` and `lang` values are DERIVED from `Capabilities().DocTypes` and `Capabilities().SupportedLangs` per the mapping in §2.5 (there is no scalar `Capabilities.Category`/`Capabilities.Lang`). |
| e | Convert `usearch sources show <name>` to be registry-backed and consistent with `list` — look the adapter up via `Registry.Get`; print full capabilities from `Capabilities()`: `DisplayName`, derived category (§2.5), `SupportedLangs` (verbatim slice), `DocTypes`, `RequiresAuth`, `AuthEnvVars` (names only), `RateLimitPerMin`, and the derived `key_set` boolean. Unknown / unregistered adapter exits with the CLI user-error code. |
| f | Share the registry builder — `sources` commands MUST build the SAME registry as the `query` path by calling `buildProductionRegistry()` (`cmd/usearch/query.go:458`) directly (both are in package `main`; the auditor confirmed no extraction/refactor is needed). Single source of truth for which adapters are registered. |
| g | Add and centralize `--format` handling on the `sources` subcommands — this is a NEW deliverable, NOT a reuse. `--format` is a per-command local flag in CLI-002 (`query.go:301`, `root.go:181`); the `sources` subcommands have none today. SPEC-CLI-003 SHALL register a `--format` flag (`human`/`text`/`json`/`markdown`/`md`) on `sources list` and `sources status`, AND introduce a single shared validation+message helper (e.g. `validateFormat(string) error`) emitting the canonical message in §2.5, replacing the divergent ad-hoc message at `query.go:132`. JSON output SHALL carry a stable `schema_version` STRING field equal to `"1"` (matching `cmd/usearch/output_json.go:23,37`). |
| h | Add a `--timeout <duration>` flag to `sources status` — bounds each adapter's `Healthcheck(ctx)` call; default `3s`. A `--timeout <= 0` value SHALL be rejected as a usage error (see REQ-CLI3-005), NOT interpreted as "infinite". |
| i | Obtain the `disabled` adapter set via the existing public `Registry.SnapshotForAdmin()` (`registry.go:251`), whose per-adapter `Status` field already encodes `disabled` (`registry.go:279-282`). The private `r.disabled` map has no public getter and §2.2 forbids registry changes; `SnapshotForAdmin` is the sanctioned read path. The set is fetched ONCE before the live probe and consulted during classification. |
| j | Define exit-code semantics for `status` — `status` is a **report, not a gate**: it exits `0` even when some adapters are `unhealthy` / `not-configured`, carrying per-adapter status in the output. Nonzero exit is reserved for usage errors (bad `--format`, `--timeout <= 0`) and registry-build failure, per CLI-002 conventions. |
| k | Preserve all CLI-002 invariants on the touched commands — stdout is payload, stderr is noise; OTel span + request ID per invocation; `goleak`-clean (all probe goroutines and per-adapter contexts release before return; each probe wraps its `Healthcheck` call in `recover()` so a panicking adapter is classified `unhealthy` rather than crashing the CLI — see §2.7). |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following. Each has a known
owner or is deliberately out of frame; this list prevents scope creep.

- **NOT changing the `sources` command tree shape** — the `list` /
  `status` / `show` subcommand registration (CLI-002 REQ-CLI2-009)
  stays. SPEC-CLI-003 only rewires their data source and implements
  the stub.
- **NOT touching `query`, `deep`, `history`, `config`, `login`,
  `completion`, or the REPL** — those surfaces are unchanged.
- **NOT modifying `internal/adapters` in any way** — no new public
  getters (e.g. no `IsDisabled(id)`), no signature changes to
  `Registry.SnapshotForAdmin`, `Registry.List/Get`, `Capabilities`, or
  the adapter `Healthcheck` interface, no new fields on `pkg/types`.
  SPEC-CLI-003 is a pure CONSUMER of these existing primitives
  (SPEC-UI-002 / SPEC-EVAL-002 own them). The `disabled` set is read via
  the EXISTING public `SnapshotForAdmin()` (§2.6); nothing in the
  registry layer changes. This constraint is what forces the
  `DocTypes`→category mapping to live in the CLI (§2.5) rather than as a
  new `Capabilities.Category` field.
- **NOT changing the `/api/admin/adapters/health` HTTP endpoint** —
  the CLI is the missing surface; the admin endpoint is reference
  behavior, not a target of change.
- **NOT adding circuit-breaker / retry / backoff logic to the health
  probe** — a single `Healthcheck(ctx)` call per adapter, bounded by
  `--timeout`. Circuit state is out of scope (registry reports
  `CircuitState: "closed"` in V1 anyway, `registry.go:327`).
- **NOT persisting health results to history** — `sources status` is a
  read-only report; it does NOT write to the CLI history backend
  (CLI-002 REQ-CLI2-010 persists `query`/`deep` only).
- **NOT a watch / poll / `--follow` mode** — one-shot probe only.
  Continuous monitoring is the admin endpoint + Prometheus territory
  (SPEC-EVAL-002), not the CLI.
- **NOT health caching across invocations** — each `sources status`
  run probes live; no on-disk cache of last result.
- **NOT adding new adapters or changing `buildProductionRegistry`
  wiring** — SPEC-CLI-003 reuses the builder as-is; adapter
  registration logic is owned by SPEC-CLI-001 / the per-adapter SPECs.
- **NOT exposing secret values** — `show` and `status` MUST report
  only env-var NAMES and a boolean key-set state, never secret
  values (mirrors the `SecretValue` -is-always-empty invariant at
  `registry.go:234-236`).
- **NOT GitHub Issue tracking on this SPEC** (`issue_number: null`).

### 2.3 Forward-Looking / Dependency Notes

[HARD] SPEC-CLI-003 `depends_on`:
- **SPEC-CLI-002** (implemented) — owns the `sources` command tree and
  the CLI-002 `--format`/exit-code conventions this SPEC extends.
- **SPEC-UI-002** (implemented) — owns the actually-used registry
  primitives: `SnapshotForAdmin()` (`registry.go:251`, `@MX:SPEC:
  SPEC-UI-002 REQ-AS-001/REQ-AK-001`) for the `disabled` set + key-set,
  and the `AdapterAdminView` type. This is the load-bearing dependency.
- **SPEC-EVAL-002** (implemented) — parity reference only: it added
  `HealthSnapshot` / `/api/admin/adapters/health`, which this CLI mirrors
  conceptually but does NOT call (telemetry is empty in a CLI process,
  see §1). Retained in `depends_on` as a reference dependency.

The deferred REQ-CLI2-009 _TBD_ that this SPEC resolves was originally
phrased as "gated on SPEC-EVAL-002 health endpoint", but the mechanism
that actually unblocks it is SPEC-UI-002's `Healthcheck`/`SnapshotForAdmin`
primitives, not the EVAL-002 telemetry endpoint — hence UI-002 is the
hard dependency.

### 2.4 Reconciliation Note — REQ-CLI2-009 output-shape divergence

SPEC-CLI-002 REQ-CLI2-009 specified `list` output as
`<name>\t<category>\t<lang>\t<auth_required:y/n>`. The shipped
implementation (`cmd/usearch/sources_cmd.go:58-61`) instead emits
`NAME\tCATEGORY\tDESCRIPTION` from the static slice. This is a
spec-vs-implementation divergence, not a deliberate redesign.

SPEC-CLI-003 reconciles to the **REQ-CLI2-009-specified shape**:
`<name>\t<category>\t<lang>\t<auth_required:y/n>`. The `name` comes from
`Adapter.Name()`; `auth_required` from `Capabilities().RequiresAuth`;
`category` and `lang` are DERIVED from `Capabilities().DocTypes` and
`Capabilities().SupportedLangs` per the mapping in §2.5 — `Capabilities`
has NO scalar `Category`/`Lang` field (`pkg/types/capabilities.go:38-62`).
The `DESCRIPTION` column is dropped from `list` (it moves to `show`,
which prints full per-adapter detail). This is called out explicitly so
the annotation cycle can ratify the column change rather than treat it
as an accidental regression.

### 2.5 Output schema — derivation rules, format set, canonical message

These rules close audit defects D1 (no scalar category/lang), D5
(`--format` is a new deliverable), and D6 (`schema_version` type). They
are the single normative source for the `sources` output schema.

**Category derivation (`Capabilities().DocTypes []DocType` → one string).**
`Capabilities` exposes `DocTypes []DocType` (`capabilities.go:43-44`),
not a scalar category. The CLI SHALL map an adapter's `DocTypes` set to a
single `category` string using this fixed priority table (first matching
`DocType` in priority order wins; ties broken by the order below):

| If DocTypes contains… | category |
|-----------------------|----------|
| `DocTypePaper` | `academic` |
| `DocTypeRepo` or `DocTypeIssue` | `code` |
| `DocTypeVideo` | `video` |
| `DocTypePost` or `DocTypeSocial` | `social` |
| `DocTypeArticle` | `news` |
| `DocTypeOther` only, or empty | `other` |

This mapping table is the SINGLE permitted hand-maintained constant and
is the explicit EXCEPTION carved out in NFR-CLI3-004 (which otherwise
forbids parallel adapter metadata). It is a pure function of `DocTypes`
(a registry-derived value), not a per-adapter hardcoded list, so it does
not reintroduce the F-06 drift. The `DocType` constants are defined at
`pkg/types/capabilities.go:14-23`.

**Lang derivation (`Capabilities().SupportedLangs []string` → one string).**
`SupportedLangs` is a slice; an empty slice means language-agnostic
(`capabilities.go:45-47`). The CLI SHALL fold it to the `lang` column as:
empty slice → `*` (language-agnostic); single element → that BCP-47 code;
multiple elements → the first element followed by `+` (e.g. `en+`). The
full slice is shown verbatim by `sources show`.

**Format set + canonical message.** The `sources` subcommands SHALL
accept `--format` values `human`, `text` (alias of `human`), `json`,
`markdown`, `md` (alias of `markdown`). A shared helper SHALL reject any
other value with the canonical stderr message:
`unsupported format '<value>'; valid: human, text, json, markdown, md`
and the CLI user-error exit code. (This canonical message supersedes the
narrower ad-hoc message at `query.go:132`; aligning `query` to it is
out of scope here but noted as a CLI-002 follow-up.)

**`schema_version` type.** JSON output SHALL set `schema_version` to the
STRING `"1"`, matching `cmd/usearch/output_json.go:23,37` (NOT the
integer `1` used by `repl.go:216`).

### 2.6 Obtaining the `disabled` set (no registry change)

The registry's `disabled` state lives in the private map `r.disabled`
(`registry.go`) with no public getter, and §2.2 forbids adding one. The
ONLY sanctioned public read path is `Registry.SnapshotForAdmin()`
(`registry.go:251`), whose returned `[]AdapterAdminView` carries a
`Status` field set to `"disabled"` for disabled adapters
(`registry.go:279-282`). `SnapshotForAdmin` is telemetry-based (NOT a
live probe) and therefore cheap and network-free, which is exactly what
is needed to learn `disabled` membership.

`sources status` SHALL call `SnapshotForAdmin()` ONCE up front, build a
`set[id]bool` of disabled adapters from the entries whose `Status ==
"disabled"`, and consult that set during classification — disabled
adapters SHALL NOT be live-probed. This makes "is it disabled?" a
pre-probe decision (resolving the D2/D3 ordering contradiction: the
disabled check no longer depends on running `Healthcheck`).

### 2.7 Concurrency, cancellation, and panic policy

These rules close audit defects D11 (ctx-compliance assumption), D12
(`errgroup.WithContext` cancellation pitfall), and D13 (panic /
non-positive timeout).

- [HARD] The probe fan-out SHALL NOT use `errgroup.WithContext`. That
  helper cancels all sibling goroutines on the FIRST returned error,
  which would violate REQ-CLI3-003 (one slow/failing adapter must not
  abort the others). Probe goroutines SHALL NOT return an error to the
  group; each SHALL record its own per-adapter result into a
  pre-allocated, index-disjoint slot (no shared-slice append). A plain
  `errgroup.Group` (no context) or `sync.WaitGroup` is acceptable.
- [HARD] Each probe SHALL create its own `ctx, cancel :=
  context.WithTimeout(parent, timeout)` and `defer cancel()`; the parent
  context is NOT cancelled by a sibling's outcome.
- [HARD] Each probe SHALL wrap the `Healthcheck(ctx)` call in a
  `recover()`; a panic SHALL be classified as `unhealthy` (with the
  recovered value summarized in the per-adapter `error` field) rather
  than crashing the process. The adapter contract does not forbid panics.
- The wall-clock bound (NFR-CLI3-001) assumes adapters honor `ctx`
  cancellation in `Healthcheck`. The adapter godoc
  (`pkg/types/adapter.go:38-40`) documents `Healthcheck` as "Cheap" but,
  unlike `Search` (`:34-35`), does NOT contractually require ctx
  compliance. The per-adapter `context.WithTimeout` bounds the deadline
  the CLI WAITS on, but a `Healthcheck` that ignores ctx and blocks could
  leak a goroutine past command return. The fake adapters used in tests
  SHALL honor ctx; real-adapter ctx-compliance is tracked as a separate
  follow-up (see §8 R7) and is NOT a blocker for this SPEC.
- `--timeout <= 0` SHALL be rejected as a usage error (REQ-CLI3-005),
  never treated as "infinite".

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-CLI3-001 | Ubiquitous | The `usearch sources` command tree SHALL build its adapter set from the SAME production registry as the `query` path — i.e. by invoking `buildProductionRegistry()` (`cmd/usearch/query.go:458`), NOT from the package-level static `knownAdapters` slice (`cmd/usearch/sources_cmd.go:16-25`). After this SPEC, `knownAdapters` SHALL be removed. All three subcommands (`list`, `status`, `show`) SHALL enumerate adapters via `Registry.List()` / resolve via `Registry.Get(id)`. An adapter that is env-gated OFF (e.g. `github` without `USEARCH_GITHUB_TOKEN`, `youtube` without `YOUTUBE_BASE_URL`, `naver` without `NAVER_CLIENT_ID`+`NAVER_CLIENT_SECRET`) SHALL NOT appear in `list`; an adapter that IS registered (e.g. `bluesky`, `query.go:499-503`) SHALL appear. | P0 | `TestSourcesListReflectsRegistry`, `TestSourcesListOmitsUnregisteredGatedAdapter`, `TestSourcesListIncludesBluesky`; a grep test `TestSourcesNoStaticKnownAdapters` asserts the static slice is gone. |
| REQ-CLI3-002 | Event-Driven | WHEN the user invokes `usearch sources status`, the CLI SHALL build the production registry (`buildProductionRegistry()`), fetch the disabled set once via `Registry.SnapshotForAdmin()` (§2.6), then probe every NON-disabled registered adapter by calling `Adapter.Healthcheck(ctx)` **directly** and concurrently per the §2.7 non-cancelling fan-out, each call bounded by its own `context.WithTimeout` derived from `--timeout` (default `3s`). The CLI (NOT `Resync`) SHALL map results into exactly one status per adapter: `disabled` (in the §2.6 disabled set — not probed); `not-configured` (`Capabilities().RequiresAuth` AND any `AuthEnvVars` unset — not probed; see acceptance note on reachability); `connected` (`Healthcheck` returned nil); `unhealthy` (`Healthcheck` returned an error, deadline exceeded, or panicked per §2.7). The CLI SHALL additionally report a `key_set` boolean per adapter, computed from `Capabilities().RequiresAuth` + `Capabilities().AuthEnvVars` (same logic as `registry.go:266-277`). `Registry.Resync` SHALL NOT be used (it yields only connected/disabled and early-returns an error on failure, `registry.go:400-423`). | P1 | `TestSourcesStatusLiveHealthcheck` (fake registry, mixed-result adapters), `TestSourcesStatusConcurrentProbe`, `TestSourcesStatusClassifiesDisabled` (disabled set from `SnapshotForAdmin`), `TestSourcesStatusClassifiesNotConfigured` (fake adapter registered with `SkipAuthCheck` then `AuthEnvVars` unset — see §5.4 note: not reachable via the production `buildProductionRegistry` path because `registry.go:151-152` enforces auth at registration), `TestSourcesStatusReportsKeySet`, `TestSourcesStatusPanicClassifiedUnhealthy` (panicking `Healthcheck` → `unhealthy`, no crash). |
| REQ-CLI3-003 | Event-Driven | WHEN an adapter's `Healthcheck(ctx)` exceeds the `--timeout` deadline, the CLI SHALL cancel that adapter's context, classify the adapter as `unhealthy`, and continue reporting all other adapters (one slow adapter SHALL NOT block or fail the whole report). The total `sources status` wall-clock SHALL NOT exceed `--timeout` plus a small fixed overhead even when N adapters all hang, because probes run concurrently and each is independently bounded. | P1 | `TestSourcesStatusTimeoutClassifiesUnhealthy` (adapter blocks past deadline → `unhealthy`), `TestSourcesStatusOneSlowDoesNotBlockOthers`, `TestSourcesStatusTotalLatencyBounded` (N hanging adapters, asserts wall-clock ≤ timeout + overhead). |
| REQ-CLI3-004 | Ubiquitous | The `sources` subcommands SHALL register a NEW per-command `--format` flag (it does not exist on `sources` today — D5) accepting `human` (alias `text`, default in TTY), `json` (default non-TTY), `markdown` (alias `md`), validated by a single shared helper that emits the canonical message in §2.5. `sources list` SHALL emit `<name>\t<category>\t<lang>\t<auth_required:y/n>` (human/text, category/lang derived per §2.5), an array of objects with those fields (json), or a Markdown table (markdown) — reconciling the REQ-CLI2-009 shape (§2.4). `sources status` SHALL emit one row per adapter `<name>\t<status>\t<keys:y/n>` (human/text), an object `{schema_version, sources:[{name, status, key_set, error?}]}` (json), or a Markdown table (markdown). The JSON `schema_version` SHALL be the STRING `"1"` (matching `output_json.go:23,37`, §2.5). Invalid `--format` SHALL exit with the CLI user-error code and the canonical §2.5 message. | P1 | `TestSourcesStatusJSONSchema` (asserts `schema_version` is JSON string `"1"`, field set stable), `TestSourcesStatusMarkdownTable`, `TestSourcesStatusHumanColumns`, `TestSourcesListJSONFormat`, `TestSourcesListMarkdownFormat`, `TestSourcesListCategoryLangDerivation` (DocTypes/SupportedLangs → §2.5 mapping), `TestSourcesFormatInvalidRejected` (canonical message), `TestSourcesFormatHelperShared` (one validation path for both subcommands). |
| REQ-CLI3-005 | Ubiquitous | `usearch sources status` SHALL be a REPORT, not a gate: it SHALL exit `0` whenever the command itself ran successfully, regardless of how many adapters are `unhealthy` / `not-configured` / `disabled` — the per-adapter status lives in the output. Nonzero exit SHALL be reserved for command-usage failures: invalid `--format` value, a `--timeout` value that is `<= 0` or unparseable as a Go duration, or failure to build the registry (each maps to the CLI-002 user-error / internal-error code as appropriate; `--timeout <= 0` is NOT treated as infinite). `usearch sources show <name>` SHALL exit with the CLI-002 user-error code when `<name>` is not a registered adapter, emitting `usearch sources: unknown adapter '<name>'` to stderr. | P1 | `TestSourcesStatusExitsZeroWithUnhealthy`, `TestSourcesStatusExitsZeroWithNotConfigured`, `TestSourcesStatusBadTimeoutExitsUserError` (covers `0`, negative, and unparseable), `TestSourcesShowUnregisteredExitsUserError`. |
| REQ-CLI3-006 | Unwanted | IF zero adapters are registered (empty registry — e.g. all adapters env-gated off in a minimal environment), THEN `usearch sources list` SHALL emit only the header row (human) / an empty array `[]` with `schema_version` (json) / a header-only table (markdown) and exit `0`; and `usearch sources status` SHALL emit only the header / empty array and exit `0` — neither command SHALL error, panic, or hang on an empty registry. The CLI SHALL NEVER emit a secret value (env-var contents) in any `sources` output; only env-var NAMES and the boolean key-set state are permitted. | P2 | `TestSourcesListEmptyRegistry`, `TestSourcesStatusEmptyRegistry`, `TestSourcesNoSecretValueLeak` (audit: no `AuthEnvVars` *values* appear in any output). |

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-CLI3-001 | Bounded Command Latency | PRECONDITION: this bound holds for adapters whose `Healthcheck` honors `ctx` cancellation. Given that, `usearch sources status` SHALL complete within `--timeout + 500 ms` overhead regardless of the number of registered adapters N, because all N probes run concurrently (§2.7 non-cancelling fan-out) and each is independently bounded by `context.WithTimeout(--timeout)`. With the default `3s` timeout and the current production adapter set (≤ 10 adapters), a fully-hung-but-ctx-respecting backend SHALL return in ≤ 3.5 s wall-clock. The adapter contract (`pkg/types/adapter.go:38-40`) does NOT guarantee `Healthcheck` ctx-compliance (unlike `Search`, `:34-35`), so a pathological ctx-ignoring `Healthcheck` could exceed the bound and leak a goroutine; this is a known adapter-side risk (§8 R7), out of scope here. CI SHALL assert the ceiling with a fake registry of N hanging-but-ctx-honoring adapters (baseline pinned in the style of SPEC-UI-002's measurable latency NFR). |
| NFR-CLI3-002 | `list` Is Network-Free (Cheap) | `usearch sources list` and `usearch sources show` SHALL perform NO live healthchecks and NO network I/O — they read only registry membership and `Capabilities()` metadata. `list` SHALL complete in ≤ 50 ms for the production adapter set. Only `sources status` is permitted to issue network probes. CI SHALL assert `list` issues zero outbound requests (verified via a fake adapter whose `Healthcheck`/`Search` would record a call; the call count MUST remain zero across a `list` invocation). |
| NFR-CLI3-003 | Goroutine Hygiene | The `sources status` handler SHALL pass `goleak.VerifyTestMain` — no goroutine SHALL leak after the command returns. The errgroup SHALL be fully awaited (`g.Wait()`) before output is emitted; every per-adapter `context.WithTimeout` SHALL have its `cancel()` deferred so hung probes release their contexts. No probe goroutine SHALL outlive the command invocation. |
| NFR-CLI3-004 | Single Source of Truth (no registry drift) | After this SPEC there SHALL be exactly one place in `cmd/usearch` that decides which adapters are registered: `buildProductionRegistry()`. The `sources` commands SHALL NOT maintain any parallel per-adapter list of names or descriptions. EXCEPTION: the `DocTypes`→category mapping table in §2.5 is permitted, because it is a pure function of the registry-derived `Capabilities().DocTypes` (NOT a hand-maintained per-adapter name list) and therefore cannot drift from registration the way the old `knownAdapters` slice did. A regression test SHALL assert that the set of adapter NAMES reported by `usearch sources list` is exactly the set produced by `buildProductionRegistry().List()` under the same environment, so the `query` path and the `sources` path can never disagree about registration again. |

---

## 4. Acceptance Criteria

Detailed Given/When/Then scenarios live in the companion file
`.moai/specs/SPEC-CLI-003/acceptance.md` (authored alongside this
revision). This section enumerates the acceptance gate per requirement;
the companion file carries the full Given/When/Then matrix.

### REQ-CLI3-001 — Registry-backed listing, single source of truth

- `usearch sources list` enumerates `buildProductionRegistry().List()`.
- `bluesky` (registered, `query.go:499-503`) appears; `github` /
  `youtube` / `naver` are ABSENT when their env vars are unset.
- The static `knownAdapters` slice is removed from the package.

### REQ-CLI3-002 — Live concurrent healthcheck + classification

- `sources status` calls `Adapter.Healthcheck(ctx)` directly per
  non-disabled adapter, concurrently via the §2.7 non-cancelling
  fan-out. `Registry.Resync` is NOT used.
- The `disabled` set comes from `SnapshotForAdmin()` (§2.6), fetched
  once before probing.
- Each adapter is classified `connected` / `unhealthy` / `disabled` /
  `not-configured`; a panicking `Healthcheck` is classified `unhealthy`;
  `key_set` is reported per adapter.

### REQ-CLI3-003 — Per-adapter timeout, one slow does not block all

- A hanging adapter is classified `unhealthy` after `--timeout`.
- Other adapters still report; total wall-clock is bounded.

### REQ-CLI3-004 — `--format` (new flag) + derived columns

- New `--format` flag on `sources list`/`status` via one shared
  validation helper; `human`/`text`, `json` (`schema_version` = string
  `"1"`), `markdown`/`md`.
- `category`/`lang` derived from `DocTypes`/`SupportedLangs` per §2.5.
- Invalid format exits user-error with the canonical §2.5 message.

### REQ-CLI3-005 — Exit-code semantics (report not gate)

- `status` exits `0` even with unhealthy/not-configured adapters.
- Usage errors (bad `--format`, bad `--timeout`) exit nonzero.
- `show <unknown>` exits user-error with the standard stderr message.

### REQ-CLI3-006 — Edge cases (empty registry, no secret leak)

- Empty registry → header/empty-array, exit `0`, no panic/hang.
- No env-var VALUE ever appears in any `sources` output.

### NFR-CLI3-001 — Bounded latency

- N-hanging-adapter fake registry returns in ≤ `--timeout + 500 ms`.

### NFR-CLI3-002 — `list` network-free

- `list`/`show` issue zero probes; fake adapter call count stays zero.

### NFR-CLI3-003 — Goroutine hygiene

- `goleak.VerifyTestMain` clean; every `WithTimeout` cancel deferred.

### NFR-CLI3-004 — No registry drift

- `sources list` adapter set == `buildProductionRegistry().List()`.

---

## 5. Technical Approach (high-level — full plan in plan.md)

### 5.1 Phasing

1. **Registry rebind** — remove `knownAdapters`; route `list` / `show`
   through `buildProductionRegistry()` + `Registry.List`/`Get`;
   reconcile `list` columns to the REQ-CLI2-009 shape (§2.4).
2. **Live status** — implement `sources status`: pre-fetch the disabled
   set via `SnapshotForAdmin()` (§2.6); §2.7 non-cancelling fan-out over
   `Registry.List()`; per-adapter `context.WithTimeout(--timeout)` +
   `recover()`; direct `Adapter.Healthcheck` (NOT `Resync`);
   classification into `connected`/`unhealthy`/`disabled`/`not-configured`
   + `key_set`.
3. **Format + exit codes** — `--format` parity for `status` (incl.
   `schema_version` JSON), `--timeout` flag, report-not-gate exit
   semantics, edge cases (empty registry, no secret leak).

### 5.2 Files touched (proposed — _TBD_ refinement in plan)

```
cmd/usearch/
├── sources_cmd.go        [MODIFY] remove knownAdapters; rebind list/show
│                                  to registry; implement status (non-
│                                  cancelling fan-out + per-adapter timeout
│                                  + recover + classification); add
│                                  --format + --timeout flags; add the
│                                  §2.5 DocTypes→category + SupportedLangs
│                                  →lang derivation helpers.
├── sources_cmd_test.go   [MODIFY/EXTEND] add REQ-CLI3 table-driven tests.
├── format.go (or similar) [NEW or MODIFY] shared validateFormat helper
│                                  emitting the canonical §2.5 message;
│                                  used by sources list + status (and a
│                                  candidate to back query later).
├── query.go              [REFERENCE] buildProductionRegistry reused
│                                  (same package main; no signature change).
```

No `internal/adapters` changes — SPEC-CLI-003 is a pure consumer of
`Registry.List/Get/SnapshotForAdmin`, `Adapter.Healthcheck`, and
`Capabilities()`. `Registry.Resync` is NOT used.

### 5.3 Classification logic (normative — closes D2/D3 ordering)

Pre-probe (once): `disabledSet = {id : v.Status == "disabled" for v in
Registry.SnapshotForAdmin()}` (§2.6).

For each adapter id in `Registry.List()`:
- If `id ∈ disabledSet` → `disabled` (NOT probed).
- Else if `Capabilities().RequiresAuth` AND any `AuthEnvVars` unset →
  `not-configured` (NOT probed; reachability note below).
- Else probe `Adapter.Healthcheck(ctx)` (own `context.WithTimeout`,
  `recover()` per §2.7):
  - nil → `connected`
  - error / deadline exceeded / panic → `unhealthy`
- `key_set` = NOT(`RequiresAuth` AND any `AuthEnvVars` unset), for every
  adapter regardless of status.

The `disabled` and `not-configured` checks are BOTH pre-probe, so no
classification step depends on the outcome or error of `Healthcheck`
(this is what the audit D3 required — `Resync`'s error-early-return
behavior is irrelevant because we never call it).

### 5.4 Test strategy

- **Registry-backed list/show**: build a registry with
  `adapters.NewRegistry` + fake adapters carrying known `Capabilities`
  (`DocTypes`, `SupportedLangs`, `RequiresAuth`); assert enumeration,
  the §2.5 derived columns, and the no-drift equality.
- **Live status**: fake adapters with scripted `Healthcheck` results
  (nil / error / ctx-honoring block / panic) → assert classification
  including `unhealthy`-on-panic.
- **`disabled` path**: register an adapter, toggle it disabled via the
  registry's existing `ToggleEnabled`, and assert `status` reads
  `disabled` from `SnapshotForAdmin()` without probing it.
- **`not-configured` reachability (D4)**: this state is NOT reachable
  through the production `buildProductionRegistry` path, because
  `RegisterWithOptions` enforces `AuthEnvVars` at registration
  (`registry.go:151-152`) — an auth-gated adapter with unset keys is
  never registered. The test SHALL register a fake auth-requiring adapter
  with `RegisterOptions{SkipAuthCheck: true}` and its `AuthEnvVars`
  unset, OR simulate a registered-then-key-lost adapter, to exercise the
  branch. This precondition is documented so the test environment is
  unambiguous.
- **Timeout**: ctx-honoring blocking `Healthcheck` + short `--timeout` →
  `unhealthy`; assert wall-clock bound with N blockers (NFR-CLI3-001).
  `--timeout <= 0` / unparseable → user-error exit (REQ-CLI3-005).
- **Format**: `cmd.SetArgs` + buffer capture for human/json/markdown;
  JSON unmarshalled to assert `schema_version` is the string `"1"` and
  the field set is stable; one test asserts both subcommands share the
  same validation path/message.
- **goleak**: `goleak.VerifyTestMain` in the package test main; probes
  use ctx-honoring fakes so no goroutine leaks.
- **No-drift**: compare `sources list` name set to
  `buildProductionRegistry().List()` under a controlled env.

### 5.5 Coverage target

85% (matches CLI-002). The handler is data-flow dense but well-bounded;
the concurrency + timeout paths are the highest-value lines to cover.

---

## 6. @MX Tag Targets

The following functions are the @MX annotation targets for the Run
phase (per `.claude/rules/moai/workflow/mx-tag-protocol.md`):

| Symbol | File | Tag | Rationale |
|--------|------|-----|-----------|
| `newSourcesStatusCmd` (rewritten) | `cmd/usearch/sources_cmd.go` | `@MX:WARN` + `@MX:REASON` | Spawns concurrent `Healthcheck` goroutines (§2.7 non-cancelling fan-out — NOT `errgroup.WithContext`) with per-adapter context timeouts and per-probe `recover()` — goroutine/lifecycle danger zone (mx-tag-protocol WARN trigger: goroutine + context). @MX:REASON SHALL note that `WithContext` is deliberately avoided to satisfy REQ-CLI3-003. |
| `newSourcesListCmd` / `newSourcesShowCmd` (rebound) | `cmd/usearch/sources_cmd.go` | `@MX:NOTE` | Registry-backed; document that the adapter set is sourced from `buildProductionRegistry` (single source of truth) — explains the F-06 fix intent. |
| classification helper (new) | `cmd/usearch/sources_cmd.go` | `@MX:NOTE` | Document the `connected`/`unhealthy`/`disabled`/`not-configured` mapping and why `HealthSnapshot` is NOT used (telemetry empty in short-lived CLI). |
| `buildProductionRegistry` | `cmd/usearch/query.go` | `@MX:ANCHOR` (if fan_in ≥ 3 after this SPEC) | Becomes shared between `query` and `sources` paths; high fan_in invariant — single source of truth for adapter registration. Demote/keep per actual caller count. |

---

## 7. Dependencies

### 7.1 Upstream SPEC dependencies (depends_on)

- **SPEC-CLI-002** (implemented) — owns the `sources` command tree, the
  CLI-002 `--format`/exit-code conventions, and the REQ-CLI2-009 `list`
  output-shape contract that §2.4 reconciles.
- **SPEC-UI-002** (implemented) — the LOAD-BEARING dependency. Owns
  `Registry.SnapshotForAdmin()` (`registry.go:251`, the public read path
  for the `disabled` set and key-set, REQ-AS-001/REQ-AK-001) and the
  `AdapterAdminView` type. Also the origin of `Registry.Resync`
  (REQ-AS-002), which this SPEC deliberately does NOT use.
- **SPEC-EVAL-002** (implemented) — PARITY-REFERENCE dependency only.
  Added `Registry.HealthSnapshot` / `AdapterHealth` /
  `/api/admin/adapters/health`, which this CLI mirrors conceptually but
  does NOT call (telemetry empty in a CLI process, §1).

### 7.2 Reused primitives (no change to these — pure consumer)

- `internal/adapters/registry.go:251` `Registry.SnapshotForAdmin()` —
  telemetry-based public view; SOLE sanctioned path to the `disabled`
  set (`Status == "disabled"`, `:279-282`) and key-set. Used pre-probe.
- `internal/adapters/registry.go:185` `Registry.List()` and `:175`
  `Registry.Get(id)` — registry enumeration.
- `pkg/types/adapter.go:38-40` `Adapter.Healthcheck(ctx) error` — the
  liveness probe, called DIRECTLY (not via `Resync`).
- `pkg/types/capabilities.go:38-62` `Adapter.Capabilities()` →
  `DocTypes`, `SupportedLangs`, `RequiresAuth`, `AuthEnvVars`,
  `DisplayName`, `RateLimitPerMin` — column derivation (§2.5) + key-set.
- `cmd/usearch/query.go:458` `buildProductionRegistry()` — the canonical
  adapter wiring; reused as the single source of truth.
- NOT used: `Registry.HealthSnapshot` (telemetry, meaningless in CLI),
  `Registry.Resync` (yields only connected/disabled, errors early).

### 7.3 External dependencies

- `golang.org/x/sync/errgroup` — already a project dependency (used by
  `internal/deepagent/tree.go:11`). No new module.

---

## 8. Risks (full register in research.md)

| # | Risk | Severity | Mitigation |
|---|------|----------|------------|
| R1 | Live `Healthcheck` hangs on a misbehaving backend | High | Per-adapter `context.WithTimeout(--timeout)`; errgroup concurrency; NFR-CLI3-001 wall-clock bound asserted in CI. |
| R2 | `sources status` issues real network calls in tests / CI | Medium | Tests use fake adapters with scripted `Healthcheck`; `list`/`show` are network-free by NFR-CLI3-002. |
| R3 | Removing `knownAdapters` breaks existing CLI-002 `sources` tests | Medium | Update `cmd/usearch/sources_cmd_test.go` to the registry-backed + REQ-CLI2-009 column shape; treat the column change as a ratified reconciliation (§2.4), not a silent regression. |
| R4 | Secret leak via `show`/`status` output | High | REQ-CLI3-006 audit test asserts no env-var VALUE appears; only NAMES + boolean key-set (mirrors `registry.go:234-236`). |
| R5 | `buildProductionRegistry` env-gating makes test output env-dependent | Medium | NFR-CLI3-004 no-drift test pins `sources list` to `buildProductionRegistry().List()` under a controlled env, not to a hardcoded expectation. |
| R6 | Goroutine leak from un-awaited probes | High | NFR-CLI3-003 goleak; await all probes before emit; deferred `cancel()` per probe. |
| R7 | Adapter `Healthcheck` ignores ctx and blocks past `--timeout` | Medium | Adapter contract does not require Healthcheck ctx-compliance (`adapter.go:38-40` vs Search `:34-35`). Per-adapter `WithTimeout` bounds the WAIT, but a ctx-ignoring probe leaks a goroutine; tests use ctx-honoring fakes; real-adapter ctx-compliance tracked as a separate follow-up. NFR-CLI3-001 states this precondition explicitly. |
| R8 | Panicking `Healthcheck` crashes the CLI | Medium | §2.7 per-probe `recover()`; panic → `unhealthy`; `TestSourcesStatusPanicClassifiedUnhealthy`. |
| R9 | `errgroup.WithContext` cancels siblings on first failure (violates REQ-CLI3-003) | High | §2.7 [HARD] forbids `WithContext`; probes never return errors to the group; per-adapter results collected into index-disjoint slots. |

---

## 9. References

### Internal (code — exact citations)

- `cmd/usearch/sources_cmd.go:16-25` — static `knownAdapters` slice
  (to be removed).
- `cmd/usearch/sources_cmd.go:58-61` — shipped `list` column shape
  (`NAME\tCATEGORY\tDESCRIPTION`) reconciled in §2.4.
- `cmd/usearch/sources_cmd.go:71-86` — `newSourcesStatusCmd` stub
  (to be implemented).
- `cmd/usearch/sources_cmd.go:88-114` — `newSourcesShowCmd` (to rebind).
- `cmd/usearch/query.go:458-514` — `buildProductionRegistry` (single
  source of truth; reused).
- `cmd/usearch/query.go:499-503` — `bluesky` registration (in registry,
  missing from static list — the F-06 evidence).
- `internal/adapters/registry.go:340-353` — `classifyHealth` (telemetry
  rule that yields `healthy` on zero calls; why HealthSnapshot is unfit).
- `internal/adapters/registry.go:363-387` — `HealthSnapshot` (telemetry
  source; NOT used).
- `internal/adapters/registry.go:251-304` — `SnapshotForAdmin` (USED:
  sole public path to the `disabled` set, `Status == "disabled"` at
  `:279-282`; key-set computation at `:266-277`).
- `internal/adapters/registry.go:394-432` — `Resync` (NOT used: status
  only connected/disabled `:420-423`; early error return `:400-402`).
- `internal/adapters/registry.go:151-152` — `RegisterWithOptions` auth
  enforcement (why `not-configured` is unreachable in production).
- `internal/adapters/registry.go:175, 185` — `Get` / `List`.
- `internal/adapters/registry.go:204-237` — `AdapterAdminView`
  (incl. `Status`, `KeySet`, `SecretValue`-always-empty invariant).
- `pkg/types/adapter.go:28-45` — `Adapter` interface; `Healthcheck`
  godoc (`:38-40`, "Cheap", no ctx-compliance guarantee); no `Category()`
  accessor exists.
- `pkg/types/capabilities.go:14-23` — `DocType` constants (category
  mapping input, §2.5).
- `pkg/types/capabilities.go:38-62` — `Capabilities` struct: `DocTypes
  []DocType`, `SupportedLangs []string`; NO scalar `Category`/`Lang`.
- `cmd/usearch/output_json.go:23,37` — `schema_version` is the string
  `"1"` (the type this SPEC pins to).
- `cmd/usearch/query.go:132, 301`, `cmd/usearch/root.go:181` —
  `--format` is a per-command local flag with a divergent error message
  (why §2.5 centralizes a canonical helper).

### Internal (SPEC + project docs)

- `.moai/specs/SPEC-CLI-002/spec.md` — parent SPEC; REQ-CLI2-006
  (`--format`), REQ-CLI2-009 (`sources` tree + deferred status/show).
- `.moai/specs/SPEC-EVAL-002/` — adapter health endpoint + primitives.
- `.moai/specs/SPEC-UI-002/` — `Resync` origin (REQ-AS-002); measurable
  latency NFR style.
- `USAGE.md:~261` — F-06 user-facing description (the "현재 상태" note:
  `sources status` → all `unknown`; static `list` omits `bluesky`).
- `.planning/codebase/CONCERNS.md` — GSD audit concerns register
  (F-06 is the audit's finding ID for this gap).

---

*End of SPEC-CLI-003 v0.1 (draft).*
