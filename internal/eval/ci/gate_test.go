package ci_test

import (
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/eval/ci"
)

func ptr(f float64) *float64 { return &f }

func passingScores() []ci.QueryScore {
	return []ci.QueryScore{
		{QueryID: "Q1", Score: ptr(0.90)},
		{QueryID: "Q2", Score: ptr(0.88)},
		{QueryID: "Q3", Score: ptr(0.95)},
	}
}

// TestGatePassesAt085MeanAnd050Floor verifies a clean pass.
// REQ-EVAL1-008.
func TestGatePassesAt085MeanAnd050Floor(t *testing.T) {
	t.Parallel()
	res := ci.Decide(passingScores(), nil, ci.Thresholds{Mean: 0.85, Floor: 0.50, OverrideCap: 5})
	if !res.Pass {
		t.Fatalf("expected pass, got %#v", res)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", res.ExitCode)
	}
}

// TestGateFailsBelowMean verifies exit 1 when mean < 0.85.
// REQ-EVAL1-008(a).
func TestGateFailsBelowMean(t *testing.T) {
	t.Parallel()
	scores := []ci.QueryScore{
		{QueryID: "Q1", Score: ptr(0.80)},
		{QueryID: "Q2", Score: ptr(0.82)},
	}
	res := ci.Decide(scores, nil, ci.Thresholds{Mean: 0.85, Floor: 0.50, OverrideCap: 5})
	if res.Pass {
		t.Fatal("expected fail below mean")
	}
	if res.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", res.ExitCode)
	}
}

// TestGateFailsBelowFloor verifies exit 1 when any query < 0.50 even if mean ok.
// REQ-EVAL1-008(b).
func TestGateFailsBelowFloor(t *testing.T) {
	t.Parallel()
	scores := []ci.QueryScore{
		{QueryID: "Q1", Score: ptr(1.0)},
		{QueryID: "Q2", Score: ptr(1.0)},
		{QueryID: "Q3", Score: ptr(0.40)}, // below floor; mean is still 0.80
	}
	res := ci.Decide(scores, nil, ci.Thresholds{Mean: 0.70, Floor: 0.50, OverrideCap: 5})
	if res.Pass {
		t.Fatal("expected fail on floor violation")
	}
	if res.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", res.ExitCode)
	}
	if !strings.Contains(strings.ToLower(res.Reason), "floor") {
		t.Errorf("reason should mention floor, got %q", res.Reason)
	}
}

// TestGateExitCode2OnJudgeError verifies null scores force exit 2.
// REQ-EVAL1-008(c).
func TestGateExitCode2OnJudgeError(t *testing.T) {
	t.Parallel()
	scores := []ci.QueryScore{
		{QueryID: "Q1", Score: ptr(0.90)},
		{QueryID: "Q2", Score: nil}, // judge unavailable → null
	}
	res := ci.Decide(scores, nil, ci.Thresholds{Mean: 0.85, Floor: 0.50, OverrideCap: 5})
	if res.Pass {
		t.Fatal("expected fail on judge error")
	}
	if res.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2", res.ExitCode)
	}
}

// TestGateFailsOnOverrideCap verifies exit 1 when active overrides exceed cap.
// REQ-EVAL1-008(d).
func TestGateFailsOnOverrideCap(t *testing.T) {
	t.Parallel()
	overrides := make([]string, 6)
	for i := range overrides {
		overrides[i] = "Qx"
	}
	res := ci.Decide(passingScores(), overrides, ci.Thresholds{Mean: 0.85, Floor: 0.50, OverrideCap: 5})
	if res.Pass {
		t.Fatal("expected fail on override cap exceeded")
	}
	if res.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", res.ExitCode)
	}
	if !strings.Contains(strings.ToLower(res.Reason), "override") {
		t.Errorf("reason should mention override, got %q", res.Reason)
	}
}

// TestGateExcludesNullFromMean verifies the mean is computed over non-null only.
// REQ-EVAL1-006 + REQ-EVAL1-008.
func TestGateExcludesNullFromMean(t *testing.T) {
	t.Parallel()
	scores := []ci.QueryScore{
		{QueryID: "Q1", Score: ptr(0.90)},
		{QueryID: "Q2", Score: ptr(0.90)},
		{QueryID: "Q3", Score: nil},
	}
	res := ci.Decide(scores, nil, ci.Thresholds{Mean: 0.85, Floor: 0.50, OverrideCap: 5})
	// Mean over non-null = 0.90 (passes mean+floor), but null forces exit 2.
	if res.Mean < 0.89 || res.Mean > 0.91 {
		t.Errorf("mean = %v, want ~0.90 (null excluded)", res.Mean)
	}
	if res.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2 (null present)", res.ExitCode)
	}
}

// TestGateSummaryLineFormat verifies the grep-friendly stdout summary.
// REQ-EVAL1-008.
func TestGateSummaryLineFormat(t *testing.T) {
	t.Parallel()
	res := ci.Decide(passingScores(), nil, ci.Thresholds{Mean: 0.85, Floor: 0.50, OverrideCap: 5})
	line := ci.SummaryLine(res)
	// EVAL-001 result=PASS|FAIL mean=<X.XXX> floor=<X.XX> overrides=<N> nulls=<N>
	for _, want := range []string{"EVAL-001", "result=PASS", "mean=", "floor=", "overrides=", "nulls="} {
		if !strings.Contains(line, want) {
			t.Errorf("summary line missing %q: %s", want, line)
		}
	}
}
