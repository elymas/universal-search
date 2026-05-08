// Package index — unit tests for Options and applyDefaults (REQ-IDX-001).
package index

import (
	"testing"
	"time"
)

func TestApplyDefaults_ZeroOptions(t *testing.T) {
	t.Parallel()
	o := applyDefaults(Options{Embedder: zeroEmbedder{}})
	if o.MaxParallel != defaultMaxParallel {
		t.Errorf("MaxParallel = %d, want %d", o.MaxParallel, defaultMaxParallel)
	}
	if o.RRFConstantK != defaultRRFConstantK {
		t.Errorf("RRFConstantK = %d, want %d", o.RRFConstantK, defaultRRFConstantK)
	}
	if o.BulkBatchSize != defaultBulkBatchSize {
		t.Errorf("BulkBatchSize = %d, want %d", o.BulkBatchSize, defaultBulkBatchSize)
	}
}

func TestApplyDefaults_PerStoreTimeouts(t *testing.T) {
	t.Parallel()
	o := applyDefaults(Options{Embedder: zeroEmbedder{}})
	wantQdrant := 200 * time.Millisecond
	wantMeili := 300 * time.Millisecond
	wantPG := 100 * time.Millisecond
	if o.PerStoreTimeout["qdrant"] != wantQdrant {
		t.Errorf("qdrant timeout = %v, want %v", o.PerStoreTimeout["qdrant"], wantQdrant)
	}
	if o.PerStoreTimeout["meili"] != wantMeili {
		t.Errorf("meili timeout = %v, want %v", o.PerStoreTimeout["meili"], wantMeili)
	}
	if o.PerStoreTimeout["pg"] != wantPG {
		t.Errorf("pg timeout = %v, want %v", o.PerStoreTimeout["pg"], wantPG)
	}
}

func TestApplyDefaults_RRFWeights(t *testing.T) {
	t.Parallel()
	o := applyDefaults(Options{Embedder: zeroEmbedder{}})
	for _, store := range []string{"qdrant", "meili", "pg"} {
		if o.RRFWeights[store] != 1.0 {
			t.Errorf("RRFWeights[%q] = %v, want 1.0", store, o.RRFWeights[store])
		}
	}
}

func TestApplyDefaults_ExplicitValuesPreserved(t *testing.T) {
	t.Parallel()
	o := applyDefaults(Options{
		Embedder:      zeroEmbedder{},
		MaxParallel:   5,
		RRFConstantK:  100,
		BulkBatchSize: 50,
	})
	if o.MaxParallel != 5 {
		t.Errorf("MaxParallel overwritten: %d", o.MaxParallel)
	}
	if o.RRFConstantK != 100 {
		t.Errorf("RRFConstantK overwritten: %d", o.RRFConstantK)
	}
	if o.BulkBatchSize != 50 {
		t.Errorf("BulkBatchSize overwritten: %d", o.BulkBatchSize)
	}
}

func TestApplyDefaults_CustomPerStoreTimeoutPreserved(t *testing.T) {
	t.Parallel()
	custom := 500 * time.Millisecond
	o := applyDefaults(Options{
		Embedder:        zeroEmbedder{},
		PerStoreTimeout: map[string]time.Duration{"qdrant": custom},
	})
	if o.PerStoreTimeout["qdrant"] != custom {
		t.Errorf("custom qdrant timeout overwritten: %v", o.PerStoreTimeout["qdrant"])
	}
	// Other stores still get defaults.
	if o.PerStoreTimeout["meili"] != 300*time.Millisecond {
		t.Errorf("meili timeout not defaulted: %v", o.PerStoreTimeout["meili"])
	}
}
