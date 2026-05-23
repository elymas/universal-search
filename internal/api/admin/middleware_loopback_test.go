package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubHandler is a trivial handler that returns 200 OK with body "ok".
var stubHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
})

func TestLoopbackOnly(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		wantCode   int
	}{
		{
			name:       "loopback IPv4 passes through",
			remoteAddr: "127.0.0.1:12345",
			wantCode:   http.StatusOK,
		},
		{
			name:       "loopback IPv6 passes through",
			remoteAddr: "[::1]:12345",
			wantCode:   http.StatusOK,
		},
		{
			name:       "external IPv4 gets 403",
			remoteAddr: "192.168.1.42:12345",
			wantCode:   http.StatusForbidden,
		},
		{
			name:       "external IPv6 gets 403",
			remoteAddr: "[2001:db8::1]:12345",
			wantCode:   http.StatusForbidden,
		},
		{
			name:       "X-Forwarded-For spoof from external IP still 403",
			remoteAddr: "192.168.1.42:12345",
			headers:    map[string]string{"X-Forwarded-For": "127.0.0.1"},
			wantCode:   http.StatusForbidden,
		},
		{
			name:       "X-Real-IP spoof from external IP still 403",
			remoteAddr: "10.0.0.5:12345",
			headers:    map[string]string{"X-Real-IP": "::1"},
			wantCode:   http.StatusForbidden,
		},
		{
			name:       "Forwarded header spoof from external IP still 403",
			remoteAddr: "172.16.0.1:54321",
			headers:    map[string]string{"Forwarded": "for=127.0.0.1"},
			wantCode:   http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			handler := LoopbackOnly(stubHandler)
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Errorf("LoopbackOnly: remoteAddr=%q headers=%v: got status %d, want %d",
					tc.remoteAddr, tc.headers, rec.Code, tc.wantCode)
			}

			// Security: 403 body must NOT contain version, hostname, stack trace, or internal paths.
			if tc.wantCode == http.StatusForbidden {
				body := rec.Body.String()
				for _, forbidden := range []string{"version", "hostname", "stack", "trace", "/internal/", "/cmd/"} {
					if body != "" && containsInsensitive(body, forbidden) {
						t.Errorf("403 body contains forbidden substring %q: %s", forbidden, body)
					}
				}
			}
		})
	}
}

// containsInsensitive reports whether s contains substr, case-insensitive.
func containsInsensitive(s, substr string) bool {
	// Simple lowercase check — sufficient for our short forbidden tokens.
	sLower := toLower(s)
	subLower := toLower(substr)
	return contains(sLower, subLower)
}

func toLower(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b = append(b, c)
	}
	return string(b)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
