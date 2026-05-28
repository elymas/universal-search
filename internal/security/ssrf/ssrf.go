// Package ssrf provides generic SSRF (Server-Side Request Forgery) protection.
//
// REQ-SEC-007: Extracted from SPEC-CACHE-001 internal/access/ssrf.go.
// Provides ValidateScheme, ValidateHost, ValidateRedirect, and PinnedIPDialer
// as reusable guards for access fetching, OIDC discovery, and future adapters.
//
// @MX:WARN: [AUTO] SSRF guard package — security-critical, do not weaken.
// @MX:REASON: Weakening any guard (scheme allowlist, private-IP deny, DNS-rebind pin,
// hostname blocklist) enables SSRF attacks including cloud metadata credential theft.
// @MX:SPEC: SPEC-SEC-001
package ssrf

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// ErrorCategory classifies a FetchError so callers can decide handling strategy.
type ErrorCategory string

const (
	// CategoryBlocked means the URL was blocked by an SSRF guard.
	CategoryBlocked ErrorCategory = "blocked"
)

// FetchError carries structured error information from SSRF validation.
type FetchError struct {
	Category ErrorCategory
	Reason   string
	Cause    error
}

func (e *FetchError) Error() string {
	if e.Cause != nil {
		return string(e.Category) + ": " + e.Reason + ": " + e.Cause.Error()
	}
	return string(e.Category) + ": " + e.Reason
}

func (e *FetchError) Unwrap() error { return e.Cause }

// Options holds SSRF guard configuration.
//
// REQ-SEC-007: Options struct with AllowPrivateNetworks, MaxRedirects,
// HostnameBlocklist, SchemeAllowlist.
type Options struct {
	// AllowPrivateNetworks permits fetching RFC1918/loopback addresses.
	// MUST be false in production; true only for test mode.
	AllowPrivateNetworks bool

	// MaxRedirects caps redirect chains. Zero defaults to 5.
	MaxRedirects int

	// HostnameBlocklist is the list of blocked hostnames (cloud metadata endpoints).
	// Nil defaults to DefaultHostnameBlocklist().
	HostnameBlocklist []string

	// SchemeAllowlist is the list of allowed URL schemes.
	// Nil defaults to DefaultSchemeAllowlist().
	SchemeAllowlist []string
}

// DefaultRedirectMaxHops is the default maximum redirect chain depth.
const DefaultRedirectMaxHops = 5

// DefaultHostnameBlocklist returns the cloud metadata hostname blocklist per D3.
//
// REQ-SEC-008: Default blocklist includes GCP, Azure, and EC2 metadata hostnames.
func DefaultHostnameBlocklist() []string {
	return []string{
		"metadata.google.internal",
		"metadata.azure.com",
		"instance-data.ec2.internal",
	}
}

// DefaultSchemeAllowlist returns the default allowed URL schemes.
//
// REQ-SEC-007: Only http and https are allowed by default.
func DefaultSchemeAllowlist() []string {
	return []string{"http", "https"}
}

// ValidateScheme checks that the URL scheme is in the allowlist.
// Returns *FetchError{CategoryBlocked} on violation.
//
// REQ-SEC-007: Scheme allowlist (http/https only).
// @MX:ANCHOR: [AUTO] Scheme validation — called from all SSRF guard sites.
// @MX:REASON: Blocking non-HTTP schemes prevents file://, ftp://, gopher:// SSRF.
func ValidateScheme(u *url.URL) error {
	return ValidateSchemeWithAllowlist(u, nil)
}

// ValidateSchemeWithAllowlist checks scheme against a custom allowlist.
func ValidateSchemeWithAllowlist(u *url.URL, allowlist []string) error {
	if len(allowlist) == 0 {
		allowlist = DefaultSchemeAllowlist()
	}
	for _, s := range allowlist {
		if u.Scheme == s {
			return nil
		}
	}
	return &FetchError{
		Category: CategoryBlocked,
		Reason:   fmt.Sprintf("scheme %q not allowed", u.Scheme),
	}
}

// ValidateHost resolves the hostname and checks that no resolved IP is a
// private/loopback address (unless AllowPrivateNetworks is set), AND checks
// the hostname against the blocklist.
//
// REQ-SEC-007: Private IP deny-by-default.
// REQ-SEC-008: Hostname blocklist for cloud metadata endpoints.
// @MX:ANCHOR: [AUTO] Host validation — high fan_in (access, auth, adapters).
// @MX:REASON: Primary SSRF guard; prevents access to internal networks and cloud metadata.
func ValidateHost(ctx context.Context, u *url.URL, opts Options) error {
	if opts.AllowPrivateNetworks {
		return nil
	}

	host := u.Hostname()

	// Guard 1: Check hostname against blocklist (before DNS resolution).
	if err := validateHostname(host, opts.HostnameBlocklist); err != nil {
		return err
	}

	// Guard 2: If the hostname is already an IP, check it directly.
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrLoopback(ip) {
			return &FetchError{
				Category: CategoryBlocked,
				Reason:   fmt.Sprintf("private/loopback IP %s for %q", ip, host),
			}
		}
		return nil
	}

	// Guard 3: DNS resolution + private IP check.
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
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

// validateHostname checks if the hostname matches any entry in the blocklist.
// Matching is case-insensitive and supports exact and suffix (wildcard) matches.
//
// REQ-SEC-008: Cloud metadata hostname blocking.
func validateHostname(host string, blocklist []string) error {
	if len(blocklist) == 0 {
		blocklist = DefaultHostnameBlocklist()
	}

	lowerHost := strings.ToLower(host)
	for _, blocked := range blocklist {
		lowerBlocked := strings.ToLower(blocked)
		// Exact match
		if lowerHost == lowerBlocked {
			return &FetchError{
				Category: CategoryBlocked,
				Reason:   fmt.Sprintf("hostname blocked: %s", host),
			}
		}
		// Suffix match: host "something.metadata.google.internal" matches "metadata.google.internal"
		if strings.HasSuffix(lowerHost, "."+lowerBlocked) {
			return &FetchError{
				Category: CategoryBlocked,
				Reason:   fmt.Sprintf("hostname blocked (suffix match): %s matches %s", host, blocked),
			}
		}
	}
	return nil
}

// ValidateRedirect re-runs scheme + host checks for a redirect destination
// and enforces the redirect hop count cap.
//
// REQ-SEC-007: Redirect re-validation with hop cap.
func ValidateRedirect(next *url.URL, opts Options, hopCount int) error {
	maxHops := opts.MaxRedirects
	if maxHops == 0 {
		maxHops = DefaultRedirectMaxHops
	}
	if hopCount > maxHops {
		return &FetchError{
			Category: CategoryBlocked,
			Reason:   fmt.Sprintf("too many redirects: %d hops exceeded limit of %d", hopCount, maxHops),
		}
	}
	if err := ValidateSchemeWithAllowlist(next, opts.SchemeAllowlist); err != nil {
		return err
	}
	if err := ValidateHost(context.Background(), next, opts); err != nil {
		return err
	}
	return nil
}

// isPrivateOrLoopback returns true if ip is in any blocked range.
// Covers: loopback, link-local, RFC1918, IPv6 ULA, IPv6 link-local, shared address space.
func isPrivateOrLoopback(ip net.IP) bool {
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

// IsPrivateOrLoopback is the exported version for use by other packages.
func IsPrivateOrLoopback(ip net.IP) bool {
	return isPrivateOrLoopback(ip)
}

// privateRanges is the deny list from D3 (SSRF guard #2).
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
		"127.0.0.0/8",    // IPv4 loopback
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

// PinnedIPDialer resolves hostname once and returns a DialContext function
// that forces all subsequent TCP connections to the pinned IP.
//
// @MX:WARN: [AUTO] DNS-rebind mitigation — do NOT remove the pinned resolution.
// @MX:REASON: Removing the pin allows DNS rebinding: a malicious host resolves
// to a public IP on first lookup and 127.0.0.1 on TCP connection.
// @MX:SPEC: SPEC-SEC-001
func PinnedIPDialer(ctx context.Context, hostname string, opts Options) (func(context.Context, string, string) (net.Conn, error), error) {
	if opts.AllowPrivateNetworks {
		d := &net.Dialer{Timeout: 10 * time.Second}
		return d.DialContext, nil
	}

	resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIPAddr(resolveCtx, hostname)
	if err != nil {
		return nil, &FetchError{
			Category: CategoryBlocked,
			Reason:   fmt.Sprintf("DNS lookup failed for %q: %v", hostname, err),
			Cause:    err,
		}
	}
	if len(ips) == 0 {
		return nil, &FetchError{
			Category: CategoryBlocked,
			Reason:   fmt.Sprintf("no IP address resolved for %q", hostname),
		}
	}

	pinnedIP := ips[0].IP
	if isPrivateOrLoopback(pinnedIP) {
		return nil, &FetchError{
			Category: CategoryBlocked,
			Reason:   fmt.Sprintf("private/loopback IP %s resolved for %q", pinnedIP, hostname),
		}
	}

	return DialContextWithPinnedIP(pinnedIP.String()), nil
}

// DialContextWithPinnedIP returns a DialContext function that forces all TCP
// connections to the given pinned IP, bypassing DNS.
func DialContextWithPinnedIP(pinnedIP string) func(context.Context, string, string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 10 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		return d.DialContext(ctx, network, net.JoinHostPort(pinnedIP, port))
	}
}
