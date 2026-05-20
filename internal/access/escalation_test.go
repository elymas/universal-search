// Package access — unit tests for shouldEscalate() cascade logic.
//
// REQ-CACHE-002: Phase 1 escalates on miss/skip.
// REQ-CACHE-003: Phase 2 escalates on success (triggers Phase 3 for body).
// REQ-CACHE-004: Phase 3 escalates on TLS error or WAF block.
// REQ-CACHE-005: Phase 4 escalates on JS challenge.
// REQ-CACHE-006: Phase 5 never escalates.
package access

import "testing"

func TestShouldEscalate_Phase1_Miss(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 1, Outcome: "miss"}
	if !shouldEscalate(a) {
		t.Error("phase 1 miss must escalate")
	}
}

func TestShouldEscalate_Phase1_Skipped(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 1, Outcome: "skipped"}
	if !shouldEscalate(a) {
		t.Error("phase 1 skipped must escalate")
	}
}

func TestShouldEscalate_Phase1_Success_NoEscalate(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 1, Outcome: "success"}
	if shouldEscalate(a) {
		t.Error("phase 1 success must NOT escalate")
	}
}

func TestShouldEscalate_Phase2_Success(t *testing.T) {
	t.Parallel()
	// Phase 2 HEAD-only probe — success means robots.txt allowed; need Phase 3 to GET body.
	a := &PhaseAttempt{Phase: 2, Outcome: "success"}
	if !shouldEscalate(a) {
		t.Error("phase 2 success must escalate to phase 3 for body")
	}
}

func TestShouldEscalate_Phase2_Skipped(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 2, Outcome: "skipped"}
	if !shouldEscalate(a) {
		t.Error("phase 2 skipped must escalate")
	}
}

func TestShouldEscalate_Phase2_Blocked_NoEscalate(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 2, Outcome: "blocked"}
	if shouldEscalate(a) {
		t.Error("phase 2 blocked (robots.txt) must NOT escalate")
	}
}

func TestShouldEscalate_Phase3_TLSError(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 3, Outcome: "failure", isTLSError: true}
	if !shouldEscalate(a) {
		t.Error("phase 3 TLS error must escalate to phase 4")
	}
}

func TestShouldEscalate_Phase3_WAF(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 3, Outcome: "blocked", isWAF: true}
	if !shouldEscalate(a) {
		t.Error("phase 3 WAF block must escalate to phase 4")
	}
}

func TestShouldEscalate_Phase3_PermanentFail_NoEscalate(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 3, Outcome: "failure"}
	if shouldEscalate(a) {
		t.Error("phase 3 generic failure must NOT escalate")
	}
}

func TestShouldEscalate_Phase3_Success_NoEscalate(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 3, Outcome: "success"}
	if shouldEscalate(a) {
		t.Error("phase 3 success must NOT escalate")
	}
}

func TestShouldEscalate_Phase4_JSChallenge(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 4, Outcome: "failure", isJSChallenge: true}
	if !shouldEscalate(a) {
		t.Error("phase 4 JS challenge must escalate to phase 5")
	}
}

func TestShouldEscalate_Phase4_NoJSChallenge_NoEscalate(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 4, Outcome: "failure", isJSChallenge: false}
	if shouldEscalate(a) {
		t.Error("phase 4 failure without JS challenge must NOT escalate")
	}
}

func TestShouldEscalate_Phase5_NeverEscalates(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"success", "failure", "timeout", "blocked"} {
		a := &PhaseAttempt{Phase: 5, Outcome: outcome, isJSChallenge: true}
		if shouldEscalate(a) {
			t.Errorf("phase 5 %s must NEVER escalate", outcome)
		}
	}
}

func TestShouldEscalate_UnknownPhase_NoEscalate(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{Phase: 99, Outcome: "failure"}
	if shouldEscalate(a) {
		t.Error("unknown phase must NOT escalate")
	}
}
