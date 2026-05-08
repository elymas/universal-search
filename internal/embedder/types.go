// Package embedder provides the Go-side HTTP client for the BGE-M3 embedding sidecar.
//
// SPEC-IDX-002: Go value types, error sentinels, HTTP client with retry + observability.
package embedder

import "errors"

// Request is the Go representation of the Python sidecar's EmbedRequest.
type Request struct {
	RequestID      string   `json:"request_id"`
	Texts          []string `json:"texts"`
	ReturnDense    bool     `json:"return_dense"`
	ReturnSparse   bool     `json:"return_sparse"`
	ReturnColbert  bool     `json:"return_colbert_vecs"`
	BatchSize      int      `json:"batch_size"`
}

// Response is the Go representation of the Python sidecar's EmbedResponse.
type Response struct {
	RequestID    string               `json:"request_id"`
	Dense        [][]float64          `json:"dense"`
	Sparse       []map[string]float64 `json:"sparse"`
	Colbert      [][][]float64        `json:"colbert"`
	Model        string               `json:"model"`
	ModelVersion string               `json:"model_version"`
	Device       string               `json:"device"`
	LatencyMs    float64              `json:"latency_ms"`
	CacheHits    int                  `json:"cache_hits"`
	CacheMisses  int                  `json:"cache_misses"`
}

// Error sentinels — callers use errors.Is to distinguish error categories.
var (
	// ErrInvalidRequest is returned when the sidecar returns HTTP 4xx.
	ErrInvalidRequest = errors.New("embedder: invalid request")
	// ErrSidecarUnreachable is returned on connection-level errors after retries.
	ErrSidecarUnreachable = errors.New("embedder: sidecar unreachable")
	// ErrTimeout is returned when the context deadline is exceeded.
	ErrTimeout = errors.New("embedder: request timed out")
	// ErrModelLoadFailed is returned when the sidecar is still loading the model.
	ErrModelLoadFailed = errors.New("embedder: model still loading")
	// ErrOutOfMemory is returned when the sidecar reports OOM (HTTP 500 + error=oom).
	ErrOutOfMemory = errors.New("embedder: inference out of memory")
)

// ModeLabel derives the Prometheus `mode` label from request flags.
// Values: "dense", "sparse", "colbert", "all" (when >= 2 modes are requested).
//
// @MX:NOTE: [AUTO] Called from client.go emitObs; used in metric label derivation.
func ModeLabel(req Request) string {
	count := 0
	last := ""
	if req.ReturnDense {
		count++
		last = "dense"
	}
	if req.ReturnSparse {
		count++
		last = "sparse"
	}
	if req.ReturnColbert {
		count++
		last = "colbert"
	}
	if count >= 2 {
		return "all"
	}
	if count == 1 {
		return last
	}
	return "dense" // default fallback (validation rejects this case before reaching client)
}
