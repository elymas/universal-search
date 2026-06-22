# SPEC-DOC-001 Migration Map

Generated: 2026-05-30 (manager-docs, T1 ANALYZE phase)
Strategy: ANALYZE → PRESERVE → IMPROVE (DDD)

## Page → Source → Strategy → Owner

| Target Page (EN)                                   | Source                                   | Strategy              | Owner                | Status |
| -------------------------------------------------- | ---------------------------------------- | --------------------- | -------------------- | ------ |
| content/en/introduction/what-is.mdx                | .moai/project/product.md §1-2            | PRESERVE (wrap)       | manager-docs         | T3     |
| content/en/introduction/personas.mdx               | .moai/project/product.md §3              | PRESERVE (wrap)       | manager-docs         | T3     |
| content/en/introduction/comparison.mdx             | .moai/project/product.md §5              | PRESERVE (wrap)       | manager-docs         | T3     |
| content/en/getting-started/prerequisites.mdx       | README.md Prerequisites table            | PRESERVE (wrap)       | manager-docs         | T4     |
| content/en/getting-started/installation.mdx        | README.md Quickstart                     | PRESERVE (wrap)       | manager-docs         | T4     |
| content/en/getting-started/first-query.mdx         | net-new                                  | HAND-WRITE            | manager-docs         | T4     |
| content/en/getting-started/operator-quickstart.mdx | net-new                                  | HAND-WRITE            | manager-docs         | T4     |
| content/en/end-users/cli-tour.mdx                  | net-new                                  | HAND-WRITE            | manager-docs         | T4     |
| content/en/end-users/web-ui-tour.mdx               | net-new                                  | HAND-WRITE            | manager-docs         | T4     |
| content/en/end-users/skill-claude.mdx              | net-new                                  | HAND-WRITE            | manager-docs         | T4     |
| content/en/end-users/mcp-integration.mdx           | net-new                                  | HAND-WRITE            | manager-docs         | T4     |
| content/en/operators/deployment.mdx                | README.md + net-new                      | HAND-WRITE            | manager-docs         | T4     |
| content/en/operators/auth-setup.mdx                | .moai/docs/MCP_OAUTH_SETUP.md            | PRESERVE (wrap)       | manager-docs         | T3     |
| content/en/operators/configuration.mdx             | net-new                                  | HAND-WRITE            | manager-docs         | T4     |
| content/en/operators/observability.mdx             | net-new                                  | HAND-WRITE            | manager-docs         | T4     |
| content/en/operators/korean-locale-setup.mdx       | net-new                                  | HAND-WRITE            | manager-docs         | T4     |
| content/en/operators/security.mdx                  | **FORWARD-REF** SPEC-SEC-001 placeholder | FORWARD-REF           | manager-docs         | T3     |
| content/en/reference/cli/query.mdx                 | cmd/usearch/query.go --help              | GENERATE (script)     | gen-cli-reference.sh | T5     |
| content/en/reference/cli/config.mdx                | cmd/usearch/config_cmd.go --help         | GENERATE (script)     | gen-cli-reference.sh | T5     |
| content/en/reference/cli/history.mdx               | cmd/usearch/history_cmd.go --help        | GENERATE (script)     | gen-cli-reference.sh | T5     |
| content/en/reference/cli/deep.mdx                  | cmd/usearch/deep_cmd.go --help           | GENERATE (script)     | gen-cli-reference.sh | T5     |
| content/en/reference/cli/sources.mdx               | cmd/usearch/sources_cmd.go --help        | GENERATE (script)     | gen-cli-reference.sh | T5     |
| content/en/reference/cli/login.mdx                 | cmd/usearch/login_cmd.go --help          | GENERATE (script)     | gen-cli-reference.sh | T5     |
| content/en/reference/cli/repl.mdx                  | cmd/usearch/repl.go --help               | GENERATE (script)     | gen-cli-reference.sh | T5     |
| content/en/reference/mcp.mdx                       | internal/mcpserver/tools/\*.go           | HAND-WRITE + GENERATE | manager-docs         | T4/T5  |
| content/en/reference/architecture.mdx              | .moai/project/tech.md                    | PRESERVE (wrap)       | manager-docs         | T3     |
| content/en/reference/configuration.mdx             | net-new (config sections)                | HAND-WRITE            | manager-docs         | T4     |
| content/en/reference/adapters/index.mdx            | net-new + SPEC-DOC-002 link              | HAND-WRITE (outline)  | manager-docs         | T5     |
| content/en/reference/dependencies.mdx              | docs/dependencies.md (link)              | PRESERVE (link)       | manager-docs         | T3     |
| content/en/troubleshooting/index.mdx               | net-new (>=10 entries)                   | HAND-WRITE            | manager-docs         | T4     |
| content/en/legal/licenses.mdx                      | docs/licenses/ (link)                    | PRESERVE (link)       | manager-docs         | T3     |
| content/en/legal/security.mdx                      | SECURITY.md (if present)                 | HAND-WRITE            | manager-docs         | T4     |

## KO Mirror Status (Tier-1 — V1)

| Target Page (KO)                                   | EN Source                                          | Status |
| -------------------------------------------------- | -------------------------------------------------- | ------ |
| content/ko/introduction/what-is.mdx                | content/en/introduction/what-is.mdx                | T6     |
| content/ko/introduction/personas.mdx               | content/en/introduction/personas.mdx               | T6     |
| content/ko/getting-started/prerequisites.mdx       | content/en/getting-started/prerequisites.mdx       | T6     |
| content/ko/getting-started/installation.mdx        | content/en/getting-started/installation.mdx        | T6     |
| content/ko/getting-started/first-query.mdx         | content/en/getting-started/first-query.mdx         | T6     |
| content/ko/getting-started/operator-quickstart.mdx | content/en/getting-started/operator-quickstart.mdx | T6     |
| content/ko/end-users/cli-tour.mdx                  | content/en/end-users/cli-tour.mdx                  | T6     |
| content/ko/operators/auth-setup.mdx                | content/en/operators/auth-setup.mdx                | T6     |
| content/ko/operators/korean-locale-setup.mdx       | KO-authoritative (no EN)                           | T6     |
| content/ko/operators/security.mdx                  | FORWARD-REF placeholder                            | T6     |
| content/ko/troubleshooting/index.mdx               | content/en/troubleshooting/index.mdx               | T6     |

## Forward References

Pages that reference SPEC-SEC-001 content not yet merged (PR #42):

- content/en/operators/security.mdx — placeholder only
- content/ko/operators/security.mdx — placeholder only
- NO cross-links to ops/security/\* files (do not exist on main)

## KO-Only (Authoritative)

- content/ko/operators/korean-locale-setup.mdx — Naver API key, mecab-ko, Korean-first ranking setup

## Deferred to V1.1

- Full KO translation of reference/ section (CLI, MCP, architecture)
- Automated a11y CI, Lighthouse CI, Playwright screenshot capture
- Docker container live publish (pending <org> resolution)
