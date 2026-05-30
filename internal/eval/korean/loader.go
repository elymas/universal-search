// Package korean implements the Korean-locale benchmark harness for
// SPEC-EVAL-003: golden-set loading + schema validation, top-3 Naver
// recall scoring, inter-rater Cohen/Light kappa, and append-only baseline
// snapshot writing.
//
// The package is a pure observer of the live adapter model. It consumes the
// REAL adapter SourceID surface (single "naver", single "koreanews", plus
// arxiv/github/...) and the Naver vertical mechanism (DocType + the
// naver_vertical request filter). It never reimplements EVAL-001's English
// citation-faithfulness harness — EVAL-003 is independent and uses its own
// golden set.
package korean

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Category enumerates the six golden-set buckets (SPEC-EVAL-003 D2).
type Category string

// Category values.
const (
	CategoryNews         Category = "news"
	CategoryBlog         Category = "blog"
	CategoryShopping     Category = "shopping"
	CategoryAcademicTech Category = "academic-tech"
	CategoryCodeMixed    Category = "code-mixed"
	CategoryCultural     Category = "cultural"
)

// ExpectedCategoryDistribution is the EXACT per-category object count the
// golden set must contain (REQ-EVAL-001, D2): 12/10/8/8/6/6 = 50.
//
// @MX:NOTE: [AUTO] Frozen distribution from SPEC-EVAL-003 D2. Changing these
// counts requires a SPEC amendment and a fresh full scoring round.
// @MX:SPEC: SPEC-EVAL-003
var ExpectedCategoryDistribution = map[Category]int{
	CategoryNews:         12,
	CategoryBlog:         10,
	CategoryShopping:     8,
	CategoryAcademicTech: 8,
	CategoryCodeMixed:    6,
	CategoryCultural:     6,
}

// GoldenSetSize is the exact number of query objects required (REQ-EVAL-001).
const GoldenSetSize = 50

// registeredSourceIDs is the allowlist of REAL adapter SourceIDs that may
// appear in a golden-set query's expected_sources. Derived from the live
// adapter Capabilities() (Adapter.Name()) surface. The legacy phantom IDs
// (naver-news / naver-blog / naver-shopping / naver-academic / daum-news /
// korea-news-crawler) are intentionally ABSENT and therefore rejected.
//
// @MX:NOTE: [AUTO] Allowlist mirrors registered adapter SourceIDs. When a new
// adapter ships, add its SourceID here so its golden-set targets validate.
// @MX:SPEC: SPEC-EVAL-003
var registeredSourceIDs = map[string]struct{}{
	"naver":      {},
	"koreanews":  {},
	"arxiv":      {},
	"github":     {},
	"hackernews": {},
	"reddit":     {},
	"bluesky":    {},
	"searxng":    {},
	"youtube":    {},
}

// validNaverVerticals is the set of live naver_vertical filter values
// (internal/adapters/naver/naver.go). "academic" is intentionally absent —
// Naver has no academic vertical in live code.
var validNaverVerticals = map[string]struct{}{
	"blog":    {},
	"news":    {},
	"web":     {},
	"shop":    {},
	"datalab": {},
}

// validRouterClasses are the SPEC-IR-001 categories a Korean golden-set query
// may expect (the set is consumed, not redefined). EVAL-003 only authors
// korean/mixed queries.
var validRouterClasses = map[string]struct{}{
	"korean": {},
	"mixed":  {},
}

// GoldenQuery is one golden-set entry (REQ-EVAL-001).
type GoldenQuery struct {
	QueryID               string   `json:"query_id"`
	QueryText             string   `json:"query_text"`
	Category              Category `json:"category"`
	ExpectedLang          string   `json:"expected_lang"`
	ExpectedRouterClass   string   `json:"expected_router_class"`
	ExpectedNaverRelevant bool     `json:"expected_naver_relevant"`
	ExpectedNaverVertical string   `json:"expected_naver_vertical,omitempty"`
	ExpectedSources       []string `json:"expected_sources"`
	Notes                 string   `json:"notes,omitempty"`
}

// SchemaError describes a golden-set validation failure with the offending
// line number (1-based) and a human-readable reason.
type SchemaError struct {
	Line   int
	Reason string
}

func (e *SchemaError) Error() string {
	return fmt.Sprintf("golden-set schema error at line %d: %s", e.Line, e.Reason)
}

// LoadGoldenSet parses a JSONL golden set from r and validates it against the
// SPEC-EVAL-003 schema. It returns the parsed queries on success, or a
// *SchemaError describing the first violation.
//
// Validation enforced:
//   - exactly 50 objects (REQ-EVAL-001).
//   - exact category distribution 12/10/8/8/6/6 (D2).
//   - required fields populated; query_id format KR-NNN; unique IDs.
//   - expected_lang ∈ {ko, mixed}; expected_router_class ∈ {korean, mixed}.
//   - every expected_sources entry is a REGISTERED SourceID (phantom IDs
//     rejected).
//   - expected_naver_vertical (when present) ∈ {blog,news,web,shop,datalab}.
//   - code-mixed queries must declare expected_lang or router_class "mixed".
//
// @MX:ANCHOR: [AUTO] Golden-set schema gate. Every benchmark run, CI schema
// check, and scoring path depends on this loader to reject phantom adapter
// IDs and malformed entries before any metric is computed (fan_in >= 3:
// scoring, snapshot, CI validate).
// @MX:REASON: A phantom SourceID (e.g. "naver-news") slipping through would
// silently make the top-3 Naver recall gate unmeasurable — the exact B1
// blocker this SPEC amendment fixed. Rejection here is a release-gate
// invariant.
// @MX:SPEC: SPEC-EVAL-003
func LoadGoldenSet(r io.Reader) ([]GoldenQuery, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var queries []GoldenQuery
	seenIDs := make(map[string]int)
	counts := make(map[Category]int)
	line := 0

	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue // tolerate blank lines between records
		}

		var q GoldenQuery
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&q); err != nil {
			return nil, &SchemaError{Line: line, Reason: fmt.Sprintf("invalid JSON: %v", err)}
		}

		if err := validateQuery(&q, line); err != nil {
			return nil, err
		}

		if prev, dup := seenIDs[q.QueryID]; dup {
			return nil, &SchemaError{
				Line:   line,
				Reason: fmt.Sprintf("duplicate query_id %q (first seen line %d)", q.QueryID, prev),
			}
		}
		seenIDs[q.QueryID] = line
		counts[q.Category]++
		queries = append(queries, q)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read golden set: %w", err)
	}

	if len(queries) != GoldenSetSize {
		return nil, &SchemaError{
			Line:   line,
			Reason: fmt.Sprintf("expected exactly %d objects, got %d", GoldenSetSize, len(queries)),
		}
	}

	if err := validateDistribution(counts); err != nil {
		return nil, err
	}

	return queries, nil
}

// validateQuery validates a single decoded query. line is 1-based for errors.
func validateQuery(q *GoldenQuery, line int) error {
	if q.QueryID == "" {
		return &SchemaError{Line: line, Reason: "missing query_id"}
	}
	if !isValidQueryID(q.QueryID) {
		return &SchemaError{Line: line, Reason: fmt.Sprintf("query_id %q must match KR-NNN", q.QueryID)}
	}
	if q.QueryText == "" {
		return &SchemaError{Line: line, Reason: "missing query_text"}
	}
	if _, ok := ExpectedCategoryDistribution[q.Category]; !ok {
		return &SchemaError{Line: line, Reason: fmt.Sprintf("unknown category %q", q.Category)}
	}
	if q.ExpectedLang != "ko" && q.ExpectedLang != "mixed" {
		return &SchemaError{Line: line, Reason: fmt.Sprintf("expected_lang %q must be ko|mixed", q.ExpectedLang)}
	}
	if _, ok := validRouterClasses[q.ExpectedRouterClass]; !ok {
		return &SchemaError{Line: line, Reason: fmt.Sprintf("expected_router_class %q must be korean|mixed", q.ExpectedRouterClass)}
	}
	if len(q.ExpectedSources) == 0 {
		return &SchemaError{Line: line, Reason: "expected_sources must be non-empty"}
	}
	for _, src := range q.ExpectedSources {
		if _, ok := registeredSourceIDs[src]; !ok {
			return &SchemaError{
				Line:   line,
				Reason: fmt.Sprintf("expected_sources contains unregistered/phantom SourceID %q", src),
			}
		}
	}
	if q.ExpectedNaverVertical != "" {
		if _, ok := validNaverVerticals[q.ExpectedNaverVertical]; !ok {
			return &SchemaError{
				Line:   line,
				Reason: fmt.Sprintf("expected_naver_vertical %q must be blog|news|web|shop|datalab", q.ExpectedNaverVertical),
			}
		}
		if !q.ExpectedNaverRelevant {
			return &SchemaError{
				Line:   line,
				Reason: "expected_naver_vertical set but expected_naver_relevant is false",
			}
		}
	}
	if len([]rune(q.Notes)) > 200 {
		return &SchemaError{Line: line, Reason: "notes exceeds 200 chars"}
	}
	if q.Category == CategoryCodeMixed && q.ExpectedLang != "mixed" && q.ExpectedRouterClass != "mixed" {
		return &SchemaError{
			Line:   line,
			Reason: "code-mixed query must declare expected_lang or expected_router_class as mixed",
		}
	}
	return nil
}

// validateDistribution checks the per-category counts match D2 exactly.
func validateDistribution(counts map[Category]int) error {
	cats := make([]Category, 0, len(ExpectedCategoryDistribution))
	for c := range ExpectedCategoryDistribution {
		cats = append(cats, c)
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i] < cats[j] })
	for _, c := range cats {
		want := ExpectedCategoryDistribution[c]
		if got := counts[c]; got != want {
			return &SchemaError{
				Line:   0,
				Reason: fmt.Sprintf("category %q count = %d, want %d", c, got, want),
			}
		}
	}
	return nil
}

// isValidQueryID reports whether id matches KR-NNN (zero-padded 3-digit).
func isValidQueryID(id string) bool {
	const prefix = "KR-"
	if !strings.HasPrefix(id, prefix) {
		return false
	}
	num := id[len(prefix):]
	if len(num) != 3 {
		return false
	}
	for _, r := range num {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
