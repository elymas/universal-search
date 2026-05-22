## Task Decomposition
SPEC: SPEC-IDX-005

| Task ID | Description | Requirement | Dependencies | Planned Files | Status |
|---------|-------------|-------------|--------------|---------------|--------|
| T-001 | Phase A: Storage foundation (PG migration + DocType enum + types) | REQ-006, REQ-007 | - | deploy/postgres/migrations/0003_answer_cache.sql, pkg/types/normalized_doc.go, internal/idx5/types.go, internal/idx5/docid.go | pending |
| T-002 | Phase B: Lookup core (embedder call + Qdrant search + threshold + staleness) | REQ-001, REQ-002, REQ-003, NFR-002 | T-001 | internal/idx5/lookup.go, internal/idx5/staleness.go, internal/idx5/config.go | pending |
| T-003 | Phase C: Hit serve (SynthesizeResponse reconstruct + headers) + async write-back | REQ-002, REQ-005, REQ-006, NFR-003 | T-002 | internal/idx5/serve.go, internal/idx5/writeback.go | pending |
| T-004 | Phase D: Citation re-validation (CACHE-001 phase2 reuse) + feedback handler | REQ-004, REQ-008, NFR-004 | T-003 | internal/idx5/citation_revalidate.go, internal/idx5/feedback.go | pending |
| T-005 | Phase E: Middleware wiring + observability + M6 exit gate test | REQ-001, REQ-005, REQ-007, REQ-009, REQ-010, NFR-001, NFR-005, NFR-006, NFR-007 | T-001, T-002, T-003, T-004 | internal/idx5/middleware.go, internal/idx5/refresh_job.go, internal/obs/metrics/idx5.go, internal/obs/metrics/metrics.go, internal/obs/obs.go, internal/obs/metrics/metrics_test.go, cmd/usearch-api/handlers/query.go, cmd/usearch-api/main.go, .moai/config/sections/deep.yaml, .env.example, internal/idx5/integration_test.go | pending |
