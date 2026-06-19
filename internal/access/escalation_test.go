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
	// SPEC-ACC-001: a confident WAF profile hit (>= wafEscalateThreshold)
	// drives the escalation that isWAF used to drive.
	a := &PhaseAttempt{
		Phase:       3,
		Outcome:     "blocked",
		profileHits: []ProfileHit{{ProfileID: "akamai", Confidence: 0.5}},
	}
	if !shouldEscalate(a) {
		t.Error("phase 3 WAF profile hit must escalate to phase 4")
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

// --- SPEC-ACC-001 REQ-ACC-013: profile-hit driven escalation ---

// TestPhaseAttemptTopProfile asserts topProfile returns the highest-
// confidence hit, false when empty.
func TestPhaseAttemptTopProfile(t *testing.T) {
	t.Parallel()
	// Empty → false.
	var empty *PhaseAttempt
	if _, ok := empty.topProfile(); ok {
		t.Error("nil attempt topProfile must return false")
	}
	a := &PhaseAttempt{}
	if _, ok := a.topProfile(); ok {
		t.Error("empty profileHits topProfile must return false")
	}
	// Multi-hit → highest confidence (slice is pre-sorted desc).
	a = &PhaseAttempt{profileHits: []ProfileHit{
		{ProfileID: "akamai", Confidence: 0.9},
		{ProfileID: "datadome", Confidence: 0.5},
	}}
	top, ok := a.topProfile()
	if !ok {
		t.Fatal("multi-hit topProfile must return true")
	}
	if top.ProfileID != "akamai" || top.Confidence != 0.9 {
		t.Errorf("topProfile = %+v, want {akamai 0.9}", top)
	}
}

// TestPhaseAttemptHasWAFProfile asserts the threshold gate.
// 0.5 ≥ 0.3 → true; 0.2 < 0.3 → false.
func TestPhaseAttemptHasWAFProfile(t *testing.T) {
	t.Parallel()
	high := &PhaseAttempt{profileHits: []ProfileHit{{ProfileID: "akamai", Confidence: 0.5}}}
	if !high.hasWAFProfile() {
		t.Error("0.5 confidence must meet wafEscalateThreshold")
	}
	low := &PhaseAttempt{profileHits: []ProfileHit{{ProfileID: "unknown", Confidence: 0.2}}}
	if low.hasWAFProfile() {
		t.Error("0.2 confidence must NOT meet wafEscalateThreshold")
	}
	empty := &PhaseAttempt{}
	if empty.hasWAFProfile() {
		t.Error("empty profileHits must not have a WAF profile")
	}
}

// TestShouldEscalatePhase3OnWAFProfile asserts a 0.5-confidence hit
// escalates Phase 3 → 4.
func TestShouldEscalatePhase3OnWAFProfile(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{
		Phase:       3,
		Outcome:     "failure",
		profileHits: []ProfileHit{{ProfileID: "akamai", Confidence: 0.5}},
	}
	if !shouldEscalate(a) {
		t.Error("phase 3 with 0.5-confidence WAF hit must escalate")
	}
}

// TestShouldEscalatePhase3OnVerdictChallenge asserts a silent-200
// challenge verdict drives escalation even without a profile hit.
func TestShouldEscalatePhase3OnVerdictChallenge(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{
		Phase:   3,
		Outcome: "failure",
		verdict: VerdictChallenge,
	}
	if !shouldEscalate(a) {
		t.Error("phase 3 VerdictChallenge must escalate")
	}
}

// TestShouldEscalatePhase3OnVerdictBlocked asserts a Blocked verdict
// also escalates.
func TestShouldEscalatePhase3OnVerdictBlocked(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{
		Phase:   3,
		Outcome: "failure",
		verdict: VerdictBlocked,
	}
	if !shouldEscalate(a) {
		t.Error("phase 3 VerdictBlocked must escalate")
	}
}

// TestShouldEscalatePhase3_UnknownProfileOnly_NoEscalate asserts that an
// unknown@0.2 hit alone does NOT escalate (0.2 < 0.3 threshold, OQ §11.6).
func TestShouldEscalatePhase3_UnknownProfileOnly_NoEscalate(t *testing.T) {
	t.Parallel()
	a := &PhaseAttempt{
		Phase:       3,
		Outcome:     "failure",
		profileHits: []ProfileHit{{ProfileID: "unknown", Confidence: 0.2}},
	}
	if shouldEscalate(a) {
		t.Error("phase 3 unknown@0.2 alone must NOT escalate (below threshold)")
	}
}
