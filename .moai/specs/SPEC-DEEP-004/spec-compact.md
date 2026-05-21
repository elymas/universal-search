# SPEC-DEEP-004 (Compact)

id: SPEC-DEEP-004 | v0.1.1 | draft | owner: expert-backend | TDD | coverage 85% | priority P0
depends_on: DEEP-001, DEEP-002, DEEP-003, LLM-001, OBS-001, IR-001, CORE-001
title: /deep quota and cost guard with Haiku pre-screen and prompt-cache reuse

## Pinned Decisions

- D1 Identity: `X-User-Id` header + `anonymous` fallback. AUTH-001 (M6) JWT fills same header.
- D2 Storage: Postgres durable + Redis hot cache write-behind. Atomic via Lua.
- D3 Cap dimensions: call-count AND $-amount, lower wins. tenant 20/day OR $5/day default.
- D4 Haiku score: ≥6 proceed / 4-5 suggest /basic / <4 reject. Hot-reload threshold.
- D5 Cache: LiteLLM built-in Redis cache. No app-level layer.
- D6 Exceed: HTTP 429 + Retry-After default. `X-Allow-Degrade: 1` opt-in for /basic fallback.
- D7 Audit retention: 90 days hot in Postgres. Archival deferred to M8.

## EARS Requirements

### Identity Module

- REQ-DEEP4-001 (Ubiquitous, P0): identity middleware reads X-User-Id and X-Tenant-Id headers, falls back to "anonymous" / default tenant, injects into request context.
- REQ-DEEP4-002 (Optional, P1): WHERE auth-001-ga env truthy, JWT sub claim takes precedence; cost_ledger.user_id stays opaque TEXT (no migration on transition).

### Haiku Pre-Screen Module

- REQ-DEEP4-003 (Event-Driven, P0): WHEN /deep passes cap-check, cost guard calls Haiku-tier model, parses `{score, rationale, suggested_mode}` JSON; parse failure increments error counter and fails open.
- REQ-DEEP4-004 (Event-Driven, P0): WHEN score ≥ proceed_threshold proceed; WHEN ≥ suggest_threshold < proceed reject with 400 + suggested_mode=basic; WHEN < suggest_threshold reject with 400 + query_rejected_by_screen. Haiku cost recorded as outcome="screen_only".
- REQ-DEEP4-005 (Unwanted, P0): IF Haiku timeout > 200ms OR LLM error THEN fail-open by default (or 503 if fail_open_on_timeout=false). Circuit breaker opens after 5 consecutive failures for 30s.

### Cost Ledger Module

- REQ-DEEP4-006 (Ubiquitous, P0): every Go-side llm.Client call writes a cost_ledger row with (user_id, tenant_id, request_id, deep_run_id, model, prompt_tokens, completion_tokens, usd_cost, cache_hit, intent_category, outcome, ts). outcome enum: {success, error, capped, degraded, screen_only}. Migration: deploy/postgres/migrations/0002_cost_ledger.sql.
- REQ-DEEP4-007 (Ubiquitous, P0): on LLM response, INCR Redis hot cache 24h sliding window bucket; enqueue cost-ledger-write Asynq job; Asynq worker batches (≤100 rows or 5s timeout) into Postgres. Redis failure → 3x retry → fail-closed (429).
- REQ-DEEP4-008 (Ubiquitous, P0): 5-min Asynq scheduled job cost-ledger-reconcile compares Postgres SUM vs Redis; drift > 0.1% triggers alarm + Redis truth-reset. Survives Redis outage.

### Cap Enforcement Module

- REQ-DEEP4-009 (Ubiquitous, P0): Redis Lua script atomically evaluates (tenant calls < max) AND (tenant usd < max), and (user calls/usd if user.enabled). Single Redis call covers eval + increment + TTL refresh.
- REQ-DEEP4-010 (Event-Driven, P0): WHEN cap exceeded THEN HTTP 429 + Retry-After header + body `{error:"cap_exceeded", dimension, remaining, reset_at}` + counter increment + decision event log (stderr JSON line, separate artifact from Postgres ledger row). Skips Haiku and pipeline.
- REQ-DEEP4-011 (Optional, P1): WHERE X-Allow-Degrade: 1 + cap exceeded THEN /basic fallback with HTTP 200 + X-Deep-Degraded header. ledger row outcome="degraded", not counted toward cap.

### Cache Reuse & Observability Module

- REQ-DEEP4-012 (Ubiquitous, P0): deploy/litellm/config.yaml enables built-in Redis cache. TTL per tier (Haiku 1h, others 24h). No app-level cache. cache_key salt = SHA256(tenant_id ‖ intent_category ‖ model ‖ messages_json) applied via LiteLLM custom cache-key callback / wrapper layer; messages payload is NOT mutated (LLM does not see salt).
- REQ-DEEP4-013 (Event-Driven, P0): WHEN x-litellm-cache-hit header → ledger row cache_hit=TRUE, usd_cost from LiteLLM. usearch_deep_cache_hits_total{tier} + usearch_deep_cache_attempts_total{tier} counters. Hit-rate below target → gauge=1.
- REQ-DEEP4-014 (Unwanted, P0): IF Redis unreachable AND redis_failure_mode="fail-closed" (default) THEN HTTP 503. WHERE fail-open override THEN skip cap eval. Recovery is detected via health probe every costguard.redis.health_check_interval_ms (default 5000ms); RehydrateWindow Asynq job fires once after 3 consecutive successful probes to rebuild from Postgres.

## Non-Functional Requirements

- NFR-DEEP4-001 Haiku latency p95 ≤ 200ms
- NFR-DEEP4-002 Ledger write latency: success path (Redis INCR + Asynq enqueue) p95 ≤ 50ms; fail-closed path (3x exponential backoff retry) p99 wall-clock ≤ 2000ms
- NFR-DEEP4-003 Cache hit-rate ≥ 30% over 24h rolling per team
- NFR-DEEP4-004 Cap-check Lua script p95 ≤ 10ms; concurrent 100-req test must have exactly cap-many passes
- NFR-DEEP4-005 Ledger reconciliation drift ≤ 0.1%
- NFR-DEEP4-006 Ledger row durability via Postgres synchronous_commit (applies to Postgres cost_ledger rows only; decision event log stderr JSON lines are out of scope)
- NFR-DEEP4-007 No PII / unbounded values in metric labels; tenant label whitelisted, others collapse to "unknown"
- NFR-DEEP4-008 deep.yaml hot-reload via SIGHUP or fsnotify
- NFR-DEEP4-009 Prometheus naming follows OBS-001 convention (usearch_deep_*); usearch_deep_outcomes_total is owned by SPEC-DEEP-001 and extended by SPEC-DEEP-002 REQ-DEEP2-008 (this SPEC does not own it)
- NFR-DEEP4-010 OTel span attributes: deep.cap.*, deep.cache.hit_ratio, deep.screen.score, deep.screen.outcome

## Exclusions

- No billing/payment integration
- No global cost dashboard (deferred to M8 SPEC-EVAL-002)
- No retroactive quota refunds
- No JWT auth implementation (delegated to SPEC-AUTH-001 M6)
- No multi-org cap hierarchy
- No real-time cost websocket push
- No ML-based Haiku threshold tuning
- No Python-side LLM cost capture in v1 (deferred to SPEC-AUTH-003 M6 reconciliation)
- No Anthropic-native prompt caching (delegated to SPEC-COST-OPT-001 M8)
- No degraded-path cost cap (REQ-DEEP4-011 X-Allow-Degrade fallback to /basic is intentionally NOT counted against cap; /basic cost ~$0.002/call vs /deep $0.05–$0.20 is ~1/30, bounding abuse; revisit in M8 or successor SPEC)

## Acceptance Scenarios

- §5.1 anonymous caller hits daily call cap → 429 + Retry-After (REQ-001, 009, 010)
- §5.2 X-User-Id caller hits $-cap → 429 with dimension=usd, remaining.usd=0 (REQ-001, 009, 010)
- §5.3 Haiku score 3 → immediate 400 reject; Haiku cost recorded as screen_only (REQ-003, 004, 006)
- §5.4 Same query within 24h → cache hit; ledger rows have cache_hit=true (REQ-006, 012, 013)
- §5.5 24h cache hit-rate ≥ 30% measured via Prometheus (REQ-013, NFR-003)
- §5.6 X-Allow-Degrade: 1 + cap exceeded → /basic fallback with HTTP 200 + X-Deep-Degraded header (REQ-011)
- §5.7 Redis outage → fail-closed 503; on recovery RehydrateWindow rebuilds from Postgres (REQ-007, 008, 014)
- Edge cap remaining=1 → that call succeeds (calls=20), next call rejected (calls=21) — boundary atomicity (REQ-009, 010, NFR-004)

## Files to Create

- internal/deepagent/costguard/middleware.go
- internal/deepagent/costguard/ledger.go
- internal/deepagent/costguard/haiku_screen.go
- internal/deepagent/costguard/cache_key.go
- internal/deepagent/costguard/cap_check.go
- internal/deepagent/costguard/reconcile_job.go
- internal/deepagent/costguard/config.go
- internal/deepagent/costguard/metrics.go
- internal/deepagent/costguard/types.go
- internal/deepagent/costguard/lua/cap_check.lua
- internal/deepagent/costguard/{*_test.go}
- deploy/postgres/migrations/0002_cost_ledger.sql
- .moai/config/sections/deep.yaml (or extend)

## Files to Modify

- cmd/usearch-api/handlers/synthesis.go (wire middleware chain)
- cmd/usearch-api/main.go (costguard init + scheduled reconcile job)
- internal/obs/metrics/metrics.go (registerCostGuard)
- internal/obs/obs.go (re-export new collectors)
- internal/obs/metrics/metrics_test.go (extend TestNoUnboundedLabels allowlist with tenant, status, tier, state)
- deploy/litellm/config.yaml (enable Redis cache with per-tier TTLs)
- .env.example (COSTGUARD_* env-vars)

---

*End of compact spec.*
