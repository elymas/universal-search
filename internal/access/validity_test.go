// Package access — RED-phase tests for the 4-layer page-validity gate.
//
// REQ-ACC-020: validatePage returns a Verdict per the §6.3 truth table.
// REQ-ACC-021 (via §6.3): VerdictChallenge / VerdictBlocked precedence.
package access

import (
	"net/http"
	"strings"
	"testing"
)

// makeResp builds a minimal *http.Response for table tests.
func makeResp(status int, cookies ...string) *http.Response {
	h := http.Header{}
	if len(cookies) > 0 {
		h["Set-Cookie"] = cookies
	}
	return &http.Response{StatusCode: status, Header: h}
}

// TestValidatePageVerdictTable covers every §6.3 truth-table row plus
// the Akamai _abck=~-1~ sensor case.
func TestValidatePageVerdictTable(t *testing.T) {
	t.Parallel()
	bigRealPage := []byte("<html><body><main><article><h1>real</h1><p>" +
		strings.Repeat("content paragraph. ", 60) +
		"</p></article></main></body></html>")
	bigNoSelector := []byte("<html><body>" +
		strings.Repeat("plain text line. ", 60) +
		"</body></html>")
	tinyChallengeBody := []byte("<html><body>cf-please-stand-by</body></html>")
	tinyNoMarker := []byte("<html><body>short</body></html>")
	tinyWithSelector := []byte("<html><body><main>x</main></body></html>")
	bigChallengeBody := []byte("<html><body>" + strings.Repeat("cf-please-stand-by ", 60) + "</body></html>")

	akamaiHit := &ProfileHit{ProfileID: "akamai", Confidence: 0.9}

	cases := []struct {
		name string
		resp *http.Response
		body []byte
		hit  *ProfileHit
		want Verdict
	}{
		{
			name: "StrongOK no L1 no L2 no L3 with L4",
			resp: makeResp(200, "_abck=AAA~~0~~"),
			body: bigRealPage,
			hit:  akamaiHit,
			want: VerdictStrongOK,
		},
		{
			name: "WeakOK no L1 no L2 no L3 no L4",
			resp: makeResp(200, "_abck=AAA~~0~~"),
			body: bigNoSelector,
			hit:  akamaiHit,
			want: VerdictWeakOK,
		},
		{
			name: "Challenge L1 yes L2 no",
			resp: makeResp(200),
			body: bigChallengeBody,
			hit:  nil,
			want: VerdictChallenge,
		},
		{
			name: "Challenge via L3 cookie sensor Akamai _abck=~-1~ L2 no",
			resp: makeResp(200, "_abck=AAA~~-1~-1~-1~"),
			body: bigRealPage,
			hit:  akamaiHit,
			want: VerdictChallenge,
		},
		{
			name: "Blocked L1 yes L2 yes",
			resp: makeResp(200),
			body: tinyChallengeBody,
			hit:  nil,
			want: VerdictBlocked,
		},
		{
			name: "Blocked via L3 yes L2 yes",
			resp: makeResp(200, "_abck=AAA~~-1~-1~"),
			body: tinyNoMarker,
			hit:  akamaiHit,
			want: VerdictBlocked,
		},
		{
			name: "Unknown no L1 no L3 L2 yes with L4",
			resp: makeResp(200),
			body: tinyWithSelector,
			hit:  nil,
			want: VerdictUnknown,
		},
		{
			name: "Unknown no L1 no L3 L2 yes no L4",
			resp: makeResp(200),
			body: tinyNoMarker,
			hit:  nil,
			want: VerdictUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validatePage(tc.resp, tc.body, tc.hit)
			if got != tc.want {
				t.Errorf("validatePage = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestValidatePageANDGating verifies a single positive signal alone does
// NOT yield StrongOK. L3 present with L4 present + normal size → Challenge.
func TestValidatePageANDGating(t *testing.T) {
	t.Parallel()
	big := []byte("<html><body><main><article>" +
		strings.Repeat("paragraph. ", 80) +
		"</article></main></body></html>")
	resp := makeResp(200, "_abck=AAA~~-1~-1~")
	hit := &ProfileHit{ProfileID: "akamai", Confidence: 0.9}
	got := validatePage(resp, big, hit)
	if got != VerdictChallenge {
		t.Errorf("AND-gate: L3 present must yield Challenge even with L4, got %q", got)
	}
}

// TestValidatePageNilHit verifies a nil hit does not crash (L3 always
// false when no profile context is available).
func TestValidatePageNilHit(t *testing.T) {
	t.Parallel()
	resp := makeResp(200)
	body := []byte("<html><body><main>" + strings.Repeat("x", 700) + "</main></body></html>")
	got := validatePage(resp, body, nil)
	if got != VerdictStrongOK {
		t.Errorf("validatePage nil hit = %q, want StrongOK", got)
	}
}

// TestValidatePageNilResp verifies a nil response yields the safest
// non-success verdict (Unknown) rather than panicking.
func TestValidatePageNilResp(t *testing.T) {
	t.Parallel()
	got := validatePage(nil, []byte("anything"), nil)
	if got != VerdictUnknown {
		t.Errorf("validatePage nil resp = %q, want Unknown", got)
	}
}

// TestValidatePageDataDomeCookieSensor covers the DataDome L3 branch:
// a datadome cookie present on the response must drive VerdictBlocked
// on a sub-threshold body and VerdictChallenge on a normal-size body.
func TestValidatePageDataDomeCookieSensor(t *testing.T) {
	t.Parallel()
	ddHit := &ProfileHit{ProfileID: "datadome", Confidence: 0.5}
	tinyBody := []byte("<html><body>short</body></html>")
	bigBody := []byte("<html><body>" + strings.Repeat("datadome ", 80) + "</body></html>")

	// L2 yes (tiny) + L3 yes (datadome cookie) → Blocked.
	resp := makeResp(200, "datadome=AAAA~xyz~1~1")
	if got := validatePage(resp, tinyBody, ddHit); got != VerdictBlocked {
		t.Errorf("datadome tiny body = %q, want Blocked", got)
	}
	// L2 no (big) + L3 yes → Challenge.
	resp2 := makeResp(200, "datadome=AAAA~xyz~1~1")
	if got := validatePage(resp2, bigBody, ddHit); got != VerdictChallenge {
		t.Errorf("datadome big body = %q, want Challenge", got)
	}
	// datadome hit but no datadome cookie set, and body has no datadome
	// marker → L1 false, L3 false, L2 false, L4 false → WeakOK.
	resp3 := makeResp(200)
	plainBig := []byte("<html><body>" + strings.Repeat("plain text. ", 80) + "</body></html>")
	if got := validatePage(resp3, plainBig, ddHit); got != VerdictWeakOK {
		t.Errorf("datadome no-cookie no-marker big body = %q, want WeakOK", got)
	}
}

// TestValidatePageNonSensorProfileDefaultFalse covers the L3 default
// branch: a profile with no sensor-cookie convention (e.g. cloudflare)
// returns false regardless of cookies.
func TestValidatePageNonSensorProfileDefaultFalse(t *testing.T) {
	t.Parallel()
	cfHit := &ProfileHit{ProfileID: "cloudflare", Confidence: 0.5}
	bigBody := []byte("<html><body>" + strings.Repeat("plain ", 120) + "</body></html>")
	resp := makeResp(200, "cf_clearance=abc")
	if got := validatePage(resp, bigBody, cfHit); got != VerdictWeakOK {
		t.Errorf("cloudflare L3 default = %q, want WeakOK (no sensor convention)", got)
	}
}

// TestLayerChallengeMarkerHitNoMatch covers the L1 branch where a hit
// is present but its profile's body markers do NOT appear in the body.
func TestLayerChallengeMarkerHitNoMatch(t *testing.T) {
	t.Parallel()
	cfHit := ProfileHit{ProfileID: "cloudflare", Confidence: 0.5}
	body := []byte("<html><body>" + strings.Repeat("plain text. ", 60) + "</body></html>")
	if layerChallengeMarker(bytesLower(body), &cfHit) {
		t.Error("L1 must be false when the hit profile's body markers are absent")
	}
}

// bytesLower is a tiny helper to avoid importing bytes in the test
// header; keeps the L1 unit test self-contained.
func bytesLower(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return out
}
