// Package main — progress emitter interface and implementations.
//
// REQ-CLI-006: All progress output goes to stderr ONLY; stdout is reserved for payload.
//
// @MX:NOTE: [AUTO] progressEmitter is the interface boundary between text and
// JSON progress modes; all CLI progress output must go through this interface.
// @MX:SPEC: SPEC-CLI-001
package main

import (
	"fmt"
	"io"
)

// progressEmitter is the interface for emitting human-readable progress to stderr.
// Implementations must never write to stdout.
type progressEmitter interface {
	// Emit writes a progress message. Format is implementation-specific.
	Emit(stage, msg string)
}

// humanProgress writes plain-text progress lines to a stderr writer.
// Used by --format text mode.
type humanProgress struct {
	w io.Writer
}

// Emit writes a formatted "[stage] msg\n" line to stderr.
func (h *humanProgress) Emit(stage, msg string) {
	fmt.Fprintf(h.w, "[%s] %s\n", stage, msg)
}

// jsonProgress is a no-op emitter used in --format json mode.
// The obs.Logger already emits structured slog JSON to stderr in that mode.
type jsonProgress struct{}

// Emit is a deliberate no-op. Structured logging handles progress in JSON mode.
func (j *jsonProgress) Emit(_, _ string) {}
