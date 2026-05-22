// Package main — tests for the deep subcommand.
// SPEC-CLI-002 REQ-CLI2-003: usearch deep "..." invokes deep agent pipeline.
package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestDeepSubcommandExists verifies "deep" is a registered subcommand.
func TestDeepSubcommandExists(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{"--help"})

	_ = cmd.Execute()
	helpTxt := buf.String()

	if !strings.Contains(helpTxt, "deep") {
		t.Errorf("help output missing 'deep' subcommand: %s", helpTxt)
	}
}

// TestDeepRequiresPrompt verifies deep subcommand requires a prompt argument.
func TestDeepRequiresPrompt(t *testing.T) {
	var out, errOut bytes.Buffer
	code := dispatch([]string{"deep"})

	if code == ExitSuccess {
		t.Error("deep without args should fail")
	}
	_ = out
	_ = errOut
}

// TestDeepHelpOutput verifies deep --help shows usage info.
func TestDeepHelpOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{"deep", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("deep --help failed: %v", err)
	}
	helpTxt := buf.String()
	if !strings.Contains(helpTxt, "deep") {
		t.Errorf("deep --help missing 'deep': %s", helpTxt)
	}
	if !strings.Contains(helpTxt, "budget") {
		t.Errorf("deep --help missing '--budget' flag: %s", helpTxt)
	}
}
