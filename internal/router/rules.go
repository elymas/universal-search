// Package router — deterministic rule-based scoring (SPEC-IR-001 §2.3).
//
// Score is a pure function over (hangul_ratio, particle_density,
// kwd_density_C, has_english_token). No randomness, no I/O, no time-dependent
// inputs — same input string always produces the same (Category, confidence,
// triggers) triple, byte-for-byte. The constants below are the source of
// truth for the formula; updating them is a SPEC-amendment-level decision.
package router

import (
	"math"
	"strings"
	"unicode"

	"github.com/elymas/universal-search/pkg/types"
)

// Threshold constants for the rule-based scorer.
//
// @MX:NOTE: [AUTO] Magic constants for the rule scorer. ConfidenceThreshold
// (τ_high) gates LLM escalation. RatioHigh / RatioLow define the Hangul
// ambiguous band. Empirical choice; tunable via SPEC amendment.
// @MX:SPEC: SPEC-IR-001
const (
	// ConfidenceThreshold (τ_high) is the rule-confidence floor above which
	// the LLM-fallback step is skipped (REQ-IR-002).
	ConfidenceThreshold = 0.85
	// RatioHigh is the Hangul-ratio floor above which a query is treated as
	// Korean-primary without LLM adjudication.
	RatioHigh = 0.30
	// RatioLow is the Hangul-ratio ceiling below which a query is treated as
	// non-Korean-primary without LLM adjudication.
	RatioLow = 0.10
)

// Tie-break order from most-specific to least-specific, used by Score when
// two raw scores tie (SPEC-IR-001 §2.3).
//
// @MX:NOTE: [AUTO] Tie-break order is fixed in code; spec.md §2.3 enumerates
// it explicitly. Reordering changes deterministic behaviour for tied inputs.
// @MX:SPEC: SPEC-IR-001
var tieBreakOrder = []Category{
	CategoryAcademic,
	CategoryKorean,
	CategorySocial,
	CategoryMixed,
	CategoryWeb,
	CategoryUnknown,
}

// Default keyword tables. Curated from .moai/project/product.md persona
// language plus manual selection of high-precision terms (research §10).
//
// @MX:NOTE: [AUTO] Keyword tables. Curated single-token, lowercase,
// high-precision terms; tokens that risk false-positive matches against
// generic queries are deliberately excluded. Update only with SPEC review.
// @MX:SPEC: SPEC-IR-001
var (
	defaultAcademicKeywords = []string{
		"transformer", "attention", "paper", "papers", "arxiv", "preprint",
		"neural", "gradient", "regression", "tokenizer", "pretrain", "scaling",
		"benchmark", "abstract", "bibtex", "thesis", "dissertation", "theorem",
		"proof", "doi", "citation", "lemma", "corollary", "axiom", "phd",
		"neurips", "iclr", "icml", "acl", "kdd",
	}
	defaultSocialKeywords = []string{
		"reddit", "subreddit", "hackernews", "ycombinator",
		"tweet", "tweets", "bluesky", "bsky",
		"youtube", "tiktok", "instagram", "discord", "telegram",
		"polymarket", "stackoverflow", "thread", "post", "discussion",
		"upvote", "downvote", "comment", "comments", "viral", "trending",
		"meme",
	}
	defaultWebKeywords = []string{
		"news", "blog", "blogs", "tutorial", "guide", "review", "compare",
		"comparison", "tips", "wiki", "wikipedia", "documentation", "docs",
		"homepage", "site", "tutorials", "guides", "reviews", "newsletter",
		"latest", "recent",
	}
)

// Rules carries the keyword tables and exposes the Score method. Construct
// via NewDefaultRules; the type is exported so callers can inject custom
// tables in tests.
type Rules struct {
	academic []string
	social   []string
	web      []string
}

// NewDefaultRules constructs Rules with the package-default keyword tables.
func NewDefaultRules() *Rules {
	return &Rules{
		academic: defaultAcademicKeywords,
		social:   defaultSocialKeywords,
		web:      defaultWebKeywords,
	}
}

// Score returns the rule-based classification for q.
//
// Implements the formula in SPEC-IR-001 §2.3:
//   - per-category raw scores from (hangul_ratio, particle_density,
//     kwd_density_C, has_english_token);
//   - argmax aggregation;
//   - fixed tie-break order (academic > korean > social > mixed > web > unknown).
//
// Returns (Category, confidence ∈ [0,1], triggers []string).
func (r *Rules) Score(q RouterQuery) (Category, float64, []string) {
	text := q.Text
	if strings.TrimFunc(text, unicode.IsSpace) == "" {
		return CategoryUnknown, 0, nil
	}

	hangul := HangulRatio(text)
	particles := ParticleDensity(text)

	tokens := strings.Fields(text)
	totalTokens := len(tokens)

	hasEnglish := false
	for _, tok := range tokens {
		if isAsciiAlphaToken(tok, 3) {
			hasEnglish = true
			break
		}
	}

	kwdAcademic := keywordDensity(tokens, r.academic)
	kwdSocial := keywordDensity(tokens, r.social)
	kwdWeb := keywordDensity(tokens, r.web)

	scoreKorean := computeScoreKorean(hangul, particles)
	scoreAcademic := clamp01(0.8*kwdAcademic + 0.2*(1-hangul))
	socialIndicator := indicator(kwdSocial > 0)
	scoreSocial := clamp01(0.7*kwdSocial + 0.2*(1-hangul) + 0.1*socialIndicator)
	webIndicator := indicator(kwdWeb > 0)
	scoreWeb := clamp01(0.5*kwdWeb + 0.3*(1-hangul) + 0.2*webIndicator)
	scoreMixed := computeScoreMixed(hangul, hasEnglish)
	maxNonUnknown := maxFloat(scoreWeb, scoreSocial, scoreAcademic, scoreKorean, scoreMixed)
	scoreUnknown := clamp01(1.0 - maxNonUnknown)

	rawScores := map[Category]float64{
		CategoryAcademic: scoreAcademic,
		CategoryKorean:   scoreKorean,
		CategorySocial:   scoreSocial,
		CategoryMixed:    scoreMixed,
		CategoryWeb:      scoreWeb,
		CategoryUnknown:  scoreUnknown,
	}

	winner, confidence := pickWinner(rawScores)
	if math.IsNaN(confidence) {
		confidence = 0.5
	}

	triggers := buildTriggers(winner, hangul, particles, kwdAcademic, kwdSocial, kwdWeb, hasEnglish, totalTokens)

	return winner, confidence, triggers
}

// computeScoreKorean implements the score_korean piecewise formula.
func computeScoreKorean(hangul, particles float64) float64 {
	switch {
	case hangul >= RatioHigh:
		return clamp01(hangul + 0.4 + 0.1*particles)
	case hangul >= RatioLow:
		return clamp01(0.3 + 0.5*(hangul-RatioLow)/(RatioHigh-RatioLow))
	default:
		return clamp01(0.1 * particles)
	}
}

// computeScoreMixed implements the score_mixed piecewise formula. Peaks at
// hangul=0.25 with peak value 0.9; floor at boundaries 0.10 and 0.40 of 0.5.
func computeScoreMixed(hangul float64, hasEnglish bool) float64 {
	if hangul < RatioLow || hangul > 0.40 {
		return 0
	}
	if !hasEnglish {
		return 0
	}
	return clamp01(0.5 + 0.4*(1.0-math.Abs(hangul-0.25)/0.15))
}

// pickWinner returns the (winner, confidence) using the fixed tie-break order.
func pickWinner(scores map[Category]float64) (Category, float64) {
	bestCat := CategoryUnknown
	bestScore := math.Inf(-1)
	for _, cat := range tieBreakOrder {
		s := scores[cat]
		if s > bestScore {
			bestScore = s
			bestCat = cat
		}
	}
	return bestCat, bestScore
}

// keywordDensity returns matched-tokens / total-tokens. Tokens are
// lower-cased before comparison; keywords MUST already be lowercase.
func keywordDensity(tokens, keywords []string) float64 {
	if len(tokens) == 0 {
		return 0
	}
	hits := 0
	for _, tok := range tokens {
		lower := strings.ToLower(tok)
		for _, kw := range keywords {
			if lower == kw {
				hits++
				break
			}
		}
	}
	return float64(hits) / float64(len(tokens))
}

// isAsciiAlphaToken reports whether tok is composed entirely of ASCII letters
// of length ≥ minLen.
func isAsciiAlphaToken(tok string, minLen int) bool {
	if len(tok) < minLen {
		return false
	}
	for _, r := range tok {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

// clamp01 bounds x into [0, 1].
func clamp01(x float64) float64 {
	if math.IsNaN(x) {
		return 0
	}
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// indicator returns 1.0 when b is true, else 0.0.
func indicator(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// maxFloat returns the maximum of vs. Returns -Inf if vs is empty (caller
// guarantees non-empty in our usage).
func maxFloat(vs ...float64) float64 {
	m := math.Inf(-1)
	for _, v := range vs {
		if v > m {
			m = v
		}
	}
	return m
}

// buildTriggers returns the names of signals that contributed non-zero mass
// to the winning category's raw score. Used as Metadata.rule_triggers.
func buildTriggers(winner Category, hangul, particles, kwdA, kwdS, kwdW float64, hasEnglish bool, totalTokens int) []string {
	out := make([]string, 0, 4)
	switch winner {
	case CategoryKorean:
		switch {
		case hangul >= RatioHigh:
			out = append(out, "hangul_ratio_high")
		case hangul >= RatioLow:
			out = append(out, "hangul_ratio_ambiguous")
		}
		if particles > 0 {
			out = append(out, "particle_density")
		}
	case CategoryAcademic:
		if kwdA > 0 {
			out = append(out, "kwd_density_academic")
		}
		out = append(out, "non_korean_baseline")
	case CategorySocial:
		if kwdS > 0 {
			out = append(out, "kwd_density_social")
			out = append(out, "social_indicator")
		}
		out = append(out, "non_korean_baseline")
	case CategoryWeb:
		if kwdW > 0 {
			out = append(out, "kwd_density_web")
			out = append(out, "web_indicator")
		}
		out = append(out, "non_korean_baseline")
	case CategoryMixed:
		out = append(out, "mixed_band")
		if hasEnglish {
			out = append(out, "has_english_token")
		}
	case CategoryUnknown:
		out = append(out, "no_strong_signal")
	}
	if totalTokens == 0 {
		return out
	}
	return out
}

// pkg/types import retention guard.
var _ types.Query
