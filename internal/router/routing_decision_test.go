// Package router_test validates RoutingDecision + JSON marshaling.
package router_test

import (
	"encoding/json"
	"testing"

	"github.com/elymas/universal-search/internal/router"
)

// TestRoutingDecisionMarshalRoundTrip asserts JSON round-trip preserves all
// fields (REQ-IR-001, REQ-IR-006).
func TestRoutingDecisionMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	orig := router.RoutingDecision{
		Category:   router.CategoryAcademic,
		Confidence: 0.92,
		AdapterSet: []string{"arxiv", "github"},
		Lang:       "en",
		Source:     router.SourceRuleBased,
		Metadata: map[string]any{
			"hangul_ratio":  0.0,
			"rule_triggers": []any{"keyword:academic"},
		},
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got router.RoutingDecision
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Category != orig.Category {
		t.Errorf("Category: got %q, want %q", got.Category, orig.Category)
	}
	if got.Confidence != orig.Confidence {
		t.Errorf("Confidence: got %v, want %v", got.Confidence, orig.Confidence)
	}
	if got.Lang != orig.Lang {
		t.Errorf("Lang: got %q, want %q", got.Lang, orig.Lang)
	}
	if got.Source != orig.Source {
		t.Errorf("Source: got %q, want %q", got.Source, orig.Source)
	}
	if len(got.AdapterSet) != len(orig.AdapterSet) {
		t.Errorf("AdapterSet len: got %d, want %d", len(got.AdapterSet), len(orig.AdapterSet))
	}
}

// TestRoutingDecisionEmptyMetadataOmitted asserts Metadata is omitted from
// JSON output when empty (clean serialization).
func TestRoutingDecisionEmptyMetadataOmitted(t *testing.T) {
	t.Parallel()

	d := router.RoutingDecision{
		Category:   router.CategoryWeb,
		Confidence: 0.5,
		AdapterSet: []string{"searxng"},
		Lang:       "en",
		Source:     router.SourceRuleBased,
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got := string(data); contains(got, "metadata") {
		t.Errorf("expected metadata field omitted; got %s", got)
	}
}

// TestRoutingDecisionEmptyAdapterSetSerializesAsArray asserts an empty
// AdapterSet is serialised as a JSON array `[]`, not `null`. This is a
// downstream-friendliness invariant: SPEC-FAN-001 will iterate AdapterSet,
// and ranging over a nil JSON value is safe in Go but ambiguous over the wire.
func TestRoutingDecisionEmptyAdapterSetSerializesAsArray(t *testing.T) {
	t.Parallel()

	d := router.RoutingDecision{
		Category:   router.CategoryUnknown,
		Confidence: 0.0,
		AdapterSet: []string{},
		Lang:       "en",
		Source:     router.SourceDefault,
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !contains(string(data), `"adapter_set":[]`) {
		t.Errorf(`expected "adapter_set":[] in %s`, string(data))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
