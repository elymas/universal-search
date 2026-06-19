// Package access — RED-phase tests for WAF profile detection (SPEC-ACC-001).
//
// REQ-ACC-010: WAFProfile table shape.
// REQ-ACC-011: confidence-ranked detection.
// REQ-ACC-012: unknown fallback + clean-200 empty.
// REQ-ACC-040: No-Site-Name rule tripwire.
package access

import (
	"math"
	"net/http"
	"strings"
	"testing"
)

// TestWAFProfilesTableShape asserts the 7-entry vendor-generic table.
// REQ-ACC-010.
func TestWAFProfilesTableShape(t *testing.T) {
	t.Parallel()
	wantIDs := map[string]bool{
		"akamai": true, "cloudflare": true, "f5": true, "aws-waf": true,
		"datadome": true, "perimeterx": true, "unknown": true,
	}
	if len(wafProfiles) != 7 {
		t.Fatalf("wafProfiles must have exactly 7 entries, got %d", len(wafProfiles))
	}
	seen := make(map[string]int, 7)
	for i, p := range wafProfiles {
		if !wantIDs[p.ID] {
			t.Errorf("entry %d has unknown ID %q", i, p.ID)
		}
		seen[p.ID]++
		if p.CookiePatterns == nil {
			t.Errorf("profile %q CookiePatterns is nil", p.ID)
		}
		if p.HeaderMarkers == nil {
			t.Errorf("profile %q HeaderMarkers is nil", p.ID)
		}
		if p.BodyMarkers == nil {
			t.Errorf("profile %q BodyMarkers is nil", p.ID)
		}
	}
	for id := range wantIDs {
		if seen[id] != 1 {
			t.Errorf("profile %q appears %d times, want exactly 1", id, seen[id])
		}
	}
}

// TestDetectProfilesRanking asserts the §2.4 additive confidence model and
// descending sort. Akamai cookie+header beats DataDome cookie-only.
// REQ-ACC-011.
func TestDetectProfilesRanking(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: 403,
		Header: http.Header{
			"Set-Cookie":          []string{"_abck=ABC~~-1~", "datadome=AAAA-BBB"},
			"X-Akamai-Request-ID": []string{"abc"},
		},
	}
	hits := detectProfiles(resp, []byte("real body content here"))
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d: %+v", len(hits), hits)
	}
	if hits[0].ProfileID != "akamai" {
		t.Errorf("top hit = %q, want akamai", hits[0].ProfileID)
	}
	if math.Abs(hits[0].Confidence-0.9) > 0.001 {
		t.Errorf("akamai confidence = %.3f, want 0.900", hits[0].Confidence)
	}
	if hits[1].ProfileID != "datadome" {
		t.Errorf("second hit = %q, want datadome", hits[1].ProfileID)
	}
	if math.Abs(hits[1].Confidence-0.5) > 0.001 {
		t.Errorf("datadome confidence = %.3f, want 0.500", hits[1].Confidence)
	}
}

// TestDetectProfilesConfidenceFormula exercises single/double/triple
// detector-class matches against the akamai profile (all three classes).
// REQ-ACC-011.
func TestDetectProfilesConfidenceFormula(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		resp *http.Response
		body []byte
		want float64
	}{
		{
			name: "cookie-only",
			resp: &http.Response{StatusCode: 403,
				Header: http.Header{"Set-Cookie": []string{"_abck=AAA"}}},
			body: []byte("nothing relevant here"),
			want: 0.5,
		},
		{
			name: "header-only",
			resp: &http.Response{StatusCode: 403,
				Header: http.Header{"X-Akamai-Req-ID": []string{"x"}}},
			body: []byte("nothing relevant here"),
			want: 0.4,
		},
		{
			name: "body-only",
			resp: &http.Response{StatusCode: 403, Header: http.Header{}},
			body: []byte("Reference #18.a8d73017"),
			want: 0.3,
		},
		{
			name: "cookie-plus-header",
			resp: &http.Response{StatusCode: 403,
				Header: http.Header{
					"Set-Cookie":      []string{"_abck=AAA"},
					"X-Akamai-Req-ID": []string{"x"},
				}},
			body: []byte("nothing relevant here"),
			want: 0.9,
		},
		{
			name: "all-three-clamped",
			resp: &http.Response{StatusCode: 403,
				Header: http.Header{
					"Set-Cookie":      []string{"_abck=AAA"},
					"X-Akamai-Req-ID": []string{"x"},
				}},
			body: []byte("Reference #18.a8d73017"),
			want: 1.0, // 0.5+0.4+0.3 = 1.2 → clamp 1.0
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := detectProfiles(tc.resp, tc.body)
			var got float64
			for _, h := range hits {
				if h.ProfileID == "akamai" {
					got = h.Confidence
				}
			}
			if math.Abs(got-tc.want) > 0.001 {
				t.Errorf("akamai confidence = %.3f, want %.3f", got, tc.want)
			}
		})
	}
}

// TestDetectProfilesUnknownFallback: 403 with no vendor signature →
// exactly one {unknown, 0.2} hit. REQ-ACC-012.
func TestDetectProfilesUnknownFallback(t *testing.T) {
	t.Parallel()
	resp := &http.Response{StatusCode: 403, Header: http.Header{}}
	hits := detectProfiles(resp, []byte("generic block page"))
	if len(hits) != 1 {
		t.Fatalf("expected 1 unknown hit, got %d: %+v", len(hits), hits)
	}
	if hits[0].ProfileID != "unknown" {
		t.Errorf("hit[0].ProfileID = %q, want unknown", hits[0].ProfileID)
	}
	if math.Abs(hits[0].Confidence-0.2) > 0.001 {
		t.Errorf("unknown confidence = %.3f, want 0.200", hits[0].Confidence)
	}
}

// TestDetectProfilesUnknownFallbackChallengeBody: a 200 body with a
// JS-challenge marker and no vendor signature also triggers unknown.
// Uses <noscript> (a jsChallengePattern that is NOT a vendor body
// marker) so no vendor profile matches but the challenge signal still
// fires the unknown fallback. REQ-ACC-012.
func TestDetectProfilesUnknownFallbackChallengeBody(t *testing.T) {
	t.Parallel()
	resp := &http.Response{StatusCode: 200, Header: http.Header{}}
	hits := detectProfiles(resp, []byte("please enable js <noscript>"))
	if len(hits) != 1 || hits[0].ProfileID != "unknown" {
		t.Errorf("expected single unknown hit, got %+v", hits)
	}
}

// TestDetectProfilesCleanResponseEmpty: clean 200, no markers → empty.
// REQ-ACC-012.
func TestDetectProfilesCleanResponseEmpty(t *testing.T) {
	t.Parallel()
	resp := &http.Response{StatusCode: 200, Header: http.Header{}}
	hits := detectProfiles(resp, []byte("a normal page body with no challenge or waf markers"))
	if len(hits) != 0 {
		t.Errorf("clean 200 must yield empty hits, got %+v", hits)
	}
}

// TestDetectProfilesSortTieBreak: two profiles with equal confidence
// must tie-break by ProfileID ascending. REQ-ACC-011.
func TestDetectProfilesSortTieBreak(t *testing.T) {
	t.Parallel()
	// Two header-only matches at 0.4 each: aws-waf and datadome both
	// expose a header detector that we can trigger independently.
	resp := &http.Response{StatusCode: 403, Header: http.Header{
		"X-DataDome": []string{"x"},
		// pick a second vendor that has a header-only detector distinct
		// from datadome; f5 exposes x-served-by.
		"X-Served-By": []string{"cache-fra"},
	}}
	hits := detectProfiles(resp, []byte("no body markers"))
	// collect confidences for the matched profiles
	var confs []float64
	var ids []string
	for _, h := range hits {
		confs = append(confs, h.Confidence)
		ids = append(ids, h.ProfileID)
	}
	// Verify descending-by-confidence then ascending-by-ID invariant.
	for i := 1; i < len(hits); i++ {
		if hits[i].Confidence > hits[i-1].Confidence {
			t.Errorf("hits not sorted desc by confidence: %+v", hits)
		}
		if hits[i].Confidence == hits[i-1].Confidence && hits[i].ProfileID < hits[i-1].ProfileID {
			t.Errorf("ties not broken by ProfileID asc: %+v", hits)
		}
	}
	_ = confs
	_ = ids
}

// TestNoSiteNameRule: no profile marker or validity selector contains a
// tripwire site-name substring. REQ-ACC-040.
func TestNoSiteNameRule(t *testing.T) {
	t.Parallel()
	tripwires := []string{".com/", "reddit", "naver", "google", "youtube", "github"}
	check := func(owner, s string) {
		low := strings.ToLower(s)
		for _, tw := range tripwires {
			if strings.Contains(low, tw) {
				t.Errorf("%s contains tripwire %q: %q", owner, tw, s)
			}
		}
	}
	for _, p := range wafProfiles {
		for _, c := range p.CookiePatterns {
			check("profile "+p.ID+" cookie", c)
		}
		for _, h := range p.HeaderMarkers {
			check("profile "+p.ID+" header", h)
		}
		for _, b := range p.BodyMarkers {
			check("profile "+p.ID+" body", b)
		}
	}
	for _, sel := range successSelectors {
		check("validity selector", sel)
	}
}
