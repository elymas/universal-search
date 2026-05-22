package audit

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"
)

// S3Client is the interface for S3 operations.
// Interface-based for MinIO + AWS S3 compatibility and mock testing.
type S3Client interface {
	PutObject(ctx context.Context, bucket, key string, data io.Reader) error
}

// Exporter handles weekly S3 export of audit partitions.
// REQ-AUTH3-005: weekly JSONL.gz, MinIO + AWS S3 compatible.
type Exporter struct {
	s3      S3Client
	emitter *Emitter
	metrics *Metrics
	cfg     Config
}

// NewExporter creates a new S3 exporter.
func NewExporter(s3 S3Client, emitter *Emitter, metrics *Metrics, cfg Config) *Exporter {
	return &Exporter{
		s3:      s3,
		emitter: emitter,
		metrics: metrics,
		cfg:     cfg,
	}
}

// ExportPartition exports a single partition to S3 as JSONL.gz.
// REQ-AUTH3-005: streams partitions older than 7d to S3.
func (e *Exporter) ExportPartition(ctx context.Context, partition PartitionInfo, rows []AuditEvent) error {
	if !e.cfg.S3Enabled {
		slog.Info("audit: S3 export disabled, skipping")
		return nil
	}

	if e.s3 == nil {
		return fmt.Errorf("audit: no S3 client configured")
	}

	start := time.Now()

	// Serialize rows as JSONL.
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)

	for _, row := range rows {
		data, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("audit: export marshal: %w", err)
		}
		data = append(data, '\n')
		if _, err := gzWriter.Write(data); err != nil {
			return fmt.Errorf("audit: export gzip write: %w", err)
		}
	}

	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("audit: export gzip close: %w", err)
	}

	// Build S3 key.
	key := fmt.Sprintf("audit/default/year=%d/month=%02d/day=01/events-%s.jsonl.gz",
		partition.RangeStart.Year(),
		partition.RangeStart.Month(),
		partition.Name,
	)

	// Upload to S3.
	if err := e.s3.PutObject(ctx, e.cfg.S3Bucket, key, &buf); err != nil {
		return fmt.Errorf("audit: S3 PUT: %w", err)
	}

	// Emit audit.export event.
	_ = e.emitter.EmitEvent(ctx, AuditEvent{
		EventType: EventAuditExport,
		Decision:  DecisionNone,
		Source:    SourceTrigger,
		Payload: map[string]interface{}{
			"s3_uri":         fmt.Sprintf("s3://%s/%s", e.cfg.S3Bucket, key),
			"partition_name": partition.Name,
			"row_count":      len(rows),
			"bytes_compressed": buf.Len(),
		},
	})

	// Record metrics.
	if e.metrics != nil {
		e.metrics.S3ExportRowsTotal.Add(float64(len(rows)))
		e.metrics.S3ExportBytesTotal.Add(float64(buf.Len()))
		e.metrics.S3ExportDurationSeconds.Observe(time.Since(start).Seconds())
	}

	slog.Info("audit: partition exported",
		"partition", partition.Name,
		"rows", len(rows),
		"bytes", buf.Len(),
		"duration", time.Since(start),
	)

	return nil
}
