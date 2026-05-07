// Package fanout — canonicalURL implements the 8 URL normalisation rules.
// SPEC-FAN-001 §2.4.
package fanout

import (
	"net/url"
	"sort"
	"strings"
)

// trackingParams is the complete set of query parameter keys to strip.
// These are well-known analytics/tracking identifiers per research §3.4.1.
// The list is a package-level constant; never mutated at runtime.
//
// @MX:NOTE: [AUTO] 11-entry tracking-param list from SPEC-FAN-001 §2.4 Rule 4.
// Add new tracking keys here only via SPEC amendment — false negatives (missed dedup)
// are preferable to false positives (incorrect merging of distinct resources).
// @MX:SPEC: SPEC-FAN-001
var trackingParams = []string{
	"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
	"gclid", "fbclid", "mc_eid", "mc_cid", "_ga", "ref", "ref_src",
}

// trackingParamSet is a fast-lookup map derived from trackingParams at init time.
var trackingParamSet = func() map[string]bool {
	m := make(map[string]bool, len(trackingParams))
	for _, p := range trackingParams {
		m[p] = true
	}
	return m
}()

// canonicalURL applies the 8 normalisation rules from §2.4 and returns the
// dedup key string. Returns a parse error for malformed URLs — the caller
// falls back to CanonicalHash in that case.
//
// Rules applied (in order):
//  1. Lowercase scheme
//  2. Lowercase host
//  3. Strip fragment
//  4. Strip 11 tracking params
//  5. Trim trailing slash from path (except root "/")
//  6. Sort remaining query params alphabetically
//  7. Preserve path case (no change)
//  8. Preserve percent-encoding (no change)
//
// The output is the dedup key, NOT the displayed URL — the original
// NormalizedDoc.URL is preserved in the returned doc slice.
func canonicalURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	// Reject schemeless or empty URLs that parse without error.
	if u.Scheme == "" || u.Host == "" {
		return "", &url.Error{Op: "parse", URL: raw, Err: url.EscapeError("missing scheme or host")}
	}

	// Rule 1: lowercase scheme.
	u.Scheme = strings.ToLower(u.Scheme)
	// Rule 2: lowercase host.
	u.Host = strings.ToLower(u.Host)
	// Rule 3: strip fragment.
	u.Fragment = ""
	u.RawFragment = ""

	// Rule 4: strip tracking params.
	q := u.Query()
	for key := range q {
		if trackingParamSet[key] {
			delete(q, key)
		}
	}

	// Rule 5: trim trailing slash from path (but preserve root "/").
	if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
	}

	// Rule 6: sort remaining query params alphabetically.
	// Re-encode the filtered query map with sorted keys.
	if len(q) == 0 {
		u.RawQuery = ""
	} else {
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			for _, v := range q[k] {
				parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
		u.RawQuery = strings.Join(parts, "&")
	}

	// Rules 7+8: preserve path case and percent-encoding — no change needed.
	return u.String(), nil
}
