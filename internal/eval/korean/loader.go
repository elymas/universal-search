// Package korean provides the Korean-locale evaluation benchmark for SPEC-EVAL-003.
//
// This package implements the 50-query golden set loader, scoring metrics
// (top-3 Naver recall, MRR@10), Cohen's kappa inter-rater agreement calculator,
// and baseline snapshot serialization for the Korean-first ranking gate.
//
// REQ-EVAL-001: Golden set schema (50 queries, 6 categories).
// REQ-EVAL-003: Scoring sheet template.
// REQ-EVAL-005: Naver-first ranking metric (top-3 recall).
package korean

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Required query count per SPEC-EVAL-001 REQ-EVAL-001.
const RequiredQueryCount = 50

// ValidCategories is the closed set of allowed category values.
// REQ-EVAL-001: news 12 / blog 10 / shopping 8 / academic-tech 8 / code-mixed 6 / cultural 6.
var ValidCategories = map[string]bool{
	"news":          true,
	"blog":          true,
	"shopping":      true,
	"academic-tech": true,
	"code-mixed":    true,
	"cultural":      true,
}

// ExpectedCategoryCounts is the required distribution per HISTORY D2.
var ExpectedCategoryCounts = map[string]int{
	"news":          12,
	"blog":          10,
	"shopping":      8,
	"academic-tech": 8,
	"code-mixed":    6,
	"cultural":      6,
}

// GoldenQuery represents a single query object from the Korean golden-set JSONL.
// REQ-EVAL-001: Required fields per the SPEC schema.
type GoldenQuery struct {
	QueryID               string   `json:"query_id"`
	QueryText             string   `json:"query_text"`
	Category              string   `json:"category"`
	ExpectedLang          string   `json:"expected_lang"`
	ExpectedRouterClass   string   `json:"expected_router_class"`
	ExpectedNaverRelevant bool     `json:"expected_naver_relevant"`
	ExpectedSources       []string `json:"expected_sources"`
	Notes                 string   `json:"notes,omitempty"`
}

// validate checks that all required fields are populated.
func (q GoldenQuery) validate() error {
	if q.QueryID == "" {
		return fmt.Errorf("missing query_id")
	}
	if q.QueryText == "" {
		return fmt.Errorf("missing query_text in %s", q.QueryID)
	}
	if !ValidCategories[q.Category] {
		return fmt.Errorf("invalid category %q in %s", q.Category, q.QueryID)
	}
	if q.ExpectedLang != "ko" && q.ExpectedLang != "mixed" {
		return fmt.Errorf("invalid expected_lang %q in %s", q.ExpectedLang, q.QueryID)
	}
	if q.ExpectedRouterClass != "korean" && q.ExpectedRouterClass != "mixed" {
		return fmt.Errorf("invalid expected_router_class %q in %s", q.ExpectedRouterClass, q.QueryID)
	}
	if len(q.ExpectedSources) == 0 {
		return fmt.Errorf("empty expected_sources in %s", q.QueryID)
	}
	return nil
}

// LoadGoldenSet reads a JSONL file and returns exactly 50 GoldenQuery objects
// with validated schema and category distribution.
//
// @MX:NOTE: [AUTO] Golden set schema changes require SPEC amendment procedure.
// @MX:SPEC: SPEC-EVAL-003 REQ-EVAL-001
func LoadGoldenSet(path string) ([]GoldenQuery, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open golden set: %w", err)
	}
	defer f.Close()

	var queries []GoldenQuery
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var q GoldenQuery
		if err := json.Unmarshal([]byte(line), &q); err != nil {
			return nil, fmt.Errorf("line %d: unmarshal: %w", lineNum, err)
		}
		if err := q.validate(); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		queries = append(queries, q)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning golden set: %w", err)
	}

	if len(queries) != RequiredQueryCount {
		return nil, fmt.Errorf("expected %d queries, got %d", RequiredQueryCount, len(queries))
	}

	// Validate category distribution.
	counts := make(map[string]int)
	for _, q := range queries {
		counts[q.Category]++
	}
	for cat, want := range ExpectedCategoryCounts {
		if got := counts[cat]; got != want {
			return nil, fmt.Errorf("category %q: expected %d, got %d", cat, want, got)
		}
	}

	return queries, nil
}
