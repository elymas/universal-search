// Package noop_test — verifies the noop reference adapter satisfies
// types.Adapter and exhibits the expected behavior.
package noop_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elymas/universal-search/internal/adapters/noop"
	"github.com/elymas/universal-search/pkg/types"
)

// TestNoopAdapterImplementsInterface confirms the runtime contract.
// The compile-time check (`var _ types.Adapter = (*noop.Adapter)(nil)`) lives
// in noop.go itself; if it fails, the package will not compile.
// REQ-CORE-002.
func TestNoopAdapterImplementsInterface(t *testing.T) {
	t.Parallel()

	var _ types.Adapter = (*noop.Adapter)(nil)

	a := noop.New("reference")
	if a.Name() != "reference" {
		t.Errorf("Name() = %q, want %q", a.Name(), "reference")
	}
	if err := a.Healthcheck(context.Background()); err != nil {
		t.Errorf("Healthcheck() = %v, want nil", err)
	}
	docs, err := a.Search(context.Background(), types.Query{})
	if err != nil {
		t.Errorf("Search() error = %v, want nil", err)
	}
	if docs != nil {
		t.Errorf("Search() docs = %v, want nil", docs)
	}
	caps := a.Capabilities()
	if caps.SourceID != "reference" {
		t.Errorf("Capabilities().SourceID = %q, want %q", caps.SourceID, "reference")
	}
}

// TestNoopAdapterHandlesCancelledContext verifies the noop honours context
// cancellation by returning ctx.Err() (a permanent cancel rather than a
// silent success).
func TestNoopAdapterHandlesCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a := noop.New("noop")
	_, err := a.Search(ctx, types.Query{})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Search after cancel: got %v, want nil or context.Canceled", err)
	}
}
