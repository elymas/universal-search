package admin

// Edge-case coverage for the adapter admin handlers: wrong HTTP method and
// missing path id. These are the request-validation branches that the
// happy-path tests in handler_adapters_test.go do not reach.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
)

func TestAdaptersHandler_MethodNotAllowed(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	h := NewAdaptersHandler(reg)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/adapters", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if body["error"] != "method not allowed" {
		t.Errorf("error = %q, want 'method not allowed'", body["error"])
	}
}

func TestResyncHandler_MethodNotAllowed(t *testing.T) {
	h := NewResyncHandler(adapters.NewRegistry(nil))
	req := httptest.NewRequest(http.MethodGet, "/api/admin/adapters/x/resync", nil)
	req.SetPathValue("id", "x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", rec.Code)
	}
}

func TestResyncHandler_MissingID(t *testing.T) {
	h := NewResyncHandler(adapters.NewRegistry(nil))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/adapters//resync", nil)
	// PathValue("id") is "" because we never set it.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if body["error"] != "missing adapter id" {
		t.Errorf("error = %q, want 'missing adapter id'", body["error"])
	}
}

func TestToggleHandler_MethodNotAllowed(t *testing.T) {
	h := NewToggleHandler(adapters.NewRegistry(nil))
	req := httptest.NewRequest(http.MethodGet, "/api/admin/adapters/x/toggle", nil)
	req.SetPathValue("id", "x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", rec.Code)
	}
}

func TestToggleHandler_MissingID(t *testing.T) {
	h := NewToggleHandler(adapters.NewRegistry(nil))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/adapters//toggle", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain message", "connection refused", "connection refused"},
		{"strips stack trace", "boom\ngoroutine 1 [running]:", "boom"},
		{"strips goroutine inline", "fail goroutine 5 stack", "fail"},
		{"strips user path", "error at /Users/dev/secret/file.go:42", "error at"},
		{"strips home path", "error at /home/dev/x.go", "error at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeErrorMessage(tt.in)
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("sanitizeErrorMessage(%q) = %q, want prefix %q", tt.in, got, tt.want)
			}
			if strings.Contains(got, "/Users/") || strings.Contains(got, "/home/") || strings.Contains(got, "goroutine ") {
				t.Errorf("sanitized output still leaks internals: %q", got)
			}
		})
	}
}
