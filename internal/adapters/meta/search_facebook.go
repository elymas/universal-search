// Package meta — Facebook disabled search stub.
// REQ-ADP10-008: always returns ErrFacebookNotSupported, zero HTTP requests.
package meta

import (
	"context"

	"github.com/elymas/universal-search/pkg/types"
)

// @MX:WARN: [AUTO] External-blocker boundary.
// @MX:REASON: wiring any Facebook scraping here violates tech.md:147 ToS mandate;
// no official API path exists.
// @MX:SPEC: SPEC-ADP-010
func (a *Adapter) searchFacebookDisabled(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
	return nil, &types.SourceError{
		Adapter:  "facebook",
		Category: types.CategoryPermanent,
		Cause:    ErrFacebookNotSupported,
	}
}
