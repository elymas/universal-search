package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/eval/ci"
)

// TestMetricOutcome verifies the gate result → metric label mapping.
func TestMetricOutcome(t *testing.T) {
	cases := []struct {
		name string
		res  ci.Result
		want string
	}{
		{"pass", ci.Result{Pass: true}, "pass"},
		{"null", ci.Result{NullCount: 2}, "null"},
		{"fail", ci.Result{Pass: false}, "fail"},
	}
	for _, c := range cases {
		if got := metricOutcome(c.res); got != c.want {
			t.Errorf("%s: metricOutcome = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestConcurrencyDefault verifies the default and env override.
func TestConcurrencyDefault(t *testing.T) {
	if got := concurrency(); got != 5 {
		t.Errorf("default concurrency = %d, want 5", got)
	}
	t.Setenv("EVAL_CONCURRENCY", "3")
	if got := concurrency(); got != 3 {
		t.Errorf("env concurrency = %d, want 3", got)
	}
	t.Setenv("EVAL_CONCURRENCY", "bogus")
	if got := concurrency(); got != 5 {
		t.Errorf("bad env concurrency = %d, want fallback 5", got)
	}
}

// TestJudgeModelDefault verifies the default judge model + override.
func TestJudgeModelDefault(t *testing.T) {
	if got := judgeModel(); got != "claude-haiku-4-5" {
		t.Errorf("default judge model = %q", got)
	}
	t.Setenv("EVAL_JUDGE_MODEL", "gpt-4o-mini")
	if got := judgeModel(); got != "gpt-4o-mini" {
		t.Errorf("env judge model = %q", got)
	}
}

// TestRunSidecarUnreachableExitsTwo verifies that when neither synthesis nor
// judge sidecars are reachable, every query becomes null and the gate exits 2
// (judge availability error) — never a false PASS.
// REQ-EVAL1-006, REQ-EVAL1-008(c).
func TestRunSidecarUnreachableExitsTwo(t *testing.T) {
	// Point both sidecars at an unroutable port and write the report to a temp
	// dir so the test does not touch the repo's .moai/eval/reports.
	t.Setenv("RESEARCHER_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("EVAL_JUDGE_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("EVAL_REPORTS_DIR", t.TempDir())
	t.Setenv("EVAL_CONCURRENCY", "5")
	// Shorten the synthesis timeout so the test does not hang.
	t.Setenv("RESEARCHER_REQUEST_TIMEOUT_SECONDS", "1")

	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (sidecars down → null → judge error); stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "result=FAIL") {
		t.Errorf("summary should be FAIL: %s", stdout.String())
	}
}
