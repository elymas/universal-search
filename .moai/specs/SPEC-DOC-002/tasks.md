# SPEC-DOC-002 Tasks — atomic decomposition

Status: draft (manager-strategy Phase 1 analysis, 2026-05-31)
Methodology: DDD (ANALYZE-PRESERVE-IMPROVE) · Harness: standard
Recommendation: **needs-plan-auditor-first** — 4 substantive spec corrections required (see Blockers/Amendments below) before run-phase unlock.

## Pre-run amendments (plan-auditor must ratify)

These are spec/code contradictions found by grep-verification, not preferences. They alter REQ acceptance, so they belong in the annotation/plan-auditor cycle, not run phase.

- **A1 — `hn` page vs `hackernews` SourceID.** `internal/adapters/hn/hn.go:101` has `SourceID: "hackernews"`. REQ-ADPDOC-001 lists the page as `hn.mdx` AND mandates "filename MUST match `Capabilities().SourceID`". These conflict. Decide: page `hackernews.mdx` (filename=SourceID, rule intact) OR keep `hn.mdx` with an explicit adapter→page mapping (rule relaxed). Affects REQ-ADPDOC-001, AC-001, EC-003, completeness script.
- **A2 — social `Capabilities()` is switch-dispatch, not a method literal.** `social.go:132` returns `blueskyCapabilities()` / `xCapabilities()` (helper funcs at `social.go:144`/`:164`). REQ-ADPDOC-007 describes "the `Capabilities()` method ... struct literal" and a per-file `{adapter}.go` walk. There is no `bluesky.go`/`x.go`. The AST tool must (a) walk `social.go`, (b) follow the two helper funcs, (c) emit 2 JSONs. Tractable, but the described mechanism is inaccurate.
- **A3 — X is a disabled v0 stub, not "degraded mode".** `xCapabilities()` Notes: "DISABLED in v0. Set USEARCH_X_ENABLED=true ... no live path wired." `RateLimitPerMin:0, DefaultMaxResults:0`. Spec frames X as `alpha`/degraded; reality is compile-time-gated stub. Decide x.mdx framing + whether X belongs in the V1 catalog/badge feed at all.
- **A4 — EVAL-002 has no `adapter-status.json` export and no `lifecycle` field.** EVAL-002 is Prometheus/Grafana: it ships `usearch:adapter_success_rate_7d` recording rule, `deploy/grafana/dashboards/adapter-reliability.json`, `/admin/health/adapters` endpoint, and a `usearch_adapter_health_status` gauge (1.0/0.5/0.0) — NOT the {lifecycle, successRate7d, verifiedAt} JSON DOC-002 D5 assumes. The 4-tier `lifecycle` taxonomy is net-new to DOC-002; EVAL-002 does not "own the lifecycle field" (§1.3 over-claims). Status-badge feed = forward-reference. Plan already has graceful degradation (static initial JSON) — confirm that as the V1 path.

Minor stale ref (auto-resolves): open Q §8.6 says "repo has no `tools/` directory" — it exists (`tools/claude-skill`). `tools/gen-adapter-ref/` is a clean sibling.

## Task status: ALL COMPLETE (2026-05-31 run phase)

## Task list (≤10 atomic, DDD-ordered, infra-before-content)

| ID | Task | REQ mapping | Depends on | Acceptance (done when) |
|----|------|-------------|------------|------------------------|
| T1 | DDD ANALYZE: produce adapter→{sourceID, page-name, .go file, Capabilities func, line, auth, rate-mechanism} registry table for all 10 adapters; mine ≥3 troubleshooting seeds + status rosetta seed per adapter from `internal/adapters/*/research.md`; capture DOC-001 surface intersection (theme/mdx-components, lychee.toml, placeholder index.mdx, surface-comparison, deployment-helm, check-bilingual-coverage.sh, ko/CONTRIBUTING.md). | (analysis basis for all) | A1–A4 ratified | `analyze-report.md` exists with 10-row registry incl. hackernews/bluesky/x special cases; troubleshooting + rosetta seeds per adapter |
| T2 | Build `tools/gen-adapter-ref/` Go tool (go/parser AST extraction of 5 Capabilities fields) with adapter registry handling the hackernews page-map + social helper-func resolution; `extract_test.go` golden + edge tests, ≥85% cov. Emit 10 `_generated/{page}.capabilities.json`. | REQ-ADPDOC-007 (partial), A1, A2 | T1 | `go test ./tools/gen-adapter-ref/...` ≥85%; running tool yields 10 JSONs matching source verbatim incl. bluesky=600, x=0 |
| T3 | `scripts/gen-adapter-reference.sh` (+ `--check` diff mode) + commit baseline `_generated/*.capabilities.json`; add `gen-adapter-ref-drift` CI job to `docs.yml`. | REQ-ADPDOC-007, NFR-ADPDOC-001 | T2 | drift job green on baseline; FAILS when a committed JSON or a real `RateLimitPerMin` is altered; runtime ≤60s |
| T4 | `scripts/check-doc-credentials.sh` (+ shared `.docs-credentials-patterns.toml`, aligned to SEC-001 `.gitleaks.toml`) + `check-doc-credentials` CI job. | REQ-ADPDOC-018 | — | injecting `ghp_`-shaped PAT fails; `<YOUR_GITHUB_TOKEN>` passes; clean baseline 0 matches |
| T5 | 3 MDX components: `StatusBadge.tsx` (build-time JSON import, 4-tier, fallback), `CapabilitiesTable.tsx` (5 fields + source footer, no hand-override), `AdapterCatalog.tsx` (filterable). Unit tests ≥85% incl. taxonomy boundary (0.949 beta / 0.950 stable) + filter + fallback. Register in `docs/mdx-components.tsx`. | REQ-ADPDOC-005/006/008/003 | T2 (JSON shape) | component tests pass; render harness shows JSON-driven badge + table; boundary cases asserted |
| T6 | `errors.mdx` (5 Category subsections, verified against `pkg/types/errors.go`) + `scripts/check-adapter-page-completeness.sh` full impl (10-section order, ≥50-char/section per NFR-ADPDOC-004, filename↔SourceID-map) + `adapter-page-completeness` + `adapter-status-staleness` CI jobs + `lychee.toml` provider allowlist (NFR-ADPDOC-005). | REQ-ADPDOC-004/002, NFR-ADPDOC-002/003/004/005 | T1, T3 | errors.mdx 5 H3; completeness job passes on errors+baseline; lychee allowlist active |
| T7 | EN content batch 1 (no-auth, non-Korean): `reddit.mdx`, `hackernews|hn.mdx`, `arxiv.mdx`, `youtube.mdx`, `searxng.mdx` + `_meta.json`. All 10 sections, `<CapabilitiesTable>`, rate-mechanism per research §1.4, ≥3 troubleshooting, ≥4 Related links. | REQ-ADPDOC-001/002/010(no-auth)/012/013/014/015/016 | T5, T6 | 5 pages + errors pass completeness CI; credential lint clean; lychee internal 100% |
| T8 | EN content batch 2 (auth + social + Korean): `github.mdx`, `naver.mdx`, `bluesky.mdx`, `x.mdx`, `koreanews.mdx`. github/naver 5-field auth Setup; naver+koreanews 3-line Korean summary + cross-link (no dup, D6); bluesky/x shared-impl callout; koreanews ≥5 troubleshooting; x framing per A3. Modify `deployment-helm.mdx` anchors. | REQ-ADPDOC-009/010(auth)/011/013/014 | T7 | 5 pages pass completeness; github 422 + naver 401 rosetta rows; x reflects disabled-stub reality; helm anchors resolve |
| T9 | `index.mdx` (`<AdapterCatalog>`, replaces placeholder) + KO Tier-1 4 pages (`index/naver/koreanews/errors`) + `ko/_meta.json` + native-reviewer log in `ko/CONTRIBUTING.md`. EVAL-002 status feed: ship static `adapter-status.json` + `adapter-status.schema.json` (A4 fallback). | REQ-ADPDOC-003/017, REQ-ADPDOC-006, NFR-ADPDOC-006 | T8 | catalog 10 rows; 4 KO files + reviewer entries; static status JSON validates against schema |
| T10 | DOC-001 coordination: extend `check-bilingual-coverage.sh` (Tier-1 required / Tier-2 excluded), `surface-comparison.mdx` cross-links; full `docs.yml` E2E (all 9 jobs green, combined ≤6 min); pre-submission self-review. | REQ-ADPDOC-017, NFR-ADPDOC-001/002 | T9 | all CI green; deleting a KO Tier-1 fails, deleting a Tier-2 EN does not; staging renders all 16 pages |

Sequencing: T1→T2→T3 and T4 (parallel) → T5,T6 → T7 → T8 → T9 → T10. Infra (drift/lint/components) precedes content per plan principle 2.

## Notes
- "80 REQ/NFR" (orchestrator count) ≠ spec's 17 EARS REQ + 6 NFR + 18 AC. Decomposition tracks the 23 REQ/NFR.
- Drift detection IS feasible: all 5 extracted Capabilities fields are static literals in every adapter (verified arxiv/github/naver/koreanews/searxng/hn/reddit/bluesky/x). Machine-readable source of truth exists.
- Code-coverage 85% applies only to the Go tool + scripts + React components. MDX content gated by completeness, not coverage.
