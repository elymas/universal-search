package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHumanProgress_Emit(t *testing.T) {
	var buf bytes.Buffer
	p := &humanProgress{w: &buf}
	p.Emit("router", "classified")
	got := buf.String()
	if !strings.Contains(got, "[router]") {
		t.Errorf("humanProgress.Emit did not contain [router]: %q", got)
	}
	if !strings.Contains(got, "classified") {
		t.Errorf("humanProgress.Emit did not contain 'classified': %q", got)
	}
}

func TestJsonProgress_EmitIsNoop(t *testing.T) {
	// jsonProgress.Emit must not panic and produces no output.
	p := &jsonProgress{}
	// Should not panic
	p.Emit("stage", "message")
}

func TestProgressEmitterInterface(t *testing.T) {
	// Verify both types satisfy the interface.
	var _ progressEmitter = &humanProgress{w: nil}
	var _ progressEmitter = &jsonProgress{}
}
