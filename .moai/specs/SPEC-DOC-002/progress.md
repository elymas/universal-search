
## 2026-05-31 — manager-strategy Phase 1 (analysis only, no code)

Recommendation: needs-plan-auditor-first. Harness: standard (confirmed).
tasks.md written: 10 atomic tasks (T1-T10), DDD infra-before-content.

Reality-check stale refs found (grep-verified):
- A1: hn page vs SourceID="hackernews" (hn.go:101) — breaks REQ-ADPDOC-001 filename=SourceID rule + AC-001/EC-003.
- A2: social Capabilities() is switch-dispatch (social.go:132) calling bluesky/xCapabilities() helpers; no bluesky.go/x.go file. Drift tool must follow helpers + walk social.go.
- A3: X is DISABLED v0 stub (xCapabilities Notes, RateLimit=0) — not "degraded/alpha" as spec frames.
- A4: EVAL-002 exports NO adapter-status.json and has NO lifecycle field (Prometheus/Grafana + /admin/health/adapters gauge 1/0.5/0). Status-badge feed = forward-ref; static JSON fallback is the V1 path.
- Minor: open Q §8.6 stale — tools/ dir exists (tools/claude-skill).

Verified OK (no stale ref):
- Capabilities 5 target fields all static literals across 10 adapters → drift detection FEASIBLE.
- errors.go Category enum = spec's 5 values (match).
- DOC-001 site is real: Nextra v4.6.1 + Next 16.2.6, content/{en,ko}/reference/adapters/index.mdx placeholder present, status: implemented.

Acceptance progress: 0/18 AC (analysis phase). Errors delta: n/a.
