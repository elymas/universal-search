package ssrf

import "strings"

// DefaultHostnameBlocklist is the cloud-metadata hostname deny list
// (SPEC-SEC-001 REQ-SEC-008). These hostnames resolve to the link-local
// metadata endpoint and are never a legitimate fetch target; blocking them by
// name is defense-in-depth on top of the 169.254.0.0/16 IP range guard.
//
// Entries are matched case-insensitively, as exact hostnames or as "*.suffix"
// patterns.
var DefaultHostnameBlocklist = []string{
	"metadata.google.internal",   // GCP
	"metadata.azure.com",         // Azure
	"instance-data.ec2.internal", // AWS
	"169.254.169.254",            // AWS/GCP/Azure IMDS (IP literal as host)
	"fd00:ec2::254",              // AWS IPv6 IMDS
}

// matchHostname reports whether host matches any blocklist entry. Matching is
// case-insensitive and trailing-dot-insensitive. An entry of the form
// "*.suffix" matches host if host equals "suffix" or ends with ".suffix".
func matchHostname(host string, blocklist []string) bool {
	h := trimDot(host)
	if h == "" {
		return false
	}
	for _, entry := range blocklist {
		e := trimDot(entry)
		if e == "" {
			continue
		}
		if strings.HasPrefix(e, "*.") {
			suffix := e[2:] // drop "*."
			if h == suffix || strings.HasSuffix(h, "."+suffix) {
				return true
			}
			continue
		}
		if h == e {
			return true
		}
	}
	return false
}
