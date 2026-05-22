package rbac

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditEmitterWritesJSONLine verifies that Emit writes a valid JSON line.
// NFR-AUTH2-004.
func TestAuditEmitterWritesJSONLine(t *testing.T) {
	var buf bytes.Buffer
	em := NewAuditEmitter(&buf, true)

	d := Decision{
		Allowed:     false,
		UserID:      "alice",
		TeamID:      "engineering",
		Resource:    "query:basic",
		Action:      "read",
		ReasonClass: "no_policy_matched",
	}

	em.Emit(d)

	// Should produce exactly one JSON line.
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte{'\n'})
	require.Len(t, lines, 1, "expected exactly one JSON line")

	var entry AuditEntry
	err := json.Unmarshal(lines[0], &entry)
	require.NoError(t, err, "output must be valid JSON")

	assert.Equal(t, "alice", entry.UserID)
	assert.Equal(t, "engineering", entry.TeamID)
	assert.Equal(t, "query:basic", entry.Resource)
	assert.Equal(t, "read", entry.Action)
	assert.False(t, entry.Allowed)
	assert.Equal(t, "no_policy_matched", entry.ReasonClass)
	assert.NotEmpty(t, entry.Timestamp, "timestamp must be set")
}

// TestAuditEmitterDisabledWritesNothing verifies no output when disabled.
func TestAuditEmitterDisabledWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	em := NewAuditEmitter(&buf, false)

	em.Emit(Decision{Allowed: true, UserID: "alice", ReasonClass: "policy_matched"})

	assert.Equal(t, 0, buf.Len(), "disabled emitter must not write")
}

// TestAuditEmitterNilWriterDefaultsToStderr verifies nil writer defaults to stderr.
func TestAuditEmitterNilWriterDefaultsToStderr(t *testing.T) {
	em := NewAuditEmitter(nil, true)
	assert.NotNil(t, em)
	// Can't easily assert stderr content, so just verify no panic.
	assert.NotPanics(t, func() {
		em.Emit(Decision{Allowed: true, UserID: "test", ReasonClass: "policy_matched"})
	})
}

// TestAuditEmitterNilReceiverIsNoop verifies Emit is safe on nil receiver.
func TestAuditEmitterNilReceiverIsNoop(t *testing.T) {
	var em *AuditEmitter
	assert.NotPanics(t, func() {
		em.Emit(Decision{Allowed: true, ReasonClass: "policy_matched"})
	})
}

// TestAuditEmitterEnabledReportsCorrectly verifies Enabled() method.
func TestAuditEmitterEnabledReportsCorrectly(t *testing.T) {
	enabled := NewAuditEmitter(nil, true)
	disabled := NewAuditEmitter(nil, false)
	var nilEmitter *AuditEmitter

	assert.True(t, enabled.Enabled())
	assert.False(t, disabled.Enabled())
	assert.False(t, nilEmitter.Enabled())
}

// TestAuditEmitterAllowDecision verifies allow decision format.
func TestAuditEmitterAllowDecision(t *testing.T) {
	var buf bytes.Buffer
	em := NewAuditEmitter(&buf, true)

	d := Decision{
		Allowed:     true,
		UserID:      "eve",
		TeamID:      "research",
		Resource:    "rbac_policy",
		Action:      "write",
		ReasonClass: "policy_matched",
	}

	em.Emit(d)

	var entry AuditEntry
	err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry)
	require.NoError(t, err)
	assert.True(t, entry.Allowed)
	assert.Equal(t, "eve", entry.UserID)
}

// TestAuditEmitterMultipleEntries verifies multiple emits produce multiple lines.
func TestAuditEmitterMultipleEntries(t *testing.T) {
	var buf bytes.Buffer
	em := NewAuditEmitter(&buf, true)

	for i := range 3 {
		em.Emit(Decision{
			Allowed:     i%2 == 0,
			UserID:      "user",
			TeamID:      "team",
			Resource:    "res",
			Action:      "act",
			ReasonClass: "policy_matched",
		})
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte{'\n'})
	assert.Len(t, lines, 3, "expected 3 JSON lines")
}
