package meili_test

import (
	"testing"

	"github.com/elymas/universal-search/internal/index/meili"
)

// TestKoreanIndexName verifies the constant for the Korean index name.
// REQ-IDX-003-011
func TestKoreanIndexName(t *testing.T) {
	t.Parallel()
	if meili.KoreanIndexName != "usearch-ko" {
		t.Errorf("expected KoreanIndexName=%q, got %q", "usearch-ko", meili.KoreanIndexName)
	}
}

// TestKoreanIndexSettings verifies searchable and filterable attributes.
// REQ-IDX-003-011
func TestKoreanIndexSettings(t *testing.T) {
	t.Parallel()
	settings := meili.KoreanIndexSettings()

	// Must have searchable attributes.
	if len(settings.SearchableAttributes) == 0 {
		t.Error("KoreanIndexSettings: no searchable attributes")
	}

	// Must include title and body.
	searchable := map[string]bool{}
	for _, a := range settings.SearchableAttributes {
		searchable[a] = true
	}
	for _, required := range []string{"title", "body"} {
		if !searchable[required] {
			t.Errorf("KoreanIndexSettings: missing searchable attribute %q", required)
		}
	}

	// Must have filterable attributes including lang, doc_type, source_id.
	filterable := map[string]bool{}
	for _, a := range settings.FilterableAttributes {
		filterable[a] = true
	}
	for _, required := range []string{"lang", "doc_type", "source_id"} {
		if !filterable[required] {
			t.Errorf("KoreanIndexSettings: missing filterable attribute %q", required)
		}
	}

	// DistinctAttribute must be set to hash for dedup.
	if settings.DistinctAttribute != "hash" {
		t.Errorf("KoreanIndexSettings: DistinctAttribute=%q, want %q", settings.DistinctAttribute, "hash")
	}
}
