// dispatch_internal_test.go — white-box tests for dispatch helper functions.
// Uses package fanout (not fanout_test) to access unexported functions.
package fanout

import (
	"context"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestDeriveAdapterCtxBasic verifies perAdapterTimeout is applied when parent has no deadline.
func TestDeriveAdapterCtxBasic(t *testing.T) {
	t.Parallel()
	timeout := 500 * time.Millisecond
	ctx, cancel := deriveAdapterCtx(context.Background(), timeout)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("want deadline set from perAdapterTimeout")
	}
	remaining := time.Until(deadline)
	// Allow ±100ms scheduling jitter.
	if remaining < 0 || remaining > timeout+100*time.Millisecond {
		t.Fatalf("unexpected deadline remaining: %v (want ~%v)", remaining, timeout)
	}
}

// TestDeriveAdapterCtxParentShorter verifies parent deadline wins when shorter than perAdapterTimeout.
func TestDeriveAdapterCtxParentShorter(t *testing.T) {
	t.Parallel()
	// Parent deadline is 100ms, perAdapterTimeout is 5s. Parent should win.
	parent, parentCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer parentCancel()

	ctx, cancel := deriveAdapterCtx(parent, 5*time.Second)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("want deadline set")
	}
	// Derived deadline should be ~= parent deadline (≤ parent's deadline).
	parentDeadline, _ := parent.Deadline()
	if deadline.After(parentDeadline) {
		t.Fatalf("derived deadline %v is after parent deadline %v", deadline, parentDeadline)
	}
}

// TestDeriveAdapterCtxAlreadyExpiredParent verifies immediately-cancelled ctx on expired parent.
func TestDeriveAdapterCtxAlreadyExpiredParent(t *testing.T) {
	t.Parallel()
	// Parent context is already past its deadline.
	parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	ctx, ctxCancel := deriveAdapterCtx(parent, 5*time.Second)
	defer ctxCancel()

	// Should be already cancelled.
	select {
	case <-ctx.Done():
		// Expected: context is done.
	default:
		t.Fatal("want context already done for expired parent deadline")
	}
}

// TestAssembleResultAllSuccess verifies assembleResult stats on all-success path.
func TestAssembleResultAllSuccess(t *testing.T) {
	t.Parallel()
	adapterSet := []string{"a", "b", "c"}
	perDocs := [][]types.NormalizedDoc{
		{{ID: "a1"}, {ID: "a2"}},
		{{ID: "b1"}},
		{{ID: "c1"}, {ID: "c2"}, {ID: "c3"}},
	}
	perErr := []error{nil, nil, nil}

	res := assembleResult(adapterSet, perDocs, perErr)
	if res.Stats.AdapterCount != 3 {
		t.Fatalf("want AdapterCount=3, got %d", res.Stats.AdapterCount)
	}
	if res.Stats.SuccessCount != 3 {
		t.Fatalf("want SuccessCount=3, got %d", res.Stats.SuccessCount)
	}
	if res.Stats.ErrorCount != 0 {
		t.Fatalf("want ErrorCount=0, got %d", res.Stats.ErrorCount)
	}
	if res.AdapterErrors != nil {
		t.Fatal("want AdapterErrors==nil")
	}
	if len(res.Docs) != 6 {
		t.Fatalf("want 6 docs, got %d", len(res.Docs))
	}
}

// TestAssembleResultAllFailure verifies assembleResult stats on all-failure path.
func TestAssembleResultAllFailure(t *testing.T) {
	t.Parallel()
	adapterSet := []string{"a", "b"}
	perDocs := [][]types.NormalizedDoc{nil, nil}
	perErr := []error{
		&types.SourceError{Adapter: "a", Category: types.CategoryPermanent, Cause: types.ErrPermanent},
		&types.SourceError{Adapter: "b", Category: types.CategoryPermanent, Cause: types.ErrPermanent},
	}

	res := assembleResult(adapterSet, perDocs, perErr)
	if res.Stats.SuccessCount != 0 {
		t.Fatalf("want SuccessCount=0, got %d", res.Stats.SuccessCount)
	}
	if res.Stats.ErrorCount != 2 {
		t.Fatalf("want ErrorCount=2, got %d", res.Stats.ErrorCount)
	}
	if res.AdapterErrors == nil {
		t.Fatal("want AdapterErrors != nil")
	}
	if len(res.Docs) != 0 {
		t.Fatalf("want 0 docs, got %d", len(res.Docs))
	}
}
