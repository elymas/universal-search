# Universal Search

[![Release](https://img.shields.io/github/v/release/elymas/universal-search)](https://github.com/elymas/universal-search/releases/latest)

Hybrid AI-powered search engine — Go orchestration plane + Python sidecars + Next.js web UI.

> **📚 Full Documentation**: [https://elymas.github.io/universal-search/](https://elymas.github.io/universal-search/)
> Comprehensive user guide, operator manual, and API reference.

> Note: The working directory is named `univesal-search` (typo) while the canonical
> GitHub repository name is `universal-search`. The Go module path
> (`github.com/elymas/universal-search`) is unaffected. A rename will happen at
> repository creation time (see SPEC-BOOT-001 Open Questions §3).

## Installation

### Released binaries (v1.0.0+)

Download pre-built binaries for macOS and Linux:

```bash
# Linux amd64
curl -L https://github.com/elymas/universal-search/releases/download/v1.0.0/usearch_1.0.0_linux_amd64.tar.gz \
  | tar xz -C /usr/local/bin/

# macOS amd64
curl -L https://github.com/elymas/universal-search/releases/download/v1.0.0/usearch_1.0.0_darwin_amd64.tar.gz \
  | tar xz -C /usr/local/bin/

# Verify installation
usearch --version  # Should print: usearch v1.0.0
```

For other architectures (arm64, Linux), see [releases](https://github.com/elymas/universal-search/releases).

### From source (development)

## Quickstart

```bash
# 1. Clone and enter the repo
git clone <repo-url>
cd univesal-search   # or universal-search after rename

# 2. Copy environment template
cp .env.example .env
# Edit .env — fill in MEILI_MASTER_KEY, POSTGRES_PASSWORD, etc.

# 3. Start all dependencies (Qdrant, Meilisearch, PG, SearXNG, LiteLLM, Redis)
make compose-up

# 4. Build the usearch binary
make build

# 5. Verify the binary
./cmd/usearch/usearch --version
# Expected: usearch v0.1.0-dev (development) or usearch v1.0.0+ (released)
```

## Prerequisites

| Tool    | Minimum version | Notes                                        |
| ------- | --------------- | -------------------------------------------- |
| Docker  | 24+             | Required for `make compose-up`               |
| Go      | 1.25+           | Required for `make build` and `make test`    |
| Python  | 3.11+           | Required for Python services                 |
| Node.js | 22+             | Required for web frontend                    |
| make    | Any             | Standard build tool                          |
| uv      | 0.4+            | Python package manager (`pip install uv`)    |
| pnpm    | 9+              | Node package manager (`npm install -g pnpm`) |

## Common Commands

```bash
make compose-up    # Start all docker-compose services
make compose-down  # Stop all services
make build         # Build cmd/usearch/usearch binary
make test          # Run Go + Python + Node tests
make lint          # Run all linters
make fmt           # Format all code
make tidy          # go mod tidy
make install-py    # uv sync (Python workspace)
make clean         # Remove build artifacts
```

## Project Documentation

- [Product overview](.moai/project/product.md)
- [Project structure](.moai/project/structure.md)
- [Tech stack](.moai/project/tech.md)
- [Roadmap](.moai/project/roadmap.md)
- [Security policy & vulnerability disclosure](SECURITY.md) — operator runbooks at [`ops/security/`](ops/security/)

## Architecture

```
cmd/usearch     → Go CLI binary (--version stub; SPEC-CLI-001 adds subcommands)
internal/       → Domain packages (stubs; filled by M1-M9 SPECs)
pkg/            → Public Go API surface
services/       → Python sidecars (researcher, storm, embedder)
web/            → Next.js 16 App Router frontend
deploy/         → docker-compose dev stack
```

## License

Apache-2.0 — see [LICENSE](LICENSE).

### SearXNG service-boundary note

Universal Search runs SearXNG (AGPL-3.0) as a separate Docker container. No
SearXNG source code is bundled, modified, or redistributed. Network communication
across the service boundary does not create a derivative work under AGPL-3.0.
If Universal Search is ever offered as a hosted SaaS product, this relationship
must be re-evaluated. See [NOTICE](NOTICE) for full attribution.
