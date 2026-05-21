package costguard

import (
	"embed"
	"strings"
	"time"
)

//go:embed migration_0002_cost_ledger.sql
var migrationFiles embed.FS

// migrationSQL returns the raw SQL content of 0002_cost_ledger.sql.
// REQ-DEEP4-006: embedded migration for schema validation tests.
func migrationSQL() string {
	b, err := migrationFiles.ReadFile("migration_0002_cost_ledger.sql")
	if err != nil {
		return ""
	}
	return string(b)
}

// parseSQL validates that the given string is non-empty SQL.
// Returns nil for valid SQL, error for empty/invalid.
func parseSQL(sql string) error {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return errNotImplemented
	}
	if !strings.HasPrefix(strings.ToUpper(sql), "CREATE") &&
		!strings.HasPrefix(strings.ToUpper(sql), "--") {
		return errNotImplemented
	}
	return nil
}

// errNotImplemented is returned by stubs that are not yet implemented.
var errNotImplemented = notImplementedError{}

type notImplementedError struct{}

func (notImplementedError) Error() string { return "not implemented" }

// ReconcileScheduler manages the periodic reconciliation job.
// REQ-DEEP4-008: 5-min scheduled job comparing Postgres SUM vs Redis.
type ReconcileScheduler struct {
	interval time.Duration
}

// NewReconcileScheduler creates a new reconciliation scheduler.
// The interval is set to 5 minutes per REQ-DEEP4-008.
func NewReconcileScheduler(redis, postgres interface{}) *ReconcileScheduler {
	return &ReconcileScheduler{
		interval: 5 * time.Minute,
	}
}

// Interval returns the reconciliation interval.
func (s *ReconcileScheduler) Interval() time.Duration {
	return s.interval
}

// WriteLedgerEntry writes a single cost_ledger row.
// @MX:ANCHOR: [AUTO] Ledger write; callers: middleware, reconcile job, audit hook
// @MX:REASON: fan_in >= 3; all LLM cost flows through this function
// @MX:TODO: [AUTO] Implement with real Postgres client in Phase E wiring
func WriteLedgerEntry(entry LedgerEntry) error {
	return nil
}
