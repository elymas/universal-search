// Package koreanews — Daum sub-source stub.
// SPEC-ADP-009 REQ-ADP9-011: Daum is permanently stubbed in v0.1.
// Daum's robots.txt (https://search.daum.net/robots.txt) contains:
//
//	User-agent: *
//	Disallow: /
//
// This prohibits all crawlers without Kakao authorisation. Any future SPEC-ADP-009-DAUM
// that activates real Daum scraping MUST include explicit legal review sign-off.
package koreanews

import (
	"context"

	"github.com/elymas/universal-search/pkg/types"
)

// searchDaum is the Daum sub-source stub. It always returns ErrDaumDisabled
// wrapped in *types.SourceError{CategoryPermanent} regardless of whether
// DaumEnabled is true. The enable flag is plumbed for future SPEC-ADP-009-DAUM
// but MUST NOT activate real scraping until a legal-review SPEC approves it.
//
// @MX:NOTE: [AUTO] Legal sentinel: Daum robots.txt forbids all crawlers.
// Never replace this stub with real scraping without a signed-off SPEC.
// @MX:SPEC: SPEC-ADP-009
func searchDaum(_ context.Context, adapterName string) ([]types.NormalizedDoc, error) {
	return nil, &types.SourceError{
		Adapter:  adapterName,
		Category: types.CategoryPermanent,
		Cause:    ErrDaumDisabled,
	}
}
