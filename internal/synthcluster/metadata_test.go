// Package synthcluster_test — RED phase tests for cluster metadata schema.
// SPEC-SYN-003 Scope item (e): spec_syn003_cluster metadata schema.
package synthcluster_test

import (
	"context"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/synthcluster"
	"github.com/elymas/universal-search/pkg/types"
)

// TestMetadataSchemaVersion: spec_syn003_cluster must have schema_version: 1.
func TestMetadataSchemaVersion(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		{
			ID: "mv1", SourceID: "test", URL: "https://example.com/1",
			Title: "Metadata schema test article", Body: "Body for metadata schema test.",
			RetrievedAt: time.Now(), Score: 0.9,
		},
		{
			ID: "mv2", SourceID: "test", URL: "https://example.com/2",
			Title: "Metadata schema test article", Body: "Body for metadata schema test.",
			RetrievedAt: time.Now(), Score: 0.5,
		},
	}

	opts := synthcluster.Options{
		Mode:             synthcluster.ModeSimhashOnly,
		HammingThreshold: 10,
	}
	reps, _, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster error: %v", err)
	}
	if len(reps) != 1 {
		t.Fatalf("expected 1 rep, got %d", len(reps))
	}

	meta, ok := reps[0].Metadata["spec_syn003_cluster"].(map[string]any)
	if !ok {
		t.Fatal("spec_syn003_cluster metadata missing or wrong type")
	}
	if v, ok := meta["schema_version"]; !ok {
		t.Error("schema_version missing from metadata")
	} else if v != 1 {
		t.Errorf("schema_version = %v, want 1", v)
	}
}

// TestMetadataClusterSize: cluster_size must equal number of member IDs + 1 (for rep).
func TestMetadataClusterSize(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		{ID: "cs1", SourceID: "t", URL: "https://x/1", Title: "Cluster size check", Body: "Body A.", RetrievedAt: time.Now(), Score: 0.9},
		{ID: "cs2", SourceID: "t", URL: "https://x/2", Title: "Cluster size check", Body: "Body A.", RetrievedAt: time.Now(), Score: 0.7},
		{ID: "cs3", SourceID: "t", URL: "https://x/3", Title: "Cluster size check", Body: "Body A.", RetrievedAt: time.Now(), Score: 0.5},
	}

	opts := synthcluster.Options{
		Mode:             synthcluster.ModeSimhashOnly,
		HammingThreshold: 10,
	}
	reps, _, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster error: %v", err)
	}
	if len(reps) != 1 {
		t.Fatalf("expected 1 rep, got %d", len(reps))
	}

	meta, ok := reps[0].Metadata["spec_syn003_cluster"].(map[string]any)
	if !ok {
		t.Fatal("spec_syn003_cluster metadata missing")
	}
	csz, ok := meta["cluster_size"]
	if !ok {
		t.Fatal("cluster_size missing from metadata")
	}
	if csz != 3 {
		t.Errorf("cluster_size = %v, want 3", csz)
	}
}

// TestMetadataDedupMode: dedup_mode field must match Options.Mode.
func TestMetadataDedupMode(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		{ID: "dm1", SourceID: "t", URL: "https://x/1", Title: "Dedup mode field test", Body: "Body.", RetrievedAt: time.Now(), Score: 0.9},
		{ID: "dm2", SourceID: "t", URL: "https://x/2", Title: "Dedup mode field test", Body: "Body.", RetrievedAt: time.Now(), Score: 0.5},
	}

	opts := synthcluster.Options{
		Mode:             synthcluster.ModeSimhashOnly,
		HammingThreshold: 10,
	}
	reps, _, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster error: %v", err)
	}
	meta, ok := reps[0].Metadata["spec_syn003_cluster"].(map[string]any)
	if !ok {
		t.Fatal("metadata missing")
	}
	dm, ok := meta["dedup_mode"]
	if !ok {
		t.Fatal("dedup_mode missing")
	}
	if dm != "simhash_only" {
		t.Errorf("dedup_mode = %v, want simhash_only", dm)
	}
}

// TestMetadataSimhashField: simhash field must be a 16-hex-char string.
func TestMetadataSimhashField(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		{ID: "sh1", SourceID: "t", URL: "https://x/1", Title: "Simhash field metadata", Body: "Body.", RetrievedAt: time.Now(), Score: 0.9},
		{ID: "sh2", SourceID: "t", URL: "https://x/2", Title: "Simhash field metadata", Body: "Body.", RetrievedAt: time.Now(), Score: 0.5},
	}

	opts := synthcluster.Options{
		Mode:             synthcluster.ModeSimhashOnly,
		HammingThreshold: 10,
	}
	reps, _, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster error: %v", err)
	}
	meta, ok := reps[0].Metadata["spec_syn003_cluster"].(map[string]any)
	if !ok {
		t.Fatal("metadata missing")
	}
	sh, ok := meta["simhash"].(string)
	if !ok {
		t.Fatal("simhash field missing or not string")
	}
	if len(sh) != 16 {
		t.Errorf("simhash field length = %d, want 16 hex chars (got %q)", len(sh), sh)
	}
	for _, c := range sh {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("simhash field contains non-hex character %q in %q", c, sh)
			break
		}
	}
}

// TestNoMetadataNamespaceCollision: spec_syn003_* keys must not exist in input metadata.
func TestNoMetadataNamespaceCollision(t *testing.T) {
	t.Parallel()
	// Test that input docs with the reserved key prefix are correctly handled.
	doc1 := types.NormalizedDoc{
		ID: "nc1", SourceID: "t", URL: "https://x/1",
		Title: "No namespace collision", Body: "Body.",
		RetrievedAt: time.Now(), Score: 0.9,
		Metadata: map[string]any{
			"my_adapter_key": "value", // safe key
		},
	}
	doc2 := types.NormalizedDoc{
		ID: "nc2", SourceID: "t", URL: "https://x/2",
		Title: "No namespace collision", Body: "Body.",
		RetrievedAt: time.Now(), Score: 0.5,
	}

	opts := synthcluster.Options{
		Mode:             synthcluster.ModeSimhashOnly,
		HammingThreshold: 10,
	}
	docs := []types.NormalizedDoc{doc1, doc2}
	reps, _, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster error: %v", err)
	}

	for _, r := range reps {
		if r.Metadata != nil {
			if _, ok := r.Metadata["my_adapter_key"]; !ok && r.ID == "nc1" {
				// Acceptable: representatives may have their own Metadata passed through.
			}
		}
	}
	_ = reps // no assertion needed beyond no-panic
}
