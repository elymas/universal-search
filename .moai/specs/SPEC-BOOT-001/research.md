# SPEC-BOOT-001 — Research Findings

## 1. Purpose

This document preserves the team research output consumed by SPEC-BOOT-001 for future SPEC authors and run-phase implementers. It condenses upstream package facts, version pin rationale, architecture decisions, and user annotations into a single authoritative reference. It is not a re-derivation of the SPEC — the SPEC is canonical for requirements.

Downstream SPECs (SPEC-DEP-001, SPEC-OBS-001, SPEC-LLM-001, SPEC-IR-001, SPEC-ADP-001, SPEC-CLI-001, SPEC-IDX-001, SPEC-UI-001) should consult this document first when their scope intersects bootstrap decisions.

## 2. Upstream Package Facts

### A. Existing Scaffold (baseline)

- The repository currently contains only `.moai/` and `.claude/` infrastructure: 20 MoAI agents installed under `.claude/agents/moai/`, project context documents under `.moai/project/`, and configuration under `.moai/config/`.
- No `go.mod`, `requirements.txt`, `package.json`, `pyproject.toml`, `Dockerfile`, or `docker-compose.yml` exists anywhere in the tree.
- Verdict: **full greenfield**. SPEC-BOOT-001 is free to choose any directory layout, tool chain, and convention without migration concerns.

### B. gpt-researcher (Apache-2.0)

- Install: `pip install gpt-researcher`.
- Python baseline: 3.11+ (upstream refuses 3.10; tested against 3.11 and 3.12).
- Mandatory env vars: `OPENAI_API_KEY`, `TAVILY_API_KEY`.
- Optional env vars: `OPENAI_BASE_URL` (useful for LiteLLM proxy routing), `LANGCHAIN_TRACING_V2`, `LANGCHAIN_API_KEY`, `LANGCHAIN_PROJECT`.
- MCP server mode lives in a separate `gptr-mcp` repository; not consumed by SPEC-BOOT-001 but noted for SPEC-IR-001.
- Consumption model: imported as a Python library inside `services/researcher/`; wrapped behind a FastAPI health endpoint in M1.

### C. SearXNG (AGPL-3.0)

- The `searxng/searxng-docker` helper repository was archived on 2026-03-28 and is no longer maintained.
- Canonical installation documentation: `https://docs.searxng.org/admin/installation-docker.html`.
- SPEC-BOOT-001 consumes the `searxng/searxng` Docker image directly and mounts a local `deploy/searxng/settings.yml`, avoiding the archived helper.
- License posture: SearXNG is AGPL-3.0. Universal Search interacts with SearXNG across a network boundary (Docker service → HTTP API), which is a **service boundary**, not a derivative work. No fork, no bundling, no static linking. Internal self-hosted use further insulates the project from AGPL distribution triggers.
- NOTICE file documents this explicitly.

### D. STORM / knowledge-storm (MIT)

- Install: `pip install knowledge-storm`.
- Python baseline: 3.11.
- Uses `litellm` natively, which is why SPEC-BOOT-001 ships a LiteLLM gateway in compose (shared by researcher, storm, and future LLM callers).
- Built-in retrieval modules: `YouRM`, `BingSearch`, `VectorRM`, `SerperRM`, `BraveRM`. These will be **substituted** by a SearXNG-backed retriever in a later SPEC (tracked against SPEC-ADP-001 and SPEC-IR-001); M1 keeps STORM stock.

### E. Version Pins (2026-04 stable)

See §3 for the full pin matrix. Key rationale bullets:

- **Qdrant v1.16.3** (Dec 2025): includes the Windows storage corruption fix; most recent stable tag as of 2026-04-12.
- **Meilisearch v1.42.1** (Apr 14, 2026): newest stable with minor regressions in v1.43 alpha avoided.
- **PostgreSQL 16.13-alpine3.23**: 16.x LTS line with patch-level 13; alpine base keeps image small.
- **LiteLLM v1.83.7-stable.patch.1** (Apr 23, 2026): most recent stable-patch tag from BerriAI; patches known proxy timeout issue from 1.83.6.
- **Redis 7-alpine**: user-confirmed (see annotation #3) based on Asynq task queue compatibility — Asynq targets Redis 6.2+ and is production-proven on 7.x.
- **SearXNG `:latest`** (M1 only): pinning strategy deferred to SPEC-DEP-001 (Open Question #1 in the SPEC).

### F. Go Layout Conventions (2026 consensus)

- `internal/` is organized **by domain**, not by type. `internal/router/`, `internal/fanout/`, `internal/adapters/` rather than `internal/handlers/`, `internal/services/`, `internal/utils/`.
- `cmd/<binary>/main.go` stays under ~50 lines. Heavy lifting lives in `internal/app/` or equivalent; `main.go` is plumbing.
- `pkg/` is **reserved for external consumers**. If no external consumer exists, the code belongs in `internal/`. SPEC-BOOT-001 creates `pkg/client/` and `pkg/types/` only as stubs in anticipation of SPEC-CLI-001 and the public Go SDK path.
- This matches `.moai/project/structure.md` §1 verbatim; no divergence.

### G. Python Package Manager

- `uv` (astral-sh) is the 2026 default for Python monorepos with sidecar services.
- Workspace feature: a single top-level `pyproject.toml` with `[tool.uv.workspace] members = ["services/*"]` plus one `pyproject.toml` per member. Single `uv.lock` at the workspace root covers all services and enforces shared-dep resolution.
- Risk: three heterogeneous services (gpt-researcher → LangChain; STORM → knowledge-storm + litellm; embedder → sentence-transformers / BLAS) may produce resolver conflicts. Contingency documented in SPEC §10 Risks.
- CI strategy: `uv sync --frozen --package <service>` per service in matrix jobs, avoiding full workspace install when unnecessary.

### H. Next.js 16 + shadcn/ui

- `create-next-app@16 web --typescript --tailwind --app --src-dir --import-alias "@/*"` generates the App Router scaffold.
- `npx shadcn@latest init` (no longer `shadcn-ui`; package renamed) produces `components.json`.
- Node 22 LTS (user-confirmed, annotation #4): latest LTS line with long support horizon; Node 20 also supported but 22 preferred for new projects.
- pnpm is the chosen package manager (workspace + disk-efficient node_modules). `pnpm-workspace.yaml` at repo root includes `web/`.

## 3. Version Pin Matrix

| Component | Version | Released | Source / Rationale |
|-----------|---------|----------|--------------------|
| Go toolchain | 1.23.x | 2024-08 line | Latest 1.23 minor; matches `.moai/project/tech.md` §2. |
| Python | 3.11+ | 2022-10 | Constrained by gpt-researcher and knowledge-storm upstreams; tech.md to be refined from 3.12+. |
| Node.js | 22 LTS | 2024-10 | Latest LTS; user-confirmed. |
| Qdrant | v1.16.3 | 2025-12 | Windows corruption fix; newest stable. |
| Meilisearch | v1.42.1 | 2026-04-14 | Newest stable; avoids v1.43 alpha regressions. |
| PostgreSQL | 16.13-alpine3.23 | 2026-Q1 | 16.x LTS + latest patch; alpine base. |
| Redis | 7-alpine | rolling | Asynq-compatible; user-confirmed. |
| SearXNG | `:latest` (M1 only) | rolling | Deferred pin (SPEC-DEP-001); `searxng-docker` archived. |
| LiteLLM | v1.83.7-stable.patch.1 | 2026-04-23 | Most recent stable-patch; proxy timeout fix. |
| uv | latest | rolling | Workspace manager; `pip install uv` in CI. |
| pnpm | latest 9.x | rolling | Workspace + disk efficiency. |
| pre-commit | latest | rolling | `pre-commit autoupdate` on first commit. |

Lockfiles committed per NFR-BOOT-001: `go.sum`, `uv.lock`, `pnpm-lock.yaml`.

## 4. Architecture Decision Summary

| # | Decision | Rationale | tech.md Alignment |
|---|----------|-----------|-------------------|
| D1 | Go module path: `github.com/elymas/universal-search` | User-confirmed (annotation #1); `elymas` GitHub account; canonical spelling (not `univesal-search`) independent of filesystem directory. | Aligns with tech.md §2 (Go 1.23+ core). |
| D2 | Go layout: `cmd/{usearch,usearch-mcp,usearch-api}`, `internal/{router,fanout,adapters,index,llm,synthesis,auth,obs,eval}`, `pkg/{client,types}`, `proto/` at repo root | Domain-organized `internal/`; three-binary entrypoint plan; `proto/` shared across services. | Matches structure.md §1 verbatim. |
| D3 | Python: uv workspace, single top-level lock, Python 3.11+, per-service `pyproject.toml` under `services/{researcher,storm,embedder}` | 2026 monorepo standard; shared resolution; service isolation. | Triggers tech.md §2 + §7 Decision Log refinement (3.12+ → 3.11+). |
| D4 | Python service skeleton: FastAPI + `src/` layout, `GET /health → 200 {"status":"ok","service":"{name}"}`, Dockerfile from `python:3.11-slim` | Minimal viable service; uniform health probe for compose healthchecks. | Consistent with tech.md §3 (FastAPI sidecar services). |
| D5 | Node: pnpm + `create-next-app@16 web --typescript --tailwind --app --src-dir --import-alias "@/*"`, then `npx shadcn@latest init` | Official Next.js 16 scaffold; shadcn CLI rename (shadcn-ui → shadcn). | Matches tech.md §4 (Next.js 16 + shadcn/ui). |
| D6 | docker-compose: six services (Qdrant v1.16.3 :6333+:6334, Meili v1.42.1 :7700, PG 16.13-alpine3.23 :5432, Redis 7-alpine :6379, SearXNG :8080, LiteLLM v1.83.7-stable.patch.1 :4000) with `depends_on: redis → {searxng, litellm}` and healthchecks on all | All deps local-first; Redis is SearXNG + LiteLLM dependency; healthchecks enable `compose up --wait` gate. | Consistent with tech.md §5 (local-first service topology). |
| D7 | Env files: root `.env.example` covers all compose vars + global API keys; per-service `services/{name}/.env.example` for service-specific vars | Two-tier separation avoids leakage; root file is sufficient for `make compose-up`. | n/a (implementation detail). |
| D8 | CI: GitHub Actions on ubuntu-latest; Go 1.23.x, Python 3.11, Node 22 LTS; matrix {go, python × 3 services, node, lint+pre-commit}; `actions/setup-*` with built-in caches | Standard triple-language matrix; pre-commit is a separate required check. | Aligns with tech.md §6 (CI expectations). |
| D9 | Pre-commit order: `pre-commit-hooks` (trailing-whitespace, EOF fixer, check-yaml, check-added-large-files maxkb=1000) → `pre-commit-golang` (gofmt, go-vet) → `ruff-pre-commit` (ruff + ruff-format) → `mirrors-prettier` → `hadolint` → `shellcheck-py` → `yamllint`. Rev pins via `pre-commit autoupdate` on first commit. | Mechanical hooks first (fail fast); then language-specific; then meta-linters. Autoupdate avoids version drift. | n/a (convention). |
| D10 | Makefile targets: `help`, `dev`, `compose-up`, `compose-down`, `compose-logs`, `build`, `test`, `test-go`, `test-go-integration`, `test-py`, `test-node`, `lint`, `fmt`, `clean`, `install-py`, `tidy`. `dev → compose-up (wait healthy) + go run`; `test` fans out (sequential locally, parallel via `-j3` in CI) | Single source of developer UX; hides docker-compose flags; cross-platform (macOS + Linux). | Matches tech.md §6 tooling expectations. |
| D11 | License: Apache-2.0 `LICENSE` at root + `NOTICE` file documenting (a) copyright elymas, (b) SearXNG AGPL service-boundary note, (c) placeholder for future third-party attributions. `docs/licenses/` as placeholder directory. | Apache-2.0 is the MoAI default; NOTICE documents the AGPL service boundary explicitly to pre-empt compliance concerns. | Aligns with project identity. |
| D12 | Open Questions deferred: Q6 SearXNG image pin strategy (`:latest` vs dated digest), Q7 proto/ granularity (one `.proto` per service default), Q_DIR filesystem directory rename. | Non-blocking for M1; tracked in SPEC §11. | n/a. |

## 5. User Annotation Log

Confirmed by user `limbowl` on 2026-04-24:

| # | Decision | Affected REQ / Decision | Notes |
|---|----------|-------------------------|-------|
| 1 | Go module path is `github.com/elymas/universal-search` (elymas account, canonical spelling) | REQ-BOOT-001, D1 | Independent of filesystem directory `univesal-search` (typo-carrying). |
| 2 | Python baseline refined to 3.11+ (from 3.12+) | REQ-BOOT-002, D3, tech.md §2/§7 | gpt-researcher and knowledge-storm upstream constraints. Triggers post-approval tech.md edit. |
| 3 | Task queue backing: `redis:7-alpine` | D6 (compose) | Asynq-proven on Redis 7; avoids latest-tag drift by pinning to 7-alpine. |
| 4 | Node LTS version: Node 22 LTS | REQ-BOOT-007, D5, D8 | Latest LTS line; Node 20 also works but 22 preferred for new projects. |

## 6. References

All URLs verified as of 2026-04-24 research pass. If a link 404s during run-phase implementation, prefer the cached content in this research document over a speculative re-fetch.

### Upstream Documentation

- Go 1.23 release notes: `https://go.dev/doc/go1.23`
- uv workspaces: `https://docs.astral.sh/uv/concepts/workspaces/`
- gpt-researcher: `https://github.com/assafelovic/gpt-researcher` (Apache-2.0)
- gptr-mcp (separate repo): `https://github.com/assafelovic/gptr-mcp`
- knowledge-storm (STORM): `https://github.com/stanford-oval/storm` (MIT)
- SearXNG docs: `https://docs.searxng.org/admin/installation-docker.html`
- SearXNG repo: `https://github.com/searxng/searxng` (AGPL-3.0)
- SearXNG-docker (archived 2026-03-28): `https://github.com/searxng/searxng-docker`
- Qdrant: `https://qdrant.tech` — v1.16.3 release: `https://github.com/qdrant/qdrant/releases/tag/v1.16.3`
- Meilisearch: `https://www.meilisearch.com` — v1.42.1 release: `https://github.com/meilisearch/meilisearch/releases/tag/v1.42.1`
- PostgreSQL 16: `https://www.postgresql.org/docs/16/`
- Redis 7: `https://redis.io/docs/latest/operate/oss_and_stack/install/`
- LiteLLM: `https://docs.litellm.ai` — stable patch releases: `https://github.com/BerriAI/litellm/releases`

### Frontend

- Next.js 16 App Router: `https://nextjs.org/docs/app`
- `create-next-app` CLI: `https://nextjs.org/docs/app/api-reference/create-next-app`
- shadcn/ui: `https://ui.shadcn.com/docs/installation/next`
- pnpm workspaces: `https://pnpm.io/workspaces`
- Tailwind CSS: `https://tailwindcss.com/docs`

### Tooling

- pre-commit framework: `https://pre-commit.com`
- pre-commit-hooks: `https://github.com/pre-commit/pre-commit-hooks`
- pre-commit-golang: `https://github.com/dnephin/pre-commit-golang`
- ruff-pre-commit: `https://github.com/astral-sh/ruff-pre-commit`
- mirrors-prettier: `https://github.com/pre-commit/mirrors-prettier`
- hadolint: `https://github.com/hadolint/hadolint`
- shellcheck-py: `https://github.com/shellcheck-py/shellcheck-py`
- yamllint: `https://github.com/adrienverge/yamllint`
- editorconfig-checker: `https://github.com/editorconfig-checker/editorconfig-checker`
- golangci-lint: `https://golangci-lint.run`

### GitHub Actions

- `actions/checkout@v4`: `https://github.com/actions/checkout`
- `actions/setup-go@v5`: `https://github.com/actions/setup-go`
- `actions/setup-python@v5`: `https://github.com/actions/setup-python`
- `actions/setup-node@v4`: `https://github.com/actions/setup-node`

### Project Context (internal)

- Product scope: `.moai/project/product.md`
- Repo layout: `.moai/project/structure.md`
- Tech stack + decision log: `.moai/project/tech.md`
- Milestone roadmap (M1-M9): `.moai/project/roadmap.md`
- Competitive landscape: `.moai/project/research.md`

---

*End of research.md — consult this document before raising new bootstrap questions in downstream SPECs.*
