package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/elymas/universal-search/internal/adapters"
)

// AdaptersHandler returns a JSON array of AdapterAdminView for all registered
// adapters. It calls Registry.SnapshotForAdmin() which is the canonical source
// of adapter metadata for the admin API.
//
// @MX:NOTE: [AUTO] Response never includes secret values; SnapshotForAdmin
// enforces this at the data layer.
// @MX:SPEC: SPEC-UI-002 REQ-AS-001, REQ-AK-001
type AdaptersHandler struct {
	reg *adapters.Registry
}

// NewAdaptersHandler constructs an AdaptersHandler.
func NewAdaptersHandler(reg *adapters.Registry) *AdaptersHandler {
	return &AdaptersHandler{reg: reg}
}

// ServeHTTP handles GET /api/admin/adapters.
func (h *AdaptersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	views := h.reg.SnapshotForAdmin()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(views)
}

// ResyncHandler triggers a health-check resync on a single adapter.
// @MX:SPEC: SPEC-UI-002 REQ-AS-002
type ResyncHandler struct {
	reg *adapters.Registry
}

// NewResyncHandler constructs a ResyncHandler.
func NewResyncHandler(reg *adapters.Registry) *ResyncHandler {
	return &ResyncHandler{reg: reg}
}

// ServeHTTP handles POST /api/admin/adapters/{id}/resync.
func (h *ResyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing adapter id"})
		return
	}

	view, err := h.reg.Resync(r.Context(), id)
	if err != nil {
		writeAdapterError(w, id, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(view)
}

// ToggleHandler toggles the enabled/disabled state of a single adapter.
// @MX:SPEC: SPEC-UI-002 REQ-AK-002
type ToggleHandler struct {
	reg *adapters.Registry
}

// NewToggleHandler constructs a ToggleHandler.
func NewToggleHandler(reg *adapters.Registry) *ToggleHandler {
	return &ToggleHandler{reg: reg}
}

// ServeHTTP handles POST /api/admin/adapters/{id}/toggle.
func (h *ToggleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing adapter id"})
		return
	}

	view, err := h.reg.ToggleEnabled(r.Context(), id)
	if err != nil {
		writeAdapterError(w, id, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(view)
}

// writeAdapterError writes a structured JSON error response for adapter
// operations. It maps Registry errors to appropriate HTTP status codes and
// sanitizes error messages to prevent leaking stack traces or internal paths.
func writeAdapterError(w http.ResponseWriter, adapterID string, err error) {
	if errors.Is(err, adapters.ErrAdapterNotFound) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":      "adapter_not_found",
			"adapter_id": adapterID,
		})
		return
	}

	var upErr *adapters.UpstreamError
	if errors.As(err, &upErr) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error":      "upstream_adapter_error",
			"adapter_id": adapterID,
			"detail":     sanitizeErrorMessage(upErr.Err.Error()),
		})
		return
	}

	// Generic internal error — sanitize aggressively.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{
		"error":      "internal_error",
		"adapter_id": adapterID,
		"detail":     sanitizeErrorMessage(err.Error()),
	})
}

// sanitizeErrorMessage strips stack traces, internal paths, and hostnames from
// error messages to prevent information leakage in API responses.
func sanitizeErrorMessage(msg string) string {
	// Remove common Go stack trace patterns.
	lines := strings.SplitN(msg, "\n", 2)
	sanitized := lines[0]

	// Strip goroutine stack traces.
	sanitized = strings.SplitN(sanitized, "goroutine ", 2)[0]

	// Strip internal Go paths.
	for _, prefix := range []string{"/usr/local/go/", "/home/", "/Users/"} {
		if idx := strings.Index(sanitized, prefix); idx >= 0 {
			sanitized = sanitized[:idx]
		}
	}

	return strings.TrimSpace(sanitized)
}
