package audit

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// mockS3Client captures PutObject calls for verification.
type mockS3Client struct {
	putCalls []mockS3Put
	err      error
}

type mockS3Put struct {
	bucket string
	key    string
	data   []byte
}

func (m *mockS3Client) PutObject(_ context.Context, bucket, key string, data io.Reader) error {
	if m.err != nil {
		return m.err
	}
	b, _ := io.ReadAll(data)
	m.putCalls = append(m.putCalls, mockS3Put{bucket: bucket, key: key, data: b})
	return nil
}

// TestS3ExportDisabledByDefault verifies no export when S3 disabled.
// REQ-AUTH3-005: S3 disabled by default.
func TestS3ExportDisabledByDefault(t *testing.T) {
	s3 := &mockS3Client{}
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil) // S3Enabled = false
	exporter := NewExporter(s3, emitter, nil, DefaultConfig())

	partition := PartitionInfo{
		Name:       "audit_events_y2026m02",
		RangeStart: mustParseTime(t, "2026-02-01"),
		RangeEnd:   mustParseTime(t, "2026-03-01"),
	}

	err := exporter.ExportPartition(context.Background(), partition, []AuditEvent{
		{EventType: EventQuerySubmit, UserID: "test"},
	})
	if err != nil {
		t.Fatalf("ExportPartition returned error: %v", err)
	}

	if len(s3.putCalls) != 0 {
		t.Errorf("Expected 0 S3 PUT calls, got %d", len(s3.putCalls))
	}
}

// TestS3ExportUploadsJSONLGz verifies JSONL.gz upload format.
// REQ-AUTH3-005: MinIO + AWS S3 compatible.
func TestS3ExportUploadsJSONLGz(t *testing.T) {
	s3 := &mockS3Client{}
	store := &mockEventStore{}
	cfg := DefaultConfig()
	cfg.S3Enabled = true
	cfg.S3Bucket = "test-audit"
	emitter := NewEmitter(store, cfg, nil)
	exporter := NewExporter(s3, emitter, nil, cfg)

	partition := PartitionInfo{
		Name:       "audit_events_y2026m02",
		RangeStart: mustParseTime(t, "2026-02-01"),
		RangeEnd:   mustParseTime(t, "2026-03-01"),
	}

	rows := []AuditEvent{
		{EventType: EventQuerySubmit, UserID: "alice"},
		{EventType: EventQuerySubmit, UserID: "bob"},
	}

	err := exporter.ExportPartition(context.Background(), partition, rows)
	if err != nil {
		t.Fatalf("ExportPartition returned error: %v", err)
	}

	if len(s3.putCalls) != 1 {
		t.Fatalf("Expected 1 S3 PUT call, got %d", len(s3.putCalls))
	}

	put := s3.putCalls[0]
	if put.bucket != "test-audit" {
		t.Errorf("Bucket = %q, want %q", put.bucket, "test-audit")
	}
	if !strings.HasSuffix(put.key, ".jsonl.gz") {
		t.Errorf("Key = %q, should end with .jsonl.gz", put.key)
	}

	// Decompress and verify JSONL.
	gzReader, err := gzip.NewReader(bytes.NewReader(put.data))
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(decompressed)), "\n")
	if len(lines) != 2 {
		t.Fatalf("Expected 2 JSONL lines, got %d", len(lines))
	}

	var first AuditEvent
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("Failed to parse first line: %v", err)
	}
	if first.UserID != "alice" {
		t.Errorf("First line UserID = %q, want %q", first.UserID, "alice")
	}
}

// TestS3ExportEmitsExportEvent verifies audit.export event emission.
// REQ-AUTH3-005, REQ-AUTH3-009.
func TestS3ExportEmitsExportEvent(t *testing.T) {
	s3 := &mockS3Client{}
	store := &mockEventStore{}
	cfg := DefaultConfig()
	cfg.S3Enabled = true
	cfg.S3Bucket = "test-audit"
	emitter := NewEmitter(store, cfg, nil)
	exporter := NewExporter(s3, emitter, nil, cfg)

	partition := PartitionInfo{
		Name:       "audit_events_y2026m02",
		RangeStart: mustParseTime(t, "2026-02-01"),
		RangeEnd:   mustParseTime(t, "2026-03-01"),
	}

	err := exporter.ExportPartition(context.Background(), partition, []AuditEvent{
		{EventType: EventQuerySubmit, UserID: "test"},
	})
	if err != nil {
		t.Fatalf("ExportPartition returned error: %v", err)
	}

	events := store.Events()
	// Should have at least 1 event (the audit.export event).
	found := false
	for _, evt := range events {
		if evt.EventType == EventAuditExport {
			found = true
			if evt.Source != SourceTrigger {
				t.Errorf("audit.export source = %q, want %q", evt.Source, SourceTrigger)
			}
		}
	}
	if !found {
		t.Error("No audit.export event emitted")
	}
}

// TestS3ExportRetriesOnFailure verifies error propagation (caller handles retry).
// REQ-AUTH3-005: retry 3x exponential backoff.
func TestS3ExportRetriesOnFailure(t *testing.T) {
	s3 := &mockS3Client{err: io.ErrUnexpectedEOF}
	store := &mockEventStore{}
	cfg := DefaultConfig()
	cfg.S3Enabled = true
	cfg.S3Bucket = "test-audit"
	emitter := NewEmitter(store, cfg, nil)
	exporter := NewExporter(s3, emitter, nil, cfg)

	partition := PartitionInfo{
		Name:       "audit_events_y2026m02",
		RangeStart: mustParseTime(t, "2026-02-01"),
		RangeEnd:   mustParseTime(t, "2026-03-01"),
	}

	err := exporter.ExportPartition(context.Background(), partition, []AuditEvent{})
	if err == nil {
		t.Error("Expected error when S3 PUT fails, got nil")
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}
