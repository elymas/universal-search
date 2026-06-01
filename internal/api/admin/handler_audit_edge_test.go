package admin

// Edge-case coverage for the audit query handler: wrong HTTP method, malformed
// limit/offset query parameters, and the querier-error path.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuditHandler_MethodNotAllowed(t *testing.T) {
	h := NewAuditHandler(&mockAuditQuerier{})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/audit/queries", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", rec.Code)
	}
}

func TestAuditHandler_InvalidLimit(t *testing.T) {
	h := NewAuditHandler(&mockAuditQuerier{})
	for _, bad := range []string{"abc", "-1"} {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/queries?limit="+bad, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("limit=%q: got %d, want 400", bad, rec.Code)
		}
		var body map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["error"] != "invalid_limit" {
			t.Errorf("limit=%q: error = %q, want invalid_limit", bad, body["error"])
		}
	}
}

func TestAuditHandler_InvalidOffset(t *testing.T) {
	h := NewAuditHandler(&mockAuditQuerier{})
	for _, bad := range []string{"xyz", "-5"} {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/queries?offset="+bad, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("offset=%q: got %d, want 400", bad, rec.Code)
		}
		var body map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["error"] != "invalid_offset" {
			t.Errorf("offset=%q: error = %q, want invalid_offset", bad, body["error"])
		}
	}
}

func TestAuditHandler_QuerierError(t *testing.T) {
	h := NewAuditHandler(&mockAuditQuerier{err: errors.New("db down")})
	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/queries", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "internal_error" {
		t.Errorf("error = %q, want internal_error", body["error"])
	}
	// The detail must not leak the underlying error text.
	if body["detail"] == "db down" {
		t.Error("internal error detail must be sanitized, not raw error text")
	}
}

func TestAuditHandler_ErrorsOnlyAndValidLimitOffset(t *testing.T) {
	// Exercises the successful happy path with explicit valid limit/offset and
	// errors_only=1 (the alternate truthy form) to cover those parse branches.
	q := &mockAuditQuerier{entries: []AuditEntry{
		{Error: true},
		{Error: false},
	}}
	h := NewAuditHandler(q)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/queries?limit=10&offset=0&errors_only=1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	// Response is a bare JSON array; errors_only=1 must filter to the one error entry.
	var entries []AuditEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("response not a JSON array: %v", err)
	}
	if len(entries) != 1 || !entries[0].Error {
		t.Errorf("errors_only=1 should return the single error entry, got %+v", entries)
	}
}
