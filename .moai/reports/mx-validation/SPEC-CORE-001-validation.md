# @MX Tag Validation Report — SPEC-CORE-001

Generated: 2026-04-26
Commit: f728aa2

## Summary

| Tag Type | Count | Compliance |
|----------|-------|------------|
| @MX:ANCHOR | 5 | All carry @MX:REASON ✓ All carry @MX:SPEC ✓ |
| @MX:WARN | 1 | Carries @MX:REASON ✓ |
| @MX:NOTE | 6 | No mandatory sub-lines required |
| @MX:TODO | 0 | All RED tests promoted to GREEN |

## Tag Inventory

### @MX:ANCHOR — 5 tags

**1. pkg/types/adapter.go:23**

```
// @MX:ANCHOR: [AUTO] Adapter contract; callers: every M3 adapter, registry,
//   FAN-001 fanout, IR-001 router, tests
// @MX:REASON: fan_in >= 12; sole boundary between source-specific code and
//   orchestration layer
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes

**2. internal/adapters/registry.go:78**

```
// @MX:ANCHOR: [AUTO] Adapter registry; callers: cmd mains, FAN-001 fanout,
//   IR-001 router, tests
// @MX:REASON: fan_in >= 3; sole sanctioned source of Adapter instances at
//   runtime
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes

**3. internal/adapters/registry.go:103**

```
// @MX:ANCHOR: [AUTO] Adapter registration entry point; callers: cmd mains,
//   integration tests, FAN-001 startup
// @MX:REASON: fan_in >= 3 across runtime + tests; the wrappedAdapter is only
//   created here
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes

**4. internal/adapters/registry.go:143**

```
// @MX:ANCHOR: [AUTO] Adapter lookup; callers: FAN-001 fanout, IR-001 router,
//   tests
// @MX:REASON: fan_in >= 3; every per-name dispatch flows through Get
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes

**5. pkg/types/errors.go:129**

```
// @MX:ANCHOR: [AUTO] Error classifier; callers: registry wrappedAdapter, FAN-001 retry policy, tests
// @MX:REASON: fan_in >= 3; sole canonical mapping from error to Category
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes

### @MX:WARN — 1 tag

**1. internal/adapters/registry.go:114**

```
// @MX:WARN: [AUTO] Duplicate-name detection is a load-bearing invariant —
//   returning *RegistryError instead of silently overwriting is intentional
// @MX:REASON: callers may expect overwrite semantics from common map-style
//   APIs; the explicit error contract prevents accidental adapter shadowing
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: present
- Comments in English: yes

### @MX:NOTE — 6 tags

**1. pkg/types/query.go:14**

```
// @MX:NOTE: [AUTO] Filters is opaque to the registry; each adapter interprets
//   its own key/value pairs
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- Comments in English: yes

**2. pkg/types/normalized_doc.go:35**

```
// @MX:NOTE: [AUTO] URL must be canonical (no tracking params). Hash is
//   content-only (SourceID|URL|Title|Body); Metadata is excluded to prevent
//   hash drift on enrichment
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- Comments in English: yes

**3. pkg/types/normalized_doc.go:87**

```
// @MX:NOTE: [AUTO] Hash is content-only (SourceID|URL|Title|Body) and
//   cached here; callers MUST NOT recompute independently to avoid drift
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- Comments in English: yes

**4. pkg/types/capabilities.go:34**

```
// @MX:NOTE: [AUTO] Field additions are non-breaking; the Intent Router
//   (IR-001) reads only the fields it needs and ignores unknown fields
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- Comments in English: yes

**5. pkg/types/errors.go:16**

```
// @MX:NOTE: [AUTO] These four sentinels are the canonical category targets for
//   errors.Is. Wrap raw HTTP/network errors in *SourceError to expose category
//   without leaking source-specific details.
```

- [AUTO] prefix: present
- @MX:SPEC: absent (optional for NOTE — not a defect)
- Comments in English: yes

**6. pkg/types/errors.go:171**

```
// @MX:NOTE: [AUTO] Canonical mapping from error to Prometheus outcome label.
//   Enum is bounded; adding a new label value requires updating NFR-CORE-002
//   allowlist test.
// @MX:SPEC: SPEC-CORE-001
```

- [AUTO] prefix: present
- Comments in English: yes

## Defects: None

All 12 tags pass compliance checks:

- Every @MX:ANCHOR carries both @MX:REASON and @MX:SPEC
- The single @MX:WARN carries @MX:REASON
- All agent-generated tags carry [AUTO] prefix
- All tag descriptions and sub-lines are in English (consistent with `code_comments: en` in language.yaml)
- @MX:TODO count is zero — all RED-phase requirements promoted to GREEN

## Notes

- `errors.go:16` @MX:NOTE intentionally omits @MX:SPEC; @MX:SPEC is optional for NOTE tags per mx-tag-protocol.md
- `registry.go` carries 3 ANCHOR tags, at the per-file limit of 3 defined in mx.yaml defaults; no demotion needed
