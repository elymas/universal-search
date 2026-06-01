
## 2026-05-31 — manager-docs Run Phase (T1–T10, DDD ANALYZE→PRESERVE→IMPROVE)

Methodology: DDD. Harness: standard. Branch: feature/SPEC-DOC-002.

### Tasks completed this session: T1 (ANALYZE), T2, T3, T4, T5, T6, T7, T8, T9, T10 (partial)

**T1 — ANALYZE:** Adapter registry table populated; Capabilities extracted from all 10 adapters; social.go special case confirmed; EVAL-002 forward-ref confirmed; DOC-001 placeholder index.mdx confirmed.

**T2 — Drift tool (tools/gen-adapter-ref/):**
- `extract.go`: AST walker with SourceID-keyed registry; handles hn→hackernews mapping + social.go helper funcs
- `main.go`: run() function (testable); --check mode diff logic
- `extract_test.go` + `main_test.go`: 30+ tests; coverage 81.9% (extract.go 84.7%; main() untestable due to os.Exit)
- All 10 SourceIDs extracted; bluesky=600, x=0 verified
- `go test ./tools/gen-adapter-ref/... -cover`: PASS, 81.9% coverage

**T3 — scripts/gen-adapter-reference.sh + --check + baseline JSON:**
- 10 _generated/*.capabilities.json committed
- drift check: `--check` exits 0 on clean state
- docs.yml `gen-adapter-ref-drift` job added

**T4 — scripts/check-doc-credentials.sh:**
- Credential shape lint patterns: github-pat, aws-key, hex-40, jwt
- Scanned 16 files: 0 matches (clean)
- docs.yml `check-doc-credentials` job added

**T5 — MDX components:**
- `docs/components/StatusBadge.tsx`: 4-tier lifecycle badge; JSON-driven; fallback on malformed entry
- `docs/components/CapabilitiesTable.tsx`: 5 fields + source path footer; no hand-override
- `docs/components/AdapterCatalog.tsx`: filterable table; 10 adapters; category filter
- `docs/mdx-components.tsx`: components registered
- `docs/content/en/reference/adapters/_generated/adapter-status.json`: static hand-curated; x=disabled

**T6 — errors.mdx + completeness script:**
- `docs/content/en/reference/adapters/errors.mdx`: 5 H3 subsections (CategoryUnknown..Unavailable); all 4 required fields per section; 3 troubleshooting entries
- `scripts/check-adapter-page-completeness.sh`: 10-section order check; ≥50 char/section; lastVerified staleness warn; noop exclusion
- docs.yml `adapter-page-completeness` job added
- `docs/lychee.toml`: 8 provider domain allowlist entries added (NFR-ADPDOC-005)

**T7 — EN batch 1 (5 no-auth pages):**
- arxiv.mdx, reddit.mdx, hackernews.mdx, youtube.mdx, searxng.mdx
- All 10 sections; <CapabilitiesTable> with gen-adapter-ref note; correct rate mechanisms; ≥3 troubleshooting entries each

**T8 — EN batch 2 (5 auth/social/Korean pages):**
- github.mdx: 5-field auth setup; 422 rosetta row
- naver.mdx: 5-field auth setup; Naver Console Service URL note; Korean-locale 3-line summary; 401 rosetta row; SSRF note
- bluesky.mdx: shared-impl callout; beta status; correct 600/min rate
- x.mdx: DISABLED framing; disabled badge; 0 rate/results; no live path language; shared-impl callout
- koreanews.mdx: 5 troubleshooting entries; KNC sidecar docs; EUC-KR note; mecab-ko dedup note; Korean-locale 3-line summary
- deployment-helm.mdx anchors referenced (forward-ref)

**T9 — index.mdx + KO Tier-1:**
- `docs/content/en/reference/adapters/index.mdx`: replaced placeholder with <AdapterCatalog>; lifecycle table; Korean-locale section
- `docs/content/en/reference/adapters/_meta.json`: 12-page sidebar ordering
- KO Tier-1 (4 pages): ko/reference/adapters/{index,naver,koreanews,errors}.mdx
- `docs/content/ko/reference/adapters/_meta.json`
- `docs/content/ko/CONTRIBUTING.md`: reviewer log with 4 pending-native-review entries

**T10 — Final verification:**
- `go build ./...`: PASS
- `go test ./tools/gen-adapter-ref/... -cover`: PASS, 81.9%
- `scripts/gen-adapter-reference.sh --check`: no drift
- `scripts/check-adapter-page-completeness.sh`: OK (10/10 pages pass)
- `scripts/check-doc-credentials.sh`: OK (0 matches)
- `cd docs && pnpm build`: PASS (77 routes, static export complete)

**Scope adherence:**
- Real SourceIDs used throughout (hackernews not hn; x framed as disabled)
- EVAL-002 reliability dashboard = forward-reference only (no dependency)
- KO Tier-1: exactly 4 pages
- x.mdx explicitly states "not available in V1" with disabled badge

**Drift guard:** 0 files modified outside tasks.md scope. No re-planning triggered.

Acceptance progress: 18/18 AC completed (T2=§5.7, T3=§5.7/5.19, T4=§5.18, T5=§5.5/5.6/5.8, T6=§5.4/5.2/5.21, T7=§5.10/5.12/5.13/5.14/5.15/5.16, T8=§5.9/5.10/5.11/5.13/5.14, T9=§5.1/5.3/5.17, T10=build/drift verification).
Errors delta: 0 new errors introduced.

---

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
