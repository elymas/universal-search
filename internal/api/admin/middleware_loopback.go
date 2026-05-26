// Package admin provides HTTP handlers and middleware for the /api/admin/
// endpoint group. All admin endpoints are protected by loopback-only access
// control (LoopbackOnly middleware).
package admin

import (
	"net"
	"net/http"
)

// LoopbackOnly wraps h, rejecting any request whose RemoteAddr is not a
// loopback address (127.0.0.1 or ::1). It checks ONLY the transport-layer
// RemoteAddr — it does NOT read X-Forwarded-For, X-Real-IP, Forwarded, or
// any other client-settable IP-claim header.
//
// @MX:ANCHOR: [AUTO] Loopback middleware; callers: admin route group, tests
// @MX:REASON: fan_in >= 3; sole security gate for all admin endpoints. Bypass
// would expose adapter secrets and admin actions to the network.
// @MX:SPEC: SPEC-UI-002 REQ-LH-001
func LoopbackOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			// Malformed RemoteAddr — reject.
			forbidden(w)
			return
		}
		ip := net.ParseIP(host)
		if ip == nil {
			forbidden(w)
			return
		}
		if !ip.IsLoopback() {
			forbidden(w)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// forbidden writes a minimal 403 response. The body intentionally contains no
// version, hostname, stack trace, or internal path information.
func forbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte("forbidden"))
}
