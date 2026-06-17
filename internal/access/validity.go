// Package access — 4-layer page-validity gate (SPEC-ACC-001).
//
// validatePage answers "is this 200 OK really a real page, or a silent
// challenge?" by AND-gating four signals per the §6.3 truth table. The
// cascade consumes the Verdict to demote a silent-200 challenge to a
// non-success outcome (REQ-ACC-021).
//
// REQ-ACC-020: Verdict production.
// REQ-ACC-021: VerdictChallenge / VerdictBlocked precedence over OK.
// REQ-ACC-022: VerdictWeakOK is a permissive success path for JSON / minimal pages.
package access

import (
	"bytes"
	"net/http"
	"strings"
)

// Verdict is the outcome of the 4-layer page-validity gate. See §6.3
// for the authoritative truth table.
type Verdict string

const (
	// VerdictStrongOK: real page, no challenge, sensor cleared, normal
	// size, success selector present (NOT L1, NOT L2, NOT L3, L4).
	VerdictStrongOK Verdict = "strong_ok"
	// VerdictWeakOK: plausible page, no challenge, normal size, but no
	// success selector (e.g. JSON API body or a minimal valid page).
	VerdictWeakOK Verdict = "weak_ok"
	// VerdictChallenge: a challenge is in flight AND the body is NOT
	// sub-threshold (L1 OR L3 present, NOT L2).
	VerdictChallenge Verdict = "challenge"
	// VerdictBlocked: a challenge/block stub on a sub-threshold body
	// (L1 OR L3 present, L2).
	VerdictBlocked Verdict = "blocked"
	// VerdictUnknown: ambiguous — no challenge signal but a sub-threshold
	// body (NOT L1, NOT L3, L2). The L4 success selector does NOT rescue
	// this case because a sub-threshold body cannot be trusted.
	VerdictUnknown Verdict = "unknown"
)

// minRealPageBytes is the body-size floor below which a response is
// treated as a stub/error/challenge block rather than a real page (L2).
//
// @MX:NOTE: [AUTO] §2.5 L2 fingerprint threshold. OQ §11.4 tracks the
// revisit decision against production telemetry.
// @MX:SPEC: SPEC-ACC-001
const minRealPageBytes = 512

// successSelectors is the set of real-content CSS selectors whose
// presence in the body counts as L4 (success-selector proof). They key
// off generic HTML structural elements (vendor-neutral, No-Site-Name).
var successSelectors = []string{
	"<main",
	"<article",
	"id=\"content\"",
	"class=\"content\"",
}

// validatePage classifies a Phase 3/4 response body through the 4-layer
// AND-gate per §6.3 and returns exactly one Verdict.
//
// Layers:
//   - L1 challenge marker: body contains a WAF body-marker (from the
//     detected profile, if any) OR a jsChallengePatterns substring.
//   - L2 body-size fingerprint: len(body) < minRealPageBytes.
//   - L3 cookie sensor: the detected profile's cookie-sensor state
//     indicates "still challenging" (e.g. Akamai _abck=~-1~).
//   - L4 success selector: body contains a real-content CSS selector.
//
// Pure function (no I/O, no time, no randomness).
//
// @MX:ANCHOR: [AUTO] The silent-200 gate; every Phase 3/4 body passes through it.
// @MX:REASON: AND-gating prevents counting a challenge as success; a bug
// here re-opens the silent-200 trap that this SPEC exists to close.
// @MX:SPEC: SPEC-ACC-001
func validatePage(resp *http.Response, body []byte, hit *ProfileHit) Verdict {
	if resp == nil {
		return VerdictUnknown
	}
	bodyLower := bytes.ToLower(body)
	l1 := layerChallengeMarker(bodyLower, hit)
	l2 := len(body) < minRealPageBytes
	l3 := layerCookieSensor(resp.Header, hit)
	l4 := layerSuccessSelector(bodyLower)

	// §6.3 truth table (AUTHORITATIVE). Challenge/Blocked take precedence
	// over OK verdicts; L2 disambiguates Challenge vs Blocked; the OK
	// verdicts require NOT L2 (a sub-threshold body cannot be trusted).
	if l1 || l3 {
		if l2 {
			return VerdictBlocked
		}
		return VerdictChallenge
	}
	// No challenge signal (NOT L1, NOT L3).
	if l2 {
		return VerdictUnknown
	}
	// NOT L1, NOT L3, NOT L2 → disambiguate on L4.
	if l4 {
		return VerdictStrongOK
	}
	return VerdictWeakOK
}

// layerChallengeMarker (L1) reports whether the body contains a
// challenge marker: a JS-challenge substring OR a body-marker from the
// detected WAF profile (if any).
func layerChallengeMarker(bodyLower []byte, hit *ProfileHit) bool {
	if containsAnyLower(bodyLower, jsChallengePatterns) {
		return true
	}
	if hit == nil {
		return false
	}
	for _, p := range wafProfiles {
		if p.ID != hit.ProfileID {
			continue
		}
		return containsAnyLower(bodyLower, p.BodyMarkers)
	}
	return false
}

// layerCookieSensor (L3) reports whether the detected profile's cookie
// sensor indicates the WAF is still challenging the client. The Akamai
// _abck cookie carries a sensor-state segment; the literal "~-1~"
// inside that cookie means "still being challenged".
func layerCookieSensor(h http.Header, hit *ProfileHit) bool {
	if hit == nil {
		return false
	}
	switch hit.ProfileID {
	case "akamai":
		// _abck=...~-1~... means still challenging; ~0~ means cleared.
		for _, c := range h["Set-Cookie"] {
			if strings.HasPrefix(strings.ToLower(c), "_abck=") && strings.Contains(c, "~-1~") {
				return true
			}
		}
		return false
	case "datadome":
		// DataDome cookie present on a 403 means the sensor blocked us.
		for _, c := range h["Set-Cookie"] {
			if strings.HasPrefix(strings.ToLower(c), "datadome=") {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// layerSuccessSelector (L4) reports whether the body contains any
// real-content CSS selector. Vendor-neutral; No-Site-Name compliant.
func layerSuccessSelector(bodyLower []byte) bool {
	for _, sel := range successSelectors {
		if bytes.Contains(bodyLower, []byte(strings.ToLower(sel))) {
			return true
		}
	}
	return false
}
