## Task Decomposition
SPEC: SPEC-DEEP-001
Version: 0.3.1
Methodology: TDD (RED-GREEN-REFACTOR)

| Task ID | Description | Milestone | Dependencies | Planned Files | Status |
|---------|-------------|-----------|--------------|---------------|--------|
| T-001 | Sidecar skeleton + LiteLLM gateway | M1 | - | services/storm/src/storm/__main__.py, app.py, models.py, gateway.py, obs.py, services/storm/tests/test_app.py, test_gateway.py, pyproject.toml, Dockerfile, .env.example | pending |
| T-002 | Injected retrieval + STORM invocation | M2 | T-001 | services/storm/src/storm/inject_rm.py, pipeline.py, services/storm/tests/test_inject_rm.py, test_pipeline.py | pending |
| T-003 | Citation translator (URL -> doc_id) | M3 | T-002 | services/storm/src/storm/citation_translator.py, services/storm/tests/test_citation_translator.py | pending |
| T-004 | Faithfulness gate (long-form) | M4 | T-003 | services/storm/src/storm/faithfulness.py, services/storm/tests/test_faithfulness.py | pending |
| T-005 | Latency + budget caps | M5 | T-002 | services/storm/src/storm/pipeline.py (mod), gateway.py (mod), app.py (mod), obs.py (mod), services/storm/tests/test_caps.py | pending |
| T-006 | Go-side client + observability | M6 | - | internal/deepreport/types.go, client.go, config.go, client_test.go, internal/obs/metrics/deepreport.go, metrics.go (mod), obs.go (mod) | pending |
| T-007 | SSE long-form streaming | M7 | T-006 | internal/streamsynth/longform.go, longform_test.go, cmd/usearch-api/handlers/deep_*.go | pending |
| T-008 | Property tests + observability validation | M8 | T-004, T-007 | services/storm/tests/test_pipeline.py (mod), internal/streamsynth/longform_test.go (mod) | pending |
