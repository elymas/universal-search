// Package koreanews — Daum stub tests.
// SPEC-ADP-009 REQ-ADP9-011: Daum always returns ErrDaumDisabled.
package koreanews

import (
	"context"
	"errors"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

func TestSearchDaum_alwaysReturnsErrDaumDisabled(t *testing.T) {
	t.Parallel()

	_, err := searchDaum(context.Background(), "koreanews")
	if err == nil {
		t.Fatal("searchDaum: expected error, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError; got %T: %v", err, err)
	}

	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v; want CategoryPermanent", se.Category)
	}

	if !errors.Is(err, ErrDaumDisabled) {
		t.Errorf("expected errors.Is(err, ErrDaumDisabled); cause = %v", se.Cause)
	}
}

func TestSearchDaum_cancelledContextStillReturnsErr(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err := searchDaum(ctx, "koreanews")
	if err == nil {
		t.Fatal("searchDaum with cancelled ctx: expected error, got nil")
	}
	if !errors.Is(err, ErrDaumDisabled) {
		t.Errorf("expected ErrDaumDisabled; got %v", err)
	}
}
