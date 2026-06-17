// Package access — vendor-generic WAF product profile detection.
//
// SPEC-ACC-001 ports the insane-search v0.5 WAF-profile concept: a
// data-driven table of 7 vendor profiles (Akamai, Cloudflare, F5, AWS
// WAF, DataDome, PerimeterX, unknown). detectProfiles returns a
// confidence-ranked list of hits; the cascade reads the top hit to
// decide Phase 3 → 4 escalation.
//
// The per-WAF TLS avoid-list from the upstream is DEFERRED to
// SPEC-CACHE-001b (it only earns its keep once an impersonation library
// supplies a real candidate set to filter).
//
// REQ-ACC-010: table shape.
// REQ-ACC-011: additive confidence model + ranking.
// REQ-ACC-012: unknown fallback + clean-200 empty.
// REQ-ACC-040: No-Site-Name rule (vendor infrastructure signatures only).
package access

import (
	"bytes"
	"math"
	"net/http"
	"sort"
	"strings"
)

// WAFProfile declares a vendor-generic WAF detector. Detectors key off
// vendor infrastructure signatures (cookie names, header prefixes,
// challenge-body markers) — NEVER off a target site's domain or brand
// (the No-Site-Name rule, REQ-ACC-040).
type WAFProfile struct {
	ID             string
	DisplayName    string
	CookiePatterns []string
	HeaderMarkers  []string
	BodyMarkers    []string
}

// ProfileHit is a single ranked detection result. Confidence is the
// additive bounded probability the matching response is gated by this
// vendor's WAF.
type ProfileHit struct {
	ProfileID  string
	Confidence float64
}

// wafEscalateThreshold is the confidence floor at which a ProfileHit
// participates in Phase 3 → 4 escalation (hasWAFProfile). unknown@0.2
// sits below it (telemetry-only) by design (OQ §11.6).
//
// @MX:NOTE: [AUTO] §2.4 escalation floor. Confidence < 0.3 does not
// force a TLS-hardening pass; only confident vendor hits do.
// @MX:SPEC: SPEC-ACC-001
const wafEscalateThreshold = 0.3

// unknownConfidence is the floor confidence recorded for an unidentified
// 403/503/challenge response (below wafEscalateThreshold by design).
const unknownConfidence = 0.2

// wafProfiles is the vendor-generic profile table. No-Site-Name rule
// (§2.3): every marker keys off vendor infrastructure, never off a
// target site's domain or brand.
//
// @MX:NOTE: [AUTO] vendor-generic data table (§2.3 No-Site-Name rule).
// Add a new vendor by appending an entry; the detection algorithm is
// data-driven and does not change.
// @MX:SPEC: SPEC-ACC-001
var wafProfiles = []WAFProfile{
	{
		ID:             "akamai",
		DisplayName:    "Akamai",
		CookiePatterns: []string{"_abck=", "ak_bmsc=", "bm_sz="},
		HeaderMarkers:  []string{"x-akamai-"},
		BodyMarkers:    []string{"reference #"},
	},
	{
		ID:             "cloudflare",
		DisplayName:    "Cloudflare",
		CookiePatterns: []string{"__cf_bm=", "cf_clearance="},
		HeaderMarkers:  []string{"cf-ray", "cf-mitigated"},
		BodyMarkers:    []string{"cf-please-stand-by", "checking if the site connection is secure"},
	},
	{
		ID:             "f5",
		DisplayName:    "F5 / BIG-IP",
		CookiePatterns: []string{"bigipserver", "ts="},
		HeaderMarkers:  []string{"x-served-by"},
		BodyMarkers:    []string{},
	},
	{
		ID:             "aws-waf",
		DisplayName:    "AWS WAF",
		CookiePatterns: []string{},
		HeaderMarkers:  []string{"x-amzn-waf", "x-amz-cf-id"},
		BodyMarkers:    []string{"aws waf"},
	},
	{
		ID:             "datadome",
		DisplayName:    "DataDome",
		CookiePatterns: []string{"datadome="},
		HeaderMarkers:  []string{"x-datadome"},
		BodyMarkers:    []string{"datadome"},
	},
	{
		ID:             "perimeterx",
		DisplayName:    "PerimeterX",
		CookiePatterns: []string{"_px", "pxhd"},
		HeaderMarkers:  []string{"x-pxpx"},
		BodyMarkers:    []string{"px-captcha"},
	},
	{
		ID:             "unknown",
		DisplayName:    "Unknown WAF",
		CookiePatterns: []string{},
		HeaderMarkers:  []string{},
		BodyMarkers:    []string{},
	},
}

// detectProfiles classifies a response against the wafProfiles table and
// returns a confidence-ranked slice of ProfileHit (descending by
// Confidence, ties broken by ProfileID ascending for determinism).
//
// Confidence model (§2.4, additive, clamped to [0.0, 1.0]):
//
//	confidence = clamp(0.5*I(cookie) + 0.4*I(header) + 0.3*I(body))
//
// A profile with zero matching detector classes produces no hit. When a
// 403/503/challenge-marked response matches no vendor, the slice
// contains exactly one {unknown, 0.2} hit. A clean 200 with no markers
// yields an empty slice.
//
// Pure function (no I/O, no time, no randomness): the same (resp, body)
// always yields the same ranked slice.
//
// @MX:ANCHOR: [AUTO] Sole WAF-classification entry; fan_in = 2 (phase3, phase4).
// @MX:REASON: confidence model + No-Site-Name invariant; changing the
// formula shifts every escalation decision downstream.
// @MX:SPEC: SPEC-ACC-001
func detectProfiles(resp *http.Response, body []byte) []ProfileHit {
	if resp == nil {
		return nil
	}
	bodyLower := bytes.ToLower(body)
	cookieText := lowerCookieText(resp.Header)
	headerKeys := lowerHeaderKeys(resp.Header)

	challengeInBody := containsAnyLower(bodyLower, jsChallengePatterns)
	statusIsBlock := resp.StatusCode == 403 || resp.StatusCode == 503
	needsUnknown := statusIsBlock || challengeInBody

	var hits []ProfileHit
	for _, p := range wafProfiles {
		if p.ID == "unknown" {
			continue // the synthetic fallback is handled below
		}
		cookieMatch := containsAnyLower(cookieText, p.CookiePatterns)
		headerMatch := anyHeaderMatches(headerKeys, p.HeaderMarkers)
		bodyMatch := containsAnyLower(bodyLower, p.BodyMarkers)
		if !cookieMatch && !headerMatch && !bodyMatch {
			continue
		}
		conf := 0.0
		if cookieMatch {
			conf += 0.5
		}
		if headerMatch {
			conf += 0.4
		}
		if bodyMatch {
			conf += 0.3
		}
		hits = append(hits, ProfileHit{ProfileID: p.ID, Confidence: clampConf(conf)})
	}
	if len(hits) == 0 && needsUnknown {
		hits = append(hits, ProfileHit{ProfileID: "unknown", Confidence: unknownConfidence})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Confidence != hits[j].Confidence {
			return hits[i].Confidence > hits[j].Confidence
		}
		return hits[i].ProfileID < hits[j].ProfileID
	})
	return hits
}

// lowerCookieText concatenates all Set-Cookie header values (lowercased)
// so a single substring scan covers every cookie. Returns a single
// space-joined byte slice.
func lowerCookieText(h http.Header) []byte {
	cs := h["Set-Cookie"]
	if len(cs) == 0 {
		return nil
	}
	joined := strings.ToLower(strings.Join(cs, " "))
	return []byte(joined)
}

// lowerHeaderKeys returns the lowercased response header keys as a slice.
func lowerHeaderKeys(h http.Header) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, strings.ToLower(k))
	}
	return keys
}

// anyHeaderMatches reports whether any lowercased response header key
// has one of the markers as a prefix.
func anyHeaderMatches(headerKeys, markers []string) bool {
	if len(markers) == 0 {
		return false
	}
	for _, m := range markers {
		lm := strings.ToLower(m)
		for _, k := range headerKeys {
			if strings.HasPrefix(k, lm) {
				return true
			}
		}
	}
	return false
}

// containsAnyLower reports whether haystack contains any of needles
// (both sides compared lowercased). Empty needles slice → false.
func containsAnyLower(haystack []byte, needles []string) bool {
	if len(needles) == 0 || len(haystack) == 0 {
		return false
	}
	for _, n := range needles {
		if n == "" {
			continue
		}
		if bytes.Contains(haystack, []byte(strings.ToLower(n))) {
			return true
		}
	}
	return false
}

// clampConf bounds v to [0.0, 1.0].
func clampConf(v float64) float64 {
	if v < 0.0 || math.IsNaN(v) {
		return 0.0
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}

// topHitOrNil returns the first (highest-confidence) hit, or nil when the
// slice is empty. Convenience for passing into validatePage.
func topHitOrNil(hits []ProfileHit) *ProfileHit {
	if len(hits) == 0 {
		return nil
	}
	return &hits[0]
}
