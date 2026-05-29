package ssrf

import (
	"context"
	"net"
	"time"
)

// PinnedIPDialer resolves hostname ONCE and returns a DialContext function that
// forces all subsequent TCP connections to the pinned IP, preventing
// DNS-rebinding attacks. Preserves access.pinnedDialContext behavior.
//
// When private networks are allowed (test mode), resolution is skipped and a
// plain net.Dialer DialContext is returned.
//
// @MX:WARN: [AUTO] DNS-rebind mitigation — do NOT remove the pinned resolution.
// @MX:REASON: Removing the pin allows DNS rebinding: a host resolves to a public
// IP on first lookup and 127.0.0.1 on the TCP connection, bypassing ValidateHost.
// @MX:SPEC: SPEC-SEC-001
func PinnedIPDialer(
	ctx context.Context,
	hostname string,
	opts Options,
	fopts FetchOptions,
) (func(context.Context, string, string) (net.Conn, error), error) {
	if allowPrivate(opts, fopts) {
		d := &net.Dialer{Timeout: 10 * time.Second}
		return d.DialContext, nil
	}

	resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIPAddr(resolveCtx, hostname)
	if err != nil {
		return nil, blocked(ReasonPrivateIP, "DNS lookup failed for %q: %v", hostname, err)
	}
	if len(ips) == 0 {
		return nil, blocked(ReasonPrivateIP, "no IP address resolved for %q", hostname)
	}

	pinnedIP := ips[0].IP
	if IsPrivateOrLoopback(pinnedIP) {
		return nil, blocked(ReasonPrivateIP, "private/loopback IP %s resolved for %q", pinnedIP, hostname)
	}

	return DialContextWithPinnedIP(pinnedIP.String()), nil
}

// DialContextWithPinnedIP returns a DialContext function that forces all TCP
// connections to the given pinnedIP, bypassing DNS. Preserves
// access.dialContextWithPinnedIP behavior.
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
