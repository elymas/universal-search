# SPEC-CLI-003 — Acceptance Criteria

Companion to `spec.md`. Full Given/When/Then matrix for each EARS
requirement and NFR. Narrative in Korean per project
`conversation_language`; Go identifiers, commands, and output samples in
English per house style. Every scenario names a binary-testable Go test
matching the `cmd/usearch/*_test.go` table-driven style.

All §-references point into `spec.md`. Output shapes follow `spec.md`
§2.5 (column derivation, format set, canonical message, `schema_version`
string `"1"`).

---

## REQ-CLI3-001 — Registry-backed listing, single source of truth

### AC-001-1 `list` reflects the live registry
- GIVEN a registry built by `buildProductionRegistry()` in which
  `bluesky` is registered (`query.go:499-503`)
- WHEN `usearch sources list` runs
- THEN stdout contains a `bluesky` row AND the adapter name set equals
  `buildProductionRegistry().List()`
- TEST: `TestSourcesListReflectsRegistry`, `TestSourcesListIncludesBluesky`

### AC-001-2 env-gated-off adapters are absent
- GIVEN `USEARCH_GITHUB_TOKEN`/`GITHUB_TOKEN`, `YOUTUBE_BASE_URL`,
  `NAVER_CLIENT_ID`/`NAVER_CLIENT_SECRET` are all unset
- WHEN `usearch sources list` runs
- THEN no `github`, `youtube`, or `naver` row appears
- TEST: `TestSourcesListOmitsUnregisteredGatedAdapter`

### AC-001-3 the static slice is gone
- GIVEN the post-SPEC `cmd/usearch/sources_cmd.go`
- WHEN the test greps the package for `knownAdapters`
- THEN no `knownAdapters` symbol exists
- TEST: `TestSourcesNoStaticKnownAdapters`

---

## REQ-CLI3-002 — Live concurrent healthcheck + classification

### AC-002-1 mixed-result classification
- GIVEN a registry of fake adapters: A `Healthcheck`→nil, B
  `Healthcheck`→error, C toggled disabled, D auth-required registered
  with `SkipAuthCheck` and `AuthEnvVars` unset
- WHEN `usearch sources status` runs
- THEN A=`connected`, B=`unhealthy`, C=`disabled`, D=`not-configured`
- TEST: `TestSourcesStatusLiveHealthcheck`

### AC-002-2 disabled comes from SnapshotForAdmin, pre-probe
- GIVEN an adapter toggled disabled via `Registry.ToggleEnabled`
- WHEN `usearch sources status` runs
- THEN that adapter is reported `disabled` AND its `Healthcheck` is
  never invoked (probe-call counter stays 0)
- TEST: `TestSourcesStatusClassifiesDisabled`

### AC-002-3 not-configured is pre-probe (reachability documented)
- GIVEN a fake auth-requiring adapter registered with
  `RegisterOptions{SkipAuthCheck: true}` and its `AuthEnvVars` unset
  (this branch is NOT reachable via `buildProductionRegistry`, which
  enforces auth at `registry.go:151-152` — documented precondition)
- WHEN `usearch sources status` runs
- THEN that adapter is reported `not-configured` AND not probed
- TEST: `TestSourcesStatusClassifiesNotConfigured`

### AC-002-4 key_set reported for every adapter
- GIVEN one auth-required adapter with all `AuthEnvVars` set and one
  without
- WHEN `usearch sources status` runs
- THEN `key_set` is `true` / `y` for the former, `false` / `n` for the
  latter
- TEST: `TestSourcesStatusReportsKeySet`

### AC-002-5 panic → unhealthy, no crash
- GIVEN a fake adapter whose `Healthcheck` panics
- WHEN `usearch sources status` runs
- THEN the process does NOT crash AND that adapter is `unhealthy` with
  the recovered message in its `error` field
- TEST: `TestSourcesStatusPanicClassifiedUnhealthy`

### AC-002-6 Resync is not used
- GIVEN the implementation
- WHEN the test greps `cmd/usearch/sources_cmd.go`
- THEN there is no call to `Registry.Resync`
- TEST: covered by `TestSourcesStatusLiveHealthcheck` design + a grep
  assertion in `TestSourcesStatusConcurrentProbe`

---

## REQ-CLI3-003 — Per-adapter timeout, one slow does not block all

### AC-003-1 timeout → unhealthy
- GIVEN a fake adapter whose `Healthcheck` blocks until ctx is cancelled
- WHEN `usearch sources status --timeout 50ms` runs
- THEN that adapter is `unhealthy`
- TEST: `TestSourcesStatusTimeoutClassifiesUnhealthy`

### AC-003-2 one slow does not abort siblings
- GIVEN adapters: one ctx-honoring blocker, three fast `connected`
- WHEN `usearch sources status --timeout 50ms` runs
- THEN the three fast adapters report `connected` (NOT cancelled by the
  blocker) AND the blocker reports `unhealthy`
- TEST: `TestSourcesStatusOneSlowDoesNotBlockOthers`

### AC-003-3 wall-clock bounded
- GIVEN N=8 ctx-honoring blocking adapters
- WHEN `usearch sources status --timeout 100ms` runs
- THEN total wall-clock ≤ `100ms + 500ms`
- TEST: `TestSourcesStatusTotalLatencyBounded`

---

## REQ-CLI3-004 — `--format` (new flag) + derived columns

### AC-004-1 JSON schema_version is string "1"
- GIVEN any registered adapters
- WHEN `usearch sources status --format json` runs
- THEN stdout parses to `{schema_version, sources:[{name,status,key_set,
  error?}]}` AND `schema_version` decodes as the JSON string `"1"`
  (NOT integer `1`)
- TEST: `TestSourcesStatusJSONSchema`

### AC-004-2 markdown table
- WHEN `usearch sources status --format markdown` (and `--format md`) run
- THEN stdout is a Markdown table with the same columns; `md` is a
  byte-equivalent alias of `markdown`
- TEST: `TestSourcesStatusMarkdownTable`

### AC-004-3 human columns
- WHEN `usearch sources status` runs in human/text mode
- THEN one row per adapter `<name>\t<status>\t<keys:y/n>`
- TEST: `TestSourcesStatusHumanColumns`

### AC-004-4 category/lang derivation (§2.5)
- GIVEN a fake adapter with `DocTypes=[DocTypePaper]`,
  `SupportedLangs=["en"]` and another with `DocTypes=[DocTypePost]`,
  `SupportedLangs=[]`
- WHEN `usearch sources list` runs
- THEN the first row shows `academic` / `en`; the second `social` / `*`
- TEST: `TestSourcesListCategoryLangDerivation`

### AC-004-5 list json + markdown
- WHEN `usearch sources list --format json` / `--format markdown` run
- THEN json is an array of `{name,category,lang,auth_required}`;
  markdown is a table with those columns
- TEST: `TestSourcesListJSONFormat`, `TestSourcesListMarkdownFormat`

### AC-004-6 invalid format → canonical message
- WHEN `usearch sources status --format yaml` runs
- THEN stderr is exactly `unsupported format 'yaml'; valid: human, text,
  json, markdown, md` AND exit code is the CLI user-error code
- TEST: `TestSourcesFormatInvalidRejected`

### AC-004-7 shared validation helper
- GIVEN both `sources list` and `sources status`
- WHEN each is given an invalid `--format`
- THEN both emit the identical canonical message via one shared helper
- TEST: `TestSourcesFormatHelperShared`

---

## REQ-CLI3-005 — Exit-code semantics (report not gate)

### AC-005-1 unhealthy → exit 0
- GIVEN adapters where some are `unhealthy`
- WHEN `usearch sources status` runs
- THEN exit code is `0`
- TEST: `TestSourcesStatusExitsZeroWithUnhealthy`

### AC-005-2 not-configured → exit 0
- GIVEN adapters where some are `not-configured`
- WHEN `usearch sources status` runs
- THEN exit code is `0`
- TEST: `TestSourcesStatusExitsZeroWithNotConfigured`

### AC-005-3 bad timeout → user-error
- WHEN `usearch sources status --timeout 0`, `--timeout -1s`, or
  `--timeout notaduration` run
- THEN each exits with the CLI user-error code (NOT treated as infinite)
- TEST: `TestSourcesStatusBadTimeoutExitsUserError`

### AC-005-4 show unknown → user-error
- WHEN `usearch sources show nonesuch` runs (not a registered adapter)
- THEN stderr is `usearch sources: unknown adapter 'nonesuch'` AND exit
  is the CLI user-error code
- TEST: `TestSourcesShowUnregisteredExitsUserError`

---

## REQ-CLI3-006 — Edge cases (empty registry, no secret leak)

### AC-006-1 empty registry, list
- GIVEN a registry with zero registered adapters
- WHEN `usearch sources list` runs in each format
- THEN human=header only; json=`{"schema_version":"1","sources":[]}` (or
  `[]` for list per §2.5); markdown=header-only table; exit `0`; no
  panic/hang
- TEST: `TestSourcesListEmptyRegistry`

### AC-006-2 empty registry, status
- GIVEN a registry with zero registered adapters
- WHEN `usearch sources status` runs
- THEN header/empty array, exit `0`, no panic/hang
- TEST: `TestSourcesStatusEmptyRegistry`

### AC-006-3 no secret value ever leaks
- GIVEN auth-required adapters whose `AuthEnvVars` are set to sentinel
  secret values in the environment
- WHEN `usearch sources list` / `show` / `status` run in every format
- THEN no env-var VALUE appears anywhere in stdout/stderr; only env-var
  NAMES (from `AuthEnvVars`) and the boolean key-set state
- TEST: `TestSourcesNoSecretValueLeak`

---

## NFR acceptance

### NFR-CLI3-001 — Bounded latency
- Covered by `TestSourcesStatusTotalLatencyBounded` (AC-003-3), using
  ctx-honoring fakes. Precondition (Healthcheck honors ctx) stated in
  `spec.md`.

### NFR-CLI3-002 — `list` is network-free
- GIVEN fake adapters whose `Healthcheck`/`Search` increment a call
  counter
- WHEN `usearch sources list` and `usearch sources show <name>` run
- THEN the call counter stays `0`
- TEST: `TestSourcesListIssuesNoProbes`

### NFR-CLI3-003 — Goroutine hygiene
- `goleak.VerifyTestMain` in the package; probes use ctx-honoring fakes;
  no goroutine outlives the command.
- TEST: package-level `TestMain` + `goleak`.

### NFR-CLI3-004 — No registry drift
- GIVEN a controlled environment
- WHEN `usearch sources list` runs
- THEN its adapter NAME set equals `buildProductionRegistry().List()`
- TEST: `TestSourcesListMatchesBuildProductionRegistry`

---

## Definition of Done

- [ ] All AC tests above pass (`go test ./cmd/usearch/...`).
- [ ] `knownAdapters` removed; `sources` list/show/status registry-backed.
- [ ] `Registry.Resync` not referenced by `sources_cmd.go`.
- [ ] `disabled` set obtained via `SnapshotForAdmin`; no `internal/adapters`
      change.
- [ ] `--format` validation centralized; canonical message; `schema_version`
      string `"1"`.
- [ ] `goleak`-clean; per-probe `recover()`; non-cancelling fan-out.
- [ ] Coverage ≥ 85% on `cmd/usearch/sources_cmd.go`.
- [ ] TRUST 5 gates pass.

---

*End of SPEC-CLI-003 acceptance.md.*
