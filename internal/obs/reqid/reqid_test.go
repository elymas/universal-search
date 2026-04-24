// Package reqid_test tests the request-ID propagation utilities (REQ-OBS-002).
package reqid_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/elymas/universal-search/internal/obs/reqid"
)

// ulidPattern matches a valid Crockford Base32 ULID (26 chars).
var ulidPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

// TestRequestIDPropagatesThroughContext verifies that WithRequestID + FromContext
// round-trips correctly, even across goroutine boundaries.
// REQ-OBS-002
func TestRequestIDPropagatesThroughContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = reqid.WithContext(ctx, "TEST-ID-001")

	var wg sync.WaitGroup
	wg.Add(1)
	var got string
	go func() {
		defer wg.Done()
		got = reqid.FromContext(ctx)
	}()
	wg.Wait()

	if got != "TEST-ID-001" {
		t.Errorf("FromContext in goroutine: got %q, want %q", got, "TEST-ID-001")
	}
}

// TestExtractRequestIDReturnsEmptyWhenAbsent ensures FromContext returns ""
// when no ID has been bound.
// REQ-OBS-002
func TestExtractRequestIDReturnsEmptyWhenAbsent(t *testing.T) {
	t.Parallel()

	id := reqid.FromContext(context.Background())
	if id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

// TestWithRequestIDPreservesParentValues ensures parent context values are
// still accessible after WithRequestID.
// REQ-OBS-002
func TestWithRequestIDPreservesParentValues(t *testing.T) {
	t.Parallel()

	type key struct{}
	parent := context.WithValue(context.Background(), key{}, "parent-val")
	ctx := reqid.WithContext(parent, "X-ID")

	if v := ctx.Value(key{}); v != "parent-val" {
		t.Errorf("parent value lost: got %v", v)
	}
	if got := reqid.FromContext(ctx); got != "X-ID" {
		t.Errorf("request ID: got %q, want %q", got, "X-ID")
	}
}

// TestRequestIDGeneratorReturnsULID checks that New() returns a 26-char
// Crockford Base32 string.
// REQ-OBS-002
func TestRequestIDGeneratorReturnsULID(t *testing.T) {
	t.Parallel()

	id := reqid.New()
	if !ulidPattern.MatchString(id) {
		t.Errorf("generated ID %q does not match ULID pattern", id)
	}
	if len(id) != 26 {
		t.Errorf("expected 26 chars, got %d", len(id))
	}
}

// TestHTTPMiddlewareGeneratesRequestIDWhenHeaderAbsent verifies that the
// ingress middleware generates a ULID when X-Request-ID is absent.
// REQ-OBS-002
func TestHTTPMiddlewareGeneratesRequestIDWhenHeaderAbsent(t *testing.T) {
	t.Parallel()

	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = reqid.FromContext(r.Context())
	})

	handler := reqid.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	// Response header must be set.
	responseID := rw.Header().Get("X-Request-ID")
	if !ulidPattern.MatchString(responseID) {
		t.Errorf("response X-Request-ID %q is not a valid ULID", responseID)
	}

	// Context value must match response header.
	if capturedID != responseID {
		t.Errorf("context ID %q != response header %q", capturedID, responseID)
	}
}

// TestHTTPMiddlewarePropagatesExistingXRequestIDHeader verifies that when the
// inbound request carries X-Request-ID the middleware passes it through unchanged.
// REQ-OBS-002
func TestHTTPMiddlewarePropagatesExistingXRequestIDHeader(t *testing.T) {
	t.Parallel()

	const fixedID = "REQ-FIXED-123"
	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = reqid.FromContext(r.Context())
	})

	handler := reqid.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", fixedID)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	if capturedID != fixedID {
		t.Errorf("context ID: got %q, want %q", capturedID, fixedID)
	}
	if rw.Header().Get("X-Request-ID") != fixedID {
		t.Errorf("response header: got %q, want %q", rw.Header().Get("X-Request-ID"), fixedID)
	}
}

// TestEgressTransportWritesRequestIDHeader verifies that the egress Transport
// wrapper copies the request ID from context onto the outbound request header.
// REQ-OBS-002
func TestEgressTransportWritesRequestIDHeader(t *testing.T) {
	t.Parallel()

	var receivedID string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedID = r.Header.Get("X-Request-ID")
	}))
	defer upstream.Close()

	client := &http.Client{
		Transport: reqid.NewTransport(http.DefaultTransport),
	}

	ctx := reqid.WithContext(context.Background(), "EGRESS-42")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream.URL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	resp.Body.Close()

	if receivedID != "EGRESS-42" {
		t.Errorf("upstream received X-Request-ID %q, want %q", receivedID, "EGRESS-42")
	}
}
