package golden_test

import (
	"testing"

	"github.com/elymas/universal-search/internal/eval/golden"
)

// TestGoldenSetCount asserts the golden set contains exactly 50 query records.
// REQ-EVAL1-001.
func TestGoldenSetCount(t *testing.T) {
	t.Parallel()
	set, err := golden.LoadQueries(golden.QueriesPath())
	if err != nil {
		t.Fatalf("LoadQueries: %v", err)
	}
	if len(set) != 50 {
		t.Fatalf("expected 50 queries, got %d", len(set))
	}
}

// TestGoldenSetSchema asserts every record has required fields and valid enums.
// REQ-EVAL1-001.
func TestGoldenSetSchema(t *testing.T) {
	t.Parallel()
	set, err := golden.LoadQueries(golden.QueriesPath())
	if err != nil {
		t.Fatalf("LoadQueries: %v", err)
	}
	validLocale := map[string]bool{"en": true, "ko": true}
	validCategory := map[string]bool{
		"factual": true, "comparison": true, "synthesis": true,
		"korean": true, "edge": true,
	}
	seen := make(map[string]bool, len(set))
	for i, q := range set {
		if q.ID == "" {
			t.Errorf("record %d: empty id", i)
		}
		if seen[q.ID] {
			t.Errorf("record %d: duplicate id %q", i, q.ID)
		}
		seen[q.ID] = true
		if q.Query == "" {
			t.Errorf("record %s: empty query", q.ID)
		}
		if !validLocale[q.Locale] {
			t.Errorf("record %s: invalid locale %q", q.ID, q.Locale)
		}
		if !validCategory[q.Category] {
			t.Errorf("record %s: invalid category %q", q.ID, q.Category)
		}
	}
}

// TestGoldenSetLocalePartition asserts the 35 EN + 15 KO partition (HISTORY D2).
// REQ-EVAL1-001.
func TestGoldenSetLocalePartition(t *testing.T) {
	t.Parallel()
	set, err := golden.LoadQueries(golden.QueriesPath())
	if err != nil {
		t.Fatalf("LoadQueries: %v", err)
	}
	var en, ko int
	for _, q := range set {
		switch q.Locale {
		case "en":
			en++
		case "ko":
			ko++
		}
	}
	if en != 35 {
		t.Errorf("expected 35 EN queries, got %d", en)
	}
	if ko != 15 {
		t.Errorf("expected 15 KO queries, got %d", ko)
	}
}

// TestCorpusDeserializes asserts every fixture parses into NormalizedDoc.
// REQ-EVAL1-002.
func TestCorpusDeserializes(t *testing.T) {
	t.Parallel()
	corpus, err := golden.LoadCorpus(golden.CorpusDir())
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	for id, doc := range corpus {
		if doc.ID == "" {
			t.Errorf("doc %q: empty ID after deserialize", id)
		}
		if doc.ID != id {
			t.Errorf("doc map key %q != doc.ID %q", id, doc.ID)
		}
		if doc.Body == "" {
			t.Errorf("doc %q: empty body (judge needs body text)", id)
		}
	}
}

// TestCorpusSize asserts the V1 floor of >= 50 docs.
// REQ-EVAL1-002.
func TestCorpusSize(t *testing.T) {
	t.Parallel()
	corpus, err := golden.LoadCorpus(golden.CorpusDir())
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	if len(corpus) < 50 {
		t.Fatalf("expected >= 50 corpus docs (V1 floor), got %d", len(corpus))
	}
}

// TestExpectedSourcesResolveToCorpus asserts all expected_sources are valid doc IDs.
// REQ-EVAL1-002.
func TestExpectedSourcesResolveToCorpus(t *testing.T) {
	t.Parallel()
	set, err := golden.LoadQueries(golden.QueriesPath())
	if err != nil {
		t.Fatalf("LoadQueries: %v", err)
	}
	corpus, err := golden.LoadCorpus(golden.CorpusDir())
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	for _, q := range set {
		for _, src := range q.ExpectedSources {
			if _, ok := corpus[src]; !ok {
				t.Errorf("query %s: expected_source %q not in corpus", q.ID, src)
			}
		}
	}
}

// TestOverridesSchemaValid asserts overrides parse and have required fields.
// REQ-EVAL1-003.
func TestOverridesSchemaValid(t *testing.T) {
	t.Parallel()
	overrides, err := golden.LoadOverrides(golden.OverridesPath())
	if err != nil {
		t.Fatalf("LoadOverrides: %v", err)
	}
	for i, o := range overrides {
		if o.QueryID == "" {
			t.Errorf("override %d: empty query_id", i)
		}
		if o.ManualOverride != "pass" && o.ManualOverride != "skip" {
			t.Errorf("override %d: invalid manual_override %q", i, o.ManualOverride)
		}
		if o.OverrideReason == "" {
			t.Errorf("override %d: empty override_reason", i)
		}
	}
}

// TestOverridesCapEnforced asserts the cap check rejects > 5 entries.
// REQ-EVAL1-003.
func TestOverridesCapEnforced(t *testing.T) {
	t.Parallel()
	tooMany := make([]golden.Override, 6)
	for i := range tooMany {
		tooMany[i] = golden.Override{QueryID: "EVAL-001-Q001", ManualOverride: "pass", OverrideReason: "x"}
	}
	if err := golden.CheckOverrideCap(tooMany, 5); err == nil {
		t.Fatal("expected cap error for 6 entries with cap 5, got nil")
	}
	if err := golden.CheckOverrideCap(tooMany[:5], 5); err != nil {
		t.Fatalf("expected no error for 5 entries with cap 5, got %v", err)
	}
}
