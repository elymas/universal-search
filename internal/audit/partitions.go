package audit

import (
	"context"
	"fmt"
	"time"
)

// PartitionInfo describes a monthly partition of audit_events.
type PartitionInfo struct {
	Name       string     `json:"partition_name"`
	RangeStart time.Time  `json:"range_start"`
	RangeEnd   time.Time  `json:"range_end"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	RowCount   int64      `json:"row_count"`
}

// PartitionManager handles monthly partition lifecycle.
// REQ-AUTH3-001: partitions created monthly via PARTITION BY RANGE (ts).
type PartitionManager struct {
	db DBTX
}

// DBTX is the minimal database interface for partition operations.
// This allows mock-based testing without a real Postgres connection.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (interface{}, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) interface{}
}

// NewPartitionManager creates a PartitionManager with the given DB interface.
func NewPartitionManager(db DBTX) *PartitionManager {
	return &PartitionManager{db: db}
}

// EnsureCurrentPartition creates partitions for the current month and next month
// if they do not already exist. Called at startup.
func (pm *PartitionManager) EnsureCurrentPartition(ctx context.Context) error {
	if pm.db == nil {
		return fmt.Errorf("audit: partition manager has no database connection")
	}

	now := time.Now().UTC()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	nextMonth := currentMonth.AddDate(0, 1, 0)

	for _, start := range []time.Time{currentMonth, nextMonth} {
		_, err := pm.db.ExecContext(ctx, "SELECT create_audit_partition($1)", start)
		if err != nil {
			return fmt.Errorf("audit: ensure partition for %s: %w", start.Format("2006-01"), err)
		}
	}
	return nil
}

// PartitionName generates the partition table name for a given time.
func PartitionName(t time.Time) string {
	return fmt.Sprintf("audit_events_y%04dm%02d", t.Year(), t.Month())
}

// MonthRange returns the start and end of the month containing t.
func MonthRange(t time.Time) (start, end time.Time) {
	start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	end = start.AddDate(0, 1, 0)
	return
}
