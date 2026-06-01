package korean

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleSnapshot(tag string) Snapshot {
	return Snapshot{
		ReleaseTag:               tag,
		RoundDate:                "2026-05-30",
		RaterIDs:                 []string{"R1", "R2", "R3"},
		MeanKappa:                0.72,
		Top3NaverRecall:          0.85,
		Top3NaverRecallPerCat:    map[Category]float64{CategoryNews: 0.9, CategoryBlog: 0.8},
		MRRTop10:                 0.78,
		MeanRankingScore:         4.2,
		RouterClassAccuracyMixed: 0.83,
		TokenizerVersion:         "mecab-ko 1.0.0",
		AdapterVersions:          map[string]string{"naver": "v0.1.0", "koreanews": "v0.1.0"},
		GoldenSetSHA256:          "abc123",
	}
}

func TestGoldenSetSHA256_Deterministic(t *testing.T) {
	t.Parallel()
	content := "line1\nline2\n"
	h1, err := GoldenSetSHA256(strings.NewReader(content))
	if err != nil {
		t.Fatalf("GoldenSetSHA256: %v", err)
	}
	h2, _ := GoldenSetSHA256(strings.NewReader(content))
	if h1 != h2 {
		t.Errorf("sha256 not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("sha256 hex length = %d, want 64", len(h1))
	}
}

func TestWriteSnapshot_ValidWrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path, err := WriteSnapshot(dir, sampleSnapshot("v1.0.0"), true)
	if err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("snapshot file not created: %v", err)
	}
	data, _ := os.ReadFile(path)
	for _, field := range []string{"release_tag", "golden_set_sha256", "tokenizer_version", "top3_naver_recall_per_category", "adapter_versions"} {
		if !strings.Contains(string(data), field) {
			t.Errorf("snapshot JSON missing field %q", field)
		}
	}
}

func TestWriteSnapshot_AppendOnly_RejectsOverwrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := WriteSnapshot(dir, sampleSnapshot("v1.0.0"), true); err != nil {
		t.Fatalf("first write: %v", err)
	}
	_, err := WriteSnapshot(dir, sampleSnapshot("v1.0.0"), true)
	if !errors.Is(err, ErrSnapshotExists) {
		t.Errorf("overwrite should return ErrSnapshotExists, got %v", err)
	}
}

func TestWriteSnapshot_InvalidRoundRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := WriteSnapshot(dir, sampleSnapshot("v1.0.0"), false)
	if !errors.Is(err, ErrRoundInvalid) {
		t.Errorf("invalid round should return ErrRoundInvalid, got %v", err)
	}
	// No file should have been created.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			t.Errorf("invalid round produced a snapshot file %q", e.Name())
		}
	}
}

func TestWriteSnapshot_RejectsPhantomAdapterID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := sampleSnapshot("v1.0.0")
	s.AdapterVersions = map[string]string{"naver-news": "v0.1.0"} // phantom
	_, err := WriteSnapshot(dir, s, true)
	if !errors.Is(err, ErrPhantomAdapterID) {
		t.Errorf("phantom adapter id should be rejected, got %v", err)
	}
}

func TestWriteSnapshot_MissingRequiredField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := sampleSnapshot("v1.0.0")
	s.GoldenSetSHA256 = ""
	if _, err := WriteSnapshot(dir, s, true); err == nil {
		t.Error("missing golden_set_sha256 should error")
	}
}

// TestSnapshotValidate_AllMissingFieldBranches covers each required-field
// rejection in Snapshot.validate independently.
func TestSnapshotValidate_AllMissingFieldBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{"missing release_tag", func(s *Snapshot) { s.ReleaseTag = "" }},
		{"missing golden_set_sha256", func(s *Snapshot) { s.GoldenSetSHA256 = "" }},
		{"missing tokenizer_version", func(s *Snapshot) { s.TokenizerVersion = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := sampleSnapshot("v1.0.0")
			tt.mutate(&s)
			if err := s.validate(); err == nil {
				t.Errorf("validate(%s) = nil, want error", tt.name)
			}
		})
	}

	// The fully-populated sample must validate cleanly (success path).
	valid := sampleSnapshot("v1.0.0")
	if err := valid.validate(); err != nil {
		t.Errorf("valid snapshot rejected: %v", err)
	}
}

func TestWriteSnapshot_Retention(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tags := []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0", "v1.4.0"}
	for _, tag := range tags {
		if _, err := WriteSnapshot(dir, sampleSnapshot(tag), true); err != nil {
			t.Fatalf("write %s: %v", tag, err)
		}
	}
	// 5 written → 4 retained live, 1 archived.
	entries, _ := os.ReadDir(dir)
	var liveJSON int
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			liveJSON++
		}
	}
	if liveJSON != SnapshotRetention {
		t.Errorf("live snapshots = %d, want %d", liveJSON, SnapshotRetention)
	}
	archived, err := os.ReadDir(filepath.Join(dir, "archive"))
	if err != nil {
		t.Fatalf("archive dir missing: %v", err)
	}
	if len(archived) != 1 {
		t.Errorf("archived = %d, want 1", len(archived))
	}
}
