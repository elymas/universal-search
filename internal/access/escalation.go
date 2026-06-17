// Package access — escalation predicates for the 5-phase cascade.
//
// REQ-CACHE-014: Pure functions encoding the escalation rules from §3.4.
// shouldEscalate is deterministic: same input → same output (no I/O, no time,
// no randomness).
package access

// shouldEscalate returns true when the cascade should attempt the next phase
// after the given phase attempt.
//
// Escalation predicates per SPEC-CACHE-001 §3.4 / §6.5:
//   - Phase 1 miss or skipped → escalate to Phase 2
//   - Phase 2 success (robots allow) or skipped → escalate to Phase 3
//   - Phase 3 TLS error or WAF status → escalate to Phase 4
//   - Phase 4 JS challenge AND PlaywrightEnabled → escalate to Phase 5
//   - Phase 5 always halts (no further phase)
//
// @MX:NOTE: [AUTO] Escalation predicates per SPEC-CACHE-001 §3.4.
// Future contributors: add new phase conditions here and update TestShouldEscalateTable.
// @MX:SPEC: SPEC-CACHE-001
func shouldEscalate(prev *PhaseAttempt) bool {
	switch prev.Phase {
	case 1:
		// Phase 1 miss, skipped, or failure (e.g. panic) → escalate to Phase 2.
		// A Phase 1 panic recovered as "failure" is effectively a miss.
		return prev.Outcome == "miss" || prev.Outcome == "skipped" || prev.Outcome == "failure"
	case 2:
		// Phase 2 robots-allow → escalate to Phase 3.
		// Disallow (blocked) halts. Timeout halts.
		return prev.Outcome == "success" || prev.Outcome == "skipped"
	case 3:
		// Phase 3 escalates on TLS error, a confident WAF profile hit,
		// or a VerdictChallenge/VerdictBlocked on an otherwise-200 body
		// (the silent-200 trap). SPEC-ACC-001 REQ-ACC-013/021.
		return prev.isTLSError ||
			prev.hasWAFProfile() ||
			prev.verdict == VerdictChallenge ||
			prev.verdict == VerdictBlocked
	case 4:
		// Phase 4 escalates ONLY on JS challenge (and PlaywrightEnabled is checked
		// in the cascade runner, not here, to keep this function pure).
		return prev.isJSChallenge
	case 5:
		// Phase 5 always halts — no further phase.
		return false
	}
	return false
}
