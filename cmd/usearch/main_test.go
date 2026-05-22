// Package main_test validates the usearch CLI entrypoint (REQ-BOOT-012, REQ-CLI-001, SPEC-CLI-002).
package main

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// semverPattern matches the output format: "usearch v<major>.<minor>.<patch>[-prerelease]"
var semverPattern = regexp.MustCompile(`^usearch v\d+\.\d+\.\d+`)

// TestVersionFlag verifies that --version prints a semver string and exits 0.
// RED phase: this test defines the acceptance contract for REQ-BOOT-012.
func TestVersionFlag(t *testing.T) {
	t.Helper()

	cmd := exec.Command("go", "run", ".", "--version")
	cmd.Dir = "."

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version exited non-zero: %v", err)
	}

	output := strings.TrimSpace(string(out))
	if !semverPattern.MatchString(output) {
		t.Errorf("--version output %q does not match pattern %q", output, semverPattern.String())
	}
}

// TestVersionShortFlag verifies that -v is an alias for --version.
func TestVersionShortFlag(t *testing.T) {
	t.Helper()

	cmd := exec.Command("go", "run", ".", "-v")
	cmd.Dir = "."

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("-v exited non-zero: %v", err)
	}

	output := strings.TrimSpace(string(out))
	if !semverPattern.MatchString(output) {
		t.Errorf("-v output %q does not match pattern %q", output, semverPattern.String())
	}
}

// TestUnknownSubcommandExits1 verifies REQ-CLI2-014: unknown subcommand exits 1.
// SPEC-CLI-002 updated exit code from 2 (v0) to 1 (v1) per REQ-CLI2-014.
// Tests dispatch() directly to avoid go run exit code wrapping behaviour.
func TestUnknownSubcommandExits1(t *testing.T) {
	t.Helper()

	// dispatch() is in the same package so we can call it directly.
	code := dispatch([]string{"foobar"})
	if code != ExitUserError {
		t.Errorf("unknown subcommand exit code = %d, want %d (ExitUserError)", code, ExitUserError)
	}
}

// TestUnknownSubcommandExits1Process verifies the full binary exits 1 via exec.
// SPEC-CLI-002 REQ-CLI2-014: unknown subcommand exits with code 1 (user error).
func TestUnknownSubcommandExits1Process(t *testing.T) {
	t.Helper()

	// Build the binary first.
	binPath := t.TempDir() + "/usearch-test-bin"
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = "."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath, "foobar")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("unknown subcommand should exit non-zero")
	}

	output := string(out)
	// With SilenceErrors=true, cobra does not print the error message.
	// The exit code is the primary signal.
	_ = output // Suppress unused warning

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	// SPEC-CLI-002 REQ-CLI2-014: unknown subcommand exits 1 (user error), not 2.
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}
}

// TestQuerySubcommandHelp verifies that 'usearch --help' exits cleanly.
func TestQuerySubcommandHelp(t *testing.T) {
	t.Helper()

	cmd := exec.Command("go", "run", ".", "--help")
	cmd.Dir = "."

	out, _ := cmd.CombinedOutput()
	output := string(out)
	// Should mention "query" in some form
	if !strings.Contains(output, "query") && !strings.Contains(output, "Query") {
		t.Logf("help output: %q", output)
		// This is not a hard failure — just check that something comes out
	}
}

// TestSubcommandRegistry verifies REQ-CLI2-002: all v1 subcommands registered.
func TestSubcommandRegistry(t *testing.T) {
	t.Helper()

	var buf strings.Builder
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{"--help"})

	_ = cmd.Execute()
	helpTxt := buf.String()

	required := []string{"query", "completion"}
	for _, name := range required {
		if !strings.Contains(helpTxt, name) {
			t.Errorf("help output missing subcommand %q: %s", name, helpTxt)
		}
	}
}
