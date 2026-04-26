// Package router_test validates the Category enum and helper mappings.
package router_test

import (
	"sort"
	"testing"

	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// TestCategoryEnumComplete asserts that all six SPEC-IR-001 Category values
// exist with their canonical string representations (REQ-IR-001).
func TestCategoryEnumComplete(t *testing.T) {
	t.Parallel()

	want := map[router.Category]string{
		router.CategoryWeb:      "web",
		router.CategorySocial:   "social",
		router.CategoryAcademic: "academic",
		router.CategoryKorean:   "korean",
		router.CategoryMixed:    "mixed",
		router.CategoryUnknown:  "unknown",
	}
	for cat, str := range want {
		if string(cat) != str {
			t.Errorf("Category %q != %q", cat, str)
		}
	}
}

// TestClassificationSourceEnumComplete asserts the source enum values.
func TestClassificationSourceEnumComplete(t *testing.T) {
	t.Parallel()

	want := map[router.ClassificationSource]string{
		router.SourceRuleBased:    "rule_based",
		router.SourceLLMFallback:  "llm_fallback",
		router.SourceDefault:      "default",
		router.SourceLangOverride: "lang_override",
	}
	for src, str := range want {
		if string(src) != str {
			t.Errorf("ClassificationSource %q != %q", src, str)
		}
	}
}

// TestCategoryEligibleDocTypesWeb asserts the web Category maps to article,
// post, other (REQ-IR-008).
func TestCategoryEligibleDocTypesWeb(t *testing.T) {
	t.Parallel()
	got := docTypeSet(t, router.CategoryEligibleDocTypes(router.CategoryWeb))
	want := docTypeSet(t, []types.DocType{types.DocTypeArticle, types.DocTypePost, types.DocTypeOther})
	if !sameStringSet(got, want) {
		t.Errorf("web eligible: got %v, want %v", got, want)
	}
}

// TestCategoryEligibleDocTypesSocial asserts social maps to post, social,
// video.
func TestCategoryEligibleDocTypesSocial(t *testing.T) {
	t.Parallel()
	got := docTypeSet(t, router.CategoryEligibleDocTypes(router.CategorySocial))
	want := docTypeSet(t, []types.DocType{types.DocTypePost, types.DocTypeSocial, types.DocTypeVideo})
	if !sameStringSet(got, want) {
		t.Errorf("social eligible: got %v, want %v", got, want)
	}
}

// TestCategoryEligibleDocTypesAcademic asserts academic maps to paper, repo,
// issue.
func TestCategoryEligibleDocTypesAcademic(t *testing.T) {
	t.Parallel()
	got := docTypeSet(t, router.CategoryEligibleDocTypes(router.CategoryAcademic))
	want := docTypeSet(t, []types.DocType{types.DocTypePaper, types.DocTypeRepo, types.DocTypeIssue})
	if !sameStringSet(got, want) {
		t.Errorf("academic eligible: got %v, want %v", got, want)
	}
}

// TestCategoryEligibleDocTypesKoreanIsAny asserts Korean Category passes ANY
// DocType (research §1.4 — Korean is language-driven, not type-driven).
func TestCategoryEligibleDocTypesKoreanIsAny(t *testing.T) {
	t.Parallel()
	got := router.CategoryEligibleDocTypes(router.CategoryKorean)
	if len(got) == 0 {
		t.Fatal("Korean Category returned no DocTypes")
	}
	// Should at minimum contain the union of web and social DocTypes.
	gotSet := docTypeSet(t, got)
	for _, must := range []types.DocType{types.DocTypeArticle, types.DocTypePost, types.DocTypePaper, types.DocTypeOther} {
		if !gotSet[string(must)] {
			t.Errorf("Korean Category should include DocType %q", must)
		}
	}
}

// TestCategoryEligibleDocTypesMixedIsAny asserts Mixed Category passes ANY.
func TestCategoryEligibleDocTypesMixedIsAny(t *testing.T) {
	t.Parallel()
	got := router.CategoryEligibleDocTypes(router.CategoryMixed)
	if len(got) == 0 {
		t.Fatal("Mixed Category returned no DocTypes")
	}
}

// TestCategoryEligibleDocTypesUnknownIsWebSocialUnion asserts that Unknown
// Category returns the union of web-eligible and social-eligible DocTypes
// (REQ-IR-008 + research OQ-6 resolution).
func TestCategoryEligibleDocTypesUnknownIsWebSocialUnion(t *testing.T) {
	t.Parallel()
	got := docTypeSet(t, router.CategoryEligibleDocTypes(router.CategoryUnknown))
	wantUnion := docTypeSet(t, []types.DocType{
		types.DocTypeArticle, types.DocTypePost, types.DocTypeOther, // web
		types.DocTypeSocial, types.DocTypeVideo, // social additions
	})
	if !sameStringSet(got, wantUnion) {
		t.Errorf("unknown eligible: got %v, want union %v", got, wantUnion)
	}
}

func docTypeSet(t *testing.T, dts []types.DocType) map[string]bool {
	t.Helper()
	m := make(map[string]bool, len(dts))
	for _, dt := range dts {
		m[string(dt)] = true
	}
	return m
}

func sameStringSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	keysA := make([]string, 0, len(a))
	for k := range a {
		keysA = append(keysA, k)
	}
	sort.Strings(keysA)
	keysB := make([]string, 0, len(b))
	for k := range b {
		keysB = append(keysB, k)
	}
	sort.Strings(keysB)
	for i := range keysA {
		if keysA[i] != keysB[i] {
			return false
		}
	}
	return true
}
