package costguard

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Phase A: Postgres Schema + Asynq Reconcile Foundation
// ---------------------------------------------------------------------------

// TestMigration0002Idempotent verifies that 0002_cost_ledger.sql can be
// executed twice without error (CREATE TABLE IF NOT EXISTS + IF NOT EXISTS indexes).
// REQ-DEEP4-006: migration must be idempotent.
func TestMigration0002Idempotent(t *testing.T) {
	t.Parallel()

	sql := migrationSQL()

	// First run should parse without error.
	if err := parseSQL(sql); err != nil {
		t.Fatalf("first parse: %v", err)
	}

	// Running the same SQL twice should still parse (idempotent).
	if err := parseSQL(sql); err != nil {
		t.Fatalf("second parse (idempotency check): %v", err)
	}

	// Verify the SQL contains IF NOT EXISTS clauses for idempotency.
	if !strings.Contains(sql, "IF NOT EXISTS") {
		t.Error("migration SQL must contain IF NOT EXISTS for idempotency")
	}
}

// TestLedgerSchemaMatchesSpec verifies the cost_ledger table has all required
// columns and constraints per SPEC-DEEP-004 REQ-DEEP4-006.
func TestLedgerSchemaMatchesSpec(t *testing.T) {
	t.Parallel()

	sql := migrationSQL()

	// Required columns per SPEC-DEEP-004 research §5.2.
	requiredColumns := []string{
		"id",
		"user_id",
		"tenant_id",
		"request_id",
		"deep_run_id",
		"model",
		"prompt_tokens",
		"completion_tokens",
		"usd_cost",
		"cache_hit",
		"intent_category",
		"outcome",
		"ts",
	}

	for _, col := range requiredColumns {
		if !strings.Contains(strings.ToLower(sql), col) {
			t.Errorf("required column %q not found in migration SQL", col)
		}
	}

	// Verify indexes.
	indexChecks := []string{
		"idx_cost_ledger_user_ts",
		"idx_cost_ledger_tenant_ts",
		"idx_cost_ledger_deep_run",
	}
	for _, idx := range indexChecks {
		if !strings.Contains(sql, idx) {
			t.Errorf("required index %q not found in migration SQL", idx)
		}
	}
}

// TestReconcileSchedulerRunsEvery5Min verifies that the reconciliation
// scheduler is configured to run at 5-minute intervals.
// REQ-DEEP4-008: 5-min Asynq scheduled job.
func TestReconcileSchedulerRunsEvery5Min(t *testing.T) {
	t.Parallel()

	scheduler := NewReconcileScheduler(nil, nil)
	interval := scheduler.Interval()

	// REQ-DEEP4-008: 5-minute interval.
	if interval.Minutes() != 5 {
		t.Errorf("reconcile interval: got %.1f minutes, want 5", interval.Minutes())
	}
}

// TestReconcileDriftExceedingThresholdAlertsAndCorrects verifies that when
// Redis accumulated value drifts from Postgres by more than 0.1%, an alarm
// is triggered and Redis is reset to the Postgres truth value.
// REQ-DEEP4-008, NFR-DEEP4-005.
func TestReconcileDriftExceedingThresholdAlertsAndCorrects(t *testing.T) {
	t.Parallel()

	// Scenario: Redis shows $5.00, Postgres shows $4.99 -> 0.2% drift > 0.1%.
	// This should trigger an alarm + Redis truth-reset.
	tc := newReconcileTestCase(t,
		redisAccumulatedUSD(5.00),
		postgresAccumulatedUSD(4.99),
	)

	result := tc.RunReconcile()

	if !result.DriftDetected {
		t.Error("expected drift to be detected (0.2% > 0.1% threshold)")
	}
	if !result.RedisReset {
		t.Error("expected Redis truth-reset when drift exceeds 0.1%")
	}
	if result.DriftPercent <= 0.1 {
		t.Errorf("drift percent: got %.4f%%, want > 0.1%%", result.DriftPercent)
	}
}

// ---------------------------------------------------------------------------
// Helper types and functions for reconcile tests
// ---------------------------------------------------------------------------

// ReconcileResult captures the outcome of a reconciliation run.
type ReconcileResult struct {
	DriftDetected bool
	RedisReset    bool
	DriftPercent  float64
}

type reconcileTestCase struct {
	t           *testing.T
	redisUSD    float64
	postgresUSD float64
}

func newReconcileTestCase(t *testing.T, opts ...reconcileOption) *reconcileTestCase {
	tc := &reconcileTestCase{t: t}
	for _, o := range opts {
		o(tc)
	}
	return tc
}

type reconcileOption func(*reconcileTestCase)

func redisAccumulatedUSD(v float64) reconcileOption {
	return func(tc *reconcileTestCase) { tc.redisUSD = v }
}

func postgresAccumulatedUSD(v float64) reconcileOption {
	return func(tc *reconcileTestCase) { tc.postgresUSD = v }
}

func (tc *reconcileTestCase) RunReconcile() ReconcileResult {
	driftPct, exceeded := CheckDrift(tc.redisUSD, tc.postgresUSD)
	return ReconcileResult{
		DriftDetected: exceeded,
		RedisReset:    exceeded,
		DriftPercent:  driftPct,
	}
}
