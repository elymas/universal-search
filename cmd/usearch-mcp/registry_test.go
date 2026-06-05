package main

import (
	"testing"
)

// TestFacebookNotRegisteredInProductionRegistry verifies no "facebook" adapter
// is registered in the production registry (REQ-ADP10-008).
func TestFacebookNotRegisteredInProductionRegistry(t *testing.T) {
	reg := buildProductionRegistry()
	for _, name := range reg.List() {
		if name == "facebook" {
			t.Error("production registry contains 'facebook' adapter, expected none")
		}
	}
}

// TestThreadsNotRegisteredWithoutToken verifies Threads is not registered when
// THREADS_ACCESS_TOKEN is absent (REQ-ADP10-001).
func TestThreadsNotRegisteredWithoutToken(t *testing.T) {
	// The test environment does not set THREADS_ACCESS_TOKEN.
	reg := buildProductionRegistry()
	for _, name := range reg.List() {
		if name == "threads" {
			t.Error("production registry contains 'threads' without token, expected none")
		}
	}
}
