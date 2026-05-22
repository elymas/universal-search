package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRouteMappingTableConsistent verifies DefaultRoutes has the expected 11 entries.
// REQ-AUTH2-005.
func TestRouteMappingTableConsistent(t *testing.T) {
	assert.Len(t, DefaultRoutes, 11, "DefaultRoutes must have exactly 11 entries")

	// Verify all entries have required fields.
	for _, r := range DefaultRoutes {
		assert.NotEmpty(t, r.Method, "Method must not be empty")
		assert.NotEmpty(t, r.Path, "Path must not be empty")
		assert.NotEmpty(t, r.Resource, "Resource must not be empty")
		assert.NotEmpty(t, r.Action, "Action must not be empty")
	}

	// Verify key routes exist.
	resources := make(map[string]bool)
	for _, r := range DefaultRoutes {
		resources[r.Resource] = true
	}

	assert.True(t, resources["query:basic"], "must map query:basic")
	assert.True(t, resources["query:deep"], "must map query:deep")
	assert.True(t, resources["audit_log"], "must map audit_log")
	assert.True(t, resources["rbac_policy"], "must map rbac_policy")
	assert.True(t, resources["member"], "must map member")
	assert.True(t, resources["api_key"], "must map api_key")
	assert.True(t, resources["adapter_config"], "must map adapter_config")
}

// TestRouteMappingQueryBasic verifies POST /query maps to query:basic.
// REQ-AUTH2-006.
func TestRouteMappingQueryBasic(t *testing.T) {
	found := false
	for _, r := range DefaultRoutes {
		if r.Path == "/query" && r.Method == "POST" {
			assert.Equal(t, "query:basic", r.Resource)
			assert.Equal(t, "read", r.Action)
			found = true
		}
	}
	assert.True(t, found, "POST /query must be in route table")
}
