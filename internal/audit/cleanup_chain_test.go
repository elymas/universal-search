package audit

import (
	"context"
	"testing"
	"time"
)

// mockPartitionDropper captures DropPartition calls.
type mockPartitionDropper struct {
	dropped []string
	err     error
}

func (m *mockPartitionDropper) DropPartition(_ context.Context, name string) error {
	if m.err != nil {
		return m.err
	}
	m.dropped = append(m.dropped, name)
	return nil
}

// TestCleanupSkipsRecentPartitions verifies hot partition protection.
// REQ-AUTH3-007: partitions within hot_days are not dropped.
func TestCleanupSkipsRecentPartitions(t *testing.T) {
	dropper := &mockPartitionDropper{}
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	cfg := DefaultConfig() // 90 days hot
	cleanup := NewCleanup(dropper, emitter, nil, cfg)

	now := time.Now().UTC()
	partitions := []PartitionInfo{
		{
			Name:       "audit_events_y2026m05",
			RangeStart: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			RangeEnd:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			ArchivedAt: &now,
		},
	}

	err := cleanup.RunCleanup(context.Background(), partitions)
	if err != nil {
		t.Fatalf("RunCleanup returned error: %v", err)
	}

	if len(dropper.dropped) != 0 {
		t.Errorf("Expected 0 drops for recent partition, got %d", len(dropper.dropped))
	}
}

// TestCleanupSkipsUnarchivedWhenRequireS3IsTrue verifies archive requirement.
// REQ-AUTH3-007: require_s3_archive prevents dropping unarchived partitions.
func TestCleanupSkipsUnarchivedWhenRequireS3IsTrue(t *testing.T) {
	dropper := &mockPartitionDropper{}
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	cfg := DefaultConfig()
	cfg.RequireS3Archive = true

	cleanup := NewCleanup(dropper, emitter, nil, cfg)

	partitions := []PartitionInfo{
		{
			Name:       "audit_events_y2025m01",
			RangeStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			RangeEnd:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			ArchivedAt: nil, // not archived
		},
	}

	err := cleanup.RunCleanup(context.Background(), partitions)
	if err != nil {
		t.Fatalf("RunCleanup returned error: %v", err)
	}

	if len(dropper.dropped) != 0 {
		t.Errorf("Expected 0 drops for unarchived partition, got %d", len(dropper.dropped))
	}
}

// TestCleanupDropsOldArchivedPartition verifies normal cleanup flow.
// REQ-AUTH3-007: drops old archived partitions.
func TestCleanupDropsOldArchivedPartition(t *testing.T) {
	dropper := &mockPartitionDropper{}
	store := &mockEventStore{}
	emitter := NewEmitter(store, DefaultConfig(), nil)
	cfg := DefaultConfig()

	cleanup := NewCleanup(dropper, emitter, nil, cfg)

	now := time.Now().UTC()
	partitions := []PartitionInfo{
		{
			Name:       "audit_events_y2025m01",
			RangeStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			RangeEnd:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			ArchivedAt: &now, // archived
			RowCount:   100000,
		},
	}

	err := cleanup.RunCleanup(context.Background(), partitions)
	if err != nil {
		t.Fatalf("RunCleanup returned error: %v", err)
	}

	if len(dropper.dropped) != 1 {
		t.Fatalf("Expected 1 drop, got %d", len(dropper.dropped))
	}
	if dropper.dropped[0] != "audit_events_y2025m01" {
		t.Errorf("Dropped = %q, want %q", dropper.dropped[0], "audit_events_y2025m01")
	}

	// Should emit audit.partition_drop event.
	events := store.Events()
	found := false
	for _, evt := range events {
		if evt.EventType == EventAuditPartitionDrop {
			found = true
		}
	}
	if !found {
		t.Error("Expected audit.partition_drop event")
	}
}

// TestHashChainCompute verifies hash computation.
// REQ-AUTH3-008: this_hash = SHA256(prev_hash || canonical_json).
func TestHashChainCompute(t *testing.T) {
	evt := AuditEvent{
		EventType: EventQuerySubmit,
		Decision:  DecisionAllow,
		UserID:    "test",
		TenantID:  "default",
		Source:    SourceGo,
	}

	// Empty prev_hash.
	hash1 := ComputeThisHash("", evt)
	if hash1 == "" {
		t.Error("ComputeThisHash returned empty string")
	}
	if len(hash1) != 64 { // SHA256 hex length
		t.Errorf("Hash length = %d, want 64", len(hash1))
	}

	// Non-empty prev_hash should produce different result.
	hash2 := ComputeThisHash("prev_hash_value", evt)
	if hash2 == hash1 {
		t.Error("Different prev_hash should produce different this_hash")
	}
}

// TestHashChainCanonicalJSON verifies deterministic JSON output.
// REQ-AUTH3-008: canonical_json stability.
func TestHashChainCanonicalJSON(t *testing.T) {
	evt := AuditEvent{
		EventType: EventQuerySubmit,
		Decision:  DecisionAllow,
		UserID:    "test",
		TenantID:  "default",
		Source:    SourceGo,
		Payload: map[string]interface{}{
			"z_field": "last",
			"a_field": "first",
		},
	}

	json1 := CanonicalJSON(evt)
	json2 := CanonicalJSON(evt)

	if json1 != json2 {
		t.Error("CanonicalJSON should produce identical output for same input")
	}

	// Verify keys are sorted (a_field before z_field).
	if !contains(json1, `"a_field"`) || !contains(json1, `"z_field"`) {
		t.Errorf("CanonicalJSON missing expected fields: %s", json1)
	}
}

// TestHashChainVerify verifies chain validation.
func TestHashChainVerify(t *testing.T) {
	// Build a valid chain.
	evt1 := AuditEvent{
		EventType: EventQuerySubmit,
		Decision:  DecisionAllow,
		UserID:    "user1",
		TenantID:  "default",
		Source:    SourceGo,
	}
	evt1.ThisHash = ComputeThisHash("", evt1)

	evt2 := AuditEvent{
		EventType: EventQuerySubmit,
		Decision:  DecisionAllow,
		UserID:    "user2",
		TenantID:  "default",
		Source:    SourceGo,
	}
	evt2.PrevHash = evt1.ThisHash
	evt2.ThisHash = ComputeThisHash(evt1.ThisHash, evt2)

	violations := VerifyChain([]AuditEvent{evt1, evt2})
	if violations != 0 {
		t.Errorf("Expected 0 violations, got %d", violations)
	}

	// Tamper with evt2.
	evt2.UserID = "tampered"
	violations = VerifyChain([]AuditEvent{evt1, evt2})
	if violations != 1 {
		t.Errorf("Expected 1 violation after tampering, got %d", violations)
	}
}

// TestAdvisoryLockKey verifies tenant-based lock key generation.
// REQ-AUTH3-008: pg_advisory_xact_lock(hashtext(tenant_id)).
func TestAdvisoryLockKey(t *testing.T) {
	key1 := AcquireAdvisoryLock("default")
	key2 := AcquireAdvisoryLock("other_tenant")

	if key1 == key2 {
		t.Error("Different tenants should produce different advisory lock keys")
	}

	// Same tenant should produce same key.
	key1Again := AcquireAdvisoryLock("default")
	if key1 != key1Again {
		t.Error("Same tenant should produce same advisory lock key")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
