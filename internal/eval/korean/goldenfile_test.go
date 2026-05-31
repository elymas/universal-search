package korean

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// goldenSetPath resolves tests/eval/korean/golden-set.jsonl relative to the
// repository root (this file lives at internal/eval/korean/).
func goldenSetPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(root, "tests", "eval", "korean", "golden-set.jsonl")
}

func TestGoldenSetFile_ParsesAndValidates(t *testing.T) {
	t.Parallel()
	f, err := os.Open(goldenSetPath(t))
	if err != nil {
		t.Fatalf("open golden set: %v", err)
	}
	defer f.Close()

	queries, err := LoadGoldenSet(f)
	if err != nil {
		t.Fatalf("golden-set.jsonl failed schema validation: %v", err)
	}
	if len(queries) != GoldenSetSize {
		t.Fatalf("golden set has %d queries, want %d", len(queries), GoldenSetSize)
	}

	counts := map[Category]int{}
	for _, q := range queries {
		counts[q.Category]++
	}
	for cat, want := range ExpectedCategoryDistribution {
		if counts[cat] != want {
			t.Errorf("category %q: got %d, want %d", cat, counts[cat], want)
		}
	}
}

// TestGoldenSetFile_NoPhantomSourceIDs is redundant with the loader (which
// rejects phantoms) but documents the invariant explicitly at the file level.
func TestGoldenSetFile_NoPhantomSourceIDs(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(goldenSetPath(t))
	if err != nil {
		t.Fatalf("read golden set: %v", err)
	}
	phantoms := []string{
		`"naver-news"`, `"naver-blog"`, `"naver-shopping"`,
		`"naver-academic"`, `"daum-news"`, `"korea-news-crawler"`,
	}
	for _, p := range phantoms {
		if strings.Contains(string(data), p) {
			t.Errorf("golden set contains phantom SourceID %s", p)
		}
	}
}

// TestGoldenSetFile_NoPII applies the same PII patterns the CI gate uses.
func TestGoldenSetFile_NoPII(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(goldenSetPath(t))
	if err != nil {
		t.Fatalf("read golden set: %v", err)
	}
	patterns := map[string]*regexp.Regexp{
		"email":        regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
		"korean_phone": regexp.MustCompile(`01[016789]-?\d{3,4}-?\d{4}`),
	}
	for name, re := range patterns {
		if loc := re.FindString(string(data)); loc != "" {
			t.Errorf("PII pattern %q matched %q in golden set", name, loc)
		}
	}
}
