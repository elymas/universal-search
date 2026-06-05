// Package social — X (Twitter) provider interface and tweet struct.
// SPEC-ADP-006-XENABLE: REQ-XEN-001 defines the pluggable provider contract.
package social

import (
	"context"

	"github.com/elymas/universal-search/pkg/types"
)

// XProvider is the pluggable interface for X (Twitter) search backends.
// Concrete implementations (X official API v2, twitterapi.io, etc.) implement
// this interface and are injected via XOptions.Provider.
//
// @MX:ANCHOR: [AUTO] Public seam for pluggable provider backends.
// @MX:REASON: interface contract; every concrete provider + the adapter normalization depend on its shape.
// @MX:SPEC: SPEC-ADP-006-XENABLE
type XProvider interface {
	// Name returns a human-readable provider identifier (e.g. "x-official", "twitterapi-io").
	Name() string
	// SearchTweets executes a search against the provider backend.
	// Returns a slice of XTweet, an opaque next-page cursor, or an error.
	SearchTweets(ctx context.Context, q types.Query) ([]XTweet, string, error)
}

// XTweet is the provider-neutral intermediate struct for X search results.
// Providers map their native response format into XTweet; the adapter then
// normalizes XTweet into types.NormalizedDoc via normalizeXTweets.
type XTweet struct {
	ID            string // Tweet ID from the provider
	Text          string // Full tweet text
	AuthorHandle  string // @handle of the tweet author (may be empty)
	URL           string // Canonical URL from provider (may be empty; adapter constructs fallback)
	LikeCount     int    // Public like count
	RepostCount   int    // Public repost/retweet count
	ReplyCount    int    // Public reply count
	QuoteCount    int    // Public quote count
	CreatedAt     string // Provider's timestamp string (best-effort parse)
}
