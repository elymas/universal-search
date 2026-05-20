// Package access — pinned-IP dialer for DNS-rebind mitigation.
//
// REQ-CACHE-013 guard #3: The hostname is resolved ONCE and all subsequent
// TCP connections are made directly to the pinned IP, preventing DNS rebinding.
package access

import (
	"context"
	"fmt"
	"net"
	"time"
)

// pinnedDialContext resolves hostname once and returns a DialContext function
// that forces all subsequent TCP connections to the pinned IP.
//
// This prevents DNS-rebinding attacks where a malicious DNS server changes
// the IP between resolution and connection.
//
// When AllowPrivateNetworks is true (test mode), resolution is skipped and a
// plain DialContext using net.Dialer is returned.
//
// @MX:WARN: [AUTO] DNS-rebind mitigation — do NOT remove the pinned resolution.
// @MX:REASON: Removing the pin allows DNS rebinding: a malicious host resolves
// to a public IP on first lookup and 127.0.0.1 on TCP connection, bypassing
// the SSRF validateHost guard.
// @MX:SPEC: SPEC-CACHE-001
func pinnedDialContext(
	ctx context.Context,
	hostname string,
	opts Options,
	fopts FetchOptions,
) (func(context.Context, string, string) (net.Conn, error), error) {
	// Skip DNS pinning when private networks are explicitly allowed (test mode).
	if opts.AllowPrivateNetworks || fopts.AllowPrivateNetworks {
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

	// Check the resolved IP against the private-network deny list.
	pinnedIP := ips[0].IP
	if isPrivateOrLoopback(pinnedIP) {
		return nil, &FetchError{
			Category: CategoryBlocked,
			Reason:   fmt.Sprintf("private/loopback IP %s resolved for %q", pinnedIP, hostname),
		}
	}

	return dialContextWithPinnedIP(pinnedIP.String()), nil
}

// dialContextWithPinnedIP returns a DialContext function that forces all TCP
// connections to the given pinnedAddr (IP string), bypassing DNS.
// This function is used by Phase 3 and Phase 4 HTTP transports.
func dialContextWithPinnedIP(pinnedIP string) func(context.Context, string, string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 10 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Replace the hostname portion of addr with the pinned IP.
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		return d.DialContext(ctx, network, net.JoinHostPort(pinnedIP, port))
	}
}
