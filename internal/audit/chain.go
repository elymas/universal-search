package audit

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ChainManager handles the optional hash chain for tamper detection.
// D6: default OFF. REQ-AUTH3-008: advisory lock per-tenant when enabled.
type ChainManager struct {
	enabled bool
}

// NewChainManager creates a new hash chain manager.
func NewChainManager(enabled bool) *ChainManager {
	return &ChainManager{enabled: enabled}
}

// Enabled reports whether the hash chain is active.
func (cm *ChainManager) Enabled() bool {
	return cm != nil && cm.enabled
}

// ComputeThisHash computes the hash for an audit event row.
// REQ-AUTH3-008: this_hash = SHA256(prev_hash || canonical_json(row_minus_hashes)).
func ComputeThisHash(prevHash string, evt AuditEvent) string {
	canonical := CanonicalJSON(evt)
	input := prevHash + canonical
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash)
}

// CanonicalJSON produces a deterministic JSON encoding of an AuditEvent.
// Keys are sorted to ensure map iteration order does not affect output.
func CanonicalJSON(evt AuditEvent) string {
	// Build a sorted-key map.
	m := make(map[string]interface{})
	m["event_type"] = string(evt.EventType)
	m["decision"] = string(evt.Decision)
	m["user_id"] = evt.UserID
	m["tenant_id"] = evt.TenantID
	if evt.TeamID != "" {
		m["team_id"] = evt.TeamID
	}
	if evt.RequestID != "" {
		m["request_id"] = evt.RequestID
	}
	m["source"] = string(evt.Source)
	if evt.Resource != "" {
		m["resource"] = evt.Resource
	}
	if evt.Action != "" {
		m["action"] = evt.Action
	}
	if evt.IP != "" {
		m["ip"] = evt.IP
	}
	if evt.Payload != nil {
		m["payload"] = sortedMap(evt.Payload)
	}

	return sortedJSON(m)
}

// sortedJSON encodes a map as JSON with sorted keys.
func sortedJSON(m map[string]interface{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := m[k]
		keyJSON, _ := json.Marshal(k)
		valJSON, _ := json.Marshal(v)
		parts = append(parts, string(keyJSON)+":"+string(valJSON))
	}

	return "{" + strings.Join(parts, ",") + "}"
}

// sortedMap recursively sorts nested maps for canonical encoding.
func sortedMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		if nested, ok := v.(map[string]interface{}); ok {
			result[k] = sortedMap(nested)
		} else {
			result[k] = v
		}
	}
	return result
}

// VerifyChain verifies a sequence of audit events forms a valid hash chain.
// Returns the number of violations found.
func VerifyChain(events []AuditEvent) int {
	if len(events) == 0 {
		return 0
	}

	violations := 0
	prevHash := ""

	for i, evt := range events {
		expected := ComputeThisHash(prevHash, evt)
		if evt.ThisHash != expected {
			violations++
		}
		prevHash = evt.ThisHash
		_ = i
	}

	return violations
}

// AcquireAdvisoryLock returns a lock key for a given tenant.
// REQ-AUTH3-008: pg_advisory_xact_lock(hashtext(tenant_id)).
func AcquireAdvisoryLock(tenantID string) int32 {
	hash := sha256.Sum256([]byte(tenantID))
	// Use first 4 bytes as int32.
	return int32(hash[0]) | int32(hash[1])<<8 | int32(hash[2])<<16 | int32(hash[3])<<24
}
