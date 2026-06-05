// Package social — X (Twitter) tweet normalization.
// SPEC-ADP-006-XENABLE: REQ-XEN-006 transforms XTweet → NormalizedDoc.
package social

import (
	"fmt"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// normalizeXTweets transforms a slice of XTweet into []types.NormalizedDoc per
// the field mapping defined in SPEC-ADP-006-XENABLE §6.5.
//
// When nextCursor is non-empty, it is attached to the LAST doc's
// Metadata["next_cursor"].
//
// @MX:NOTE: [AUTO] Every X doc passes through this single transform; NormalizedDoc
// field-mapping integrity gate for X.
// @MX:SPEC: SPEC-ADP-006-XENABLE
func normalizeXTweets(tweets []XTweet, nextCursor string, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	if len(tweets) == 0 {
		return nil, nil
	}

	docs := make([]types.NormalizedDoc, 0, len(tweets))

	for i, tweet := range tweets {
		doc := transformXTweet(tweet, retrievedAt)
		// Surface next_cursor only on the last doc.
		if i == len(tweets)-1 && nextCursor != "" {
			if doc.Metadata == nil {
				doc.Metadata = make(map[string]any)
			}
			doc.Metadata["next_cursor"] = nextCursor
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

// transformXTweet converts a single XTweet into a NormalizedDoc.
func transformXTweet(tweet XTweet, retrievedAt time.Time) types.NormalizedDoc {
	tweetURL := tweet.URL
	if tweetURL == "" {
		if tweet.AuthorHandle != "" {
			tweetURL = fmt.Sprintf("https://x.com/%s/status/%s", tweet.AuthorHandle, tweet.ID)
		} else {
			tweetURL = fmt.Sprintf("https://x.com/i/status/%s", tweet.ID)
		}
	}

	snippet := truncateRunes(tweet.Text, snippetMaxRunes)

	// Best-effort parse of CreatedAt.
	var publishedAt time.Time
	if tweet.CreatedAt != "" {
		for _, layout := range []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05.000Z",
		} {
			if t, err := time.Parse(layout, tweet.CreatedAt); err == nil {
				publishedAt = t.UTC()
				break
			}
		}
	}

	score := normalizeScore(tweet.LikeCount, tweet.RepostCount)
	body := tweet.Text

	meta := map[string]any{
		"handle":       tweet.AuthorHandle,
		"tweet_id":     tweet.ID,
		"like_count":   tweet.LikeCount,
		"repost_count": tweet.RepostCount,
		"reply_count":  tweet.ReplyCount,
		"quote_count":  tweet.QuoteCount,
		"posted_at":    tweet.CreatedAt,
		"sub_source":   "x",
		"provider":     "",
	}

	return types.NormalizedDoc{
		ID:          "x:" + tweet.ID,
		SourceID:    "x",
		URL:         tweetURL,
		Title:       snippet,
		Body:        body,
		Snippet:     snippet,
		PublishedAt: publishedAt,
		RetrievedAt: retrievedAt,
		Author:      tweet.AuthorHandle,
		Score:       score,
		Lang:        "",
		DocType:     types.DocTypePost,
		Hash:        "",
		Metadata:    meta,
	}
}
