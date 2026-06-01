package korean

// Table-driven coverage for validateQuery's schema-error branches. Each case
// starts from a valid query and mutates exactly one field to trigger a specific
// rejection. The valid baseline also covers the success path.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"testing"
)

func validBaseQuery() GoldenQuery {
	return GoldenQuery{
		QueryID:             "KR-001",
		QueryText:           "테스트 질의",
		Category:            CategoryNews,
		ExpectedLang:        "ko",
		ExpectedRouterClass: "korean",
		ExpectedSources:     []string{"naver"},
	}
}

func TestValidateQuery(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(q *GoldenQuery)
		wantErr bool
	}{
		{"valid baseline", func(*GoldenQuery) {}, false},
		{"missing query_id", func(q *GoldenQuery) { q.QueryID = "" }, true},
		{"bad query_id format", func(q *GoldenQuery) { q.QueryID = "BAD-1" }, true},
		{"missing query_text", func(q *GoldenQuery) { q.QueryText = "" }, true},
		{"unknown category", func(q *GoldenQuery) { q.Category = Category("nope") }, true},
		{"bad expected_lang", func(q *GoldenQuery) { q.ExpectedLang = "en" }, true},
		{"bad router_class", func(q *GoldenQuery) { q.ExpectedRouterClass = "web" }, true},
		{"empty expected_sources", func(q *GoldenQuery) { q.ExpectedSources = nil }, true},
		{"phantom source", func(q *GoldenQuery) { q.ExpectedSources = []string{"phantom"} }, true},
		{"valid mixed lang+class", func(q *GoldenQuery) {
			q.ExpectedLang = "mixed"
			q.ExpectedRouterClass = "mixed"
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := validBaseQuery()
			tt.mutate(&q)
			err := validateQuery(&q, 1)
			if tt.wantErr && err == nil {
				t.Errorf("validateQuery(%s) = nil, want error", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateQuery(%s) = %v, want nil", tt.name, err)
			}
		})
	}
}
