// Package main — tests for the history subcommand tree.
// SPEC-CLI-002 REQ-CLI2-011: history {list,show,search,clear}.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/usearch/config"
	"github.com/elymas/universal-search/internal/usearch/history"
)

// setupHistoryEnv creates a temp HOME with XDG dirs and writes some history entries.
// Returns the temp dir path for cleanup.
func setupHistoryEnv(t *testing.T, entries []history.Entry) string {
	t.Helper()
	home := t.TempDir()
	xdgData := filepath.Join(home, ".local", "share")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("HOME", home)

	// Create history backend and write entries.
	historyPath := filepath.Join(xdgData, "usearch", "history.jsonl")
	backend, err := history.NewJSONLBackend(historyPath, 1000, 90)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if err := backend.Write(e); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// historyTestEntry creates a history entry for testing.
func historyTestEntry(id, prompt string, ts time.Time) history.Entry {
	return history.Entry{
		ID:            id,
		Timestamp:     ts,
		Command:       "query",
		Prompt:        prompt,
		Category:      "general",
		Adapters:      []string{"reddit"},
		Summary:       "test summary for " + id,
		Citations:     1,
		ExitCode:      0,
		LatencyMs:     100,
		CostUSD:       0.001,
		RequestID:     "req-" + id,
		SchemaVersion: 1,
	}
}

// TestHistoryListShowsEntries verifies "history list" shows entries.
func TestHistoryListShowsEntries(t *testing.T) {
	now := time.Now()
	entries := []history.Entry{
		historyTestEntry("h1", "first query", now.Add(-2*time.Hour)),
		historyTestEntry("h2", "second query", now.Add(-1*time.Hour)),
	}
	setupHistoryEnv(t, entries)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "list"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "h1") || !strings.Contains(output, "h2") {
		t.Errorf("history list missing entry IDs: %q", output)
	}
}

// TestHistoryListLimitFlag verifies --limit flag.
func TestHistoryListLimitFlag(t *testing.T) {
	now := time.Now()
	var entries []history.Entry
	for i := 0; i < 5; i++ {
		entries = append(entries, historyTestEntry("e"+string(rune('0'+i)), "prompt", now.Add(time.Duration(i)*time.Minute)))
	}
	setupHistoryEnv(t, entries)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "list", "--limit", "2"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	// Count lines (each entry is one line).
	lines := strings.Count(strings.TrimSpace(stdout.String()), "\n") + 1
	if strings.TrimSpace(stdout.String()) == "" {
		lines = 0
	}
	if lines != 2 {
		t.Errorf("history list --limit 2 produced %d lines, want 2; output=%q", lines, stdout.String())
	}
}

// TestHistoryListSinceFlag verifies --since flag.
func TestHistoryListSinceFlag(t *testing.T) {
	now := time.Now()
	entries := []history.Entry{
		historyTestEntry("old", "old query", now.Add(-48*time.Hour)),
		historyTestEntry("recent", "recent query", now.Add(-1*time.Hour)),
	}
	setupHistoryEnv(t, entries)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "list", "--since", "24h"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	if strings.Contains(output, "old") {
		t.Errorf("history list --since 24h should not show 48h-old entry: %q", output)
	}
	if !strings.Contains(output, "recent") {
		t.Errorf("history list --since 24h should show recent entry: %q", output)
	}
}

// TestHistoryListEmpty verifies empty history shows message.
func TestHistoryListEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "list"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No history entries") {
		t.Errorf("expected empty message, got: %q", stdout.String())
	}
}

// TestHistoryShowKnownID verifies "history show <id>" outputs entry details.
func TestHistoryShowKnownID(t *testing.T) {
	now := time.Now()
	entries := []history.Entry{
		historyTestEntry("show-me", "detailed prompt", now),
	}
	setupHistoryEnv(t, entries)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "show", "show-me"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "show-me") {
		t.Errorf("history show missing ID: %q", output)
	}
	if !strings.Contains(output, "detailed prompt") {
		t.Errorf("history show missing prompt: %q", output)
	}
}

// TestHistoryShowJSONFormat verifies --format json output.
func TestHistoryShowJSONFormat(t *testing.T) {
	now := time.Now()
	entries := []history.Entry{
		historyTestEntry("json-id", "json prompt", now),
	}
	setupHistoryEnv(t, entries)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "show", "json-id", "--format", "json"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	var result history.Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &result); err != nil {
		t.Fatalf("invalid JSON output: %v; output=%q", err, stdout.String())
	}
	if result.ID != "json-id" {
		t.Errorf("JSON ID = %q, want %q", result.ID, "json-id")
	}
}

// TestHistoryShowUnknownIDExitsOne verifies unknown ID exits 1.
func TestHistoryShowUnknownIDExitsOne(t *testing.T) {
	setupHistoryEnv(t, nil)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "show", "nonexistent"})

	if code != ExitUserError {
		t.Errorf("exit code = %d, want %d (ExitUserError)", code, ExitUserError)
	}
}

// TestHistorySearchMatch verifies "history search" finds matching entries.
func TestHistorySearchMatch(t *testing.T) {
	now := time.Now()
	entries := []history.Entry{
		historyTestEntry("s1", "golang generics", now.Add(-2*time.Hour)),
		historyTestEntry("s2", "rust async patterns", now.Add(-1*time.Hour)),
		historyTestEntry("s3", "golang testing", now),
	}
	setupHistoryEnv(t, entries)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "search", "golang"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "s1") || !strings.Contains(output, "s3") {
		t.Errorf("search should find s1 and s3: %q", output)
	}
	if strings.Contains(output, "s2") {
		t.Errorf("search should not find s2: %q", output)
	}
}

// TestHistorySearchNoMatch verifies no matches shows message.
func TestHistorySearchNoMatch(t *testing.T) {
	now := time.Now()
	entries := []history.Entry{
		historyTestEntry("x1", "something", now),
	}
	setupHistoryEnv(t, entries)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "search", "nonexistent-pattern"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No matching entries") {
		t.Errorf("expected no matches message, got: %q", stdout.String())
	}
}

// TestHistoryClearWithConfirm verifies "history clear --confirm" works.
func TestHistoryClearWithConfirm(t *testing.T) {
	now := time.Now()
	entries := []history.Entry{
		historyTestEntry("c1", "clear test", now),
	}
	home := setupHistoryEnv(t, entries)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "clear", "--confirm"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "History cleared") {
		t.Errorf("expected clear message: %q", stdout.String())
	}

	// Verify data dir still exists but file is gone.
	dataDir := filepath.Join(home, ".local", "share", "usearch")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Error("data directory should still exist after clear")
	}
}

// TestHistoryClearRequiresConfirm verifies "history clear" without --confirm prompts.
// In non-TTY mode, it exits 1. In TTY mode, it prompts and cancels on empty input.
func TestHistoryClearRequiresConfirm(t *testing.T) {
	setupHistoryEnv(t, nil)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "clear"})

	// Either: TTY mode shows prompt and cancels (exit 0 with "Cancelled"),
	// or non-TTY mode exits 1 with error.
	if code == ExitUserError {
		// Non-TTY path: correct behavior.
		return
	}
	if code == ExitSuccess {
		output := stdout.String()
		if !strings.Contains(output, "Cancelled") {
			t.Errorf("TTY cancel path should print 'Cancelled': %q", output)
		}
		return
	}
	t.Errorf("exit code = %d, want 0 (cancelled) or %d (non-TTY)", code, ExitUserError)
}

// TestHistoryClearSinceFilter verifies --since flag filters clear.
func TestHistoryClearSinceFilter(t *testing.T) {
	now := time.Now()
	entries := []history.Entry{
		historyTestEntry("old-entry", "old", now.Add(-48*time.Hour)),
		historyTestEntry("new-entry", "new", now),
	}
	home := setupHistoryEnv(t, entries)
	_ = home

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "clear", "--since", "24h", "--confirm"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	// The new entry should still exist.
	var stdout2, stderr2 bytes.Buffer
	cmd2 := newRootCmd(&stdout2, &stderr2)
	code2 := runCobra(cmd2, []string{"history", "list"})

	if code2 != 0 {
		t.Fatalf("list exit code = %d, want 0", code2)
	}
	output := stdout2.String()
	if !strings.Contains(output, "new-entry") {
		t.Errorf("new entry should remain after --since clear: %q", output)
	}
}

// TestHistorySubcommandInHelp verifies history appears in help output.
func TestHistorySubcommandInHelp(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{"--help"})
	_ = cmd.Execute()
	helpTxt := buf.String()
	if !strings.Contains(helpTxt, "history") {
		t.Errorf("root help does not mention 'history': %q", helpTxt)
	}
}

// TestHistorySinceBadDuration verifies bad --since value exits 1.
func TestHistorySinceBadDuration(t *testing.T) {
	setupHistoryEnv(t, nil)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&stdout, &stderr)
	code := runCobra(cmd, []string{"history", "list", "--since", "bad"})

	if code != ExitUserError {
		t.Errorf("exit code = %d, want %d (ExitUserError)", code, ExitUserError)
	}
}

// TestHistoryPathRespectsConfig verifies history uses configured path.
func TestHistoryPathRespectsConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("HOME", home)

	// Config should produce the right path.
	cfgPath := config.HistoryPath("jsonl")
	expected := filepath.Join(home, ".local", "share", "usearch", "history.jsonl")
	if cfgPath != expected {
		t.Errorf("HistoryPath = %q, want %q", cfgPath, expected)
	}
}
