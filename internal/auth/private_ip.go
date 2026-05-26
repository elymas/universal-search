package auth

import (
	"fmt"
	"net"
	"net/netip"
)

// checkPrivateIP verifies that the hostname does not resolve to a private IP range.
// REQ-AUTH1-011: RFC 1918 / fc00::/7 / loopback / link-local block.
func checkPrivateIP(hostname string) error {
	// Skip check for empty hostname (should not happen after URL parse)
	if hostname == "" {
		return nil
	}

	// Try to parse as IP directly first
	ip := net.ParseIP(hostname)
	if ip == nil {
		// It's a hostname, resolve it
		addrs, err := net.LookupHost(hostname)
		if err != nil {
			// DNS resolution failure is not a private IP error
			// Let discovery proceed; the actual connection will fail if unreachable
			return nil
		}
		if len(addrs) == 0 {
			return nil
		}
		ip = net.ParseIP(addrs[0])
		if ip == nil {
			return nil
		}
	}

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return nil
	}

	if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return fmt.Errorf("auth: issuer host %q resolves to private IP range; set auth.oidc.allow_private_issuer=true for dev/CI", hostname)
	}

	return nil
}

// isPrivateIP reports whether the given IP string is in a private/reserved range.
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	return addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast()
}
