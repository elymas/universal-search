// Package social — X (Twitter) search path.
// REQ-ADP6-008 / SPEC-ADP-006-XENABLE: X is gated by env + provider presence.
package social

import (
	"context"
	"errors"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// xEnabledEnvVar is the environment variable that gates X adapter activation.
const xEnabledEnvVar = "USEARCH_X_ENABLED"

// searchX implements the X search path: returns ErrXDisabled when the env var
// is unset, ErrXProviderNotConfigured when env is "true" but no provider is wired,
// or executes the live provider path when both gates are satisfied.
//
// @MX:ANCHOR: [AUTO] Sole entry for the X sub-source; live + disabled dispatch.
// @MX:REASON: env+provider gating; changing the branch order changes disabled/live semantics.
// @MX:SPEC: SPEC-ADP-006-XENABLE
func (a *Adapter) searchX(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// Gate 1: env var must be "true".
	if a.envLookup(xEnabledEnvVar) != "true" {
		return nil, ErrXDisabled
	}

	// Gate 2: provider must be configured.
	if a.xProvider == nil {
		return nil, ErrXProviderNotConfigured
	}

	// Gate 3: query must be non-empty.
	if isBlankQuery(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "x",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// Live path: call provider.
	tweets, nextCursor, err := a.xProvider.SearchTweets(ctx, q)
	if err != nil {
		var se *types.SourceError
		if errors.As(err, &se) {
			return nil, se
		}
		return nil, &types.SourceError{
			Adapter:  "x",
			Category: types.CategoryUnavailable,
			Cause:    err,
		}
	}

	docs, normErr := normalizeXTweets(tweets, nextCursor, time.Now().UTC())
	if normErr != nil {
		return nil, &types.SourceError{
			Adapter:  "x",
			Category: types.CategoryPermanent,
			Cause:    normErr,
		}
	}

	// Stamp provider name on metadata.
	provName := a.xProvider.Name()
	for i := range docs {
		if docs[i].Metadata == nil {
			docs[i].Metadata = make(map[string]any)
		}
		docs[i].Metadata["provider"] = provName
	}

	return docs, nil
}
