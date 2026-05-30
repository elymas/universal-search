package korean

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testdataPath(t *testing.T, parts ...string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(append([]string{root, "tests", "eval", "korean"}, parts...)...)
}

// ExpectedSheetHeader is the prescribed scoring-sheet CSV header (REQ-EVAL-003).
const ExpectedSheetHeader = "query_id,rater_id,ranking_score,source_relevance,code_switching_handling,tokenization_quality,top3_naver_hit,mrr_top10,notes"

func TestScoringSheetTemplate_Header(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(testdataPath(t, "scoring-sheet-template.csv"))
	if err != nil {
		t.Fatalf("read scoring sheet template: %v", err)
	}
	firstLine := strings.SplitN(strings.TrimRight(string(data), "\r\n"), "\n", 2)[0]
	firstLine = strings.TrimRight(firstLine, "\r")
	if firstLine != ExpectedSheetHeader {
		t.Errorf("CSV header mismatch:\n got:  %q\n want: %q", firstLine, ExpectedSheetHeader)
	}
}

// TestBaselinePlaceholder_MatchesSchema confirms the placeholder example
// deserializes into the Snapshot struct and is explicitly marked a placeholder
// (zeroed metrics), never mistaken for a real baseline.
func TestBaselinePlaceholder_MatchesSchema(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(testdataPath(t, "baseline-snapshots", "v1.0.0.example.json"))
	if err != nil {
		t.Fatalf("read placeholder snapshot: %v", err)
	}

	// The placeholder carries an extra _comment field; decode loosely first to
	// assert the disclaimer is present.
	var loose map[string]any
	if err := json.Unmarshal(data, &loose); err != nil {
		t.Fatalf("placeholder is not valid JSON: %v", err)
	}
	comment, _ := loose["_comment"].(string)
	if !strings.Contains(strings.ToUpper(comment), "PLACEHOLDER") {
		t.Errorf("placeholder snapshot must declare itself a PLACEHOLDER in _comment")
	}

	// Strip the comment and confirm the remaining shape maps onto Snapshot.
	delete(loose, "_comment")
	clean, _ := json.Marshal(loose)
	var s Snapshot
	if err := json.Unmarshal(clean, &s); err != nil {
		t.Fatalf("placeholder does not match Snapshot schema: %v", err)
	}
	if s.ReleaseTag == "" || s.GoldenSetSHA256 == "" || s.TokenizerVersion == "" {
		t.Errorf("placeholder missing required schema fields: %+v", s)
	}
	if len(s.Top3NaverRecallPerCat) != len(ExpectedCategoryDistribution) {
		t.Errorf("placeholder per-category map has %d buckets, want %d",
			len(s.Top3NaverRecallPerCat), len(ExpectedCategoryDistribution))
	}
	// Placeholder metrics must be zeroed — it is NOT a real baseline.
	if s.MeanKappa != 0 || s.Top3NaverRecall != 0 {
		t.Errorf("placeholder metrics must be zeroed; got kappa=%v recall=%v", s.MeanKappa, s.Top3NaverRecall)
	}
}
