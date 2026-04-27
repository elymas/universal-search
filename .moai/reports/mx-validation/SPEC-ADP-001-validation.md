# @MX Tag Validation Report — SPEC-ADP-001

Generated: 2026-04-27
Commits: 41372d4 (TDD impl) + e3d1f7d (alloc refactor)

## Summary

| Tag Type   | Count | Compliance                                                            |
|------------|-------|------------------------------------------------------------------------|
| @MX:ANCHOR | 2     | All carry @MX:REASON ✓ All carry @MX:SPEC ✓                           |
| @MX:WARN   | 1     | Carries @MX:REASON ✓ Carries @MX:SPEC ✓                               |
| @MX:NOTE   | 6     | All carry @MX:SPEC ✓ (1 hygiene fix applied during sync — see Note 6) |
| @MX:TODO   | 0     | All RED tests promoted to GREEN                                       |

**Total tags: 9** (2 ANCHOR + 1 WARN + 6 NOTE) on `internal/adapters/reddit/`.

Per-file usage (within ADP-001 scope):

| File                                    | ANCHOR | WARN | NOTE |
|-----------------------------------------|--------|------|------|
| `internal/adapters/reddit/search.go`    | 1      | 0    | 0    |
| `internal/adapters/reddit/parse.go`     | 1      | 0    | 0    |
| `internal/adapters/reddit/client.go`    | 0      | 1    | 2    |
| `internal/adapters/reddit/errors.go`    | 0      | 0    | 1    |
| `internal/adapters/reddit/score.go`     | 0      | 0    | 2    |
| `internal/adapters/reddit/bench_test.go`| 0      | 0    | 1    |

No file exceeds the per-file caps (3 ANCHOR / 5 WARN) from `.moai/config/sections/mx.yaml`.

**Out of scope** (already validated under their owning SPECs):

- `internal/adapters/registry.go` carries 3 ANCHOR + 1 WARN under SPEC-CORE-001 (validated in `.moai/reports/mx-validation/SPEC-CORE-001-validation.md`)
- `internal/adapters/noop/noop.go` is the SPEC-CORE-001 reference adapter

## Tag Inventory

### @MX:ANCHOR — 2 tags

**1. `internal/adapters/reddit/search.go:38`**

```go
// @MX:ANCHOR: [AUTO] Sole public entry point for Reddit search. Callers:
// registry wrappedAdapter (via types.Adapter), FAN-001 fanout, CLI-001, tests.
// @MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001
// + SYN-001 and all downstream SPECs that depend on []types.NormalizedDoc.
// @MX:SPEC: SPEC-ADP-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes (consistent with `code_comments: en`)

**2. `internal/adapters/reddit/parse.go:66`**

```go
// @MX:ANCHOR: [AUTO] NormalizedDoc field-mapping integrity gate. Every Reddit
// doc passes through this single transform function. A bug here corrupts every
// document returned by the adapter.
// @MX:REASON: fan_in = 1 (Search) but invariant-bearing; field-mapping changes
// require careful coordination with SPEC-IDX-001 and SPEC-SYN-001 consumers.
// @MX:SPEC: SPEC-ADP-001
```

- [AUTO] prefix: present
- @MX:REASON: present (fan_in = 1 acknowledged; ANCHOR justified as `External system integration point` / invariant-bearing field-mapping gate per protocol)
- @MX:SPEC: present
- Comments in English: yes
- Note: `parseListing` is unexported but is the canonical NormalizedDoc field-mapping transform for the adapter; the ANCHOR rationale is invariant-bearing rather than caller-count driven, which the protocol allows for `External system integration point` annotations.

### @MX:WARN — 1 tag

**1. `internal/adapters/reddit/client.go:66`**

```go
// @MX:WARN: [AUTO] Outbound network call. The CheckRedirect policy on
// a.httpClient enforces the SSRF guard; do not bypass or replace the client
// without re-applying the allowlist.
// @MX:REASON: removing CheckRedirect re-opens SSRF via Reddit CDN redirects.
// @MX:SPEC: SPEC-ADP-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes

### @MX:NOTE — 6 tags

**1. `internal/adapters/reddit/client.go:20`**

```go
// @MX:NOTE: [AUTO] 4-entry security boundary. Adding a host here requires a
// security review — a permissive allowlist re-opens SSRF via Reddit's CDN
// redirect infrastructure.
// @MX:SPEC: SPEC-ADP-001
```

- [AUTO] prefix: present
- @MX:SPEC: present
- Comments in English: yes

**2. `internal/adapters/reddit/client.go:98`**

```go
// @MX:NOTE: [AUTO] HTTP-status rosetta stone. Future contributors adding new
// status-code handling should update this switch first, then add a test row in
// TestCategorizeStatusTable.
// @MX:SPEC: SPEC-ADP-001
```

- [AUTO] prefix: present
- @MX:SPEC: present
- Comments in English: yes

**3. `internal/adapters/reddit/errors.go:30`**

```go
// @MX:NOTE: [AUTO] RFC 7231 §7.1.3: integer-seconds tried first, then HTTP-date.
// 60s cap aligns with the operational p95 budget; default 5s when missing.
// @MX:SPEC: SPEC-ADP-001
```

- [AUTO] prefix: present
- @MX:SPEC: present
- Comments in English: yes

**4. `internal/adapters/reddit/score.go:11`**

```go
// @MX:NOTE: [AUTO] Empirical inflection point: score=100 -> ~0.88.
// Updating this constant changes ranking weights downstream.
// @MX:SPEC: SPEC-ADP-001
```

- [AUTO] prefix: present
- @MX:SPEC: present
- Comments in English: yes

**5. `internal/adapters/reddit/score.go:19`**

```go
// @MX:NOTE: [AUTO] Semantic center: score=0 -> 0.5 (neutral).
// 0.5 is the midpoint of the [0, 1] normalized range.
// @MX:SPEC: SPEC-ADP-001
```

- [AUTO] prefix: present
- @MX:SPEC: present
- Comments in English: yes

**6. `internal/adapters/reddit/bench_test.go:23`** — HYGIENE FIX APPLIED IN THIS SYNC

Pre-sync state:

```go
// BenchmarkParseListing25Docs measures parsing performance for the standard
// 25-document fixture. The benchmark aims for p50 ≤ 5ms / allocs ≤ 250.
//
// @MX:NOTE: [AUTO] Performance sentinel — if allocs/op regresses beyond 250
// or ns/op beyond 5_000_000, investigate transformation or JSON decode path.
```

Defects detected:

- **Stale alloc threshold** in NOTE description (`beyond 250`) — NFR-ADP-001 was revised in HISTORY iteration 3 from `≤ 250` to `≤ 500` (commit `e99d6d8`) after empirical baseline measured 460 allocs/op on the reference fixture. The inline tag text was not updated alongside the SPEC table.
- **Missing `@MX:SPEC: SPEC-ADP-001`** sub-line — every other ADP-001 tag carries this; sole outlier.
- Surrounding godoc on the function (line 21, `allocs ≤ 250`) carried the same stale number.

Sync-time fix:

- Description aligned to current NFR-ADP-001 target (`regresses beyond 500`).
- Function godoc on line 21 aligned to `allocs ≤ 500 (NFR-ADP-001 revised in HISTORY iteration 3 from ≤ 250 after empirical baseline)`.
- `@MX:SPEC: SPEC-ADP-001` sub-line appended.

Post-fix state:

```go
// BenchmarkParseListing25Docs measures parsing performance for the standard
// 25-document fixture. The benchmark aims for p50 ≤ 5ms / allocs ≤ 500
// (NFR-ADP-001 revised in HISTORY iteration 3 from ≤ 250 after empirical baseline).
//
// @MX:NOTE: [AUTO] Performance sentinel — if allocs/op regresses beyond 500
// or ns/op beyond 5_000_000, investigate transformation or JSON decode path.
// @MX:SPEC: SPEC-ADP-001
```

- [AUTO] prefix: present
- @MX:SPEC: present (ADDED)
- Comments in English: yes

## Defects: None remaining

After the `bench_test.go` hygiene fix, all 9 ADP-001 tags pass compliance:

- Every @MX:ANCHOR carries both @MX:REASON and @MX:SPEC ✓
- The single @MX:WARN carries @MX:REASON and @MX:SPEC ✓
- Every @MX:NOTE carries @MX:SPEC ✓
- All agent-generated tags carry the [AUTO] prefix ✓
- All tag descriptions and sub-lines are in English (consistent with `code_comments: en` in `.moai/config/sections/language.yaml`) ✓
- @MX:TODO count is zero — all RED-phase requirements promoted to GREEN ✓
- Per-file ANCHOR limit (3) not exceeded: no file carries more than 1 ANCHOR ✓
- Per-file WARN limit (5) not exceeded: `client.go` carries 1 WARN ✓

## Notes

- `parse.go::parseListing` ANCHOR uses `fan_in = 1 (Search)` — explicitly justified as invariant-bearing (NormalizedDoc field-mapping integrity gate) rather than caller-count driven. Protocol §"When to Add Tags" allows ANCHOR for `External system integration point` independent of fan_in count, since field-mapping is the canonical integration-point invariant for ADP-001: every NormalizedDoc returned to FAN-001 / IDX-001 / SYN-001 consumers passes through this single transform function.
- `internal/adapters/registry.go` carries SPEC-CORE-001-scoped tags (3 ANCHOR + 1 WARN, validated under `.moai/reports/mx-validation/SPEC-CORE-001-validation.md`) and `internal/adapters/noop/noop.go` is the SPEC-CORE-001 reference adapter — both excluded from this report.
- Performance numbers in `bench_test.go` godoc and `@MX:NOTE` are now in sync with the run-phase NFR-ADP-001 amendment (iteration 3, commit `e99d6d8`). Future SPEC-amendment changes to NFR-ADP-001 alloc target should update both locations atomically (the inline godoc/NOTE and the `### NFR-ADP-001` row in `spec.md` §4).
