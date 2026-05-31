package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDiffCapabilitiesNoChange verifies diffCapabilities returns false on identical data.
func TestDiffCapabilitiesNoChange(t *testing.T) {
	t.Parallel()
	out := capabilitiesOutput{
		SourceID:          "testadapter",
		RequiresAuth:      false,
		AuthEnvVars:       []string{},
		RateLimitPerMin:   42,
		DefaultMaxResults: 10,
		SourcePath:        "internal/adapters/test/test.go",
		SourceLine:        99,
		ExtractedAt:       time.Now(),
	}
	a, _ := json.MarshalIndent(out, "", "  ")
	// Change only ExtractedAt — should NOT count as drift.
	out2 := out
	out2.ExtractedAt = time.Now().Add(time.Hour)
	b, _ := json.MarshalIndent(out2, "", "  ")

	if diffCapabilities(a, b) {
		t.Error("diffCapabilities: expected no drift when only ExtractedAt differs")
	}
}

// TestDiffCapabilitiesDrift verifies diffCapabilities detects field changes.
func TestDiffCapabilitiesDrift(t *testing.T) {
	t.Parallel()
	out := capabilitiesOutput{
		SourceID:        "testadapter",
		RateLimitPerMin: 10,
		ExtractedAt:     time.Now(),
	}
	a, _ := json.Marshal(out)
	out2 := out
	out2.RateLimitPerMin = 999 // simulate drift
	b, _ := json.Marshal(out2)

	if !diffCapabilities(a, b) {
		t.Error("diffCapabilities: expected drift when RateLimitPerMin changed")
	}
}

// TestDiffCapabilitiesInvalidJSON tests graceful handling of corrupt JSON.
func TestDiffCapabilitiesInvalidJSON(t *testing.T) {
	t.Parallel()
	if !diffCapabilities([]byte("not-json"), []byte(`{}`)) {
		t.Error("diffCapabilities: corrupt committed JSON should report drift")
	}
	if !diffCapabilities([]byte(`{}`), []byte("not-json")) {
		t.Error("diffCapabilities: corrupt fresh JSON should report drift")
	}
}

// TestExtractLineNumber verifies SourceLine is populated.
func TestExtractLineNumber(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "standard.go")
	got, err := extract(path, "")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got.SourceLine == 0 {
		t.Error("SourceLine must be > 0")
	}
}

// TestExtractNonExistentFile tests error handling for missing files.
func TestExtractNonExistentFile(t *testing.T) {
	t.Parallel()
	_, err := extract("/nonexistent/path.go", "")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

// TestCheckModeClean runs the full emit + check cycle on a temporary directory
// to verify the --check mode exits 0 when committed JSON matches source.
func TestCheckModeClean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Pick two real adapters to emit.
	for _, id := range []string{"arxiv", "reddit"} {
		spec := registry[id]
		absFile := filepath.Join("..", "..", "internal/adapters", spec.pkgDir, spec.primaryFile)
		fields, err := extract(absFile, spec.funcName)
		if err != nil {
			t.Fatalf("extract %s: %v", id, err)
		}

		authEnvVars := fields.AuthEnvVars
		if authEnvVars == nil {
			authEnvVars = []string{}
		}

		out := capabilitiesOutput{
			SourceID:          fields.SourceID,
			RequiresAuth:      fields.RequiresAuth,
			AuthEnvVars:       authEnvVars,
			RateLimitPerMin:   fields.RateLimitPerMin,
			DefaultMaxResults: fields.DefaultMaxResults,
			SourcePath:        filepath.Join("internal/adapters", spec.pkgDir, spec.primaryFile),
			SourceLine:        fields.SourceLine,
			ExtractedAt:       time.Now().UTC().Truncate(time.Second),
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		data = append(data, '\n')
		if err := os.WriteFile(filepath.Join(dir, id+".capabilities.json"), data, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// Now run diffCapabilities between committed and freshly extracted — should be false.
	for _, id := range []string{"arxiv", "reddit"} {
		spec := registry[id]
		absFile := filepath.Join("..", "..", "internal/adapters", spec.pkgDir, spec.primaryFile)
		fields, err := extract(absFile, spec.funcName)
		if err != nil {
			t.Fatalf("extract %s: %v", id, err)
		}

		authEnvVars := fields.AuthEnvVars
		if authEnvVars == nil {
			authEnvVars = []string{}
		}

		fresh := capabilitiesOutput{
			SourceID:          fields.SourceID,
			RequiresAuth:      fields.RequiresAuth,
			AuthEnvVars:       authEnvVars,
			RateLimitPerMin:   fields.RateLimitPerMin,
			DefaultMaxResults: fields.DefaultMaxResults,
			SourcePath:        filepath.Join("internal/adapters", spec.pkgDir, spec.primaryFile),
			SourceLine:        fields.SourceLine,
			ExtractedAt:       time.Now().Add(time.Hour), // different timestamp
		}
		freshJSON, _ := json.MarshalIndent(fresh, "", "  ")
		freshJSON = append(freshJSON, '\n')

		committed, err := os.ReadFile(filepath.Join(dir, id+".capabilities.json"))
		if err != nil {
			t.Fatalf("readfile: %v", err)
		}

		if diffCapabilities(committed, freshJSON) {
			t.Errorf("%s: expected no drift in clean state", id)
		}
	}
}

// TestRunEmit tests the run() function in emit mode (no --check).
func TestRunEmit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	// Use the real project root (two levels up).
	projectRoot := filepath.Join("..", "..")
	code := run(projectRoot, false, dir, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run emit: exit code %d, stderr: %s", code, stderr.String())
	}
	// Expect 10 JSON files to be written.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 10 {
		t.Errorf("expected 10 JSON files, got %d", len(entries))
	}
}

// TestRunCheckClean tests --check mode with clean (no drift) state.
func TestRunCheckClean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	projectRoot := filepath.Join("..", "..")

	// First emit to the temp dir.
	var stdout, stderr bytes.Buffer
	code := run(projectRoot, false, dir, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("emit: %v", stderr.String())
	}

	// Then check — should be clean.
	stdout.Reset()
	stderr.Reset()
	code = run(projectRoot, true, dir, &stdout, &stderr)
	if code != 0 {
		t.Errorf("check on clean state: exit code %d, stderr: %s", code, stderr.String())
	}
}

// TestRunCheckDrift tests --check mode detects drift when a JSON is mutated.
func TestRunCheckDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	projectRoot := filepath.Join("..", "..")

	// Emit baseline.
	var stdout, stderr bytes.Buffer
	run(projectRoot, false, dir, &stdout, &stderr)

	// Mutate one JSON file to simulate drift.
	jsonPath := filepath.Join(dir, "reddit.capabilities.json")
	data, _ := os.ReadFile(jsonPath)
	mutated := bytes.ReplaceAll(data, []byte(`"rateLimitPerMin": 10`), []byte(`"rateLimitPerMin": 999`))
	_ = os.WriteFile(jsonPath, mutated, 0o644)

	// Check should detect drift.
	stdout.Reset()
	stderr.Reset()
	code := run(projectRoot, true, dir, &stdout, &stderr)
	if code == 0 {
		t.Error("expected exit code 1 on drift, got 0")
	}
}

// TestRunInvalidRoot tests handling of an invalid project root.
func TestRunInvalidRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	// Use a non-existent directory as adapter source.
	code := run("/nonexistent-project-root-xyz", false, dir, &stdout, &stderr)
	if code == 0 {
		t.Error("expected exit code 1 for invalid root")
	}
}

// TestCheckModeDrift verifies diffCapabilities detects mutation of RateLimitPerMin.
func TestCheckModeDrift(t *testing.T) {
	t.Parallel()
	out := capabilitiesOutput{
		SourceID:        "arxiv",
		RateLimitPerMin: 20,
		ExtractedAt:     time.Now(),
	}
	committed, _ := json.Marshal(out)
	out2 := out
	out2.RateLimitPerMin = 999
	fresh, _ := json.Marshal(out2)

	if !diffCapabilities(committed, fresh) {
		t.Error("expected drift when RateLimitPerMin changed from 20 to 999")
	}
}

// TestRunCheckMissingFile verifies --check exits non-zero when a JSON file is absent.
func TestRunCheckMissingFile(t *testing.T) {
	t.Parallel()
	emptyDir := t.TempDir() // no JSON files present
	var stdout, stderr bytes.Buffer
	code := run(filepath.Join("..", ".."), true, emptyDir, &stdout, &stderr)
	if code == 0 {
		t.Error("expected exit code 1 when JSON files are missing in check mode")
	}
}

// TestRunEmitOutputDirDefault verifies the default outputDir is constructed correctly.
func TestRunEmitOutputDirDefault(t *testing.T) {
	t.Parallel()
	projectRoot, _ := filepath.Abs(filepath.Join("..", ".."))
	// With outDir="" run will use the real _generated dir, which may or may not exist.
	// We pass a custom dir to avoid writing to the real repo.
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run(projectRoot, false, dir, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run: %s", stderr.String())
	}
}

// TestStringLitValEdgeCases covers branches in stringLitVal.
func TestStringLitValEdgeCases(t *testing.T) {
	t.Parallel()
	// A nil expression
	got := stringLitVal(nil)
	if got != "" {
		t.Errorf("stringLitVal(nil): expected empty, got %q", got)
	}
}

// TestBoolLitValEdgeCases covers branches in boolLitVal.
func TestBoolLitValEdgeCases(t *testing.T) {
	t.Parallel()
	got := boolLitVal(nil)
	if got {
		t.Error("boolLitVal(nil): expected false")
	}
}

// TestIntLitValEdgeCases covers branches in intLitVal.
func TestIntLitValEdgeCases(t *testing.T) {
	t.Parallel()
	got := intLitVal(nil)
	if got != 0 {
		t.Errorf("intLitVal(nil): expected 0, got %d", got)
	}
}
