# SPEC-IDX-005 (Compact)

id: SPEC-IDX-005 | v0.1.0 | draft | owner: expert-backend | TDD | coverage 85% | priority P1
depends_on: IDX-001, IDX-004, AUTH-001, AUTH-002, CACHE-001, SYN-001, FAN-001
title: Team-shared answer reuse via pre-fanout lookup with configurable staleness
milestone: M6 — Team plane (release-driving deliverable, primary driver of dedup hit rate ≥30% exit criterion)

## Pinned Decisions

- D1 Similarity threshold default 0.92. deep.yaml hot-reload + per-team override.
- D2 Per-category staleness TTL via IR-001 category: web=1h, social=30m, academic=30d, korean=1h, mixed/unknown=2h.
- D3 Soft-stale (age >= 0.5*TTL) → SERVE + async refresh. Hard-stale (age >= TTL) → MISS. Lazy evict via next-write overwrite.
- D4 Citation re-validation default LAZY. eager_top_n / eager_all opt-in. Reuses CACHE-001 Phase 2 HEAD probe.
- D5 Single thumbs-down → immediate force_stale. Multi-user threshold deferred.
- D6 Force-refresh via ?force_refresh=true query param OR X-Force-Refresh: 1 header.
- D7 Hit serve = SYN-001 SynthesizeResponse shape + extra X-Cache headers.
- D8 Qdrant: reuse existing usearch_docs collection with doc_type="cached_answer" payload tag.
- D9 PG: new answer_cache table. Migration deploy/postgres/migrations/0003_answer_cache.sql.
- D10 V1 no Redis hot lookup (Qdrant latency sufficient).

## EARS Requirements

### Lookup Pipeline Module

- REQ-IDX5-001 (Event-Driven, P0): WHEN /query arrives without force_refresh, middleware extracts team_id from JWT context, computes query embedding via existing embedder, calls index.Search(IndexQuery{TeamID, DocTypes:[DocTypeCachedAnswer], MaxResults:1}); fanout decision deferred to REQ-002.
- REQ-IDX5-002 (Event-Driven, P0): WHEN top-1 cosine >= threshold (default 0.92, per-team override) AND staleness ∈ {fresh, soft-stale}, serve cached SynthesizeResponse with HTTP 200 + X-Cache: HIT|SOFT-HIT + X-Cache-Age-Seconds + X-Cache-Score. Otherwise fall-through to fanout.
- REQ-IDX5-003 (Event-Driven, P0): WHEN cached_answer record returned, evaluate staleness via per-category TTL: age < 0.5*ttl → fresh; 0.5*ttl <= age < ttl → soft-stale; age >= ttl OR force_stale=TRUE → hard-stale.
- REQ-IDX5-004 (Optional, P1): WHERE citation_revalidation = "eager_top_n" (default N=3), parallel HEAD probe top-N citation URLs (timeout 200ms via CACHE-001 Phase 2 reuse); 4xx strip + X-Cache-Citation-Stale: 1 header; timeout/5xx ignore.
- REQ-IDX5-005 (Event-Driven, P0): WHEN soft-stale hit serves, enqueue Asynq idx5-refresh job; worker runs fanout + synthesis, upserts same doc_id (idempotent). hit_count + last_served_at async-update on every serve.

### Storage & Cache Write Module

- REQ-IDX5-006 (Ubiquitous, P0): on fanout MISS path, fire-and-forget async write to Qdrant (point_id = docID("answer-cache", queryHash + ":" + team_id), payload = {doc_type, team_id, category, created_at, ttl_seconds, force_stale: false}) AND PG answer_cache INSERT ON CONFLICT (doc_id) DO UPDATE. Migration: deploy/postgres/migrations/0003_answer_cache.sql.
- REQ-IDX5-007 (Ubiquitous, P0): all lookup AND write use team_id as isolation key. Trust IDX-004 multi-tenancy enforcement (Qdrant payload filter + Meili tenant token + PG row-level security). doc_id constructed with team_id. Cross-tenant leak probability MUST be exactly zero.

### Feedback & Observability Module

- REQ-IDX5-008 (Event-Driven, P1): WHEN POST /feedback {request_id, score: -1}, recover (team_id, cached_doc_id) via in-memory 24h LRU; UPDATE answer_cache SET force_stale=TRUE WHERE doc_id AND team_id; idempotent on duplicate. Unmapped request_id → counter increment.
- REQ-IDX5-009 (Ubiquitous, P0): register 7 Prometheus collectors (usearch_idx5_*): lookups_total{outcome}, lookup_duration_seconds, dedup_hit_rate{team_id_hashed}, reuse_latency_ms{outcome}, stale_evictions_total{category, mode}, feedback_marks_total{score}, feedback_unmapped_total. Cardinality allowlist extension: team_id_hashed (bounded whitelist) + new outcome enum values.
- REQ-IDX5-010 (Ubiquitous, P0): emit OTel parent span attributes (idx5.lookup.outcome, .similarity_score, .cached_age_seconds, .cached_doc_id, .ttl_remaining_seconds, .citation.revalidation_mode, .citation.stripped_count) and ONE slog INFO JSON line per lookup (event_type="idx5.lookup"). Schema additive-only, forward-compatible with SPEC-AUTH-003 audit subsystem.

## Non-Functional Requirements

- NFR-IDX5-001 M6 dedup hit-rate ≥30% (24h rolling per team) — PRIMARY M6 EXIT GATE
- NFR-IDX5-002 Lookup overhead on MISS path p95 ≤ 50ms
- NFR-IDX5-003 Reuse latency on HIT path p95 ≤ 200ms
- NFR-IDX5-004 Cross-tenant leak probability exactly 0 (CRITICAL SECURITY INVARIANT)
- NFR-IDX5-005 Stale eviction lag ≤ 1 lookup cycle (lazy evict)
- NFR-IDX5-006 Refresh job concurrency safe (idempotent via IDX-001 ON CONFLICT (doc_id) DO UPDATE) + goleak.VerifyNone PASS
- NFR-IDX5-007 deep.yaml hot-reload via SIGHUP / fsnotify (reuse DEEP-004 pattern)

## Exclusions

- No multi-user threshold for feedback marking (single thumbs-down → immediate stale)
- No Redis hot lookup cache layer (Qdrant similarity search sufficient)
- No partial-overlap signal logging (single best-match only)
- No explicit zstd compression on answer text (PG TOAST handles)
- No eager re-validation citation pool dedup
- No mixed-category TTL inheritance (V1 fixed 2h)
- No team admin bulk evict endpoint (deferred to SPEC-AUTH-004 M7)
- No detailed citation reuse audit trail (deferred to SPEC-AUTH-003 M6)
- No per-team refresh job rate-limiting (V1 global pool)
- No mid-content cache invalidation (SPEC-SYN-002 territory)
- No Anthropic-native prompt cache replay (deferred to SPEC-COST-OPT-001 M8)
- No GitHub Issue tracking on this SPEC (issue_number: null)

## Acceptance Scenarios

- §5.1 team T user fresh hit → 200 + X-Cache: HIT (REQ-001, 002, 007)
- §5.2 team T user soft-stale hit → 200 serve + async refresh enqueue (REQ-001, 002, 003, 005)
- §5.3 team T user hard-stale → MISS + fanout fall-through + write (REQ-002, 003, 006)
- §5.4 sub-threshold similarity → MISS + fanout + write (REQ-001, 002)
- §5.5 M6 EXIT GATE: 100-query synthetic traffic dedup hit-rate ≥30% (NFR-001, REQ-009)
- §5.6 cross-tenant probe: team U reuses team T cached answer → MISS forced (REQ-007, NFR-004)
- §5.7 feedback thumbs-down → next lookup hard-stale → fanout re-runs (REQ-005, 008)
- §5.8 citation re-validation eager_top_n: 404 citation stripped from response (REQ-004)
- Edge1 force_refresh=true bypasses lookup even on hit-eligible state (REQ-001)
- Edge2 TTL boundary: age = exactly TTL → hard-stale (REQ-003 boundary atomicity)

## Files to Create

- internal/idx5/middleware.go
- internal/idx5/lookup.go
- internal/idx5/staleness.go
- internal/idx5/serve.go
- internal/idx5/writeback.go
- internal/idx5/citation_revalidate.go
- internal/idx5/feedback.go
- internal/idx5/refresh_job.go
- internal/idx5/config.go
- internal/idx5/metrics.go
- internal/idx5/types.go
- internal/idx5/docid.go
- internal/idx5/{*_test.go}
- internal/idx5/integration_test.go (M6 exit gate)
- internal/obs/metrics/idx5.go
- deploy/postgres/migrations/0003_answer_cache.sql

## Files to Modify

- cmd/usearch-api/handlers/query.go (wire IDX5 middleware before fanout, async write after MISS)
- cmd/usearch-api/main.go (idx5.New init + refresh worker + scheduled hooks)
- internal/obs/metrics/metrics.go (registerIDX5)
- internal/obs/obs.go (re-export new collectors)
- internal/obs/metrics/metrics_test.go (extend TestNoUnboundedLabels allowlist with team_id_hashed + new outcomes)
- pkg/types/normalized_doc.go (add DocTypeCachedAnswer enum)
- .moai/config/sections/deep.yaml (costguard.idx5.* section)
- .env.example (IDX5_* env-vars)

---

*End of compact spec.*
