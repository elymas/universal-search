// Package router_test — benchmarks for NFR-IR-001 rule-based path performance.
package router_test

import (
	"context"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/obs/metrics"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// BenchmarkClassifyRulePath100Chars exercises the rule-based path on a 100-
// character ASCII academic query. NFR-IR-001 requires p50 ≤ 1ms with bounded
// allocations.
func BenchmarkClassifyRulePath100Chars(b *testing.B) {
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName:    "router-bench",
		ServiceVersion: "test",
		LogLevel:       "ERROR",
	})
	if err != nil {
		b.Fatalf("obs.Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()
	o.Metrics = metrics.NewRegistry()

	reg := adapters.NewRegistry(nil)
	for _, name := range []string{"naver", "daum", "rss_korean", "hackernews", "searxng", "arxiv", "github"} {
		ad := &stubAdapter{name: name, caps: types.Capabilities{
			SourceID:       name,
			DocTypes:       []types.DocType{types.DocTypeArticle, types.DocTypePost},
			SupportedLangs: nil,
		}}
		_ = reg.Register(ad)
	}

	r, err := router.New(router.Options{Registry: reg, Obs: o})
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	text := pad100("transformer attention paper neural gradient regression scaling benchmark")
	q := router.RouterQuery{Query: types.Query{Text: text}}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := r.Classify(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRulesScore exercises the Score function in isolation.
func BenchmarkRulesScore(b *testing.B) {
	rules := router.NewDefaultRules()
	q := router.RouterQuery{Query: types.Query{Text: pad100("transformer attention paper neural gradient")}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = rules.Score(q)
	}
}

func pad100(s string) string {
	if len(s) >= 100 {
		return s
	}
	rep := 100 - len(s)
	return s + strings.Repeat(" x", rep/2)
}
