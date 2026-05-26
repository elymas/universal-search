package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// AuditEntry represents a single query audit record for the admin API response.
// Fields are derived from the audit subsystem but shaped for the admin UI.
//
// @MX:SPEC: SPEC-UI-002 REQ-AV-001, REQ-AV-002, REQ-AV-003, REQ-AV-004
type AuditEntry struct {
	ID        int       `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	LatencyMs int       `json:"latency_ms"`
	Tokens    int       `json:"tokens"`
	Sources   []string  `json:"sources"`
	Error     bool      `json:"error"`
}

// AuditQuerier is the read interface for audit query data. The admin handler
// depends on this interface, not on the concrete audit store, following
// dependency inversion.
type AuditQuerier interface {
	// QueryEntries returns audit entries matching the given parameters.
	// Returns the entries, an optional next cursor, and an error.
	QueryEntries(ctx context.Context, limit, offset int, errorsOnly bool, cursor string) ([]AuditEntry, string, error)
}

// AuditHandler handles GET /api/admin/audit/queries.
// @MX:SPEC: SPEC-UI-002 REQ-AV-001, REQ-AV-002, REQ-AV-003, REQ-AV-004
type AuditHandler struct {
	querier AuditQuerier
}

// NewAuditHandler constructs an AuditHandler.
func NewAuditHandler(querier AuditQuerier) *AuditHandler {
	return &AuditHandler{querier: querier}
}

// ServeHTTP handles GET /api/admin/audit/queries.
//
// Query parameters:
//   - limit (default 50): maximum number of entries to return
//   - offset (default 0): number of entries to skip
//   - errors_only (default false): only return entries with errors
//   - cursor (optional): pagination cursor for forward-only navigation
func (h *AuditHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	// Parse limit (default 50).
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":  "invalid_limit",
				"detail": fmt.Sprintf("limit must be a non-negative integer, got %q", v),
			})
			return
		}
		limit = parsed
	}

	// Parse offset (default 0).
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":  "invalid_offset",
				"detail": fmt.Sprintf("offset must be a non-negative integer, got %q", v),
			})
			return
		}
		offset = parsed
	}

	// Parse errors_only (default false).
	errorsOnly := false
	if v := r.URL.Query().Get("errors_only"); v == "true" || v == "1" {
		errorsOnly = true
	}

	// Parse cursor (optional).
	cursor := r.URL.Query().Get("cursor")
	if cursor != "" && !isValidCursor(cursor) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":  "invalid_cursor",
			"detail": fmt.Sprintf("cursor contains invalid characters: %q", cursor),
		})
		return
	}

	// Query the audit store.
	entries, _, err := h.querier.QueryEntries(r.Context(), limit, offset, errorsOnly, cursor)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":  "internal_error",
			"detail": "failed to query audit entries",
		})
		return
	}

	// Ensure nil slice becomes empty array (not null).
	if entries == nil {
		entries = []AuditEntry{}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(entries)
}

// isValidCursor checks whether a cursor string contains only safe characters
// (alphanumeric, dash, underscore, period). This prevents injection through
// cursor values.
func isValidCursor(cursor string) bool {
	for _, c := range cursor {
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z'
		isDigit := c >= '0' && c <= '9'
		isPunct := c == '-' || c == '_' || c == '.'
		if !isLower && !isUpper && !isDigit && !isPunct {
			return false
		}
	}
	return len(cursor) <= 256
}
