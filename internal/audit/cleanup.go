package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// PartitionDropper handles dropping old audit partitions.
type PartitionDropper interface {
	DropPartition(ctx context.Context, partitionName string) error
}

// Cleanup handles nightly retention cleanup.
// REQ-AUTH3-007: drops partitions older than hot_days.
type Cleanup struct {
	dropper PartitionDropper
	emitter *Emitter
	metrics *Metrics
	cfg     Config
}

// NewCleanup creates a new cleanup handler.
func NewCleanup(dropper PartitionDropper, emitter *Emitter, metrics *Metrics, cfg Config) *Cleanup {
	return &Cleanup{
		dropper: dropper,
		emitter: emitter,
		metrics: metrics,
		cfg:     cfg,
	}
}

// RunCleanup drops partitions older than hot_days.
// REQ-AUTH3-007: nightly job, drops via audit_admin role.
// @MX:WARN: [AUTO] DROP PARTITION is irreversible
// @MX:REASON: DROP is not revertible. require_s3_archive check must pass before execution.
func (c *Cleanup) RunCleanup(ctx context.Context, partitions []PartitionInfo) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -c.cfg.RetentionHotDays)

	for _, p := range partitions {
		// Skip recent partitions.
		if !p.RangeStart.Before(cutoff) {
			slog.Debug("audit: skipping recent partition", "partition", p.Name)
			continue
		}

		// Check S3 archive requirement.
		if c.cfg.RequireS3Archive && p.ArchivedAt == nil {
			slog.Debug("audit: skipping unarchived partition", "partition", p.Name)
			continue
		}

		// Drop partition via audit_admin role.
		if c.dropper != nil {
			if err := c.dropper.DropPartition(ctx, p.Name); err != nil {
				slog.Error("audit: failed to drop partition", "partition", p.Name, "error", err)
				continue
			}
		}

		// Emit partition_drop event.
		_ = c.emitter.EmitEvent(ctx, AuditEvent{
			EventType: EventAuditPartitionDrop,
			Decision:  DecisionNone,
			Source:    SourceTrigger,
			Payload: map[string]interface{}{
				"partition_name": p.Name,
				"row_count":      p.RowCount,
				"range_start":    p.RangeStart.Format(time.RFC3339),
				"range_end":      p.RangeEnd.Format(time.RFC3339),
				"archived":       p.ArchivedAt != nil,
			},
		})

		if c.metrics != nil {
			c.metrics.PartitionDropTotal.Inc()
		}

		slog.Info("audit: partition dropped",
			"partition", p.Name,
			"rows", p.RowCount,
		)
	}

	return nil
}

// CleanupOlderThan returns a human-readable description of the cutoff.
func (c *Cleanup) CleanupOlderThan() string {
	cutoff := time.Now().UTC().AddDate(0, 0, -c.cfg.RetentionHotDays)
	return fmt.Sprintf("partitions before %s", cutoff.Format("2006-01-02"))
}
