// Package access — SSRF guard functions.
//
// REQ-CACHE-013: Four HARD-rule SSRF guards.
// D6: Scheme allowlist, private/loopback deny-by-default, DNS-rebind
// mitigation (pinnedIPDialer), redirect re-validation with hop cap.
// These are pure functions (given resolved IPs) and have no side effects.
package access

import (
	"context"
	"fmt"
	"net"
	"net/url"
)

// validateScheme checks that the URL scheme is http or https.
// Returns *FetchError{CategoryBlocked} on violation.
func validateScheme(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return &FetchError{
			Category: CategoryBlocked,
			Reason:   fmt.Sprintf("scheme %q not allowed", u.Scheme),
		}
	}
	return nil
}

// validateHost resolves the hostname and checks that no resolved IP is a
// private, loopback, or link-local address (unless AllowPrivateNetworks is set).
// Returns *FetchError{CategoryBlocked} on violation.
func validateHost(ctx context.Context, u *url.URL, opts Options, fopts FetchOptions) error {
	if opts.AllowPrivateNetworks || fopts.AllowPrivateNetworks {
		return nil
	}

	host := u.Hostname()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		// On DNS resolution failure we block by default (fail-closed).
		return &FetchError{
			Category: CategoryBlocked,
			Reason:   fmt.Sprintf("DNS lookup failed for %q: %v", host, err),
			Cause:    err,
		}
	}

	for _, ipAddr := range ips {
		if isPrivateOrLoopback(ipAddr.IP) {
			return &FetchError{
				Category: CategoryBlocked,
				Reason:   fmt.Sprintf("private/loopback IP %s resolved for %q", ipAddr.IP, host),
			}
		}
	}
	return nil
}

// validateRedirect re-runs scheme + host checks for a redirect destination
// and enforces the redirect hop count cap.
// Returns *FetchError on any violation.
func validateRedirect(next *url.URL, opts Options, fopts FetchOptions, hopCount int) error {
	maxHops := opts.RedirectMaxHops
	if maxHops == 0 {
		maxHops = defaultRedirectMaxHops
	}
	if hopCount > maxHops {
		return &FetchError{
			Category: CategoryBlocked,
			Reason:   fmt.Sprintf("too many redirects: %d hops exceeded limit of %d", hopCount, maxHops),
		}
	}
	if err := validateScheme(next); err != nil {
		return err
	}
	// For redirect validation we use a background context for the DNS lookup
	// since we don't have the per-phase context here.
	if err := validateHost(context.Background(), next, opts, fopts); err != nil {
		return err
	}
	return nil
}

// isPrivateOrLoopback returns true if ip is in any of:
//   - Loopback: 127.0.0.0/8, ::1
//   - Link-local: 169.254.0.0/16 (including AWS metadata 169.254.169.254),
//     fe80::/10
//   - RFC1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
//   - IPv6 ULA: fc00::/7
func isPrivateOrLoopback(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// Check known private CIDR ranges.
	for _, cidr := range privateRanges {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// privateRanges is the deny list from D6 (SSRF guard #2).
var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16", // link-local + AWS metadata endpoint
		"fc00::/7",       // IPv6 ULA
		"fe80::/10",      // IPv6 link-local
		"::1/128",        // IPv6 loopback
		"127.0.0.0/8",    // IPv4 loopback (belt-and-suspenders)
		"100.64.0.0/10",  // Shared address space (RFC 6598)
	}
	for _, c := range cidrs {
		_, network, err := net.ParseCIDR(c)
		if err != nil {
			panic("access: invalid CIDR in privateRanges: " + c)
		}
		privateRanges = append(privateRanges, network)
	}
}
