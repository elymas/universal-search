// Package access — pinned-IP dialer for DNS-rebind mitigation.
//
// REQ-CACHE-013 guard #3: The hostname is resolved ONCE and all subsequent
// TCP connections are made directly to the pinned IP, preventing DNS rebinding.
//
// SPEC-SEC-001 REQ-SEC-007 (DDD IMPROVE): delegates to the generic
// internal/security/ssrf package; the wrappers preserve the CACHE-001
// call-site signatures and translate the generic error into *FetchError.
package access

import (
	"context"
	"net"

	secssrf "github.com/elymas/universal-search/internal/security/ssrf"
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
	dialFn, err := secssrf.PinnedIPDialer(ctx, hostname, toSecOptions(opts), toSecFetchOptions(fopts))
	if err != nil {
		return nil, fromSecError(err)
	}
	return dialFn, nil
}

// dialContextWithPinnedIP returns a DialContext function that forces all TCP
// connections to the given pinnedAddr (IP string), bypassing DNS.
// This function is used by Phase 3 and Phase 4 HTTP transports.
func dialContextWithPinnedIP(pinnedIP string) func(context.Context, string, string) (net.Conn, error) {
	return secssrf.DialContextWithPinnedIP(pinnedIP)
}
