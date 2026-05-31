package admin

import (
	"encoding/json"
	"net/http"

	"github.com/elymas/universal-search/internal/adapters"
)

// AdaptersHealthResponse is the JSON body of GET /api/admin/adapters/health.
type AdaptersHealthResponse struct {
	// Adapters is the per-adapter health array.
	Adapters []adapters.AdapterHealth `json:"adapters"`
}

// AdaptersHealthHandler serves GET /api/admin/adapters/health, a readiness
// surface for adapter reliability (SPEC-EVAL-002 REQ-EVAL2-010b). It reuses the
// existing admin mux + LoopbackOnly middleware — NO new server or port.
//
// Responds 200 when all adapters are healthy or degraded, 503 when any adapter
// is unhealthy (a readiness probe can gate on this).
//
// @MX:ANCHOR: [AUTO] Admin health HTTP boundary; callers: admin route group, tests, readiness probes
// @MX:REASON: fan_in >= 3; loopback-gated readiness surface. The 503-on-unhealthy contract is consumed by external probes.
// @MX:SPEC: SPEC-EVAL-002 REQ-EVAL2-010
type AdaptersHealthHandler struct {
	reg *adapters.Registry
}

// NewAdaptersHealthHandler constructs an AdaptersHealthHandler.
func NewAdaptersHealthHandler(reg *adapters.Registry) *AdaptersHealthHandler {
	return &AdaptersHealthHandler{reg: reg}
}

// ServeHTTP handles GET /api/admin/adapters/health.
func (h *AdaptersHealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	health := h.reg.HealthSnapshot()

	statusCode := http.StatusOK
	for _, a := range health {
		if a.Status == "unhealthy" {
			statusCode = http.StatusServiceUnavailable
			break
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(AdaptersHealthResponse{Adapters: health})
}
