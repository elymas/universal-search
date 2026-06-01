package sse_test

// Coverage for Writer.Close (a documented no-op kept for interface symmetry).
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"net/http/httptest"
	"testing"

	"github.com/elymas/universal-search/internal/sse"
)

func TestWriterClose(t *testing.T) {
	w := sse.NewWriter(httptest.NewRecorder())
	if err := w.Close(); err != nil {
		t.Errorf("Close() = %v, want nil (no-op)", err)
	}
}
