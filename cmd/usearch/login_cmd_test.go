// Package main — tests for the login subcommand.
// SPEC-CLI-002 REQ-CLI2-007: usearch login {status,logout} skeleton.
package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestLoginSubcommandExists verifies "login" is a registered subcommand.
func TestLoginSubcommandExists(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{"--help"})

	_ = cmd.Execute()
	helpTxt := buf.String()

	if !strings.Contains(helpTxt, "login") {
		t.Errorf("help output missing 'login' subcommand: %s", helpTxt)
	}
}

// TestLoginStatusNotAuthenticated verifies login status shows "not authenticated".
func TestLoginStatusNotAuthenticated(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{"login", "status"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("login status failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "not authenticated") && !strings.Contains(output, "Not authenticated") {
		t.Errorf("login status should indicate not authenticated: %s", output)
	}
}

// TestLoginLogout verifies login logout succeeds (no-op when not authenticated).
func TestLoginLogout(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{"login", "logout"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("login logout failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "logged out") && !strings.Contains(output, "Logged out") && !strings.Contains(output, "No active") {
		t.Errorf("login logout unexpected output: %s", output)
	}
}
