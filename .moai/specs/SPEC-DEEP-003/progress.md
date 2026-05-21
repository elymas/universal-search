## SPEC-DEEP-003 Progress

- Started: 2026-05-21
- Phase 0.9: Language skills = moai-lang-go, moai-lang-python
- Phase 0.95: Execution mode = Full Pipeline (22 files, 4 domains, TDD)
- UltraThink: activated (4+ domains, architectural patterns, 22 files)

### Phase E: Observability + Integration + Fallback (2026-05-22)

#### T-E-001..004 [RED]: Metrics Tests - PASS
- `internal/deepagent/tree_metrics_test.go` created
- TestMetricsRegistration: 2 collectors registered
- TestMetricsCardinalityBounded: 18 + 2 = 20 series max
- TestExpandTreeMetricsObserved: N observations = N nodes
- TestOTelSpanParentLinkage + TestOTelTraceDepthMatchesTreeDepth

#### T-E-005 [GREEN]: Metrics Implementation - PASS
- `internal/deepagent/tree_metrics.go` created
  - TreeMetricsRecorder with histogram + counter
  - TreeHooks for nil-safe observability callbacks
  - startNodeSpan OTel helper
- `internal/obs/metrics/deeptree.go` created
  - registerDeepTree(pr) registration function
- `internal/obs/metrics/metrics.go` modified
  - DeepTreeNodeExpand + DeepTreeTotalTokens fields added to Registry
  - "depth" label added to cardinality allowlist
- `internal/obs/obs.go` modified
  - DeepTreeNodeExpand() + DeepTreeTotalTokens() re-exports added
- `internal/deepagent/tree.go` modified
  - TreeConfig.Hooks field added
  - processNode wired with OnNodeComplete, OnNodeFailed, OnNodeBudgetExceeded hooks

#### T-E-006 [RED]: Integration Tests - PASS
- `tests/integration/deep_tree_test.go` created
- TestDeepTreeEndToEndHappyPath: stubbed sidecar + persistence
- TestDeepTreeDEEP002RegressionGreen: backward compatibility

#### T-E-007 [RED]: Fallback Tests - PASS
- TestExpandTreeBreadthZeroFallback
- TestExpandTreeDepthZeroFallback
- TestExpandTreeBreadthAndDepthZeroFallback
- TestFallbackHeaderEmitted + TestFallbackHeaderBodyUnchanged

#### T-E-008 [GREEN]: Wiring + Config - PASS
- `internal/deepagent/config.go` modified
  - TreeConfigExtra struct added
  - DefaultTreeConfig() + NewTreeConfigFromEnv() added
  - FallbackHeader() helper added
- `internal/deepagent/agents.go` modified
  - DeepTreeMode enum added (None, Active, FallbackBreadthZero, FallbackDepthZero)
  - DetermineTreeMode() routing function added
  - FallbackHeaderValue() HTTP header helper added
- `.moai/config/sections/deep.yaml` created
- `.env.example` updated with DEEP_TREE_* documentation

#### T-E-009 [REFACTOR]: Final Verification - PASS
- go test -race ./internal/deepagent/... : OK
- go test -race ./internal/obs/... : OK
- go test -tags=integration -race ./tests/integration/ : OK
- go vet ./internal/deepagent/... : clean
- go vet ./internal/obs/... : clean
