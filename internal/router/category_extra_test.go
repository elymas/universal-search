package router_test

// Coverage for the exported Category enumeration helpers and the empty-match
// fallback path in selectAdapterSet (reached through Classify).
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"context"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

func TestAllCategoriesEnumeration(t *testing.T) {
	got := router.AllCategories()
	want := []router.Category{
		router.CategoryWeb, router.CategorySocial, router.CategoryAcademic,
		router.CategoryKorean, router.CategoryMixed, router.CategoryUnknown,
	}
	if len(got) != len(want) {
		t.Fatalf("AllCategories len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllCategories[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Every enumerated category must report IsValid == true.
	for _, c := range got {
		if !c.IsValid() {
			t.Errorf("Category %q reported not valid", c)
		}
	}
}

func TestCategoryIsValid_RejectsUnknownString(t *testing.T) {
	if router.Category("totally-made-up").IsValid() {
		t.Error("an undeclared category string must report IsValid() == false")
	}
}

// TestSelectAdapterSetFallbackFlag forces the empty-intersection fallback in
// selectAdapterSet by registering adapters that can never match a web/English
// query (Korean-only language, non-web doc types, non-empty SupportedLangs so
// the lang-agnostic fallback set is also empty). The decision must carry the
// adapter_set_fallback metadata flag.
func TestSelectAdapterSetFallbackFlag(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	// Korean-only paper adapter: paper is not in the web doc-type set, and the
	// [ko] language excludes an English query; SupportedLangs is non-empty so it
	// is also excluded from the lang-agnostic fallback set.
	if err := reg.Register(newStubAdapter("ko_paper", []types.DocType{types.DocTypePaper}, []string{"ko"})); err != nil {
		t.Fatalf("register: %v", err)
	}

	r, err := router.New(router.Options{Registry: reg, Obs: newTestObs(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	dec, err := r.Classify(context.Background(), router.RouterQuery{
		Query: types.Query{Text: "best programming languages 2026"},
	})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	if len(dec.AdapterSet) != 0 {
		t.Fatalf("expected empty adapter set, got %v", dec.AdapterSet)
	}
	if v, _ := dec.Metadata["adapter_set_fallback"].(bool); !v {
		t.Error("Metadata.adapter_set_fallback must be true when no adapter matches")
	}
}
