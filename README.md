# Universal Search

A team-oriented search engine that combines SearXNG metasearch, gpt-researcher deep research, and STORM long-form synthesis into a single Go orchestration plane with a shared hybrid index (Qdrant + Meilisearch + PostgreSQL).

## Quickstart

```bash
git clone https://github.com/elymas/universal-search.git
cd universal-search
cp .env.example .env
make compose-up
make build
./cmd/usearch/usearch --version
```

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (with Compose V2)
- [Go 1.23+](https://go.dev/dl/)
- [Python 3.11+](https://www.python.org/downloads/)
- [Node 22+](https://nodejs.org/)
- [make](https://www.gnu.org/software/make/)

## Project Documentation

- [Product overview](.moai/project/product.md)
- [Repository structure](.moai/project/structure.md)
- [Tech stack decisions](.moai/project/tech.md)
- [Project roadmap](.moai/project/roadmap.md)

## License

This project is licensed under the [Apache License 2.0](LICENSE).

**SearXNG note**: SearXNG is consumed as a Docker image and runs as an external service across a network boundary. Universal Search does not modify, fork, or statically link SearXNG source code. This service-boundary relationship does not create an AGPL-3.0 obligation on the Universal Search codebase. See [NOTICE](NOTICE) for full attribution.
