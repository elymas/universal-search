// Package types_test — NormalizedDoc tests for SPEC-CORE-001 REQ-CORE-001/007.
package types_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// fullyPopulatedDoc returns a NormalizedDoc with every field set to a
// distinguishable non-zero value. Reused by JSON, hash, and validate tests.
func fullyPopulatedDoc() types.NormalizedDoc {
	pub := time.Date(2026, 4, 20, 12, 34, 56, 0, time.UTC)
	ret := time.Date(2026, 4, 26, 9, 0, 0, 0, time.UTC)
	return types.NormalizedDoc{
		ID:          "reddit:abc123",
		SourceID:    "reddit",
		URL:         "https://reddit.com/r/golang/comments/abc123",
		Title:       "Why Go's stdlib slog is great",
		Body:        "stdlib slog gives us structured logging without external deps.",
		Snippet:     "stdlib slog gives us...",
		PublishedAt: pub,
		RetrievedAt: ret,
		Author:      "u/gopher42",
		Score:       0.87,
		Lang:        "en",
		DocType:     types.DocTypePost,
		Citations:   []string{"arxiv:2401.12345", "github:golang/go#67890"},
		Metadata:    map[string]any{"upvotes": 1234, "subreddit": "golang", "is_oc": true},
		Hash:        "deadbeefcafebabe",
	}
}

// TestNormalizedDocFieldSet verifies the struct declares exactly 15 exported
// fields with the documented JSON tags.
// REQ-CORE-001.
func TestNormalizedDocFieldSet(t *testing.T) {
	t.Parallel()

	rt := reflect.TypeOf(types.NormalizedDoc{})

	want := []struct {
		Name string
		Tag  string
	}{
		{"ID", "id"},
		{"SourceID", "source_id"},
		{"URL", "url"},
		{"Title", "title"},
		{"Body", "body"},
		{"Snippet", "snippet"},
		{"PublishedAt", "published_at"},
		{"RetrievedAt", "retrieved_at"},
		{"Author", "author"},
		{"Score", "score"},
		{"Lang", "lang"},
		{"DocType", "doc_type"},
		{"Citations", "citations,omitempty"},
		{"Metadata", "metadata,omitempty"},
		{"Hash", "hash"},
	}

	if got := rt.NumField(); got != len(want) {
		t.Fatalf("NormalizedDoc field count = %d, want %d", got, len(want))
	}
	for i, w := range want {
		f := rt.Field(i)
		if f.Name != w.Name {
			t.Errorf("field[%d].Name = %q, want %q", i, f.Name, w.Name)
		}
		gotTag := f.Tag.Get("json")
		if gotTag != w.Tag {
			t.Errorf("field[%d] (%s).json tag = %q, want %q", i, f.Name, gotTag, w.Tag)
		}
	}
}

// TestNormalizedDocJSONRoundTrip verifies a fully-populated doc round-trips
// through json.Marshal / json.Unmarshal preserving every field, including
// Metadata with mixed types.
// REQ-CORE-001.
func TestNormalizedDocJSONRoundTrip(t *testing.T) {
	t.Parallel()

	orig := fullyPopulatedDoc()
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var round types.NormalizedDoc
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Scalars and strings.
	if round.ID != orig.ID {
		t.Errorf("ID round-trip: got %q, want %q", round.ID, orig.ID)
	}
	if round.SourceID != orig.SourceID {
		t.Errorf("SourceID round-trip: got %q, want %q", round.SourceID, orig.SourceID)
	}
	if round.URL != orig.URL {
		t.Errorf("URL round-trip: got %q, want %q", round.URL, orig.URL)
	}
	if round.Title != orig.Title {
		t.Errorf("Title round-trip mismatch")
	}
	if round.Body != orig.Body {
		t.Errorf("Body round-trip mismatch")
	}
	if round.Score != orig.Score {
		t.Errorf("Score round-trip: got %v, want %v", round.Score, orig.Score)
	}
	if round.DocType != orig.DocType {
		t.Errorf("DocType round-trip: got %v, want %v", round.DocType, orig.DocType)
	}
	if round.Hash != orig.Hash {
		t.Errorf("Hash round-trip mismatch")
	}

	// time.Time uses RFC-3339; equality via Equal not ==.
	if !round.PublishedAt.Equal(orig.PublishedAt) {
		t.Errorf("PublishedAt round-trip: got %v, want %v", round.PublishedAt, orig.PublishedAt)
	}
	if !round.RetrievedAt.Equal(orig.RetrievedAt) {
		t.Errorf("RetrievedAt round-trip: got %v, want %v", round.RetrievedAt, orig.RetrievedAt)
	}

	// Citations slice.
	if len(round.Citations) != len(orig.Citations) {
		t.Fatalf("Citations length: got %d, want %d", len(round.Citations), len(orig.Citations))
	}
	for i := range orig.Citations {
		if round.Citations[i] != orig.Citations[i] {
			t.Errorf("Citations[%d]: got %q, want %q", i, round.Citations[i], orig.Citations[i])
		}
	}

	// Metadata map: numeric values become float64 after JSON round-trip.
	upv, ok := round.Metadata["upvotes"]
	if !ok {
		t.Error("Metadata missing upvotes")
	} else if f, isFloat := upv.(float64); !isFloat || int(f) != 1234 {
		t.Errorf("Metadata.upvotes = %v (%T), want 1234", upv, upv)
	}
	if v := round.Metadata["subreddit"]; v != "golang" {
		t.Errorf("Metadata.subreddit = %v, want golang", v)
	}
	if v := round.Metadata["is_oc"]; v != true {
		t.Errorf("Metadata.is_oc = %v, want true", v)
	}

	// CanonicalHash must be re-derivable on the round-tripped doc.
	if round.CanonicalHash() != orig.CanonicalHash() {
		t.Errorf("CanonicalHash differs after round-trip: %q vs %q",
			round.CanonicalHash(), orig.CanonicalHash())
	}
}

// TestNormalizedDocValidateRequiredFields covers the four required-field
// checks plus a fully-populated baseline.
// REQ-CORE-001, REQ-CORE-007.
func TestNormalizedDocValidateRequiredFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		mutate    func(*types.NormalizedDoc)
		wantField string // empty = nil error expected
	}{
		{"complete", func(*types.NormalizedDoc) {}, ""},
		{"missing ID", func(d *types.NormalizedDoc) { d.ID = "" }, "ID"},
		{"missing SourceID", func(d *types.NormalizedDoc) { d.SourceID = "" }, "SourceID"},
		{"missing URL", func(d *types.NormalizedDoc) { d.URL = "" }, "URL"},
		{"zero RetrievedAt", func(d *types.NormalizedDoc) { d.RetrievedAt = time.Time{} }, "RetrievedAt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := fullyPopulatedDoc()
			tc.mutate(&d)
			err := d.Validate()
			if tc.wantField == "" {
				if err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want *ValidationError for field %q", tc.wantField)
			}
			var ve *types.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("errors.As(*ValidationError) = false, err = %v", err)
			}
			if ve.Field != tc.wantField {
				t.Errorf("ValidationError.Field = %q, want %q", ve.Field, tc.wantField)
			}
		})
	}
}

// TestValidationErrorWrapsFieldName is an explicit alias for the
// ValidationError-recovery path in REQ-CORE-007.
func TestValidationErrorWrapsFieldName(t *testing.T) {
	t.Parallel()

	d := fullyPopulatedDoc()
	d.URL = ""
	err := d.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want non-nil")
	}
	var ve *types.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("errors.As(*ValidationError) = false")
	}
	if ve.Field != "URL" {
		t.Errorf("ve.Field = %q, want URL", ve.Field)
	}
}

// TestNormalizedDocCanonicalHashStable verifies CanonicalHash is deterministic
// over repeated calls on the same doc.
// REQ-CORE-001.
func TestNormalizedDocCanonicalHashStable(t *testing.T) {
	t.Parallel()

	d := fullyPopulatedDoc()
	a := d.CanonicalHash()
	b := d.CanonicalHash()
	if a != b {
		t.Errorf("CanonicalHash unstable: %q != %q", a, b)
	}
	if a == "" {
		t.Error("CanonicalHash returned empty string")
	}
	// 16-char lowercase hex per acceptance §5 REQ-CORE-001.
	if len(a) != 16 {
		t.Errorf("CanonicalHash length = %d, want 16", len(a))
	}
	if a != strings.ToLower(a) {
		t.Errorf("CanonicalHash not lowercase: %q", a)
	}
	for _, c := range a {
		isDigit := c >= '0' && c <= '9'
		isHex := c >= 'a' && c <= 'f'
		if !isDigit && !isHex {
			t.Errorf("CanonicalHash contains non-hex char %q", c)
			break
		}
	}
}

// TestCanonicalHashIgnoresMetadata verifies two docs that differ only in
// Metadata produce identical hashes (Metadata is not part of the canonical
// content quartet).
// REQ-CORE-001.
func TestCanonicalHashIgnoresMetadata(t *testing.T) {
	t.Parallel()

	d1 := fullyPopulatedDoc()
	d2 := fullyPopulatedDoc()
	d2.Metadata = map[string]any{"upvotes": 9999, "different": "value"}

	if d1.CanonicalHash() != d2.CanonicalHash() {
		t.Errorf("CanonicalHash should ignore Metadata: %q vs %q",
			d1.CanonicalHash(), d2.CanonicalHash())
	}
}

// TestCanonicalHashChangesWithContent verifies hash IS affected by the
// canonical fields (SourceID, URL, Title, Body).
func TestCanonicalHashChangesWithContent(t *testing.T) {
	t.Parallel()

	base := fullyPopulatedDoc()
	baseHash := base.CanonicalHash()

	mutations := []struct {
		name   string
		mutate func(*types.NormalizedDoc)
	}{
		{"SourceID changes hash", func(d *types.NormalizedDoc) { d.SourceID = "hackernews" }},
		{"URL changes hash", func(d *types.NormalizedDoc) { d.URL = "https://example.com/x" }},
		{"Title changes hash", func(d *types.NormalizedDoc) { d.Title = "Different" }},
		{"Body changes hash", func(d *types.NormalizedDoc) { d.Body = "different body" }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := fullyPopulatedDoc()
			tc.mutate(&d)
			if d.CanonicalHash() == baseHash {
				t.Errorf("CanonicalHash unchanged after %s", tc.name)
			}
		})
	}
}
