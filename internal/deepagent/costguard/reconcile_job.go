package costguard

// ReconcileJob compares Postgres SUM(usd_cost) vs Redis accumulated values
// and corrects drift exceeding the 0.1% threshold.
// REQ-DEEP4-008, NFR-DEEP4-005.
type ReconcileJob struct{}

// NewReconcileJob creates a new ReconcileJob.
func NewReconcileJob() *ReconcileJob {
	return &ReconcileJob{}
}

// CheckDrift compares two values and returns the drift percentage.
// Drift is calculated as |a - b| / max(a, b) * 100.
// Returns true if drift exceeds the 0.1% threshold.
func CheckDrift(redisUSD, postgresUSD float64) (driftPct float64, exceeded bool) {
	if redisUSD == 0 && postgresUSD == 0 {
		return 0, false
	}
	maxVal := redisUSD
	if postgresUSD > maxVal {
		maxVal = postgresUSD
	}
	if maxVal == 0 {
		return 0, false
	}
	driftPct = abs(redisUSD-postgresUSD) / maxVal * 100
	return driftPct, driftPct > 0.1
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
