// Package naver — Naver JSON response → []types.NormalizedDoc transform.
// REQ-ADP8-006: parseXxx functions map each vertical's JSON envelope to NormalizedDoc.
package naver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
	"unicode/utf8"

	"github.com/elymas/universal-search/pkg/types"
)

// snippetMaxRunes is the maximum rune length for NormalizedDoc.Snippet.
// Truncation appends the UTF-8 ellipsis character "…" (U+2026, 3 bytes).
const snippetMaxRunes = 280

// naverCommonResponse is the shared top-level Naver search API response envelope.
type naverCommonResponse struct {
	LastBuildDate string `json:"lastBuildDate"`
	Total         int    `json:"total"`
	Start         int    `json:"start"`
	Display       int    `json:"display"`
}

// naverBlogItem is a single blog search result item.
type naverBlogItem struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
	BloggerName string `json:"bloggername"`
	BloggerLink string `json:"bloggerlink"`
	PostDate    string `json:"postdate"`
}

// naverBlogResponse is the Naver blog search API response.
type naverBlogResponse struct {
	naverCommonResponse
	Items []naverBlogItem `json:"items"`
}

// naverNewsItem is a single news search result item.
type naverNewsItem struct {
	Title        string `json:"title"`
	OriginalLink string `json:"originallink"`
	Link         string `json:"link"`
	Description  string `json:"description"`
	PubDate      string `json:"pubDate"`
}

// naverNewsResponse is the Naver news search API response.
type naverNewsResponse struct {
	naverCommonResponse
	Items []naverNewsItem `json:"items"`
}

// naverWebItem is a single web search result item.
type naverWebItem struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
}

// naverWebResponse is the Naver web search API response.
type naverWebResponse struct {
	naverCommonResponse
	Items []naverWebItem `json:"items"`
}

// naverShopItem is a single shopping search result item.
type naverShopItem struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Image       string `json:"image"`
	LPrice      string `json:"lprice"`
	HPrice      string `json:"hprice"`
	MallName    string `json:"mallName"`
	ProductID   string `json:"productId"`
	ProductType string `json:"productType"`
	Brand       string `json:"brand"`
	Maker       string `json:"maker"`
	Category1   string `json:"category1"`
	Category2   string `json:"category2"`
	Category3   string `json:"category3"`
	Category4   string `json:"category4"`
}

// naverShopResponse is the Naver shopping search API response.
type naverShopResponse struct {
	naverCommonResponse
	Items []naverShopItem `json:"items"`
}

// syntheticID computes a stable 16-char hex ID for a NormalizedDoc given its URL.
// Uses sha256(url)[:8] → 16 hex chars, mirroring NormalizedDoc.CanonicalHash().
func syntheticID(link string) string {
	h := sha256.Sum256([]byte(link))
	return hex.EncodeToString(h[:8])
}

// truncateRunes truncates s to at most maxRunes runes.
// If truncation occurs, the UTF-8 ellipsis "…" (U+2026) is appended, replacing
// the last character so the total is still maxRunes runes.
func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	// Build up to (maxRunes - 1) runes, then append "…".
	count := 0
	target := maxRunes - 1
	for i := range s {
		if count >= target {
			return s[:i] + "…"
		}
		count++
	}
	return s + "…"
}

// nextCursorValue computes the next cursor value for pagination.
// Naver uses 1-based start parameter. Returns "" if there are no more pages.
// A next cursor exists if (start + display) <= min(total, 1000).
func nextCursorValue(start, display, total int) string {
	next := start + display
	// Naver caps at offset 1000 (start+display-1 <= 1000, i.e., next <= 1001)
	if next > 1000 {
		return ""
	}
	if next > total {
		return ""
	}
	return fmt.Sprintf("%d", next)
}

// surfaceCursor sets Metadata["next_cursor"] on the last doc if a next page exists.
func surfaceCursor(docs []types.NormalizedDoc, start, display, total int) {
	if len(docs) == 0 {
		return
	}
	cursor := nextCursorValue(start, display, total)
	if cursor == "" {
		return
	}
	last := &docs[len(docs)-1]
	if last.Metadata == nil {
		last.Metadata = make(map[string]any)
	}
	last.Metadata["next_cursor"] = cursor
}

// parseBlogResponse parses a Naver blog JSON response body into NormalizedDoc values.
//
// @MX:ANCHOR: [AUTO] Blog NormalizedDoc field-mapping integrity gate.
// @MX:REASON: fan_in = Search (via vertical dispatch); field-mapping changes
// require coordination with SPEC-IDX-001 and SPEC-SYN-001 consumers.
// @MX:SPEC: SPEC-ADP-008
func parseBlogResponse(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	var resp naverBlogResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &types.SourceError{
			Adapter:  "naver",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("naver: malformed blog JSON: %w", err),
		}
	}

	if len(resp.Items) == 0 {
		return nil, nil
	}

	docs := make([]types.NormalizedDoc, 0, len(resp.Items))
	for _, item := range resp.Items {
		title := stripHTML(item.Title)
		description := stripHTML(item.Description)
		snippet := truncateRunes(description, snippetMaxRunes)

		publishedAt := parseBlogPostDate(item.PostDate)

		meta := map[string]any{
			"bloggername": item.BloggerName,
			"bloggerlink": item.BloggerLink,
		}

		doc := types.NormalizedDoc{
			ID:          syntheticID(item.Link),
			SourceID:    "naver",
			URL:         item.Link,
			Title:       title,
			Body:        description,
			Snippet:     snippet,
			PublishedAt: publishedAt,
			RetrievedAt: retrievedAt,
			Author:      item.BloggerName,
			Score:       defaultScore,
			Lang:        "ko",
			DocType:     types.DocTypePost,
			Metadata:    meta,
		}
		docs = append(docs, doc)
	}

	surfaceCursor(docs, resp.Start, resp.Display, resp.Total)
	return docs, nil
}

// parseBlogPostDate parses the Naver blog postdate field (format "YYYYMMDD").
// Returns zero time on empty string or parse failure.
func parseBlogPostDate(postdate string) time.Time {
	if postdate == "" {
		return time.Time{}
	}
	t, err := time.Parse("20060102", postdate)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// parseNewsResponse parses a Naver news JSON response body into NormalizedDoc values.
func parseNewsResponse(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	var resp naverNewsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &types.SourceError{
			Adapter:  "naver",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("naver: malformed news JSON: %w", err),
		}
	}

	if len(resp.Items) == 0 {
		return nil, nil
	}

	docs := make([]types.NormalizedDoc, 0, len(resp.Items))
	for _, item := range resp.Items {
		title := stripHTML(item.Title)
		description := stripHTML(item.Description)
		snippet := truncateRunes(description, snippetMaxRunes)

		// Use originallink as the canonical URL; fall back to Naver link.
		canonicalURL := item.Link
		if item.OriginalLink != "" {
			canonicalURL = item.OriginalLink
		}

		// Derive author from the domain of originallink.
		author := newsAuthorFromURL(item.OriginalLink)

		publishedAt := parseNewsPubDate(item.PubDate)

		doc := types.NormalizedDoc{
			ID:          syntheticID(canonicalURL),
			SourceID:    "naver",
			URL:         canonicalURL,
			Title:       title,
			Body:        description,
			Snippet:     snippet,
			PublishedAt: publishedAt,
			RetrievedAt: retrievedAt,
			Author:      author,
			Score:       defaultScore,
			Lang:        "ko",
			DocType:     types.DocTypeArticle,
		}
		docs = append(docs, doc)
	}

	surfaceCursor(docs, resp.Start, resp.Display, resp.Total)
	return docs, nil
}

// newsAuthorFromURL extracts the hostname of the originallink URL to use as author.
// Returns "" if originallink is empty or cannot be parsed.
func newsAuthorFromURL(originalLink string) string {
	if originalLink == "" {
		return ""
	}
	u, err := url.Parse(originalLink)
	if err != nil {
		return ""
	}
	return u.Host
}

// parseNewsPubDate parses the Naver news pubDate field (RFC1123Z or RFC1123).
// Returns zero time on empty string or parse failure.
func parseNewsPubDate(pubDate string) time.Time {
	if pubDate == "" {
		return time.Time{}
	}
	// Try RFC1123Z first (e.g., "Wed, 07 May 2026 09:00:00 +0900").
	if t, err := time.Parse(time.RFC1123Z, pubDate); err == nil {
		return t.UTC()
	}
	// Fallback to RFC1123 (e.g., "Wed, 07 May 2026 09:00:00 KST").
	if t, err := time.Parse(time.RFC1123, pubDate); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// parseWebResponse parses a Naver web search JSON response body into NormalizedDoc values.
func parseWebResponse(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	var resp naverWebResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &types.SourceError{
			Adapter:  "naver",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("naver: malformed web JSON: %w", err),
		}
	}

	if len(resp.Items) == 0 {
		return nil, nil
	}

	docs := make([]types.NormalizedDoc, 0, len(resp.Items))
	for _, item := range resp.Items {
		title := stripHTML(item.Title)
		description := stripHTML(item.Description)
		snippet := truncateRunes(description, snippetMaxRunes)

		doc := types.NormalizedDoc{
			ID:          syntheticID(item.Link),
			SourceID:    "naver",
			URL:         item.Link,
			Title:       title,
			Body:        description,
			Snippet:     snippet,
			RetrievedAt: retrievedAt,
			Score:       defaultScore,
			Lang:        "ko",
			DocType:     types.DocTypeOther,
		}
		docs = append(docs, doc)
	}

	surfaceCursor(docs, resp.Start, resp.Display, resp.Total)
	return docs, nil
}

// parseShopResponse parses a Naver shopping search JSON response body into NormalizedDoc values.
func parseShopResponse(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	var resp naverShopResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &types.SourceError{
			Adapter:  "naver",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("naver: malformed shop JSON: %w", err),
		}
	}

	if len(resp.Items) == 0 {
		return nil, nil
	}

	docs := make([]types.NormalizedDoc, 0, len(resp.Items))
	for _, item := range resp.Items {
		title := stripHTML(item.Title)

		meta := map[string]any{
			"lprice":       item.LPrice,
			"hprice":       item.HPrice,
			"mall_name":    item.MallName,
			"product_id":   item.ProductID,
			"product_type": item.ProductType,
			"image":        item.Image,
		}
		if item.Brand != "" {
			meta["brand"] = item.Brand
		}
		if item.Maker != "" {
			meta["maker"] = item.Maker
		}
		// Category hierarchy.
		if item.Category1 != "" {
			meta["category1"] = item.Category1
		}
		if item.Category2 != "" {
			meta["category2"] = item.Category2
		}
		if item.Category3 != "" {
			meta["category3"] = item.Category3
		}
		if item.Category4 != "" {
			meta["category4"] = item.Category4
		}

		doc := types.NormalizedDoc{
			ID:          syntheticID(item.Link),
			SourceID:    "naver",
			URL:         item.Link,
			Title:       title,
			Body:        title,
			Snippet:     truncateRunes(title, snippetMaxRunes),
			RetrievedAt: retrievedAt,
			Score:       defaultScore,
			Lang:        "ko",
			DocType:     types.DocTypeOther,
			Metadata:    meta,
		}
		docs = append(docs, doc)
	}

	surfaceCursor(docs, resp.Start, resp.Display, resp.Total)
	return docs, nil
}
