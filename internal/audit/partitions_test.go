package audit

import (
	"testing"
	"time"
)

// TestPartitionName verifies partition naming convention.
func TestPartitionName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-02-01", "audit_events_y2026m02"},
		{"2026-12-01", "audit_events_y2026m12"},
		{"2027-01-15", "audit_events_y2027m01"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ts, err := time.Parse("2006-01-02", tt.input)
			if err != nil {
				t.Fatal(err)
			}
			got := PartitionName(ts)
			if got != tt.want {
				t.Errorf("PartitionName(%s) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestMonthRange verifies month boundary calculation.
func TestMonthRange(t *testing.T) {
	ts := time.Date(2026, 2, 15, 10, 30, 0, 0, time.UTC)
	start, end := MonthRange(ts)

	wantStart := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	if !start.Equal(wantStart) {
		t.Errorf("MonthRange start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("MonthRange end = %v, want %v", end, wantEnd)
	}
}

// TestEnsureCurrentPartition_nilDB verifies graceful error on nil DB.
func TestEnsureCurrentPartition_nilDB(t *testing.T) {
	pm := NewPartitionManager(nil)
	err := pm.EnsureCurrentPartition(nil)
	if err == nil {
		t.Error("Expected error with nil DB, got nil")
	}
}

// TestPartitionInfo_fields verifies PartitionInfo struct.
func TestPartitionInfo_fields(t *testing.T) {
	now := time.Now().UTC()
	pi := PartitionInfo{
		Name:       "audit_events_y2026m05",
		RangeStart: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		RangeEnd:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		ArchivedAt: nil,
		RowCount:   180000,
	}

	if pi.Name != "audit_events_y2026m05" {
		t.Errorf("Name = %q, want %q", pi.Name, "audit_events_y2026m05")
	}
	if pi.RowCount != 180000 {
		t.Errorf("RowCount = %d, want 180000", pi.RowCount)
	}
	if pi.ArchivedAt != nil {
		t.Error("ArchivedAt should be nil")
	}

	// Test with archived_at set.
	pi.ArchivedAt = &now
	if pi.ArchivedAt == nil {
		t.Error("ArchivedAt should not be nil after setting")
	}
}
