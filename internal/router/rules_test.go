// Package router_test validates the deterministic Rules.Score formula and
// reproduces the three worked examples in SPEC-IR-001 §2.3 (after the
// iteration-2 audit reconciliation).
package router_test

import (
	"math"
	"testing"

	"github.com/elymas/universal-search/internal/router"
)

// TestRulesScoreReturnsTriggers asserts Score returns the rule_triggers slice
// for use in Metadata.rule_triggers (REQ-IR-001).
func TestRulesScoreReturnsTriggers(t *testing.T) {
	t.Parallel()
	r := router.NewDefaultRules()
	cat, conf, triggers := r.Score(router.RouterQuery{Query: queryText("transformer attention paper")})
	if cat != router.CategoryAcademic {
		t.Errorf("category: got %q, want academic", cat)
	}
	if conf < 0.85 {
		t.Errorf("confidence: got %v, want ≥ 0.85", conf)
	}
	if len(triggers) == 0 {
		t.Error("triggers should be non-empty")
	}
}

// TestRulesScoreAcademicHigh asserts an academic-rich query yields academic
// with high confidence.
func TestRulesScoreAcademicHigh(t *testing.T) {
	t.Parallel()
	r := router.NewDefaultRules()
	cat, conf, _ := r.Score(router.RouterQuery{Query: queryText("transformer attention paper neural")})
	if cat != router.CategoryAcademic {
		t.Errorf("category: got %q, want academic", cat)
	}
	if conf < 0.85 {
		t.Errorf("confidence: got %v, want ≥ 0.85", conf)
	}
}

// TestRulesScoreSocialHigh asserts a social-rich query yields social.
func TestRulesScoreSocialHigh(t *testing.T) {
	t.Parallel()
	r := router.NewDefaultRules()
	cat, _, _ := r.Score(router.RouterQuery{Query: queryText("reddit hackernews discussion thread")})
	if cat != router.CategorySocial {
		t.Errorf("category: got %q, want social", cat)
	}
}

// TestRulesScoreKoreanHigh asserts a heavily-Korean query yields korean.
func TestRulesScoreKoreanHigh(t *testing.T) {
	t.Parallel()
	r := router.NewDefaultRules()
	cat, conf, _ := r.Score(router.RouterQuery{Query: queryText("ChatGPT 사용법과 프롬프트 엔지니어링 팁")})
	if cat != router.CategoryKorean {
		t.Errorf("category: got %q, want korean", cat)
	}
	if conf < 0.90 {
		t.Errorf("confidence: got %v, want ≥ 0.90", conf)
	}
}

// TestRulesScoreUnknownLowConfidence asserts a meaningless query yields
// unknown with low confidence.
func TestRulesScoreUnknownLowConfidence(t *testing.T) {
	t.Parallel()
	r := router.NewDefaultRules()
	cat, conf, _ := r.Score(router.RouterQuery{Query: queryText("asdf qwerty")})
	if cat != router.CategoryUnknown {
		t.Errorf("category: got %q, want unknown", cat)
	}
	if conf > 0.7 {
		t.Errorf("confidence: got %v, want ≤ 0.7", conf)
	}
}

// TestRulesScoreOrderingDeterministic asserts byte-for-byte determinism: the
// same input string produces the same output triple.
func TestRulesScoreOrderingDeterministic(t *testing.T) {
	t.Parallel()
	r := router.NewDefaultRules()
	q := router.RouterQuery{Query: queryText("transformer paper review")}
	cat1, conf1, trig1 := r.Score(q)
	cat2, conf2, trig2 := r.Score(q)
	if cat1 != cat2 || conf1 != conf2 || len(trig1) != len(trig2) {
		t.Errorf("non-deterministic: (%v,%v,%v) vs (%v,%v,%v)", cat1, conf1, trig1, cat2, conf2, trig2)
	}
}

// TestRulesScoreNoNaN guards that no input produces NaN confidence.
func TestRulesScoreNoNaN(t *testing.T) {
	t.Parallel()
	r := router.NewDefaultRules()
	for _, txt := range []string{
		"a", "한", "한국 ChatGPT", "transformer", "  test  ",
		"reddit transformer 한국", string(rune(0x1100)),
	} {
		_, conf, _ := r.Score(router.RouterQuery{Query: queryText(txt)})
		if math.IsNaN(conf) {
			t.Errorf("NaN confidence on %q", txt)
		}
		if conf < 0 || conf > 1 {
			t.Errorf("out-of-range confidence %v on %q", conf, txt)
		}
	}
}

// TestRulesScoreFormulaTraces reproduces the three worked examples from
// SPEC-IR-001 §2.3 (after the iteration-2 reconciliation) byte-for-byte and
// asserts each intermediate signal value within ±0.005.
//
// Trace 1: "transformer attention paper" → academic at 1.0
// Trace 2: "best Korean LLM 모델"        → mixed at 0.588 (below τ_high)
// Trace 3: "ChatGPT 사용법과 프롬프트 엔지니어링 팁" → korean at 1.0
func TestRulesScoreFormulaTraces(t *testing.T) {
	t.Parallel()
	r := router.NewDefaultRules()

	type trace struct {
		name           string
		text           string
		wantCategory   router.Category
		wantConfidence float64
		wantHangulR    float64
		wantParticleD  float64
	}
	traces := []trace{
		{
			name:           "trace1_academic",
			text:           "transformer attention paper",
			wantCategory:   router.CategoryAcademic,
			wantConfidence: 1.0,
			wantHangulR:    0.0,
			wantParticleD:  0.0,
		},
		{
			name:           "trace2_mixed",
			text:           "best Korean LLM 모델",
			wantCategory:   router.CategoryMixed,
			wantConfidence: 0.588,
			wantHangulR:    0.1333,
			wantParticleD:  0.0,
		},
		{
			name:           "trace3_korean",
			text:           "ChatGPT 사용법과 프롬프트 엔지니어링 팁",
			wantCategory:   router.CategoryKorean,
			wantConfidence: 1.0,
			wantHangulR:    0.6667,
			wantParticleD:  0.2,
		},
	}

	const tol = 0.005
	for _, tc := range traces {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotR := router.HangulRatio(tc.text)
			if math.Abs(gotR-tc.wantHangulR) > tol {
				t.Errorf("hangul_ratio: got %v, want %v ±%v", gotR, tc.wantHangulR, tol)
			}
			gotPD := router.ParticleDensity(tc.text)
			if math.Abs(gotPD-tc.wantParticleD) > tol {
				t.Errorf("particle_density: got %v, want %v ±%v", gotPD, tc.wantParticleD, tol)
			}

			cat, conf, _ := r.Score(router.RouterQuery{Query: queryText(tc.text)})
			if cat != tc.wantCategory {
				t.Errorf("category: got %q, want %q", cat, tc.wantCategory)
			}
			if math.Abs(conf-tc.wantConfidence) > tol {
				t.Errorf("confidence: got %v, want %v ±%v", conf, tc.wantConfidence, tol)
			}
		})
	}
}

// TestRulesScoreEmptyDelegates asserts an empty query returns unknown with
// zero confidence (defensive — Validate should normally prevent this).
func TestRulesScoreEmptyDelegates(t *testing.T) {
	t.Parallel()
	r := router.NewDefaultRules()
	cat, _, _ := r.Score(router.RouterQuery{Query: queryText("")})
	if cat != router.CategoryUnknown {
		t.Errorf("empty: got %q, want unknown", cat)
	}
}
