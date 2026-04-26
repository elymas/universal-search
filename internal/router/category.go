// Package router — Category enum and DocType eligibility mapping.
// SPEC-IR-001: REQ-IR-001, REQ-IR-008.
package router

import "github.com/elymas/universal-search/pkg/types"

// Category enumerates the six classification outcomes the Intent Router emits.
type Category string

// Category values.
const (
	// CategoryWeb tags generic web-search queries (news, blogs, general info).
	CategoryWeb Category = "web"
	// CategorySocial tags social-platform queries (Reddit, HN, X, Bluesky, YouTube).
	CategorySocial Category = "social"
	// CategoryAcademic tags scholarly queries (arXiv, GitHub repos/issues, papers).
	CategoryAcademic Category = "academic"
	// CategoryKorean tags Korean-locale queries (Naver, Daum, Korean RSS).
	CategoryKorean Category = "korean"
	// CategoryMixed tags multi-category code-mixed queries.
	CategoryMixed Category = "mixed"
	// CategoryUnknown tags queries the router could not confidently classify.
	// Recoverable, NOT terminal — see REQ-IR-008.
	CategoryUnknown Category = "unknown"
)

// AllCategories returns every Category value in stable enumeration order.
// Used by golden-fixture coverage assertions.
func AllCategories() []Category {
	return []Category{
		CategoryWeb, CategorySocial, CategoryAcademic,
		CategoryKorean, CategoryMixed, CategoryUnknown,
	}
}

// IsValid reports whether c is one of the six declared Category values.
func (c Category) IsValid() bool {
	switch c {
	case CategoryWeb, CategorySocial, CategoryAcademic,
		CategoryKorean, CategoryMixed, CategoryUnknown:
		return true
	}
	return false
}

// ClassificationSource describes how a RoutingDecision was reached.
type ClassificationSource string

// ClassificationSource values.
const (
	// SourceRuleBased — decision came from the deterministic rule scorer.
	SourceRuleBased ClassificationSource = "rule_based"
	// SourceLLMFallback — decision came from the LLM-fallback adjudicator.
	SourceLLMFallback ClassificationSource = "llm_fallback"
	// SourceDefault — decision is the default when both rules and LLM yielded
	// no signal (rare; used by graceful degradation paths).
	SourceDefault ClassificationSource = "default"
	// SourceLangOverride — decision incorporates a caller-supplied Lang hint
	// (REQ-IR-004). Currently used as a Metadata flag rather than a Source.
	SourceLangOverride ClassificationSource = "lang_override"
)

// IsValid reports whether s is one of the declared sources.
func (s ClassificationSource) IsValid() bool {
	switch s {
	case SourceRuleBased, SourceLLMFallback, SourceDefault, SourceLangOverride:
		return true
	}
	return false
}

// CategoryEligibleDocTypes returns the DocType set considered eligible for
// dispatch when a RoutingDecision lands in category c (REQ-IR-008).
//
// Semantics:
//   - web      → {article, post, other}
//   - social   → {post, social, video}
//   - academic → {paper, repo, issue}
//   - korean   → ANY (Korean is language-driven, see research §1.4)
//   - mixed    → ANY (multi-category by definition)
//   - unknown  → union(web, social) — Unknown is recoverable, not terminal
//     (REQ-IR-008 + research OQ-6 resolution).
//
// The "ANY" case (korean, mixed) returns every declared DocType. Callers
// performing a set-membership test will admit every adapter.
//
// @MX:NOTE: [AUTO] DocType eligibility table — central to REQ-IR-008
// AdapterSet selection. Update only with a SPEC amendment.
// @MX:SPEC: SPEC-IR-001
func CategoryEligibleDocTypes(c Category) []types.DocType {
	switch c {
	case CategoryWeb:
		return []types.DocType{types.DocTypeArticle, types.DocTypePost, types.DocTypeOther}
	case CategorySocial:
		return []types.DocType{types.DocTypePost, types.DocTypeSocial, types.DocTypeVideo}
	case CategoryAcademic:
		return []types.DocType{types.DocTypePaper, types.DocTypeRepo, types.DocTypeIssue}
	case CategoryKorean, CategoryMixed:
		return allDocTypes()
	case CategoryUnknown:
		// Union of web {article, post, other} + social {post, social, video}.
		return []types.DocType{
			types.DocTypeArticle,
			types.DocTypePost,
			types.DocTypeOther,
			types.DocTypeSocial,
			types.DocTypeVideo,
		}
	}
	return nil
}

// allDocTypes returns every canonical DocType. Used for "ANY" Categories.
func allDocTypes() []types.DocType {
	return []types.DocType{
		types.DocTypeArticle,
		types.DocTypePost,
		types.DocTypePaper,
		types.DocTypeVideo,
		types.DocTypeRepo,
		types.DocTypeIssue,
		types.DocTypeSocial,
		types.DocTypeOther,
	}
}
