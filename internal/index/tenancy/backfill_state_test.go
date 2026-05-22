package tenancy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBackfillStateFileNotExist(t *testing.T) {
	t.Parallel()
	state, err := LoadBackfillState("/nonexistent/state.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Error("expected non-nil empty state")
	}
}

func TestSaveAndLoadBackfillState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := &BackfillState{
		PG:     StoreProgress{LastDocID: "doc-001", Processed: 100},
		Qdrant: StoreProgress{LastDocID: "doc-001", Processed: 100},
		Meili:  StoreProgress{LastDocID: "doc-001", Processed: 100},
	}

	if err := SaveBackfillState(path, state); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadBackfillState(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.PG.LastDocID != "doc-001" {
		t.Errorf("PG.LastDocID = %q, want 'doc-001'", loaded.PG.LastDocID)
	}
	if loaded.PG.Processed != 100 {
		t.Errorf("PG.Processed = %d, want 100", loaded.PG.Processed)
	}
}

func TestDeleteBackfillState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := &BackfillState{}
	SaveBackfillState(path, state)

	if err := DeleteBackfillState(path); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("state file should be deleted")
	}
}

func TestDeleteBackfillStateIdempotent(t *testing.T) {
	t.Parallel()
	err := DeleteBackfillState("/nonexistent/state.json")
	if err != nil {
		t.Errorf("deleting nonexistent file should not error: %v", err)
	}
}
