// Package meta — Search dispatch.
// REQ-ADP10-001: sub-source routing for Search calls.
package meta

import (
	"context"
	"fmt"

	"github.com/elymas/universal-search/pkg/types"
)

// Search dispatches to the appropriate sub-source search implementation.
//
// @MX:ANCHOR: [AUTO] Sole entry for Meta fanout (Threads + Facebook).
// @MX:REASON: contract boundary; sub-source dispatch.
// @MX:SPEC: SPEC-ADP-010
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	switch a.subSource {
	case "threads":
		return a.searchThreads(ctx, q)
	case "facebook":
		return a.searchFacebookDisabled(ctx, q)
	default:
		return nil, &types.SourceError{
			Adapter:  a.subSource,
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("meta: unknown subSource %q", a.subSource),
		}
	}
}
