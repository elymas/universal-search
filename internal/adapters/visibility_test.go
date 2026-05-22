package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAdapterVisibilityEnum verifies all three visibility levels are defined.
// REQ-AUTH2-008.
func TestAdapterVisibilityEnum(t *testing.T) {
	assert.Equal(t, AdapterVisibility(0), VisibilityTeamShared)
	assert.Equal(t, AdapterVisibility(1), VisibilityPersonal)
	assert.Equal(t, AdapterVisibility(2), VisibilityAdminOnly)
}

// TestPersonalAdapterPolicyShape verifies MakePersonalPolicyRow generates
// the correct policy shape.
// REQ-AUTH2-008.
func TestPersonalAdapterPolicyShape(t *testing.T) {
	row := MakePersonalPolicyRow("alice", "engineering", "gmail")

	assert.Equal(t, []string{"alice", "engineering", "adapter:gmail:alice", "read", "allow"}, row)
}

// TestMakePersonalPolicyRowDifferentInputs verifies the helper works
// with different inputs.
func TestMakePersonalPolicyRowDifferentInputs(t *testing.T) {
	row := MakePersonalPolicyRow("bob", "research", "drive")
	assert.Equal(t, "bob", row[0])
	assert.Equal(t, "research", row[1])
	assert.Equal(t, "adapter:drive:bob", row[2])
	assert.Equal(t, "read", row[3])
	assert.Equal(t, "allow", row[4])
}
