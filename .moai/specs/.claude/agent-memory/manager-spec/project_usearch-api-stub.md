---
name: usearch-api-stub
description: usearch-api HTTP server was a non-functional stub; SPEC-API-001 owns it; the in-code "SPEC-IR-001" reference is stale/wrong
metadata:
  type: project
---

The `usearch-api` HTTP server backend was a non-functional stub until SPEC-API-001
(drafted 2026-06-04). `cmd/usearch-api/main.go` built a mux + obs but never called
ListenAndServe — it printed "not implemented" and exited 0. The Next.js frontend
(`web/src/lib/api.ts`) had no working backend; only the CLI search path worked.

**Stale-reference trap:** the stub code attributed the HTTP server to "SPEC-IR-001"
(package comment "Full implementation lands in SPEC-IR-001" + stderr "see
SPEC-IR-001"). This is WRONG. SPEC-IR-001 is "Intent Router v0" — explicitly
library-only, no HTTP endpoint, and its own spec.md already names SPEC-API-001 as
the HTTP owner. **SPEC-API-001 owns the HTTP API server.**

**Why:** misattributed code comments sent prior readers looking in the wrong SPEC;
the HTTP server had no SPEC at all until SPEC-API-001.

**How to apply:** when planning anything touching `cmd/usearch-api/`, treat
SPEC-API-001 as the owner. The key engineering constraint: CLI pipeline build
helpers (`buildProductionRegistry`/`buildRouter`/`buildProductionSynth`) live in
`package main` under `cmd/usearch/` and CANNOT be imported by another main package —
they must be extracted to a shared `internal/` package (proposed `internal/searchpipe/`).
Related: [[spec-stale-code-assumptions]] (auto-memory) — always verify cited
paths/IDs before implementing draft SPECs in this repo.
