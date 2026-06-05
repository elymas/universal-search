package main

import (
	"os"
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

// TestXRegistrationGatedByEnv verifies X is only registered when USEARCH_X_ENABLED=true.
// SPEC-ADP-006-XENABLE: NFR-XEN-002.
func TestXRegistrationGatedByEnv(t *testing.T) {
	// Ensure X env var is NOT set in the test environment.
	origX := os.Getenv("USEARCH_X_ENABLED")
	os.Unsetenv("USEARCH_X_ENABLED")
	defer func() {
		if origX != "" {
			os.Setenv("USEARCH_X_ENABLED", origX)
		}
	}()

	reg := buildProductionRegistry()
	for _, name := range reg.List() {
		if name == "x" {
			t.Error("production registry contains 'x' without USEARCH_X_ENABLED, expected none")
		}
	}
}
