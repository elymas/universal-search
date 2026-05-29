package korean

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// createTempJSONL creates a temporary JSONL file with the given lines.
func createTempJSONL(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test-golden.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp jsonl: %v", err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, line := range lines {
		if _, err := w.WriteString(line + "\n"); err != nil {
			t.Fatalf("write temp jsonl line: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("flush temp jsonl: %v", err)
	}
	return path
}

// makeQueryJSON builds a JSON object string for a golden query.
func makeQueryJSON(id, text, category, lang, routerClass string, naverRelevant bool, sources []string) string {
	q := GoldenQuery{
		QueryID:               id,
		QueryText:             text,
		Category:              category,
		ExpectedLang:          lang,
		ExpectedRouterClass:   routerClass,
		ExpectedNaverRelevant: naverRelevant,
		ExpectedSources:       sources,
	}
	b, _ := json.Marshal(q)
	return string(b)
}

// ---------- Loader tests (RED phase) ----------

func TestLoadGoldenSet_50Objects(t *testing.T) {
	// Build 50 queries with correct category distribution.
	var lines []string
	cats := map[string]int{
		"news": 12, "blog": 10, "shopping": 8,
		"academic-tech": 8, "code-mixed": 6, "cultural": 6,
	}
	i := 0
	for cat, count := range cats {
		for j := range count {
			id := fmt.Sprintf("KR-%03d", i+j+1)
			lang := "ko"
			router := "korean"
			if cat == "code-mixed" {
				lang = "mixed"
				router = "mixed"
			}
			lines = append(lines, makeQueryJSON(id, "test query "+id, cat, lang, router, true, []string{"naver-news"}))
		}
		i += count
	}

	path := createTempJSONL(t, lines)
	queries, err := LoadGoldenSet(path)
	if err != nil {
		t.Fatalf("LoadGoldenSet returned error: %v", err)
	}
	if got := len(queries); got != 50 {
		t.Errorf("got %d queries, want 50", got)
	}
}

func TestLoadGoldenSet_CategoryDistribution(t *testing.T) {
	var lines []string
	cats := map[string]int{
		"news": 12, "blog": 10, "shopping": 8,
		"academic-tech": 8, "code-mixed": 6, "cultural": 6,
	}
	i := 0
	for cat, count := range cats {
		for j := range count {
			id := fmt.Sprintf("KR-%03d", i+j+1)
			lang := "ko"
			router := "korean"
			if cat == "code-mixed" {
				lang = "mixed"
				router = "mixed"
			}
			lines = append(lines, makeQueryJSON(id, "q "+id, cat, lang, router, true, []string{"naver-news"}))
		}
		i += count
	}

	path := createTempJSONL(t, lines)
	queries, err := LoadGoldenSet(path)
	if err != nil {
		t.Fatalf("LoadGoldenSet returned error: %v", err)
	}

	counts := make(map[string]int)
	for _, q := range queries {
		counts[q.Category]++
	}
	expected := map[string]int{
		"news": 12, "blog": 10, "shopping": 8,
		"academic-tech": 8, "code-mixed": 6, "cultural": 6,
	}
	for cat, want := range expected {
		if got := counts[cat]; got != want {
			t.Errorf("category %q: got %d, want %d", cat, got, want)
		}
	}
}

func TestLoadGoldenSet_AllRequiredFields(t *testing.T) {
	// A query with a missing required field should be detected.
	incomplete := `{"query_id":"KR-001","query_text":"test","category":"news"}`
	path := createTempJSONL(t, []string{incomplete})

	_, err := LoadGoldenSet(path)
	if err == nil {
		t.Error("expected error for missing required fields, got nil")
	}
}

func TestLoadGoldenSet_InvalidJSON_ReturnsError(t *testing.T) {
	path := createTempJSONL(t, []string{"{invalid json"})
	_, err := LoadGoldenSet(path)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestLoadGoldenSet_EmptyFile_ReturnsError(t *testing.T) {
	path := createTempJSONL(t, []string{})
	_, err := LoadGoldenSet(path)
	if err == nil {
		t.Error("expected error for empty file, got nil")
	}
}

func TestLoadGoldenSet_WrongCount_ReturnsError(t *testing.T) {
	// Only 49 queries — should fail.
	var lines []string
	cats := map[string]int{
		"news": 11, "blog": 10, "shopping": 8,
		"academic-tech": 8, "code-mixed": 6, "cultural": 6,
	}
	i := 0
	for cat, count := range cats {
		for j := range count {
			id := fmt.Sprintf("KR-%03d", i+j+1)
			lines = append(lines, makeQueryJSON(id, "q "+id, cat, "ko", "korean", true, []string{"naver-news"}))
		}
		i += count
	}
	path := createTempJSONL(t, lines)
	_, err := LoadGoldenSet(path)
	if err == nil {
		t.Error("expected error for wrong query count, got nil")
	}
}

func TestLoadGoldenSet_InvalidCategory_ReturnsError(t *testing.T) {
	line := makeQueryJSON("KR-001", "test", "invalid-cat", "ko", "korean", true, []string{"naver-news"})
	path := createTempJSONL(t, []string{line})
	// Pad to 50 with valid entries to test category validation.
	var lines []string
	lines = append(lines, line)
	cats := map[string]int{
		"news": 12, "blog": 10, "shopping": 8,
		"academic-tech": 8, "code-mixed": 6, "cultural": 4,
	}
	i := 1
	for cat, count := range cats {
		for j := range count {
			id := fmt.Sprintf("KR-%03d", i+j+1)
			lines = append(lines, makeQueryJSON(id, "q "+id, cat, "ko", "korean", true, []string{"naver-news"}))
		}
		i += count
	}
	path = createTempJSONL(t, lines)
	_, err := LoadGoldenSet(path)
	if err == nil {
		t.Error("expected error for invalid category, got nil")
	}
}

func TestLoadGoldenSet_BlankLinesSkipped(t *testing.T) {
	var lines []string
	cats := map[string]int{
		"news": 12, "blog": 10, "shopping": 8,
		"academic-tech": 8, "code-mixed": 6, "cultural": 6,
	}
	i := 0
	for cat, count := range cats {
		for j := range count {
			id := fmt.Sprintf("KR-%03d", i+j+1)
			lang := "ko"
			router := "korean"
			if cat == "code-mixed" {
				lang = "mixed"
				router = "mixed"
			}
			lines = append(lines, makeQueryJSON(id, "q "+id, cat, lang, router, true, []string{"naver-news"}))
		}
		i += count
	}
	// Insert blank lines — should be ignored.
	lines = append([]string{"", ""}, lines...)
	path := createTempJSONL(t, lines)
	queries, err := LoadGoldenSet(path)
	if err != nil {
		t.Fatalf("LoadGoldenSet with blank lines: %v", err)
	}
	if got := len(queries); got != 50 {
		t.Errorf("got %d queries with blank lines, want 50", got)
	}
}
