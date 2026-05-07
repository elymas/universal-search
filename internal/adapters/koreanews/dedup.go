// Package koreanews — intra-adapter URL canonicalization and deduplication.
// SPEC-ADP-009: dedupDocs mirrors SPEC-FAN-001 §2.4 8-rule URL canonicalization.
// Operates BEFORE FAN-001 cross-adapter dedup; FAN-001 then dedups across
// koreanews + naver + reddit + etc.
package koreanews

import (
	"net/url"
	"strings"

	"github.com/elymas/universal-search/pkg/types"
)

// dedupDocs deduplicates docs by canonical URL using first-occurrence-wins.
// Returns the deduplicated slice and the count of dropped duplicates.
//
// URL canonicalization rules (mirror SPEC-FAN-001 §2.4 8 rules):
//  1. Lowercase the scheme and host.
//  2. Remove default ports (80 for http, 443 for https).
//  3. Remove trailing slashes from the path (except root "/").
//  4. Sort query parameters alphabetically.
//  5. Remove known tracking parameters (utm_*, fbclid, gclid, ref, source).
//  6. Remove empty query parameters.
//  7. Remove fragment identifiers (#...).
//  8. Normalize percent-encoding (decode safe chars, uppercase hex digits).
//
// If URL canonicalization fails (malformed URL), falls back to CanonicalHash()
// for dedup key to avoid silently dropping unparseable documents.
//
// @MX:NOTE: [AUTO] Intra-adapter URL canonicalization mirroring SPEC-FAN-001 §2.4.
// Operates BEFORE FAN-001 cross-adapter dedup.
// @MX:SPEC: SPEC-ADP-009
func dedupDocs(docs []types.NormalizedDoc) ([]types.NormalizedDoc, int) {
	if len(docs) == 0 {
		return docs, 0
	}

	seen := make(map[string]struct{}, len(docs))
	result := make([]types.NormalizedDoc, 0, len(docs))
	dropped := 0

	for _, doc := range docs {
		key := canonicalKey(doc)
		if _, exists := seen[key]; exists {
			dropped++
			continue
		}
		seen[key] = struct{}{}
		result = append(result, doc)
	}

	return result, dropped
}

// canonicalKey returns the dedup key for a document.
// Tries URL canonicalization first; falls back to CanonicalHash() on failure.
func canonicalKey(doc types.NormalizedDoc) string {
	canonical, err := canonicalizeURL(doc.URL)
	if err != nil || canonical == "" {
		return doc.CanonicalHash()
	}
	return canonical
}

// trackingParams is the set of query parameter names that are removed during
// URL canonicalization (mirrors SPEC-FAN-001 §2.4 rule 5).
var trackingParams = map[string]bool{
	"utm_source":   true,
	"utm_medium":   true,
	"utm_campaign": true,
	"utm_term":     true,
	"utm_content":  true,
	"fbclid":       true,
	"gclid":        true,
	"ref":          true,
	"source":       true,
}

// canonicalizeURL applies the 8 canonicalization rules to rawURL.
// Returns an error if rawURL cannot be parsed.
func canonicalizeURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Rule 1: lowercase scheme and host.
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	// Rule 2: remove default ports.
	host := u.Hostname()
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		u.Host = host
	}

	// Rule 3: remove trailing slashes from path (except root "/").
	if len(u.Path) > 1 {
		u.Path = strings.TrimRight(u.Path, "/")
	}

	// Rule 7: remove fragment.
	u.Fragment = ""

	// Rules 4, 5, 6: process query parameters.
	q := u.Query()
	for k := range q {
		// Rule 5: remove tracking params.
		if trackingParams[strings.ToLower(k)] {
			delete(q, k)
			continue
		}
		// Rule 6: remove empty params.
		if len(q[k]) == 0 || (len(q[k]) == 1 && q[k][0] == "") {
			delete(q, k)
		}
	}
	// Rule 4: sort (url.Values.Encode sorts alphabetically).
	u.RawQuery = q.Encode()

	return u.String(), nil
}
