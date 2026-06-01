package koreanews

// Coverage for the gofeed item/feed accessor helpers: published-time fallback
// chain, author extraction, and channel link, including the nil/empty branches.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestItemPublished(t *testing.T) {
	pub := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	upd := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)

	t.Run("prefers PublishedParsed", func(t *testing.T) {
		it := &gofeed.Item{PublishedParsed: &pub, UpdatedParsed: &upd}
		if got := itemPublished(it); !got.Equal(pub) {
			t.Errorf("itemPublished = %v, want %v", got, pub)
		}
	})
	t.Run("falls back to UpdatedParsed", func(t *testing.T) {
		it := &gofeed.Item{UpdatedParsed: &upd}
		if got := itemPublished(it); !got.Equal(upd) {
			t.Errorf("itemPublished = %v, want %v", got, upd)
		}
	})
	t.Run("zero time when neither set", func(t *testing.T) {
		it := &gofeed.Item{}
		if got := itemPublished(it); !got.IsZero() {
			t.Errorf("itemPublished = %v, want zero", got)
		}
	})
}

func TestItemAuthor(t *testing.T) {
	t.Run("returns author name", func(t *testing.T) {
		it := &gofeed.Item{Author: &gofeed.Person{Name: "기자"}}
		if got := itemAuthor(it); got != "기자" {
			t.Errorf("itemAuthor = %q, want 기자", got)
		}
	})
	t.Run("empty when author nil", func(t *testing.T) {
		if got := itemAuthor(&gofeed.Item{}); got != "" {
			t.Errorf("itemAuthor = %q, want empty", got)
		}
	})
	t.Run("empty when name blank", func(t *testing.T) {
		it := &gofeed.Item{Author: &gofeed.Person{Name: ""}}
		if got := itemAuthor(it); got != "" {
			t.Errorf("itemAuthor = %q, want empty", got)
		}
	})
}

func TestFeedLink(t *testing.T) {
	if got := feedLink(&gofeed.Feed{Link: "https://news.example.com"}); got != "https://news.example.com" {
		t.Errorf("feedLink = %q, want the channel link", got)
	}
	if got := feedLink(nil); got != "" {
		t.Errorf("feedLink(nil) = %q, want empty", got)
	}
}
