package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/pkg/types"
)

// TestLoopbackRejectionOnAllAdminEndpoints verifies that every admin endpoint
// rejects requests from non-loopback IPs. This is a regression test bundle
// covering REQ-LH-001.
//
// When a new admin endpoint is added, it MUST be added to this test's
// endpoint list to ensure loopback enforcement is not accidentally bypassed.
func TestLoopbackRejectionOnAllAdminEndpoints(t *testing.T) {
	t.Parallel()

	reg := adapters.NewRegistry(nil)
	a := &testAdapter{name: "test", caps: types.Capabilities{SourceID: "test"}}
	_ = reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true})

	querier := &mockAuditQuerier{}

	// Define all admin endpoints. Every new admin endpoint MUST be added here.
	endpoints := []struct {
		name    string
		method  string
		path    string
		handler http.Handler
	}{
		{
			name:    "GET /api/admin/adapters",
			method:  http.MethodGet,
			path:    "/api/admin/adapters",
			handler: LoopbackOnly(NewAdaptersHandler(reg)),
		},
		{
			name:    "POST /api/admin/adapters/{id}/resync",
			method:  http.MethodPost,
			path:    "/api/admin/adapters/test/resync",
			handler: LoopbackOnly(NewResyncHandler(reg)),
		},
		{
			name:    "POST /api/admin/adapters/{id}/toggle",
			method:  http.MethodPost,
			path:    "/api/admin/adapters/test/toggle",
			handler: LoopbackOnly(NewToggleHandler(reg)),
		},
		{
			name:    "GET /api/admin/audit/queries",
			method:  http.MethodGet,
			path:    "/api/admin/audit/queries",
			handler: LoopbackOnly(NewAuditHandler(querier)),
		},
	}

	externalIPs := []string{
		"192.168.1.42:12345",
		"10.0.0.1:9999",
		"172.16.0.1:54321",
		"8.8.8.8:80",
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()
			for _, remoteAddr := range externalIPs {
				req := httptest.NewRequest(ep.method, ep.path, nil)
				req.RemoteAddr = remoteAddr
				if ep.path == "/api/admin/adapters/test/resync" || ep.path == "/api/admin/adapters/test/toggle" {
					req.SetPathValue("id", "test")
				}

				rec := httptest.NewRecorder()
				ep.handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusForbidden {
					t.Errorf("RemoteAddr=%q: got status %d, want 403", remoteAddr, rec.Code)
				}

				// Verify the forbidden response does not leak JSON or internal info.
				body := rec.Body.String()
				if body != "forbidden" {
					t.Errorf("RemoteAddr=%q: got body %q, want %q", remoteAddr, body, "forbidden")
				}

				// Verify it's not JSON (plain text response for security).
				var js map[string]interface{}
				if err := json.Unmarshal([]byte(body), &js); err == nil {
					t.Errorf("RemoteAddr=%q: forbidden response is JSON, expected plain text", remoteAddr)
				}
			}
		})
	}

	// Verify loopback IPs are accepted on all endpoints.
	t.Run("loopback IPs accepted on all endpoints", func(t *testing.T) {
		t.Parallel()
		for _, ep := range endpoints {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			if ep.path == "/api/admin/adapters/test/resync" || ep.path == "/api/admin/adapters/test/toggle" {
				req.SetPathValue("id", "test")
			}

			rec := httptest.NewRecorder()
			ep.handler.ServeHTTP(rec, req)

			if rec.Code == http.StatusForbidden {
				t.Errorf("%s: loopback IP got 403, expected non-403", ep.name)
			}
		}
	})
}
