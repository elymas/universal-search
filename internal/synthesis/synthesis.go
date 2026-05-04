// Package synthesis provides the Go-side client for the Python researcher sidecar.
//
// SPEC-SYN-001: Basic synthesis v0.
// The synthesis pipeline is: Go HTTP client → Python FastAPI sidecar → LiteLLM proxy → LLM.
// Value-type re-exports allow callers to import only this package.
package synthesis
