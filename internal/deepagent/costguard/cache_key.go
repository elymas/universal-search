package costguard

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// PrefixCacheKey generates a deterministic cache key that includes tenant
// and intent category to prevent cross-tenant cache collisions.
// REQ-DEEP4-012: cache_key salt = SHA256(tenant_id || intent_category || model || messages_json).
// @MX:NOTE: [AUTO] Tenant prefix prevents cross-tenant data leak (REQ-DEEP4-012)
//
// The messages payload is NOT mutated — the salt is applied as a cache-key
// prefix/wrapper layer, not injected into the LLM request.
func PrefixCacheKey(tenantID, intentCategory, model, messagesJSON string) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s|%s|%s|%s", tenantID, intentCategory, model, messagesJSON)
	return hex.EncodeToString(h.Sum(nil))
}

// CacheKeyPrefix returns just the tenant+intent prefix portion.
// Useful for debugging and cache analysis.
func CacheKeyPrefix(tenantID, intentCategory string) string {
	return fmt.Sprintf("cg:%s:%s", tenantID, intentCategory)
}
