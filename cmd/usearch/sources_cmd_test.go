// Package main — tests for the sources subcommand.
// SPEC-CLI-002 REQ-CLI2-004: usearch sources {list,status,show}.
package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestSourcesSubcommandExists verifies "sources" is a registered subcommand.
func TestSourcesSubcommandExists(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{"--help"})

	_ = cmd.Execute()
	helpTxt := buf.String()

	if !strings.Contains(helpTxt, "sources") {
		t.Errorf("help output missing 'sources' subcommand: %s", helpTxt)
	}
}

// TestSourcesListOutput verifies sources list shows adapter names.
func TestSourcesListOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{"sources", "list"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources list failed: %v", err)
	}
	output := buf.String()
	// Should contain known adapter names from the codebase.
	for _, name := range []string{"reddit", "arxiv"} {
		if !strings.Contains(output, name) {
			t.Errorf("sources list missing adapter %q: %s", name, output)
		}
	}
}

// TestSourcesHelpOutput verifies sources --help shows subcommands.
func TestSourcesHelpOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{"sources", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources --help failed: %v", err)
	}
	helpTxt := buf.String()
	for _, sub := range []string{"list", "status", "show"} {
		if !strings.Contains(helpTxt, sub) {
			t.Errorf("sources --help missing subcommand %q: %s", sub, helpTxt)
		}
	}
}
