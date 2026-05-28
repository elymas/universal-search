package korean

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// KappaThreshold is the minimum Light's mean-kappa for a valid round.
// REQ-EVAL-004: 0.6 (substantial agreement per Landis-Koch 1977).
const KappaThreshold = 0.6

// MaxRetainedSnapshots is the maximum number of snapshots kept in the main directory.
// NFR-EVAL-003: 4 most recent. Older snapshots are archived.
const MaxRetainedSnapshots = 4

// SnapshotMeta contains metadata passed by the caller for the snapshot.
type SnapshotMeta struct {
	TokenizerVersion string            `json:"tokenizer_version"`
	AdapterVersions  map[string]string `json:"adapter_versions"`
	GoldenSetSHA256  string            `json:"golden_set_sha256"`
}

// BaselineSnapshot represents a complete baseline snapshot JSON.
// REQ-EVAL-008: All fields are required.
type BaselineSnapshot struct {
	ReleaseTag             string            `json:"release_tag"`
	RoundDate              string            `json:"round_date"`
	RaterIDs               []string          `json:"rater_ids"`
	MeanKappa              float64           `json:"mean_kappa"`
	Top3NaverRecall        float64           `json:"top3_naver_recall"`
	PerCategory            map[string]float64 `json:"per_category"`
	MRRTop10               float64           `json:"mrr_top10"`
	MeanRankingScore       float64           `json:"mean_ranking_score"`
	RouterClassAccuracyMixed float64         `json:"router_class_accuracy_mixed"`
	TokenizerVersion       string            `json:"tokenizer_version"`
	AdapterVersions        map[string]string `json:"adapter_versions"`
	GoldenSetSHA256        string            `json:"golden_set_sha256"`
}

// computeMeanRankingScore computes the average ranking score across all raters and queries.
func computeMeanRankingScore(round Round) float64 {
	var sum int
	var count int
	for _, sheet := range round.RaterSheets {
		for _, score := range sheet {
			sum += score.RankingScore
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count)
}

// computeRouterClassAccuracyMixed computes the percentage of code-mixed queries
// where IR-001 correctly classified as "mixed".
// REQ-EVAL-007: Router-classification-accuracy@code-mixed.
func computeRouterClassAccuracyMixed(round Round, gold []GoldenQuery) float64 {
	// Find code-mixed queries.
	var codeMixedQueries []GoldenQuery
	for _, q := range gold {
		if q.Category == "code-mixed" {
			codeMixedQueries = append(codeMixedQueries, q)
		}
	}
	if len(codeMixedQueries) == 0 {
		return 0
	}

	// Count code_switching_handling scores (proxy for correct classification).
	// If a code-mixed query has a non-nil code_switching_handling, it was
	// evaluated, meaning the rater observed the mixed content.
	if len(round.RaterSheets) == 0 {
		return 0
	}

	evaluated := 0
	for _, q := range codeMixedQueries {
		for _, score := range round.RaterSheets[0] {
			if score.QueryID == q.QueryID && score.CodeSwitchingHandling != nil {
				evaluated++
				break
			}
		}
	}

	return float64(evaluated) / float64(len(codeMixedQueries))
}

// extractRaterIDs extracts unique rater IDs from the round.
func extractRaterIDs(round Round) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, sheet := range round.RaterSheets {
		if len(sheet) > 0 {
			id := sheet[0].RaterID
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// WriteSnapshot creates a baseline snapshot JSON file for a valid scoring round.
//
// @MX:WARN: [AUTO] Append-only policy violation rejects write
// @MX:REASON: snapshots are immutable evidence; overwrite is rejected to prevent baseline tampering
func WriteSnapshot(round Round, gold []GoldenQuery, tag, dir string, meta SnapshotMeta) error {
	// Compute kappa from rater sheets.
	raterData := extractRaterScores(round)
	meanKappa, err := LightMeanKappa(raterData)
	if err != nil {
		return fmt.Errorf("compute kappa: %w", err)
	}

	// Check if round is valid (kappa >= threshold).
	if meanKappa < KappaThreshold {
		// Invalid round — do not write snapshot.
		return nil
	}

	// Check append-only: reject if file already exists.
	snapshotPath := filepath.Join(dir, tag+".json")
	if _, err := os.Stat(snapshotPath); err == nil {
		return fmt.Errorf("snapshot %s already exists (append-only policy)", tag)
	}

	// Compute all metrics.
	top3Recall := Top3NaverRecall(round, gold)
	perCat := PerCategoryRecall(round, gold)
	mrr := MRRAt10(round, gold)
	meanScore := computeMeanRankingScore(round)
	routerAccMixed := computeRouterClassAccuracyMixed(round, gold)

	snapshot := BaselineSnapshot{
		ReleaseTag:               tag,
		RoundDate:                time.Now().UTC().Format(time.RFC3339),
		RaterIDs:                 extractRaterIDs(round),
		MeanKappa:                meanKappa,
		Top3NaverRecall:          top3Recall,
		PerCategory:              perCat,
		MRRTop10:                 mrr,
		MeanRankingScore:         meanScore,
		RouterClassAccuracyMixed: routerAccMixed,
		TokenizerVersion:         meta.TokenizerVersion,
		AdapterVersions:          meta.AdapterVersions,
		GoldenSetSHA256:          meta.GoldenSetSHA256,
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	if err := os.WriteFile(snapshotPath, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}

	// Enforce retention policy.
	enforceRetentionPolicy(dir)

	return nil
}

// extractRaterScores extracts ranking score arrays from rater sheets for kappa computation.
func extractRaterScores(round Round) [][]int {
	var result [][]int
	for _, sheet := range round.RaterSheets {
		var scores []int
		for _, score := range sheet {
			scores = append(scores, score.RankingScore)
		}
		result = append(result, scores)
	}
	return result
}

// enforceRetentionPolicy ensures at most MaxRetainedSnapshots snapshots in the main dir.
// Older snapshots are moved to the archive/ subdirectory.
// NFR-EVAL-003: Keep 4 most recent, archive older.
func enforceRetentionPolicy(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// Collect snapshot files.
	var snapshots []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			snapshots = append(snapshots, e.Name())
		}
	}

	if len(snapshots) <= MaxRetainedSnapshots {
		return
	}

	// Sort by name (assuming version tags sort correctly).
	sort.Strings(snapshots)

	// Archive the oldest ones.
	archiveDir := filepath.Join(dir, "archive")
	os.MkdirAll(archiveDir, 0o755)

	toArchive := len(snapshots) - MaxRetainedSnapshots
	for i := range toArchive {
		src := filepath.Join(dir, snapshots[i])
		dst := filepath.Join(archiveDir, snapshots[i])
		os.Rename(src, dst)
	}
}

// ComputeGoldenSetSHA256 computes the SHA256 hash of a golden set JSONL file.
// Deterministic: same file content always produces the same hash.
func ComputeGoldenSetSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read golden set for SHA256: %w", err)
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}
