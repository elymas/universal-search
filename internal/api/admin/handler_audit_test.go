package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockAuditQuerier implements AuditQuerier for testing.
type mockAuditQuerier struct {
	entries []AuditEntry
	nextCur string
	err     error
}

func (m *mockAuditQuerier) QueryEntries(_ context.Context, limit, offset int, errorsOnly bool, cursor string) ([]AuditEntry, string, error) {
	if m.err != nil {
		return nil, "", m.err
	}

	// Simple mock: return all entries up to limit from offset.
	if offset >= len(m.entries) {
		return []AuditEntry{}, "", nil
	}

	end := offset + limit
	if end > len(m.entries) {
		end = len(m.entries)
	}

	result := m.entries[offset:end]

	if errorsOnly {
		var filtered []AuditEntry
		for _, e := range result {
			if e.Error {
				filtered = append(filtered, e)
			}
		}
		if filtered == nil {
			filtered = []AuditEntry{}
		}
		return filtered, m.nextCur, nil
	}

	return result, m.nextCur, nil
}

func TestHandleAuditQueries(t *testing.T) {
	t.Parallel()

	// Build sample entries for testing.
	now := time.Now().UTC().Truncate(time.Millisecond)
	entries := make([]AuditEntry, 55)
	for i := range entries {
		entries[i] = AuditEntry{
			ID:        i + 1,
			Timestamp: now.Add(time.Duration(i) * time.Second),
			LatencyMs: 100 + i,
			Tokens:    50 + i,
			Sources:   []string{"reddit", "hackernews"},
			Error:     i%10 == 0, // every 10th entry is an error
		}
	}

	t.Run("default pagination returns up to 50 entries", func(t *testing.T) {
		t.Parallel()
		querier := &mockAuditQuerier{entries: entries}
		handler := LoopbackOnly(NewAuditHandler(querier))

		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/queries", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Errorf("Content-Type: got %q, want application/json", ct)
		}

		var result []AuditEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if got := len(result); got != 50 {
			t.Errorf("got %d entries, want 50", got)
		}
	})

	t.Run("errors_only=true returns only error entries", func(t *testing.T) {
		t.Parallel()
		querier := &mockAuditQuerier{entries: entries}
		handler := LoopbackOnly(NewAuditHandler(querier))

		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/queries?errors_only=true", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200", rec.Code)
		}

		var result []AuditEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}

		for _, e := range result {
			if !e.Error {
				t.Errorf("got non-error entry with ID %d when errors_only=true", e.ID)
			}
		}
	})

	t.Run("empty audit log returns empty array", func(t *testing.T) {
		t.Parallel()
		querier := &mockAuditQuerier{entries: nil}
		handler := LoopbackOnly(NewAuditHandler(querier))

		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/queries", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200", rec.Code)
		}

		body := strings.TrimSpace(rec.Body.String())
		if body != "[]" {
			t.Errorf("got %q, want []", body)
		}
	})

	t.Run("limit and offset navigation works", func(t *testing.T) {
		t.Parallel()
		querier := &mockAuditQuerier{entries: entries}
		handler := LoopbackOnly(NewAuditHandler(querier))

		// Request offset=50, limit=10 to get entries 50-59 (the last 5).
		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/queries?limit=10&offset=50", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200", rec.Code)
		}

		var result []AuditEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if got := len(result); got != 5 {
			t.Errorf("got %d entries with offset=50 limit=10, want 5", got)
		}
	})

	t.Run("invalid cursor returns 400", func(t *testing.T) {
		t.Parallel()
		querier := &mockAuditQuerier{entries: entries}
		handler := LoopbackOnly(NewAuditHandler(querier))

		req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/queries?cursor=invalid!cursor", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want 400; body: %s", rec.Code, rec.Body.String())
		}

		var errResp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if errResp["error"] != "invalid_cursor" {
			t.Errorf("error: got %q, want %q", errResp["error"], "invalid_cursor")
		}
	})
}
