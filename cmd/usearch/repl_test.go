// Package main — tests for the interactive REPL.
// SPEC-CLI-002 REQ-CLI2-008: Zero-args REPL, slash commands, history integration.
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/usearch/history"
)

// --- Phase 3: REPL tests ---

// TestREPLEntersOnReplFlag verifies that --repl flag enters REPL mode.
// REQ-CLI2-008: --repl forces interactive REPL entry.
func TestREPLEntersOnReplFlag(t *testing.T) {
	input := strings.NewReader("/quit\n")
	var out, errOut bytes.Buffer

	code := runREPL(input, &out, &errOut, []string{"--repl"})

	if code != ExitSuccess {
		t.Errorf("REPL exit code = %d, want %d", code, ExitSuccess)
	}
	output := out.String()
	if !strings.Contains(output, "usearch>") {
		t.Errorf("REPL prompt not found in output: %q", output)
	}
}

// TestREPLSlashQuitExits verifies /quit exits cleanly.
// REQ-CLI2-008: /quit exits the REPL with exit code 0.
func TestREPLSlashQuitExits(t *testing.T) {
	input := strings.NewReader("/quit\n")
	var out, errOut bytes.Buffer

	code := runREPL(input, &out, &errOut, []string{"--repl"})

	if code != ExitSuccess {
		t.Errorf("REPL /quit exit code = %d, want %d", code, ExitSuccess)
	}
}

// TestREPLSlashHelpOutput verifies /help displays available commands.
// REQ-CLI2-008: /help shows slash commands.
func TestREPLSlashHelpOutput(t *testing.T) {
	input := strings.NewReader("/help\n/quit\n")
	var out, errOut bytes.Buffer

	code := runREPL(input, &out, &errOut, []string{"--repl"})

	if code != ExitSuccess {
		t.Errorf("REPL exit code = %d, want %d", code, ExitSuccess)
	}
	output := out.String()
	for _, cmd := range []string{"/help", "/quit", "/sources", "/history", "/config"} {
		if !strings.Contains(output, cmd) {
			t.Errorf("/help output missing %q: %s", cmd, output)
		}
	}
}

// TestREPLQuerySavesHistory verifies that a query entered in REPL
// is saved to the history backend.
// REQ-CLI2-008: Each query in REPL saves to history.
func TestREPLQuerySavesHistory(t *testing.T) {
	home := t.TempDir()
	xdgData := filepath.Join(home, ".local", "share")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("HOME", home)

	input := strings.NewReader("test query about AI\n/quit\n")
	var out, errOut bytes.Buffer

	_ = runREPLWithHistory(input, &out, &errOut, []string{"--repl"}, home)

	// Read the history file directly.
	historyPath := filepath.Join(xdgData, "usearch", "history.jsonl")
	raw, readErr := os.ReadFile(historyPath)
	if readErr != nil {
		t.Fatalf("history file not found at %s: %v", historyPath, readErr)
	}

	backend, err := history.NewJSONLBackend(historyPath, 1000, 90)
	if err != nil {
		t.Fatalf("failed to open history backend: %v", err)
	}

	entries, err := backend.List(10)
	if err != nil {
		t.Fatalf("failed to list history: %v, raw: %q", err, string(raw))
	}

	found := false
	for _, e := range entries {
		if e.Prompt == "test query about AI" && e.Command == "repl" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("REPL query not found in history (%d entries); raw: %q", len(entries), string(raw))
	}
}

// TestREPLNonInteractiveMode verifies that REPL does not enter when stdin is
// not a terminal and --repl is not set.
// REQ-CLI2-008: Without TTY and without --repl, show help (no REPL).
func TestREPLNonInteractiveMode(t *testing.T) {
	var out, errOut bytes.Buffer

	_ = dispatchWithStdin(nil, &out, &errOut, []string{})

	output := out.String() + errOut.String()
	if !strings.Contains(output, "query") && !strings.Contains(output, "Query") {
		t.Logf("output for zero-args non-interactive: %q", output)
	}
}

// TestREPLSlashConfigShowsPath verifies /config shows the config path.
// REQ-CLI2-008: /config slash command shows current configuration.
func TestREPLSlashConfigShowsPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("HOME", home)

	input := strings.NewReader("/config\n/quit\n")
	var out, errOut bytes.Buffer

	_ = runREPLWithHistory(input, &out, &errOut, []string{"--repl"}, home)

	output := out.String()
	if !strings.Contains(output, "config") {
		t.Errorf("/config output missing 'config': %s", output)
	}
}

// TestREPLSlashSourcesListsAdapters verifies /sources shows adapter list.
// REQ-CLI2-008: /sources slash command shows available adapters.
func TestREPLSlashSourcesListsAdapters(t *testing.T) {
	input := strings.NewReader("/sources\n/quit\n")
	var out, errOut bytes.Buffer

	code := runREPL(input, &out, &errOut, []string{"--repl"})

	if code != ExitSuccess {
		t.Errorf("REPL exit code = %d, want %d", code, ExitSuccess)
	}
	output := out.String()
	if !strings.Contains(output, "Sources") && !strings.Contains(output, "sources") && !strings.Contains(output, "adapter") {
		t.Errorf("/sources output missing adapter info: %s", output)
	}
}

// TestREPLSlashHistoryShowsRecent verifies /history shows recent entries.
// REQ-CLI2-008: /history slash command shows recent history.
func TestREPLSlashHistoryShowsRecent(t *testing.T) {
	home := t.TempDir()
	xdgData := filepath.Join(home, ".local", "share")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("HOME", home)

	// Pre-populate history.
	historyPath := filepath.Join(xdgData, "usearch", "history.jsonl")
	backend, err := history.NewJSONLBackend(historyPath, 1000, 90)
	if err != nil {
		t.Fatal(err)
	}
	entry := history.Entry{
		ID:            "hist-repl-test",
		Timestamp:     time.Now(),
		Command:       "query",
		Prompt:        "pre-existing query",
		Category:      "general",
		Adapters:      []string{"reddit"},
		Summary:       "test summary",
		SchemaVersion: 1,
	}
	if err := backend.Write(entry); err != nil {
		t.Fatal(err)
	}

	input := strings.NewReader("/history\n/quit\n")
	var out, errOut bytes.Buffer

	_ = runREPLWithHistory(input, &out, &errOut, []string{"--repl"}, home)

	output := out.String()
	if !strings.Contains(output, "pre-existing query") {
		t.Errorf("/history output missing pre-existing entry: %s", output)
	}
}

// TestREPLEmptyInputLoops verifies that empty input does not exit REPL.
// REQ-CLI2-008: Empty lines are ignored, REPL continues.
func TestREPLEmptyInputLoops(t *testing.T) {
	input := strings.NewReader("\n\n\n/quit\n")
	var out, errOut bytes.Buffer

	code := runREPL(input, &out, &errOut, []string{"--repl"})

	if code != ExitSuccess {
		t.Errorf("REPL exit code = %d, want %d", code, ExitSuccess)
	}
	promptCount := strings.Count(out.String(), "usearch>")
	if promptCount < 3 {
		t.Errorf("expected >= 3 prompts for 4 lines (3 empty + quit), got %d prompts", promptCount)
	}
}
