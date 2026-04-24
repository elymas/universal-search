// Package reqid provides request-ID generation and context propagation.
// Request IDs are ULID strings (26-char Crockford Base32) generated per
// inbound request. They flow through context.Context and are injected into
// all outbound HTTP headers and slog records.
//
// REQ-OBS-002: Every HTTP or gRPC request SHALL carry an X-Request-ID value
// bound to context and propagated on all downstream calls.
package reqid

import (
	"context"
	"crypto/rand"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
)

// contextKey is an unexported type for keys stored in context.Context to
// prevent collision with other packages.
type contextKey struct{}

// New generates a new monotonic ULID using a cryptographically random entropy
// source. Returns a 26-character Crockford Base32 string.
// @MX:ANCHOR: [AUTO] Central ID generator; callers: reqid.Middleware, reqid_test, obs.Init
// @MX:REASON: fan_in >= 3; format change here affects all request tracing
func New() string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

// WithContext returns a derived context carrying the given request ID.
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext extracts the request ID from ctx. Returns an empty string if
// no ID has been bound.
func FromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKey{}).(string)
	return v
}

// Middleware is an HTTP ingress handler that reads or generates an X-Request-ID,
// binds it to the request context, and writes it to the response header.
// @MX:ANCHOR: [AUTO] HTTP ingress boundary; callers: cmd/usearch-api, obs.Init, tests
// @MX:REASON: fan_in >= 3; must maintain X-Request-ID contract for all ingress traffic
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = New()
		}
		ctx := WithContext(r.Context(), id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// transport is an http.RoundTripper wrapper that injects X-Request-ID from
// context onto outbound requests.
type transport struct {
	next http.RoundTripper
}

// NewTransport wraps an existing http.RoundTripper with request-ID egress
// injection. If the context carries a request ID it is written to the
// outbound X-Request-ID header.
func NewTransport(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &transport{next: next}
}

// RoundTrip injects X-Request-ID from context before delegating to the
// wrapped transport.
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	id := FromContext(req.Context())
	if id != "" {
		// Clone the request to avoid mutating the caller's headers.
		clone := req.Clone(req.Context())
		clone.Header.Set("X-Request-ID", id)
		req = clone
	}
	return t.next.RoundTrip(req)
}
