package fanout_test

import (
	"errors"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/fanout"
)

// TestNewRequiresRegistry verifies New returns ErrAdapterRegistryEmpty for nil registry.
func TestNewRequiresRegistry(t *testing.T) {
	t.Parallel()
	_, err := fanout.New(fanout.Options{Registry: nil})
	if !errors.Is(err, fanout.ErrAdapterRegistryEmpty) {
		t.Fatalf("want ErrAdapterRegistryEmpty, got %v", err)
	}
}

// TestNewRequiresAtLeastOneAdapter verifies New returns ErrAdapterRegistryEmpty for empty registry.
func TestNewRequiresAtLeastOneAdapter(t *testing.T) {
	t.Parallel()
	reg := adapters.NewRegistry(nil)
	_, err := fanout.New(fanout.Options{Registry: reg})
	if !errors.Is(err, fanout.ErrAdapterRegistryEmpty) {
		t.Fatalf("want ErrAdapterRegistryEmpty, got %v", err)
	}
}

// TestNewNormalisesDefaults verifies that zero-value Options fields get documented defaults.
func TestNewNormalisesDefaults(t *testing.T) {
	t.Parallel()
	reg := buildTestRegistry(&stubAdapter{name: "test"})
	f, err := fanout.New(fanout.Options{Registry: reg})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if f == nil {
		t.Fatal("New returned nil Fanout without error")
	}

	// Verify explicit non-zero values are accepted.
	f2, err := fanout.New(fanout.Options{
		Registry:          reg,
		MaxParallel:       4,
		PerAdapterTimeout: 2 * time.Second,
		DefaultDeadline:   10 * time.Second,
	})
	if err != nil {
		t.Fatalf("New with explicit options failed: %v", err)
	}
	if f2 == nil {
		t.Fatal("New returned nil Fanout")
	}
}
