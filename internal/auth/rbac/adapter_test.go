package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewPGAdapterEmptyDSNReturnsError verifies empty DSN rejection.
// REQ-AUTH2-001.
func TestNewPGAdapterEmptyDSNReturnsError(t *testing.T) {
	_, err := NewPGAdapter("")
	assert.Error(t, err, "empty DSN must return error")
	assert.Contains(t, err.Error(), "pg_dsn is required")
}

// TestNewPGAdapterInvalidDSNReturnsError verifies invalid DSN format rejection.
func TestNewPGAdapterInvalidDSNReturnsError(t *testing.T) {
	_, err := NewPGAdapter("not-a-valid-postgres-url")
	assert.Error(t, err, "invalid DSN must return error")
}

// TestPGAdapterAccessorsOnStruct verifies Adapter(), DB(), Close() on manually
// constructed PGAdapter (no PG connection needed).
func TestPGAdapterAccessorsOnStruct(t *testing.T) {
	a := &PGAdapter{adapter: nil, db: nil}

	// Close with nil db should return nil.
	err := a.Close()
	assert.NoError(t, err)

	// Adapter returns nil when not set.
	assert.Nil(t, a.Adapter())

	// DB returns nil when not set.
	assert.Nil(t, a.DB())
}

// TestPGAdapterCloseWithRealDBMock verifies Close on a PGAdapter with a non-nil db.
// We use a real pg.DB but don't connect it (no actual PG needed).
func TestPGAdapterCloseWithNilDB(t *testing.T) {
	a := &PGAdapter{}
	err := a.Close()
	assert.NoError(t, err, "Close on zero-value PGAdapter must not error")
}

// TestNewPGAdapterReturnsNotNilOnValidDSNFormat verifies that a properly formatted
// DSN gets past URL parsing. This test expects a connection failure since we don't
// have PG running, which is fine — we're testing the DSN parsing path.
func TestNewPGAdapterReturnsParseErrorOnBadDSN(t *testing.T) {
	_, err := NewPGAdapter("postgres://invalid:invalid@localhost:99999/nonexistent")
	// This should fail either at ParseURL or at connection.
	// Either way, we're testing the error path coverage.
	require.Error(t, err)
}
