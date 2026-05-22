package tenancy

import (
	"encoding/json"
	"os"
)

// BackfillState tracks progress of multi-store backfill for crash-resume.
// REQ-IDX4-011: state.json per-store last_processed_doc_id.
type BackfillState struct {
	PG     StoreProgress `json:"pg"`
	Qdrant StoreProgress `json:"qdrant"`
	Meili  StoreProgress `json:"meili"`
}

// StoreProgress tracks per-store backfill progress.
type StoreProgress struct {
	LastDocID string `json:"last_processed_doc_id"`
	Processed int    `json:"processed"`
}

// LoadBackfillState reads backfill state from a JSON file.
func LoadBackfillState(path string) (*BackfillState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &BackfillState{}, nil
		}
		return nil, err
	}
	var state BackfillState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// SaveBackfillState writes backfill state to a JSON file.
func SaveBackfillState(path string, state *BackfillState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// DeleteBackfillState removes the state file (after successful completion).
func DeleteBackfillState(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
