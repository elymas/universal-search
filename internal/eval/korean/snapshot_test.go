package korean

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// ---------- Snapshot tests (RED phase) ----------

func TestSnapshot_ValidRound_WritesFile(t *testing.T) {
	dir := t.TempDir()
	tag := "v1.0.0"

	// Build a valid round with mean-kappa >= 0.6.
	queries := makeTestGoldenQueries(5, 10)
	sheet1 := make([]RaterScore, 10)
	sheet2 := make([]RaterScore, 10)
	sheet3 := make([]RaterScore, 10)
	for i := range 10 {
		id := fmt.Sprintf("KR-%03d", i+1)
		sheet1[i] = RaterScore{QueryID: id, RaterID: "R1", RankingScore: 5, Top3NaverHit: true}
		sheet2[i] = RaterScore{QueryID: id, RaterID: "R2", RankingScore: 5, Top3NaverHit: true}
		sheet3[i] = RaterScore{QueryID: id, RaterID: "R3", RankingScore: 5, Top3NaverHit: true}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet1, sheet2, sheet3}}

	err := WriteSnapshot(round, queries, tag, dir, SnapshotMeta{
		TokenizerVersion: "mecab-ko-dic-2.1.1",
		AdapterVersions:  map[string]string{"naver-news": "1.0.0"},
		GoldenSetSHA256:  "abc123",
	})
	if err != nil {
		t.Fatalf("WriteSnapshot returned error: %v", err)
	}

	path := filepath.Join(dir, tag+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("snapshot file was not created")
	}
}

func TestSnapshot_InvalidRound_DoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	tag := "v0.9.0"

	// Build a round where raters completely disagree → kappa will be low.
	queries := makeTestGoldenQueries(5, 10)
	sheet1 := make([]RaterScore, 10)
	sheet2 := make([]RaterScore, 10)
	for i := range 10 {
		id := fmt.Sprintf("KR-%03d", i+1)
		sheet1[i] = RaterScore{QueryID: id, RaterID: "R1", RankingScore: 1 + i%5}
		sheet2[i] = RaterScore{QueryID: id, RaterID: "R2", RankingScore: 5 - i%5}
	}
	// Only 2 raters — but we need to force low kappa.
	// Use completely inverted ratings.
	round := Round{RaterSheets: [][]RaterScore{sheet1, sheet2}}

	err := WriteSnapshot(round, queries, tag, dir, SnapshotMeta{
		TokenizerVersion: "mecab-ko-dic-2.1.1",
		AdapterVersions:  map[string]string{"naver-news": "1.0.0"},
		GoldenSetSHA256:  "abc123",
	})
	// Should succeed but mark round as invalid and NOT write the snapshot.
	if err != nil {
		t.Fatalf("WriteSnapshot returned unexpected error: %v", err)
	}

	path := filepath.Join(dir, tag+".json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("snapshot file should NOT be created for invalid round")
	}
}

func TestSnapshot_AppendOnly_RejectsOverwrite(t *testing.T) {
	dir := t.TempDir()
	tag := "v1.0.0"

	// Create a snapshot first.
	queries := makeTestGoldenQueries(5, 10)
	sheet1 := make([]RaterScore, 10)
	sheet2 := make([]RaterScore, 10)
	sheet3 := make([]RaterScore, 10)
	for i := range 10 {
		id := fmt.Sprintf("KR-%03d", i+1)
		sheet1[i] = RaterScore{QueryID: id, RaterID: "R1", RankingScore: 5, Top3NaverHit: true}
		sheet2[i] = RaterScore{QueryID: id, RaterID: "R2", RankingScore: 5, Top3NaverHit: true}
		sheet3[i] = RaterScore{QueryID: id, RaterID: "R3", RankingScore: 5, Top3NaverHit: true}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet1, sheet2, sheet3}}
	meta := SnapshotMeta{
		TokenizerVersion: "mecab-ko-dic-2.1.1",
		AdapterVersions:  map[string]string{"naver-news": "1.0.0"},
		GoldenSetSHA256:  "abc123",
	}

	err := WriteSnapshot(round, queries, tag, dir, meta)
	if err != nil {
		t.Fatalf("first WriteSnapshot: %v", err)
	}

	// Try to write again with the same tag — should fail.
	err = WriteSnapshot(round, queries, tag, dir, meta)
	if err == nil {
		t.Error("expected error for overwrite, got nil")
	}
}

func TestSnapshot_RetentionPolicy_KeepsLatestFour(t *testing.T) {
	dir := t.TempDir()

	// Create 5 snapshots.
	queries := makeTestGoldenQueries(5, 10)
	sheet1 := make([]RaterScore, 10)
	sheet2 := make([]RaterScore, 10)
	sheet3 := make([]RaterScore, 10)
	for i := range 10 {
		id := fmt.Sprintf("KR-%03d", i+1)
		sheet1[i] = RaterScore{QueryID: id, RaterID: "R1", RankingScore: 5, Top3NaverHit: true}
		sheet2[i] = RaterScore{QueryID: id, RaterID: "R2", RankingScore: 5, Top3NaverHit: true}
		sheet3[i] = RaterScore{QueryID: id, RaterID: "R3", RankingScore: 5, Top3NaverHit: true}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet1, sheet2, sheet3}}

	tags := []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0", "v1.4.0"}
	for _, tag := range tags {
		err := WriteSnapshot(round, queries, tag, dir, SnapshotMeta{
			TokenizerVersion: "mecab-ko-dic-2.1.1",
			AdapterVersions:  map[string]string{"naver-news": "1.0.0"},
			GoldenSetSHA256:  "abc123",
		})
		if err != nil {
			t.Fatalf("WriteSnapshot %s: %v", tag, err)
		}
	}

	// The oldest snapshot (v1.0.0) should be moved to archive/.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	snapshotCount := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			snapshotCount++
		}
	}
	if snapshotCount != 4 {
		t.Errorf("found %d snapshots in dir, want 4", snapshotCount)
	}

	// Check archive directory.
	archiveDir := filepath.Join(dir, "archive")
	archiveEntries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("read archive dir: %v", err)
	}
	archiveCount := 0
	for _, e := range archiveEntries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			archiveCount++
		}
	}
	if archiveCount != 1 {
		t.Errorf("found %d archived snapshots, want 1", archiveCount)
	}
}

func TestSnapshot_JSONSchema_AllFields(t *testing.T) {
	dir := t.TempDir()
	tag := "v1.0.0"

	queries := makeTestGoldenQueries(5, 10)
	sheet1 := make([]RaterScore, 10)
	sheet2 := make([]RaterScore, 10)
	sheet3 := make([]RaterScore, 10)
	for i := range 10 {
		id := fmt.Sprintf("KR-%03d", i+1)
		sheet1[i] = RaterScore{QueryID: id, RaterID: "R1", RankingScore: 5, Top3NaverHit: true}
		sheet2[i] = RaterScore{QueryID: id, RaterID: "R2", RankingScore: 5, Top3NaverHit: true}
		sheet3[i] = RaterScore{QueryID: id, RaterID: "R3", RankingScore: 5, Top3NaverHit: true}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet1, sheet2, sheet3}}
	meta := SnapshotMeta{
		TokenizerVersion: "mecab-ko-dic-2.1.1",
		AdapterVersions:  map[string]string{"naver-news": "1.0.0", "naver-blog": "1.0.0"},
		GoldenSetSHA256:  "deadbeef",
	}

	err := WriteSnapshot(round, queries, tag, dir, meta)
	if err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, tag+".json"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	// REQ-EVAL-008: Verify all required fields are present.
	requiredFields := []string{
		"release_tag", "round_date", "rater_ids", "mean_kappa",
		"top3_naver_recall", "per_category", "mrr_top10",
		"mean_ranking_score", "router_class_accuracy_mixed",
		"tokenizer_version", "adapter_versions", "golden_set_sha256",
	}
	for _, field := range requiredFields {
		if _, ok := snapshot[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestSnapshot_GoldenSetSHA256_Deterministic(t *testing.T) {
	dir := t.TempDir()

	queries := makeTestGoldenQueries(5, 10)
	sheet1 := make([]RaterScore, 10)
	sheet2 := make([]RaterScore, 10)
	sheet3 := make([]RaterScore, 10)
	for i := range 10 {
		id := fmt.Sprintf("KR-%03d", i+1)
		sheet1[i] = RaterScore{QueryID: id, RaterID: "R1", RankingScore: 5, Top3NaverHit: true}
		sheet2[i] = RaterScore{QueryID: id, RaterID: "R2", RankingScore: 5, Top3NaverHit: true}
		sheet3[i] = RaterScore{QueryID: id, RaterID: "R3", RankingScore: 5, Top3NaverHit: true}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet1, sheet2, sheet3}}
	meta := SnapshotMeta{
		TokenizerVersion: "mecab-ko-dic-2.1.1",
		AdapterVersions:  map[string]string{"naver-news": "1.0.0"},
		GoldenSetSHA256:  "fixed-hash-value",
	}

	// Write two snapshots with same golden set hash.
	err := WriteSnapshot(round, queries, "v1.0.0", dir, meta)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}

	data1, _ := os.ReadFile(filepath.Join(dir, "v1.0.0.json"))
	var snap1 struct {
		GoldenSetSHA256 string `json:"golden_set_sha256"`
	}
	json.Unmarshal(data1, &snap1)

	if snap1.GoldenSetSHA256 != "fixed-hash-value" {
		t.Errorf("golden_set_sha256 = %q, want %q", snap1.GoldenSetSHA256, "fixed-hash-value")
	}
}

func TestSnapshot_MeanRankingScore(t *testing.T) {
	dir := t.TempDir()

	queries := makeTestGoldenQueries(5, 5)
	sheet1 := []RaterScore{
		{QueryID: "KR-001", RaterID: "R1", RankingScore: 4},
		{QueryID: "KR-002", RaterID: "R1", RankingScore: 5},
		{QueryID: "KR-003", RaterID: "R1", RankingScore: 3},
		{QueryID: "KR-004", RaterID: "R1", RankingScore: 4},
		{QueryID: "KR-005", RaterID: "R1", RankingScore: 4},
	}
	sheet2 := []RaterScore{
		{QueryID: "KR-001", RaterID: "R2", RankingScore: 4},
		{QueryID: "KR-002", RaterID: "R2", RankingScore: 5},
		{QueryID: "KR-003", RaterID: "R2", RankingScore: 3},
		{QueryID: "KR-004", RaterID: "R2", RankingScore: 4},
		{QueryID: "KR-005", RaterID: "R2", RankingScore: 4},
	}
	sheet3 := []RaterScore{
		{QueryID: "KR-001", RaterID: "R3", RankingScore: 4},
		{QueryID: "KR-002", RaterID: "R3", RankingScore: 5},
		{QueryID: "KR-003", RaterID: "R3", RankingScore: 3},
		{QueryID: "KR-004", RaterID: "R3", RankingScore: 4},
		{QueryID: "KR-005", RaterID: "R3", RankingScore: 4},
	}
	round := Round{RaterSheets: [][]RaterScore{sheet1, sheet2, sheet3}}

	err := WriteSnapshot(round, queries, "v1.0.0", dir, SnapshotMeta{
		TokenizerVersion: "mecab-ko-dic-2.1.1",
		AdapterVersions:  map[string]string{"naver-news": "1.0.0"},
		GoldenSetSHA256:  "abc",
	})
	if err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "v1.0.0.json"))
	var snap struct {
		MeanRankingScore float64 `json:"mean_ranking_score"`
	}
	json.Unmarshal(data, &snap)

	// Mean of all scores across 3 raters: (4+5+3+4+4)*3 / 15 = 20*3/15 = 4.0
	want := 4.0
	if math.Abs(snap.MeanRankingScore-want) > 1e-6 {
		t.Errorf("mean_ranking_score = %.6f, want %.6f", snap.MeanRankingScore, want)
	}
}
