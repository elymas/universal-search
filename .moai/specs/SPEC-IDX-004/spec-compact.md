# SPEC-IDX-004 (Compact)

id: SPEC-IDX-004 | v0.1.0 | draft | owner: expert-backend | TDD | coverage 85% | priority P0
depends_on: IDX-001, IDX-002, IDX-003, AUTH-001, AUTH-002, OBS-001
blocks: IDX-005
title: Shared index multi-tenancy
milestone: M6 — Team plane (retrieval-layer enforcement gate; enables M6 exit criterion "shared index dedup hits ≥30%" via IDX-005)

## Pinned Decisions

- D1 Qdrant Tiered Multitenancy: single `usearch_docs` collection + `team_id` keyword payload index with `is_tenant=true`. Large teams promoted to dedicated `usearch_docs__team_<hash>` via config + admin CLI. v1.0 manual list only; doc-count auto-tier deferred to SPEC-IDX-007.
- D2 Meilisearch per-tenant tokens: `meilisearch-go.GenerateTenantToken` (HMAC-SHA256, admin key UID signed) + `searchRules` with `team_id = "<T>"` filter on both indexes (`usearch_docs`, `usearch_docs_ko`). TTL default 15min + background refresh 60s before expiry. In-process cache keyed by `(team_id, user_id, api_key_uid)`.
- D3 PG enforcement: `docs.team_id` `NOT NULL DEFAULT 'default'` (migration 0004) + `user_id TEXT NULL` column (migration 0005) + composite indexes `(team_id, source_id)`, `(team_id, published_at DESC)`, partial `(team_id, user_id) WHERE user_id IS NOT NULL`.
- D4 Personal context tier: AUTH-002 `Adapter.Visibility() = user_private` → doc stored with `user_id` payload + retrieval filter `team_id = $T AND (user_id = $U OR user_id = "")`. Default `team_shared`. `public` uses reserved sentinel `__public__`.
- D5 Tenancy mode env var `INDEX_MULTI_TENANCY_MODE`: `enforced` (v1.0 default) / `permissive` / `legacy`. `enforced` rejects `TeamID == ""` with `ErrTeamIDRequired` sentinel.
- D6 Backfill admin CLI `usearch admin backfill-team --default-team <id> [--dry-run] [--batch-size 1000]`: PG batch UPDATE + Qdrant `set_payload` + Meili `UpdateDocuments`. Resumable via `internal/index/backfill/state.json`.
- D7 Cross-team leak prevention: caller MUST NOT set `NormalizedDoc.Metadata["team_id"]`. IDX-004 silent-overwrites with ctx-extracted value + WARN slog. Blocks adapter-spoofing attack vector.
- D8 Observability: new label `team_id_hashed = hex(sha256(team_id))[:8]` + bounded enum `visibility` (3 values). `__public__` kept as plaintext. Plain `team_id` MUST NOT appear in labels/span attributes. New metric family `usearch_index_tenant_token_*`.
- D9 No separate token validation cache. Meili validates self. Go-side keeps issuance cache only. 401 → re-issue + 1-retry.
- D10 V1 `__public__` lives in default-tier collection with payload tag + `should: [team_id == $T, team_id == "__public__"]` filter. Dedicated public collection deferred (public doc share <5%, negligible perf impact).

## EARS Requirements

### Tenancy Enforcement Module

- REQ-IDX4-001 (Event-Driven, P0): WHEN `index.Search`/`Upsert` called in `enforced` mode, dispatch checks `extractTeamID(ctx)` or `q.TeamID` first; empty → `ErrTeamIDRequired` immediate return (embedder/store fanout skipped). `permissive`/`legacy` modes preserve legacy behavior. Mode transition requires process restart.
- REQ-IDX4-002 (Ubiquitous, P0): Upsert silently overwrites caller-provided `Metadata["team_id"]` with ctx-extracted value + emits WARN slog (`event_type: "idx4.upsert.team_id_overridden"`). Blocks adapter-spoofing attack vector.
- REQ-IDX4-003 (Event-Driven, P0): `authctx.TeamIDKey`/`UserIDKey` extracted from JWT context (AUTH-001) OR fallback to `INDEX_DEFAULT_TEAM` env var. Context key convention matches DEEP-004 `costguard.UserIDKey` pattern. Both absent → empty (triggers REQ-001 sentinel in enforced mode).

### Qdrant Tiered Multitenancy Module

- REQ-IDX4-004 (Ubiquitous, P0): `EnsureCollection` adds `{"field_name":"team_id","field_schema":{"type":"keyword","is_tenant":true}}` payload index. Idempotent on existing collections. HNSW `payload_m`/`m=0` deferred to SPEC-IDX-007.
- REQ-IDX4-005 (Optional, P1): admin CLI `usearch admin tier-promote --team <id>` creates dedicated `usearch_docs__team_<sha256(team_id)[:16]>` collection + streaming move from default-tier + dispatch routing + dry-run support. v1.0 manual list only.
- REQ-IDX4-006 (Ubiquitous, P0): Qdrant Search filter synthesizes `must: [team_id == $T]` + optional `user_id` clause for `user_private` visibility + optional `should: [team_id == "__public__"]` for `IncludePublic`. `__public__` rejected as user input.
- REQ-IDX4-007 (Ubiquitous, P0): `__public__` sentinel rejected in 4 entry points: JWT claim (AUTH-001 reject), env var (startup validation), backfill CLI, tier-promote CLI. Accepted ONLY as `Adapter.Visibility() = public` result.

### Meili Tenant Tokens Module

- REQ-IDX4-008 (Event-Driven, P0): WHEN Meili Search called in `enforced` mode, fetch/issue tenant token via `meilisearch-go.GenerateTenantToken(apiKeyUID, searchRules, &TenantTokenOptions{ExpiresAt: now+15m})`. `searchRules` includes `team_id = "<T>"` filter on both `usearch_docs` and `usearch_docs_ko`. Upsert/EnsureIndex/Settings continue using admin API key directly.
- REQ-IDX4-009 (Ubiquitous, P0): Token cache (`sync.Map` + per-entry expires_at) refreshes 60s before expiry (background worker), uses `sync.Once` per `(team, user, key_uid)` triplet, honors ctx cancellation (`goleak.VerifyNone` PASS), exposes revocation hook (DI seam for AUTH-002 `team.member.removed` event). Emits 3 metric families: `usearch_index_tenant_token_issued_total{tier,outcome}`, `_revoked_total{tier}`, `_validation_failures_total{tier,outcome}`.

### PG Migration & Backfill Module

- REQ-IDX4-010 (Ubiquitous, P0): Migration `0004_team_id_not_null.sql` (single-transaction: backfill NULL → `'default'` + `ALTER ... SET NOT NULL` + composite indexes) + migration `0005_user_id_column.sql` (`user_id TEXT NULL` + partial index). Both idempotent.
- REQ-IDX4-011 (Optional, P1): admin CLI `usearch admin backfill-team --default-team <id> [--dry-run] [--batch-size 1000]` patches PG (UPDATE batch with sub-query LIMIT) + Qdrant (`set_payload`) + Meili (`UpdateDocuments` partial). Resumable via `state.json` per-store last_processed_doc_id. Verifies completion (`count == 0` NULL rows). Emits `usearch_index_tenant_backfill_total{store,outcome}`.

## Non-Functional Requirements

- NFR-IDX4-001 Cross-team leak probability EXACTLY 0 — CRITICAL SECURITY INVARIANT (4-layer defense: Qdrant `is_tenant=true` payload filter + Meili tenant token + PG NOT NULL + dispatch sentinel). Load-bearing for IDX-005 NFR-IDX5-004.
- NFR-IDX4-002 Same-team dedup correctness via IDX-001 ON CONFLICT (doc_id) DO UPDATE reuse. Prerequisite for M6 exit criterion "dedup ≥30%".
- NFR-IDX4-003 Enforcement overhead p95 ≤ 10ms (Qdrant `is_tenant=true` co-location <5% pre-filter overhead; Meili token cache hit-rate ≥99%).
- NFR-IDX4-004 Token cache concurrency safe under 50 goroutine × 100 calls (sync.Once per triplet, single issuance per cold combo, `goleak.VerifyNone` PASS).
- NFR-IDX4-005 Backfill atomicity & resumability (PG sub-query LIMIT batching, state.json crash-resume, dry-run accuracy, NOT NULL race-safe).
- NFR-IDX4-006 Qdrant tier-promote regression p95 latency degradation ≤ 10% vs IDX-001 baseline.
- NFR-IDX4-007 Observability cardinality bounded (`team_id_hashed` = SHA256[:8] = 4×10^9 bound but real production 100-10000 teams safe; `visibility` 3-value enum; plain `team_id` never in labels).
- NFR-IDX4-008 AUTH-001/002 ship-before forward-compat (`INDEX_DEFAULT_TEAM` env var fallback + visibility hook default `team_shared` for unimplemented adapters).

## Exclusions

- No Qdrant HNSW `payload_m`/`m=0` reconfiguration (deferred to SPEC-IDX-007 post-V1 with benchmark data)
- No doc-count auto-tier promotion (v1.0 manual list only)
- No dedicated public collection (v1.0 single collection with payload tag + `should` clause; <5% public doc share)
- No separate token validation cache (Meili validates; Go-side issuance cache only)
- No `team_id` ↔ `tenant_id` column rename across cost_ledger/docs (deferred to AUTH-003 audit log SPEC)
- No AUTH-002 visibility hook concrete wire-up (interface + DI seam only; default `team_shared`)
- No `team.member.removed` event concrete wire-up (hook point + DI seam only)
- No AUTH-001/002 self-implementation (consume context keys only; forward-compat via env var fallback)
- No cross-region tenancy (single-cluster assumption; multi-region GDPR deferred post-V1)
- No migration rollback automation (operator manually undoes if needed; sufficient guidance in runbook)
- No GitHub Issue tracking (`issue_number: 0`)

## Acceptance Scenarios

- §5.1 team T enforced mode upsert + search round-trip → team_id correctly stamped (REQ-001, 002, 003, 004)
- §5.2 Meili tenant token issuance + cache reuse + Korean shard applied (REQ-008, 009)
- §5.3 personal context tier: user_private adapter doc visible only to ingestor (REQ-002, 006)
- §5.4 `__public__` sentinel doc synthesized into all-team retrieval (REQ-006, 007)
- §5.5 NFR regression: 100-query mixed-team traffic p95 latency ≤10% degradation (NFR-003, 006)
- §5.6 **M6 EXIT CONTRIBUTION**: cross-team probe — team U attempting team T data access → EXACTLY 0 leak (REQ-001, 006, 007, 008, NFR-001)
- §5.7 backfill admin CLI dry-run + execute + resume from crash (REQ-011, NFR-005)
- §5.8 Qdrant tier-promote admin CLI: large team promoted to dedicated collection (REQ-005)
- Edge1 tenancy mode transition (`permissive` → `enforced`) NULL row rejection behavior (REQ-001 boundary)
- Edge2 token cache concurrency: 50 goroutine × 100 calls race-free (NFR-004)

## Files to Create

- internal/index/tenancy/mode.go (`ParseMode`, `ErrTeamIDRequired` sentinel)
- internal/index/tenancy/mode_test.go
- internal/index/auth/context.go (`authctx.TeamIDKey/UserIDKey`, `extractTeamID/UserID` + env fallback)
- internal/index/auth/context_test.go
- internal/index/tenant/types.go (`TenantTokenEntry`, `TenancyMode`, `Visibility` enums)
- internal/index/tenant/issuer.go (`meilisearch-go.GenerateTenantToken` wrapper)
- internal/index/tenant/cache.go (sync.Map + refresh worker + revocation hook DI seam)
- internal/index/tenant/issuer_test.go
- internal/index/tenant/cache_test.go (concurrency, goleak, ttl)
- internal/index/backfill/state.go (state.json schema + resume logic)
- internal/index/backfill/cli.go (admin backfill-team implementation)
- internal/index/backfill/cli_test.go (dry-run, batch, resume, verify)
- cmd/usearch/admin/backfill.go (CLI entry point)
- cmd/usearch/admin/tier_promote.go (CLI entry point)
- cmd/usearch/admin/tier_promote_test.go
- internal/index/pg/migrations_test.go (testcontainers migration verification)
- internal/index/tenant_integration_test.go (M6 cross-team probe + latency budget)
- internal/obs/metrics/tenant.go (4 collectors + registerIDX4 helper)
- deploy/postgres/migrations/0004_team_id_not_null.sql
- deploy/postgres/migrations/0005_user_id_column.sql

## Files to Modify

- internal/index/dispatch.go (tenancy sentinel + Upsert silent overwrite + remove v0.1 `team_id: nil` + user_id payload for user_private)
- internal/index/qdrant/client.go (`EnsureCollection` is_tenant=true + Search filter visibility-aware synthesis + `__public__` sentinel handling)
- internal/index/meili/client.go (Search uses tenant token; Upsert keeps admin key; user_id filterable added)
- internal/index/meili/korean_shard.go (user_id filterable added)
- internal/index/index.go:108 (FilterableAttributes extended with `user_id`)
- internal/index/types.go:25-27 (`IndexQuery.TeamID` v0.1→v1.0 semantics; add `UserID`, `IncludePublic` fields)
- internal/index/pg/client.go (INSERT/SELECT paths use ctx-team_id + user_id for user_private)
- cmd/usearch/main.go (admin sub-commands `backfill-team`, `tier-promote` + startup tenancy mode validation)
- internal/obs/metrics/metrics.go (registerIDX4 call + cardinality allowlist extension)
- internal/obs/metrics/metrics.go:171-176 (allowlist + `team_id_hashed`, `visibility`, `tier`)
- internal/obs/obs.go (re-export 4 new collectors)
- internal/obs/metrics/metrics_test.go (`TestNoUnboundedLabels` allowlist extension)
- pkg/types/adapter.go (`Adapter.Visibility()` interface method; unimplemented adapters default `team_shared`)
- .moai/config/sections/index.yaml (`qdrant.tiering.dedicated_teams: []`, `meili.tenant_token.ttl_minutes: 15`, `default_team`)
- .env.example (`INDEX_MULTI_TENANCY_MODE`, `INDEX_DEFAULT_TEAM`, `MEILI_TENANT_TOKEN_TTL_MINUTES`)

## M6 Contribution

본 SPEC은 M6 exit criterion "shared index dedup hits ≥30%" (`.moai/project/
roadmap.md:155`) 의 ENABLING INVARIANT. IDX-005 (team-shared answer reuse)
가 이 invariant 위에 build되어 fanout 이전에 team-scoped lookup을 수행
한다. §5.6 cross-team probe acceptance test가 통과해야 IDX-005의 NFR-IDX5-004
(cross-tenant leak == 0)가 보장되며, 그 위에서 IDX-005의 `TestDedupHitRate
At30PctOnSyntheticTraffic` (M6 PRIMARY GATE)이 측정 가능 상태에 진입한다.

---

*End of compact spec.*
