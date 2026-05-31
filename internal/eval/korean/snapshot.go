package korean

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// SnapshotRetention is the number of most-recent baseline snapshots kept in the
// live directory; older snapshots are archived, never deleted (NFR-EVAL-003).
const SnapshotRetention = 4

// ErrSnapshotExists is returned when a snapshot for the same release tag
// already exists. Snapshots are append-only — never overwritten (REQ-EVAL-008).
var ErrSnapshotExists = errors.New("korean eval: snapshot already exists for this release tag (append-only)")

// ErrRoundInvalid is returned when WriteSnapshot is asked to persist a round
// that did not reach the κ gate (REQ-EVAL-009: invalid rounds produce no
// snapshot).
var ErrRoundInvalid = errors.New("korean eval: cannot snapshot an invalid round (mean-κ < 0.6)")

// ErrPhantomAdapterID is returned when adapter_versions contains a SourceID
// that is not a registered adapter (REQ-EVAL-008).
var ErrPhantomAdapterID = errors.New("korean eval: adapter_versions contains an unregistered/phantom SourceID")

// Snapshot is a release-tagged baseline of one valid scoring round
// (REQ-EVAL-008). All numeric metrics are recorded; per-category recall is
// observational. The struct serializes to the append-only baseline JSON.
type Snapshot struct {
	ReleaseTag               string               `json:"release_tag"`
	RoundDate                string               `json:"round_date"`
	RaterIDs                 []string             `json:"rater_ids"`
	MeanKappa                float64              `json:"mean_kappa"`
	Top3NaverRecall          float64              `json:"top3_naver_recall"`
	Top3NaverRecallPerCat    map[Category]float64 `json:"top3_naver_recall_per_category"`
	MRRTop10                 float64              `json:"mrr_top10"`
	MeanRankingScore         float64              `json:"mean_ranking_score"`
	RouterClassAccuracyMixed float64              `json:"router_class_accuracy_mixed"`
	TokenizerVersion         string               `json:"tokenizer_version"`
	AdapterVersions          map[string]string    `json:"adapter_versions"`
	GoldenSetSHA256          string               `json:"golden_set_sha256"`
}

// validate ensures the snapshot carries the required gate-bearing fields and
// only registered adapter SourceIDs in adapter_versions.
func (s *Snapshot) validate() error {
	if s.ReleaseTag == "" {
		return errors.New("korean eval: snapshot missing release_tag")
	}
	if s.GoldenSetSHA256 == "" {
		return errors.New("korean eval: snapshot missing golden_set_sha256")
	}
	if s.TokenizerVersion == "" {
		return errors.New("korean eval: snapshot missing tokenizer_version")
	}
	for src := range s.AdapterVersions {
		if _, ok := registeredSourceIDs[src]; !ok {
			return fmt.Errorf("%w: %q", ErrPhantomAdapterID, src)
		}
	}
	return nil
}

// GoldenSetSHA256 computes the hex SHA256 of the golden-set bytes read from r.
// Recorded in each snapshot so a golden-set change is detectable (REQ-EVAL-008).
func GoldenSetSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("hash golden set: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// WriteSnapshot persists a snapshot for a VALID round to dir as
// {release_tag}.json. It refuses to overwrite an existing file (append-only)
// and refuses invalid rounds. After writing, it enforces the 4-snapshot
// retention policy, archiving older snapshots into dir/archive.
//
// @MX:NOTE: [AUTO] Append-only baseline writer. A snapshot is the immutable
// evidence of one valid Korean-ranking round; once written it is never
// modified (REQ-EVAL-008). Overwrite attempts return ErrSnapshotExists.
// @MX:SPEC: SPEC-EVAL-003
func WriteSnapshot(dir string, s Snapshot, roundValid bool) (string, error) {
	if !roundValid {
		return "", ErrRoundInvalid
	}
	if err := s.validate(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create snapshot dir: %w", err)
	}

	path := filepath.Join(dir, s.ReleaseTag+".json")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("%w: %s", ErrSnapshotExists, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat snapshot: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}
	data = append(data, '\n')
	// 0o444: read-only on disk reinforces the append-only invariant.
	if err := os.WriteFile(path, data, 0o444); err != nil {
		return "", fmt.Errorf("write snapshot: %w", err)
	}

	if err := enforceRetention(dir); err != nil {
		return path, fmt.Errorf("snapshot written but retention failed: %w", err)
	}
	return path, nil
}

// enforceRetention keeps the SnapshotRetention most-recent snapshots in dir and
// moves older ones into dir/archive. Recency is determined by file modification
// time, then by name for determinism. Archived snapshots are never deleted.
func enforceRetention(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read snapshot dir: %w", err)
	}

	type snap struct {
		name string
		mod  int64
	}
	var snaps []snap
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", e.Name(), err)
		}
		snaps = append(snaps, snap{name: e.Name(), mod: info.ModTime().UnixNano()})
	}
	if len(snaps) <= SnapshotRetention {
		return nil
	}

	// Newest first: most recent mod time wins; tie-break by name descending.
	sort.Slice(snaps, func(i, j int) bool {
		if snaps[i].mod != snaps[j].mod {
			return snaps[i].mod > snaps[j].mod
		}
		return snaps[i].name > snaps[j].name
	})

	archiveDir := filepath.Join(dir, "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}
	for _, s := range snaps[SnapshotRetention:] {
		src := filepath.Join(dir, s.name)
		dst := filepath.Join(archiveDir, s.name)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("archive %s: %w", s.name, err)
		}
	}
	return nil
}
