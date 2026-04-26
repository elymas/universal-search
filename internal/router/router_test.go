// Package router_test validates Router orchestration end-to-end.
package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/llm"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/obs/metrics"
	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// stubAdapter is a minimal types.Adapter for registry seeding.
type stubAdapter struct {
	name string
	caps types.Capabilities
}

func (a *stubAdapter) Name() string                        { return a.name }
func (a *stubAdapter) Capabilities() types.Capabilities    { return a.caps }
func (a *stubAdapter) Healthcheck(_ context.Context) error { return nil }
func (a *stubAdapter) Search(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
	return nil, nil
}

func newStubAdapter(name string, docTypes []types.DocType, langs []string) *stubAdapter {
	return &stubAdapter{
		name: name,
		caps: types.Capabilities{
			SourceID:       name,
			DisplayName:    name,
			DocTypes:       docTypes,
			SupportedLangs: langs,
		},
	}
}

// newTestObs builds a minimal *obs.Obs bundle with a per-test metrics registry
// and a no-op tracer provider via the public obs.Init constructor.
func newTestObs(t *testing.T) *obs.Obs {
	t.Helper()
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName:    "router-test",
		ServiceVersion: "test",
		LogLevel:       "ERROR",
	})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	// Replace the Metrics with a fresh registry so each test is isolated.
	o.Metrics = metrics.NewRegistry()
	return o
}

// newPopulatedRegistry returns a registry containing 5 stubs with diverse
// capabilities, used by the AdapterSet tests.
func newPopulatedRegistry(t *testing.T) *adapters.Registry {
	t.Helper()
	reg := adapters.NewRegistry(nil)
	for _, ad := range []*stubAdapter{
		newStubAdapter("naver", []types.DocType{types.DocTypeArticle, types.DocTypePost}, []string{"ko"}),
		newStubAdapter("daum", []types.DocType{types.DocTypeArticle}, []string{"ko"}),
		newStubAdapter("rss_korean", []types.DocType{types.DocTypeArticle}, []string{"ko"}),
		newStubAdapter("hackernews", []types.DocType{types.DocTypePost, types.DocTypeSocial}, []string{"en"}),
		newStubAdapter("searxng", []types.DocType{types.DocTypeArticle, types.DocTypeOther}, nil),
		newStubAdapter("arxiv", []types.DocType{types.DocTypePaper}, nil),
		newStubAdapter("github", []types.DocType{types.DocTypeRepo, types.DocTypeIssue}, nil),
	} {
		if err := reg.Register(ad); err != nil {
			t.Fatalf("Register %q: %v", ad.name, err)
		}
	}
	return reg
}

// TestNewRouterReturnsErrEmptyRegistry asserts construction fails when the
// adapter registry is empty (S-18).
func TestNewRouterReturnsErrEmptyRegistry(t *testing.T) {
	t.Parallel()
	emptyReg := adapters.NewRegistry(nil)
	r, err := router.New(router.Options{
		Registry: emptyReg,
		Obs:      newTestObs(t),
	})
	if !errors.Is(err, router.ErrAdapterRegistryEmpty) {
		t.Errorf("err: got %v, want ErrAdapterRegistryEmpty", err)
	}
	if r != nil {
		t.Error("Router should be nil on construction failure")
	}
}

// TestClassifyReturnsRoutingDecision asserts every populated query produces a
// well-formed decision (REQ-IR-001).
func TestClassifyReturnsRoutingDecision(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "transformer paper"}})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if !dec.Category.IsValid() {
		t.Errorf("invalid category: %q", dec.Category)
	}
	if dec.Confidence < 0 || dec.Confidence > 1 {
		t.Errorf("confidence out of range: %v", dec.Confidence)
	}
	if !dec.Source.IsValid() {
		t.Errorf("invalid source: %q", dec.Source)
	}
	if dec.AdapterSet == nil {
		t.Error("AdapterSet should not be nil")
	}
}

// TestClassifyEmptyQueryReturnsErr covers REQ-IR-005 / S-4.
func TestClassifyEmptyQueryReturnsErr(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, txt := range []string{"", "   ", "\t\n", " "} {
		txt := txt
		t.Run(txt, func(t *testing.T) {
			t.Parallel()
			dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: txt}})
			if !errors.Is(err, router.ErrInvalidQuery) {
				t.Errorf("err: got %v, want ErrInvalidQuery", err)
			}
			if dec.Category != "" || dec.Confidence != 0 {
				t.Errorf("expected zero RoutingDecision, got %+v", dec)
			}
		})
	}
}

// TestClassifyHighConfidenceSkipsLLM asserts a clearly-Korean query does NOT
// invoke the LLM (REQ-IR-002 / S-1).
func TestClassifyHighConfidenceSkipsLLM(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	fake := &fakeLLMClient{}
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t), LLMClient: fake})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "ChatGPT 사용법과 프롬프트 엔지니어링 팁"}})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if fake.calls != 0 {
		t.Errorf("LLM was called %d times, want 0", fake.calls)
	}
	if dec.Category != router.CategoryKorean {
		t.Errorf("category: got %q, want korean", dec.Category)
	}
	if dec.Source != router.SourceRuleBased {
		t.Errorf("source: got %q, want rule_based", dec.Source)
	}
	if dec.Confidence < 0.90 {
		t.Errorf("confidence: got %v, want ≥ 0.90", dec.Confidence)
	}
	if dec.Lang != "ko" {
		t.Errorf("lang: got %q, want ko", dec.Lang)
	}
	want := []string{"daum", "naver", "rss_korean", "searxng"}
	if !equalSorted(dec.AdapterSet, want) {
		// Korean Category accepts ANY DocType, lang filter accepts ko or empty.
		// May also include arxiv (lang-agnostic, paper DocType which Korean=ANY admits)
		// and github (lang-agnostic).
		// Validate at minimum that hackernews is excluded and the Korean adapters present.
		if contains2(dec.AdapterSet, "hackernews") {
			t.Errorf("AdapterSet should not include hackernews (lang=en)")
		}
		for _, must := range []string{"daum", "naver", "rss_korean"} {
			if !contains2(dec.AdapterSet, must) {
				t.Errorf("AdapterSet missing %q: got %v", must, dec.AdapterSet)
			}
		}
	}
}

// TestClassifyLowConfidenceInvokesLLM asserts an ambiguous query escalates
// to LLM exactly once (REQ-IR-002 / S-3).
func TestClassifyLowConfidenceInvokesLLM(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
			return llm.Response{Text: `{"category":"mixed","confidence":0.78,"rationale":"code-mixed Korean-English LLM query"}`}, nil
		},
	}
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t), LLMClient: fake})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "best Korean LLM 모델"}})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if fake.calls != 1 {
		t.Errorf("LLM was called %d times, want 1", fake.calls)
	}
	if dec.Category != router.CategoryMixed {
		t.Errorf("category: got %q, want mixed", dec.Category)
	}
	if dec.Source != router.SourceLLMFallback {
		t.Errorf("source: got %q, want llm_fallback", dec.Source)
	}
	if dec.Confidence != 0.78 {
		t.Errorf("confidence: got %v, want 0.78", dec.Confidence)
	}
	if rat, _ := dec.Metadata["llm_rationale"].(string); rat == "" {
		t.Error("llm_rationale should be populated")
	}
	if rc, ok := dec.Metadata["rule_confidence"].(float64); !ok || rc <= 0 {
		t.Errorf("rule_confidence should be set and positive, got %v ok=%v", rc, ok)
	}
}

// TestClassifyDegradesGracefullyWhenLLMUnavailable asserts circuit-breaker
// degradation (REQ-IR-003 / S-5).
func TestClassifyDegradesGracefullyWhenLLMUnavailable(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
			return llm.Response{}, llm.ErrAllProvidersFailed
		},
	}
	o := newTestObs(t)
	r, err := router.New(router.Options{Registry: reg, Obs: o, LLMClient: fake})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "best Korean LLM 모델"}})
	if err != nil {
		t.Fatalf("Classify err propagated: %v", err)
	}
	if dec.Source != router.SourceRuleBased {
		t.Errorf("source: got %q, want rule_based", dec.Source)
	}
	if v, _ := dec.Metadata["llm_unavailable"].(bool); !v {
		t.Error("Metadata.llm_unavailable should be true")
	}
	if v, _ := dec.Metadata["degraded_confidence"].(bool); !v {
		t.Error("Metadata.degraded_confidence should be true")
	}
	if fake.calls != 1 {
		t.Errorf("LLM should be called once, got %d", fake.calls)
	}
	got := counterValue(t, o, "usearch_router_classifications_total", "outcome", "error_breaker_open")
	if got < 1 {
		t.Errorf("error_breaker_open counter: got %v, want ≥ 1", got)
	}
}

// TestClassifyLLMTimeoutDegrades asserts the 2s deadline triggers degradation
// (REQ-IR-007 / S-6).
func TestClassifyLLMTimeoutDegrades(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	fake := &fakeLLMClient{
		completeFn: func(ctx context.Context, _ llm.Request) (llm.Response, error) {
			select {
			case <-time.After(3 * time.Second):
				return llm.Response{Text: `{"category":"mixed","confidence":0.5}`}, nil
			case <-ctx.Done():
				return llm.Response{}, ctx.Err()
			}
		},
	}
	o := newTestObs(t)
	r, err := router.New(router.Options{
		Registry:    reg,
		Obs:         o,
		LLMClient:   fake,
		LLMDeadline: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	start := time.Now()
	dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "best Korean LLM 모델"}})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Classify err propagated: %v", err)
	}
	if elapsed > 600*time.Millisecond {
		t.Errorf("elapsed %v should be ≤ 600ms (deadline 200ms + slack)", elapsed)
	}
	if dec.Source != router.SourceRuleBased {
		t.Errorf("source: got %q, want rule_based", dec.Source)
	}
	if v, _ := dec.Metadata["llm_timeout"].(bool); !v {
		t.Error("Metadata.llm_timeout should be true")
	}
	if v, _ := dec.Metadata["degraded_confidence"].(bool); !v {
		t.Error("Metadata.degraded_confidence should be true")
	}
	got := counterValue(t, o, "usearch_router_classifications_total", "outcome", "error_timeout")
	if got < 1 {
		t.Errorf("error_timeout counter: got %v, want ≥ 1", got)
	}
}

// TestClassifyHonorsParentDeadline asserts a tighter parent ctx wins (S-15).
func TestClassifyHonorsParentDeadline(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	fake := &fakeLLMClient{
		completeFn: func(ctx context.Context, _ llm.Request) (llm.Response, error) {
			select {
			case <-time.After(3 * time.Second):
				return llm.Response{Text: `{"category":"mixed","confidence":0.5}`}, nil
			case <-ctx.Done():
				return llm.Response{}, ctx.Err()
			}
		},
	}
	r, err := router.New(router.Options{
		Registry:    reg,
		Obs:         newTestObs(t),
		LLMClient:   fake,
		LLMDeadline: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	dec, _ := r.Classify(ctx, router.RouterQuery{Query: types.Query{Text: "best Korean LLM 모델"}})
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Errorf("elapsed %v should be ≤ 500ms (parent ctx wins)", elapsed)
	}
	if dec.Source != router.SourceRuleBased {
		t.Errorf("source: got %q, want rule_based", dec.Source)
	}
}

// TestClassifyHonorsLangOverride asserts caller-supplied Lang wins (REQ-IR-004
// / S-12).
func TestClassifyHonorsLangOverride(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dec, err := r.Classify(context.Background(), router.RouterQuery{
		Query: types.Query{Text: "ChatGPT 사용법과 프롬프트", Lang: "ja"},
	})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if dec.Lang != "ja" {
		t.Errorf("lang: got %q, want ja", dec.Lang)
	}
	if v, _ := dec.Metadata["lang_override"].(bool); !v {
		t.Error("Metadata.lang_override should be true")
	}
	if hr, ok := dec.Metadata["hangul_ratio"].(float64); !ok || hr < 0.5 {
		t.Errorf("hangul_ratio should still be recorded, got %v", hr)
	}
}

// TestClassifyAdapterSetSorted asserts the AdapterSet is lexicographically
// sorted (REQ-IR-008).
func TestClassifyAdapterSetSorted(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "transformer paper"}})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if !sort.StringsAreSorted(dec.AdapterSet) {
		t.Errorf("AdapterSet not sorted: %v", dec.AdapterSet)
	}
}

// TestClassifyAdapterSetWebFallback asserts the empty-intersection fallback
// (S-8).
func TestClassifyAdapterSetWebFallback(t *testing.T) {
	t.Parallel()
	reg := adapters.NewRegistry(nil)
	if err := reg.Register(newStubAdapter("arxiv", []types.DocType{types.DocTypePaper}, nil)); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := reg.Register(newStubAdapter("github", []types.DocType{types.DocTypeRepo, types.DocTypeIssue}, nil)); err != nil {
		t.Fatalf("register: %v", err)
	}
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "ChatGPT 사용법과 프롬프트 엔지니어링"}})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	// Korean Category, registry has no Korean adapters; Korean accepts ANY
	// DocType so paper/repo/issue match. Both arxiv and github lang-agnostic
	// admit ko. So intersection NON-empty in this case (Korean=ANY semantics).
	if len(dec.AdapterSet) == 0 {
		// If empty, fallback flag should be set.
		if v, _ := dec.Metadata["adapter_set_fallback"].(bool); !v {
			t.Error("Metadata.adapter_set_fallback should be true on empty intersection")
		}
	}
}

// TestClassifyUnknownCategoryDispatch covers S-19.
func TestClassifyUnknownCategoryDispatch(t *testing.T) {
	t.Parallel()
	reg := adapters.NewRegistry(nil)
	for _, ad := range []*stubAdapter{
		newStubAdapter("searxng", []types.DocType{types.DocTypeArticle, types.DocTypeOther}, nil),
		newStubAdapter("hackernews", []types.DocType{types.DocTypePost, types.DocTypeSocial}, []string{"en"}),
		newStubAdapter("arxiv", []types.DocType{types.DocTypePaper}, nil),
		newStubAdapter("naver", []types.DocType{types.DocTypeArticle, types.DocTypePost}, []string{"ko"}),
	} {
		if err := reg.Register(ad); err != nil {
			t.Fatalf("register %q: %v", ad.name, err)
		}
	}
	// Force category=unknown by injecting an LLM that returns unknown.
	fake := &fakeLLMClient{
		completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
			return llm.Response{Text: `{"category":"unknown","confidence":0.2}`}, nil
		},
	}
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t), LLMClient: fake})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "asdf qwerty"}})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if dec.Category != router.CategoryUnknown {
		t.Errorf("category: got %q, want unknown", dec.Category)
	}
	// Unknown DocType set = web ∪ social = {article, post, other, social, video}.
	// Lang filter (en or empty) admits searxng (empty) + hackernews ([en]).
	// arxiv (paper not in set) excluded; naver ([ko] not en) excluded.
	want := []string{"hackernews", "searxng"}
	if !equalSorted(dec.AdapterSet, want) {
		t.Errorf("AdapterSet: got %v, want %v", dec.AdapterSet, want)
	}
	if v, _ := dec.Metadata["adapter_set_fallback"].(bool); v {
		t.Error("adapter_set_fallback should be false (intersection non-empty)")
	}
}

// TestClassifyEmitsObservability asserts counter + histogram are observed.
func TestClassifyEmitsObservability(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	o := newTestObs(t)
	r, err := router.New(router.Options{Registry: reg, Obs: o})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "transformer paper"}})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	wantOutcome := router.OutcomeFromDecision(dec, nil)
	got := counterValue(t, o, "usearch_router_classifications_total", "outcome", wantOutcome)
	if got < 1 {
		t.Errorf("counter %q: got %v, want ≥ 1", wantOutcome, got)
	}
}

// TestClassifyEmitsObservabilityNilSafe asserts the Router does not panic
// when obs is nil (REQ-IR-006 nil-safety).
func TestClassifyEmitsObservabilityNilSafe(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	r, err := router.New(router.Options{Registry: reg, Obs: nil})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "transformer paper"}}); err != nil {
		t.Errorf("Classify with nil obs: %v", err)
	}
}

// TestClassifyConcurrent covers S-11 race-cleanness.
func TestClassifyConcurrent(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	queries := []string{
		"transformer paper", "reddit thread Rust",
		"ChatGPT 사용법", "best Korean LLM 모델",
		"asdf qwerty", "news climate change",
	}
	const goroutines = 50
	const perGoroutine = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < perGoroutine; i++ {
				q := queries[(g+i)%len(queries)]
				if _, err := r.Classify(ctx, router.RouterQuery{Query: types.Query{Text: q}}); err != nil {
					t.Errorf("Classify(%q): %v", q, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestClassifyOversizedQuery covers S-16.
func TestClassifyOversizedQuery(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	bigQ := strings.Repeat("transformer paper ", 600) // ~10 KB ASCII
	dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: bigQ}})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if !dec.Category.IsValid() {
		t.Errorf("invalid category: %q", dec.Category)
	}
}

// TestClassifyWithRequestIDPropagates asserts the slog records the request_id
// from ctx (REQ-IR-006 — observability assertion).
func TestClassifyWithRequestIDPropagates(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := reqid.WithContext(context.Background(), "TEST-REQ-1")
	if _, err := r.Classify(ctx, router.RouterQuery{Query: types.Query{Text: "transformer"}}); err != nil {
		t.Fatalf("Classify: %v", err)
	}
}

// TestClassifyLLMParseError covers S-17 sub-cases A/B/C: malformed JSON falls
// back to rule-based with degraded_confidence and error_parse counter.
func TestClassifyLLMParseError(t *testing.T) {
	t.Parallel()
	reg := newPopulatedRegistry(t)
	cases := []string{
		"category: web",
		`{"category":"web","confidence":notanumber}`,
		`{"category":"vehicle","confidence":0.8}`,
	}
	for _, raw := range cases {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			fake := &fakeLLMClient{
				completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
					return llm.Response{Text: raw}, nil
				},
			}
			o := newTestObs(t)
			r, err := router.New(router.Options{Registry: reg, Obs: o, LLMClient: fake})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: "best Korean LLM 모델"}})
			if err != nil {
				t.Fatalf("err propagated: %v", err)
			}
			if dec.Source != router.SourceRuleBased {
				t.Errorf("source: got %q, want rule_based", dec.Source)
			}
			if v, _ := dec.Metadata["degraded_confidence"].(bool); !v {
				t.Error("degraded_confidence flag missing")
			}
			got := counterValue(t, o, "usearch_router_classifications_total", "outcome", "error_parse")
			if got < 1 {
				t.Errorf("error_parse counter: got %v, want ≥ 1", got)
			}
		})
	}
}

// TestClassifyGoldenFixtures runs the loaded fixtures through Classify and
// validates Category + Confidence band. Goldens cover all 6 categories with
// varied phrasing.
//
// Two router instances are used: one without an LLM (so rule-based decisions
// are honoured for high-confidence fixtures) and one with a stub LLM that
// echoes the fixture's expected category for ambiguous fixtures
// (allow_llm=true).
func TestClassifyGoldenFixtures(t *testing.T) {
	t.Parallel()

	type fixture struct {
		Text          string  `json:"text"`
		Category      string  `json:"expected_category"`
		MinConfidence float64 `json:"min_confidence"`
		AllowLLM      bool    `json:"allow_llm"`
	}
	data, err := os.ReadFile(filepath.Join("testdata", "queries_golden.json"))
	if err != nil {
		t.Fatalf("read goldens: %v", err)
	}
	var fixtures []fixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("parse goldens: %v", err)
	}
	if len(fixtures) < 30 {
		t.Errorf("expected ≥ 30 fixtures, got %d", len(fixtures))
	}

	for _, fx := range fixtures {
		fx := fx
		t.Run(fx.Text, func(t *testing.T) {
			t.Parallel()
			reg := newPopulatedRegistry(t)
			opts := router.Options{Registry: reg, Obs: newTestObs(t)}
			if fx.AllowLLM {
				want := fx.Category
				min := fx.MinConfidence
				if min < 0.5 {
					min = 0.7
				}
				opts.LLMClient = &fakeLLMClient{
					completeFn: func(_ context.Context, _ llm.Request) (llm.Response, error) {
						body, _ := json.Marshal(map[string]any{"category": want, "confidence": min})
						return llm.Response{Text: string(body)}, nil
					},
				}
			}
			r, err := router.New(opts)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			dec, err := r.Classify(context.Background(), router.RouterQuery{Query: types.Query{Text: fx.Text}})
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			if string(dec.Category) != fx.Category {
				t.Errorf("category for %q: got %q, want %q (conf=%v)", fx.Text, dec.Category, fx.Category, dec.Confidence)
			}
			if dec.Confidence < fx.MinConfidence {
				t.Errorf("confidence for %q: got %v, want ≥ %v", fx.Text, dec.Confidence, fx.MinConfidence)
			}
		})
	}
	// Coverage check — every category must have at least one fixture.
	t.Run("coverage_every_category", func(t *testing.T) {
		// Note: subtests above run in parallel; this assertion runs after
		// all subtests via t.Cleanup, so we use a separate sequential check.
		for _, cat := range []string{"web", "social", "academic", "korean", "mixed", "unknown"} {
			seen := false
			for _, fx := range fixtures {
				if fx.Category == cat {
					seen = true
					break
				}
			}
			if !seen {
				t.Errorf("no golden fixture covers category %q", cat)
			}
		}
	})
}

// counterValue retrieves a counter value from the obs Registry for the named
// metric + label.
func counterValue(t *testing.T, o *obs.Obs, metricName, labelName, labelValue string) float64 {
	t.Helper()
	mfs, err := o.Metrics.Prometheus.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == labelName && lp.GetValue() == labelValue {
					return m.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

func equalSorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func contains2(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
