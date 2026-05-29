package golden_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elymas/universal-search/internal/eval/golden"
)

// TestLoadQueriesMissingFile verifies a clear error on a missing file.
func TestLoadQueriesMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := golden.LoadQueries(filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Fatal("expected error for missing queries file")
	}
}

// TestLoadQueriesBadJSON verifies a parse error names the line.
func TestLoadQueriesBadJSON(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "q.jsonl")
	if err := os.WriteFile(p, []byte("{not json}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := golden.LoadQueries(p); err == nil {
		t.Fatal("expected parse error for malformed JSON line")
	}
}

// TestLoadQueriesSkipsBlankLines verifies blank lines are tolerated.
func TestLoadQueriesSkipsBlankLines(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "q.jsonl")
	content := `{"id":"Q1","query":"a","locale":"en","category":"factual"}

{"id":"Q2","query":"b","locale":"ko","category":"korean"}
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := golden.LoadQueries(p)
	if err != nil {
		t.Fatalf("LoadQueries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 records (blank skipped), got %d", len(got))
	}
}

// TestLoadCorpusMissingDir verifies an error on a missing corpus dir.
func TestLoadCorpusMissingDir(t *testing.T) {
	t.Parallel()
	if _, err := golden.LoadCorpus(filepath.Join(t.TempDir(), "nodir")); err == nil {
		t.Fatal("expected error for missing corpus dir")
	}
}

// TestLoadCorpusBadJSON verifies a parse error names the file.
func TestLoadCorpusBadJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{nope}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := golden.LoadCorpus(dir); err == nil {
		t.Fatal("expected parse error for malformed corpus fixture")
	}
}

// TestLoadCorpusIgnoresNonJSON verifies non-.json files are skipped.
func TestLoadCorpusIgnoresNonJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := golden.LoadCorpus(dir)
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 docs, got %d", len(got))
	}
}

// TestLoadOverridesMissingFileIsEmpty verifies a missing file yields nil.
func TestLoadOverridesMissingFileIsEmpty(t *testing.T) {
	t.Parallel()
	got, err := golden.LoadOverrides(filepath.Join(t.TempDir(), "no.json"))
	if err != nil {
		t.Fatalf("expected nil error for missing overrides, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil overrides, got %#v", got)
	}
}

// TestLoadOverridesBadJSON verifies a parse error on malformed JSON.
func TestLoadOverridesBadJSON(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "o.json")
	if err := os.WriteFile(p, []byte("{not an array}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := golden.LoadOverrides(p); err == nil {
		t.Fatal("expected parse error for malformed overrides")
	}
}
