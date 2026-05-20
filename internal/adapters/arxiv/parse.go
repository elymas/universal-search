package arxiv

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/elymas/universal-search/pkg/types"
)

// atomFeed is the top-level structure of an arXiv Atom feed.
type atomFeed struct {
	TotalResults int         `xml:"http://a9.com/-/spec/opensearch/1.1/ totalResults"`
	StartIndex   int         `xml:"http://a9.com/-/spec/opensearch/1.1/ startIndex"`
	ItemsPerPage int         `xml:"http://a9.com/-/spec/opensearch/1.1/ itemsPerPage"`
	Entries      []atomEntry `xml:"http://www.w3.org/2005/Atom entry"`
}

// atomAuthor holds a single author name.
type atomAuthor struct {
	Name string `xml:"http://www.w3.org/2005/Atom name"`
}

// atomLink holds a <link> element.
type atomLink struct {
	Href  string `xml:"href,attr"`
	Rel   string `xml:"rel,attr"`
	Title string `xml:"title,attr"`
	Type  string `xml:"type,attr"`
}

// atomCategory holds a <category> element.
type atomCategory struct {
	Term   string `xml:"term,attr"`
	Scheme string `xml:"scheme,attr"`
}

// atomPrimaryCategory holds an <arxiv:primary_category> element.
type atomPrimaryCategory struct {
	Term string `xml:"term,attr"`
}

// atomEntry is a single paper entry in the feed.
type atomEntry struct {
	ID         string         `xml:"http://www.w3.org/2005/Atom id"`
	Published  string         `xml:"http://www.w3.org/2005/Atom published"`
	Updated    string         `xml:"http://www.w3.org/2005/Atom updated"`
	Title      string         `xml:"http://www.w3.org/2005/Atom title"`
	Summary    string         `xml:"http://www.w3.org/2005/Atom summary"`
	Authors    []atomAuthor   `xml:"http://www.w3.org/2005/Atom author"`
	Links      []atomLink     `xml:"http://www.w3.org/2005/Atom link"`
	Categories []atomCategory `xml:"http://www.w3.org/2005/Atom category"`

	// arXiv extension elements.
	DOI             string              `xml:"http://arxiv.org/schemas/atom doi"`
	JournalRef      string              `xml:"http://arxiv.org/schemas/atom journal_ref"`
	Comment         string              `xml:"http://arxiv.org/schemas/atom comment"`
	PrimaryCategory atomPrimaryCategory `xml:"http://arxiv.org/schemas/atom primary_category"`
}

const snippetMaxRunes = 280

// parseFeed decodes an arXiv Atom XML body into a slice of NormalizedDoc.
// retrievedAt is set as the RetrievedAt timestamp on every returned doc.
// Returns a *types.SourceError{CategoryPermanent} on XML decode failure.
//
// // @MX:ANCHOR: [AUTO] Hot parse path called by (*Adapter).Search on every response.
// // @MX:REASON: fan_in >= 3 (Search, BenchmarkParseFeed25Entries, parse_test.go direct calls)
// // @MX:SPEC: SPEC-ADP-003 REQ-ADP3-006
func parseFeed(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, &types.SourceError{
			Adapter:  "arxiv",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("arxiv: xml decode: %w", err),
		}
	}

	if len(feed.Entries) == 0 {
		return nil, nil
	}

	totalResults := feed.TotalResults
	startIndex := feed.StartIndex
	hasNextPage := startIndex+len(feed.Entries) < totalResults

	docs := make([]types.NormalizedDoc, 0, len(feed.Entries))
	for i, entry := range feed.Entries {
		doc := transformEntry(entry, retrievedAt)

		// First doc: surface total_results from feed envelope (OPTIONAL per §6.3).
		if i == 0 {
			doc.Metadata["total_results"] = totalResults
		}

		// Last doc: surface next_cursor when more pages remain (REQUIRED per §6.3).
		if i == len(feed.Entries)-1 && hasNextPage {
			doc.Metadata["next_cursor"] = strconv.Itoa(startIndex + len(feed.Entries))
		}

		docs = append(docs, doc)
	}
	return docs, nil
}

// transformEntry converts a single atomEntry to a NormalizedDoc.
func transformEntry(entry atomEntry, retrievedAt time.Time) types.NormalizedDoc {
	rawID := entry.ID

	// Strip the canonical prefix to get the bare arXiv ID (e.g., "2403.12345v2").
	bareID := strings.TrimPrefix(rawID, "http://arxiv.org/abs/")

	title := collapseWS(entry.Title)
	body := collapseWS(entry.Summary)

	snippet := truncateRunes(body, snippetMaxRunes)
	if snippet == "" {
		snippet = truncateRunes(title, snippetMaxRunes)
	}

	publishedAt, _ := time.Parse(time.RFC3339, entry.Published)

	// Collect author names in submission order.
	authors := make([]string, 0, len(entry.Authors))
	for _, a := range entry.Authors {
		authors = append(authors, a.Name)
	}
	firstAuthor := ""
	if len(authors) > 0 {
		firstAuthor = authors[0]
	}

	// Collect all category terms.
	categories := make([]string, 0, len(entry.Categories))
	for _, c := range entry.Categories {
		categories = append(categories, c.Term)
	}

	// Build the URL: use the abs link (the entry ID is canonical).
	url := rawID

	// Build required metadata map (§6.3).
	meta := map[string]any{
		"arxiv_id":         bareID,
		"authors":          authors,
		"primary_category": entry.PrimaryCategory.Term,
		"categories":       categories,
		"published_at":     entry.Published,
		"updated_at":       entry.Updated,
	}

	// Optional metadata (§6.3).
	if entry.DOI != "" {
		meta["doi"] = entry.DOI
	}
	if entry.JournalRef != "" {
		meta["journal_ref"] = entry.JournalRef
	}
	if entry.Comment != "" {
		meta["comment"] = entry.Comment
	}
	// PDF link (optional).
	for _, link := range entry.Links {
		if link.Title == "pdf" {
			meta["pdf_url"] = link.Href
			break
		}
	}

	return types.NormalizedDoc{
		ID:          bareID,
		SourceID:    "arxiv",
		URL:         url,
		Title:       title,
		Body:        body,
		Snippet:     snippet,
		PublishedAt: publishedAt.UTC(),
		RetrievedAt: retrievedAt,
		Author:      firstAuthor,
		Score:       constantScore,
		Lang:        "",
		DocType:     types.DocTypePaper,
		Citations:   nil,
		Metadata:    meta,
		Hash:        "",
	}
}

// collapseWS collapses all whitespace runs in s to a single space and trims
// leading/trailing whitespace. Equivalent to strings.Join(strings.Fields(s), " ").
func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncateRunes truncates s to maxRunes runes. If truncation occurs, the suffix
// "..." is appended (consuming 3 of the maxRunes budget). Returns s unchanged
// if len([]rune(s)) <= maxRunes.
func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes-3]) + "..."
}
