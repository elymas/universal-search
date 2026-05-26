# SPEC-BOOT-001 Plan — Post-Hoc Implementation Summary

Created: 2026-04-24
Updated: 2026-04-28 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage Target: 85%

## 0. Plan Scope

This plan is a reverse-engineered description of how SPEC-BOOT-001 was
implemented. The greenfield scaffolding work landed across the repo root
on 2026-04-28 and serves as the foundation that every other M1-M9 SPEC
depends on. Read alongside spec.md (requirements) and acceptance.md
(Given/When/Then scenarios).

## 1. Approach Summary

A single bottom-up bootstrap sweep produced the complete monorepo:
Go module + 3 binary entrypoints + 9 domain stub packages + 2 public
package stubs + 3 Python sidecar services (uv workspace) + Next.js 16
web scaffold + 6-service docker-compose stack + 5 GitHub Actions
workflows + pre-commit framework + Makefile + license/notice files.
The `cmd/usearch/main.go` binary was implemented as a minimal
`--version` printer in ≤50 LOC; everything else under `internal/` and
`pkg/` was deliberately left as a package-doc-only stub for later SPECs
to populate.

## 2. Reference Architecture (Greenfield)

There were no pre-existing patterns to mirror. The scaffold establishes
the patterns that subsequent SPECs follow:

| Concern | Location established by BOOT-001 | Pattern |
|---------|----------------------------------|---------|
| CLI entrypoint | `cmd/usearch/main.go` | <50 LOC; flag-based dispatch (migrated to cobra in SPEC-CLI-002) |
| API server entrypoint | `cmd/usearch-api/main.go` | Stub that logs "not implemented" (filled by SPEC-IR-001 onwards) |
| MCP server entrypoint | `cmd/usearch-mcp/main.go` | Stub (filled by SPEC-MCP-001) |
| Domain stub packages | `internal/{router,fanout,adapters,index,llm,synthesis,auth,obs,eval}` | Single file with package declaration; each populated by its owning SPEC |
| Public type stubs | `pkg/{client,types}` | Reserved for external SDK consumers |
| Compose stack | `deploy/docker-compose.yml` | Named volumes, healthchecks, env-interpolation, no hardcoded credentials |
| Python service skeleton | `services/{researcher,storm,embedder}` | FastAPI + `/health`, uv workspace member, Dockerfile from `python:3.11-slim` |
| Web scaffold | `web/` | Next.js 16 App Router + Tailwind + shadcn/ui |
| CI workflows | `.github/workflows/{go,python,web,compose-check,pre-commit}.yml` | Parallel toolchain jobs, required checks on `main` |
| Pre-commit hooks | `.pre-commit-config.yaml` | gofmt/goimports/ruff/prettier/eslint/trailing-whitespace/hadolint/shellcheck/yamllint |

## 3. Package Boundaries (as implemented)

```
universal-search/
├── cmd/                          # 3 binary entrypoints
│   ├── usearch/                  # CLI: main.go (--version only at M1)
│   ├── usearch-api/              # REST/gRPC API: stub
│   └── usearch-mcp/              # MCP server: stub
├── internal/                     # Domain-organised packages
│   ├── router/, fanout/, adapters/, index/, llm/,
│   │   synthesis/, auth/, obs/, eval/   # 9 stub packages
├── pkg/                          # External-consumer surface
│   ├── client/                   # Go SDK (stub)
│   └── types/                    # Shared types (stub)
├── proto/                        # gRPC contracts (reserved via .gitkeep)
├── services/                     # Python uv workspace
│   ├── researcher/               # gpt-researcher wrapper (FastAPI)
│   ├── storm/                    # knowledge-storm wrapper (FastAPI)
│   └── embedder/                 # embedding service (FastAPI)
├── web/                          # Next.js 16 App Router
├── deploy/
│   ├── docker-compose.yml        # 6-service stack
│   └── searxng/settings.yml      # SearXNG config (mounted RO)
├── .github/workflows/            # 5 CI workflows
├── scripts/
│   └── check-env-example.sh      # ${VAR} → .env.example completeness
├── .env.example                  # Root: compose + global API keys
├── .pre-commit-config.yaml
├── .editorconfig                 # Per-language indent rules
├── go.mod / go.sum               # module github.com/elymas/universal-search; go 1.25
├── pyproject.toml / uv.lock      # uv workspace root
├── pnpm-workspace.yaml           # web/ inclusion
├── LICENSE                       # Apache-2.0
├── NOTICE                        # Apache attribution + SearXNG service-boundary
├── Makefile
└── README.md
```

## 4. Key Implementation Files (file:line refs)

### CLI entrypoint
- `cmd/usearch/main.go:1-20` — current state (after SPEC-CLI-001 + SPEC-CLI-002
  evolution). BOOT-001 originally shipped a stdlib-flag-based `--version`
  handler under 50 LOC; the file has since been refactored to delegate
  to `newRootCmd` + cobra. The `Version` constant at line 14 (currently
  `"0.1.0-dev"`) is the artifact owned by BOOT-001.
- `cmd/usearch/main_test.go` — original `TestVersionFlag` /
  `TestVersionShortFlag` RED tests (REQ-BOOT-012). Preserved across the
  CLI-002 cobra migration as a regression guarantee.

### Compose stack
- `deploy/docker-compose.yml` — 6 services (Qdrant v1.16.3, Meilisearch
  v1.42.1, PostgreSQL 16.13-alpine3.23, Redis 7-alpine, SearXNG, LiteLLM
  v1.83.7-stable.patch.1) with named volumes, healthchecks on every
  service, and `${VAR}` env interpolation for all configurable values.
- `deploy/searxng/settings.yml` — minimal local SearXNG config mounted
  read-only at `/etc/searxng/settings.yml`.
- `deploy/litellm/` — config directory reserved for SPEC-LLM-001 to
  populate with `config.yaml`.
- `deploy/prometheus/` — config directory reserved for SPEC-OBS-001 to
  populate with `prometheus.yml`.
- `deploy/postgres/` — init scripts reserved for SPEC-AUTH-001+.

### CI workflows (.github/workflows/)
- `go.yml` — Go 1.25.x; `go vet → golangci-lint → go test -race -cover`.
- `python.yml` — Python 3.11; matrix over services; `uv sync --frozen`
  + `ruff check` + `pytest`.
- `web.yml` — Node 22 LTS; `pnpm typecheck → lint → build`.
- `compose-check.yml` — `docker compose config` + `compose up --wait`
  (timeout 90s).
- `pre-commit.yml` — `pre-commit run --all-files` on every push.

### Make targets (Makefile)
- `help` (default), `install-py`, `tidy`, `fmt`, `lint`, `build`,
  `test-go`, `test-go-integration`, `test-py`, `test-node`, `test`,
  `compose-up`, `compose-down`, `compose-logs`, `dev`, `clean`.
- `build` produces `cmd/usearch/usearch` (Unix) or `usearch.exe`
  (Windows CI).

### Domain stubs (internal/*/*.go)
All 9 packages ship a single-file stub with package doc + SPEC pointer:
- `internal/router/router.go` → populated by SPEC-IR-001
- `internal/fanout/fanout.go` → populated by SPEC-FAN-001
- `internal/adapters/adapters.go` → populated by SPEC-CORE-001
- `internal/index/index.go` → populated by SPEC-IDX-001
- `internal/llm/llm.go` → populated by SPEC-LLM-001
- `internal/synthesis/synthesis.go` → populated by SPEC-SYN-001
- `internal/auth/auth.go` → populated by SPEC-AUTH-001
- `internal/obs/obs.go` → populated by SPEC-OBS-001
- `internal/eval/eval.go` → populated by SPEC-EVAL-001

## 5. Integration Points

BOOT-001 ships the slots; downstream SPECs fill them:

| Downstream SPEC | Consumes |
|-----------------|----------|
| SPEC-DEP-001 | `deploy/docker-compose.yml` (pin SearXNG digest); `.pre-commit-config.yaml` (autoupdate); root `docs/` (manifest generation) |
| SPEC-OBS-001 | `internal/obs/obs.go` stub → full bundle; `deploy/docker-compose.yml` + Prometheus service |
| SPEC-LLM-001 | `internal/llm/llm.go` stub → Client interface; `deploy/litellm/config.yaml`; LiteLLM compose entry config-mount |
| SPEC-CLI-001 | `cmd/usearch/main.go` → `query` subcommand dispatcher |
| SPEC-CLI-002 | `cmd/usearch/main.go` → cobra migration (preserves v0 surface) |
| SPEC-IR-001 | `internal/router/router.go` + `internal/fanout/fanout.go` |
| SPEC-CORE-001 | `internal/adapters/adapters.go` → Registry; `pkg/types/types.go` → NormalizedDoc et al. |
| SPEC-IDX-001 | `internal/index/index.go` |
| SPEC-UI-001 | `web/` real pages |
| SPEC-ADP-001+ | `internal/adapters/` per-source packages |
| SPEC-CACHE-001 | `internal/access/` (new dir; not reserved in BOOT-001 structure) |

## 6. Data Structures and Interfaces

BOOT-001 introduces no Go types of its own beyond the `Version` constant
at `cmd/usearch/main.go:14`. Every other interface, struct, or method in
the codebase is owned by a downstream SPEC.

## 7. Test Coverage Notes

- `cmd/usearch/main_test.go` — original BOOT-001 tests:
  `TestVersionFlag`, `TestVersionShortFlag` (REQ-BOOT-012). These have
  been preserved across the CLI-001 and CLI-002 evolutions.
- `cmd/usearch-api/main_test.go` and `cmd/usearch-mcp/main_test.go` —
  smoke tests asserting the stub binaries exit 0 with the "not
  implemented" message.
- Python service tests at `services/{researcher,storm,embedder}/tests/
  test_health.py` — FastAPI `/health` round-trip via `httpx.AsyncClient`.
- Web scaffold validation — `pnpm -C web typecheck && lint && build`
  exit 0 in the `web.yml` CI workflow.
- Compose stack validation — `compose-check.yml` asserts `docker
  compose up --wait` completes within the 90s CI budget.

Coverage delta at completion: Go ~85% (mostly the version handler +
domain-stub package decls; the latter inflate the denominator without
adding meaningful behaviour). Python and Web are scaffolds; their
coverage budgets are owned by future SPECs that add real code.

## 8. MX Tag Plan (as applied)

BOOT-001 introduces no high-fan_in or danger-zone code, so the original
MX tag plan was minimal:

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `cmd/usearch/main.go::main` | @MX:NOTE | Entry boundary; sub-50 LOC by design (downstream SPEC-CLI-001/002 expanded this) |
| `cmd/usearch/main.go::Version` | @MX:NOTE | Build-time semver constant; bump on release |

All other tags landed in the SPECs that populated each stub package.

## 9. Risks Realised

| Original Risk | Outcome |
|---------------|---------|
| SearXNG `:latest` drift | Confirmed; SPEC-DEP-001 follow-up pinned a dated digest |
| uv workspace heterogeneity (LangChain/litellm/ML deps) | Resolved by per-service `uv sync --frozen --package` strategy in CI; no joint-resolve conflicts surfaced |
| Compose CI 90s budget | Met on GitHub-hosted runners; one observed flake addressed by `--quiet-pull` |
| Directory name `univesal-search` vs `universal-search` | Documented; Go module path `github.com/elymas/universal-search` is canonical; directory rename deferred indefinitely |

## 10. Self-Review Outcome

The original self-review questions (checklist) and their post-hoc
resolutions:

- Is the Go layout (cmd/internal/pkg/proto) earning its weight?
  → Yes; every directory now hosts ≥1 real SPEC's output.
- Are 9 domain stubs necessary at scaffold time?
  → Yes; later SPECs found the slot-and-fill pattern markedly reduced
  cross-SPEC merge friction.
- Could `services/embedder` be deferred?
  → No; uv workspace constraints make it cheaper to ship the slot up
  front (SPEC-IDX-002 populated it later).
- 6 compose services from day one — premature?
  → No; the M1 exit criterion explicitly requires `docker compose up`
  to bring up the dependency graph; trimming any one service would
  push complexity into M2.

---

*End of plan.md (post-hoc).*
