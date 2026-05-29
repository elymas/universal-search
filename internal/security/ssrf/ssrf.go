package ssrf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Reason classifies why an SSRF guard blocked a request. The values form the
// bounded label set for the usearch_security_ssrf_blocks_total{reason} metric
// (SPEC-SEC-001 REQ-SEC-009, NFR-SEC-007 cardinality cap).
type Reason string

const (
	ReasonScheme          Reason = "scheme"
	ReasonPrivateIP       Reason = "private_ip"
	ReasonRedirectHop     Reason = "redirect_hop"
	ReasonDNSRebind       Reason = "dns_rebind"
	ReasonHostnameBlocked Reason = "hostname_allowlist"
)

// Error is the typed error returned by the SSRF guards. It carries a Reason so
// callers (access, auth, adapters) can map the block to a metric label and,
// where applicable, translate to their own error type without losing the
// classification.
//
// The package intentionally defines its own error type (not access.FetchError)
// to avoid an import cycle: access imports ssrf, never the reverse.
type Error struct {
	Reason  Reason
	Message string
}

func (e *Error) Error() string { return e.Message }

// blocked constructs a block Error with the given reason and message.
func blocked(reason Reason, format string, args ...any) *Error {
	return &Error{Reason: reason, Message: fmt.Sprintf(format, args...)}
}

// ReasonOf extracts the SSRF block Reason from an error (unwrapping as needed),
// or "" if the error chain contains no *Error.
func ReasonOf(err error) Reason {
	var e *Error
	if errors.As(err, &e) {
		return e.Reason
	}
	return ""
}

// ValidateScheme checks that the URL scheme is in the allowlist (http/https by
// default). Preserves access.validateScheme behavior.
//
// @MX:ANCHOR: [AUTO] Generic SSRF scheme guard; reused by access + auth + adapters.
// @MX:REASON: fan_in >= 3 (access cascade, auth discovery, future adapters); a
// behavior change here weakens SSRF protection across every caller.
// @MX:SPEC: SPEC-SEC-001
func ValidateScheme(u *url.URL) error {
	return ValidateSchemeWith(u, DefaultSchemeAllowlist)
}

// ValidateSchemeWith checks the scheme against an explicit allowlist.
func ValidateSchemeWith(u *url.URL, allow []string) error {
	for _, s := range allow {
		if u.Scheme == s {
			return nil
		}
	}
	return blocked(ReasonScheme, "scheme %q not allowed", u.Scheme)
}

// ValidateHost resolves the URL hostname and blocks the request when:
//   - the hostname matches Options.HostnameBlocklist (REQ-SEC-008, NEW), OR
//   - any resolved IP is private/loopback/link-local (unless private networks
//     are allowed via Options or the per-call FetchOptions override).
//
// On DNS failure it blocks (fail-closed). This preserves access.validateHost
// behavior and adds the hostname blocklist layer.
//
// @MX:ANCHOR: [AUTO] Generic SSRF host guard; reused by access + auth + adapters.
// @MX:REASON: fan_in >= 3; the private-IP + hostname blocklist decision is the
// single SSRF chokepoint — a regression here exposes cloud-metadata credentials.
// @MX:SPEC: SPEC-SEC-001
func ValidateHost(ctx context.Context, u *url.URL, opts Options, fopts FetchOptions) error {
	host := u.Hostname()

	// Hostname blocklist is enforced even when private networks are allowed,
	// because cloud-metadata hostnames are never a legitimate fetch target.
	if r := matchHostname(host, hostnameBlocklist(opts)); r {
		return blocked(ReasonHostnameBlocked, "hostname blocked: %s", host)
	}

	if allowPrivate(opts, fopts) {
		return nil
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return blocked(ReasonPrivateIP, "DNS lookup failed for %q: %v", host, err)
	}
	for _, ipAddr := range ips {
		if IsPrivateOrLoopback(ipAddr.IP) {
			return blocked(ReasonPrivateIP, "private/loopback IP %s resolved for %q", ipAddr.IP, host)
		}
	}
	return nil
}

// ValidateRedirect re-runs scheme + host checks for a redirect destination and
// enforces the hop cap. Preserves access.validateRedirect behavior, including
// the use of a background context for the redirect-time host lookup.
//
// @MX:NOTE: [AUTO] Redirect re-validation re-runs the full host guard per hop;
// same-host redirects still increment the hop count to prevent loops.
func ValidateRedirect(next *url.URL, opts Options, fopts FetchOptions, hopCount int) error {
	if hopCount > redirectMaxHops(opts) {
		return blocked(ReasonRedirectHop,
			"too many redirects: %d hops exceeded limit of %d", hopCount, redirectMaxHops(opts))
	}
	if err := ValidateSchemeWith(next, schemeAllowlist(opts)); err != nil {
		return err
	}
	if err := ValidateHost(context.Background(), next, opts, fopts); err != nil {
		return err
	}
	return nil
}

// IsPrivateOrLoopback reports whether ip is loopback, link-local, RFC1918,
// IPv6 ULA, or RFC 6598 shared address space. Identical classification to the
// original access.isPrivateOrLoopback (CIDR table preserved verbatim).
func IsPrivateOrLoopback(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, cidr := range privateRanges {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// privateRanges is the deny list extracted verbatim from access (D6 guard #2).
var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16", // link-local + AWS/GCP/Azure metadata endpoint
		"fc00::/7",       // IPv6 ULA
		"fe80::/10",      // IPv6 link-local
		"::1/128",        // IPv6 loopback
		"127.0.0.0/8",    // IPv4 loopback (belt-and-suspenders)
		"100.64.0.0/10",  // Shared address space (RFC 6598)
	}
	for _, c := range cidrs {
		_, network, err := net.ParseCIDR(c)
		if err != nil {
			panic("ssrf: invalid CIDR in privateRanges: " + c)
		}
		privateRanges = append(privateRanges, network)
	}
}

// trimDot lowercases and strips a trailing dot from a hostname for matching.
func trimDot(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
}
