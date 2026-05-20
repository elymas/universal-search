// Package embedder is the Go client for the SPEC-IDX-002 BGE-M3 embedding sidecar.
//
// Usage:
//
//	cfg := embedder.ConfigFromEnv()
//	client, err := embedder.New(cfg, obsBundle)
//	resp, err := client.Embed(ctx, embedder.Request{
//	    RequestID: "req-001",
//	    Texts:     []string{"hello", "world"},
//	    ReturnDense: true,
//	})
//
// Re-exports (value types visible to consumers):
//   - Request, Response
//   - Config, ConfigFromEnv
//   - Client, New
//   - ErrInvalidRequest, ErrSidecarUnreachable, ErrTimeout, ErrModelLoadFailed, ErrOutOfMemory
//   - ModeLabel

// All exported symbols are defined in their respective source files:
// types.go, config.go, client.go.
package embedder
