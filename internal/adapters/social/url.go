// Package social — AT-URI parsing and Bluesky URL construction.
// REQ-ADP6-006: AT-URI rkey extraction; bsky.app URL construction.
package social

import (
	"errors"
	"strings"
)

// parseATURI extracts the DID (or handle) and rkey from an AT-URI.
// Format: at://<did-or-handle>/<collection>/<rkey>
// Returns (did, rkey, nil) on success.
func parseATURI(uri string) (did, rkey string, err error) {
	const prefix = "at://"
	if uri == "" {
		return "", "", errors.New("social: empty AT-URI")
	}
	if !strings.HasPrefix(uri, prefix) {
		return "", "", errors.New("social: AT-URI missing at:// prefix")
	}
	rest := strings.TrimPrefix(uri, prefix)
	// rest = "<did>/<collection>/<rkey>"
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 3 || parts[2] == "" {
		return "", "", errors.New("social: AT-URI has insufficient path segments")
	}
	return parts[0], parts[2], nil
}

// constructBlueskyURL builds the canonical bsky.app post URL from handle and rkey.
func constructBlueskyURL(handle, rkey string) string {
	return "https://bsky.app/profile/" + handle + "/post/" + rkey
}
