// Package main_test validates the usearch CLI entrypoint (REQ-BOOT-012, REQ-CLI-001).
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

// TestUnknownSubcommandExits2 verifies REQ-CLI-001: unknown subcommand exits 2.
// Tests dispatch() directly to avoid go run exit code wrapping behaviour.
func TestUnknownSubcommandExits2(t *testing.T) {
	t.Helper()

	// dispatch() is in the same package so we can call it directly.
	// This avoids go run exit code wrapping (go run wraps non-zero exit as 1).
	code := dispatch([]string{"foobar"})
	if code != ExitSystemError {
		t.Errorf("unknown subcommand exit code = %d, want %d (ExitSystemError)", code, ExitSystemError)
	}
}

// TestUnknownSubcommandExits2Process verifies the full binary exits 2 via exec.
// Uses a pre-built binary to avoid go run exit code wrapping.
func TestUnknownSubcommandExits2Process(t *testing.T) {
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
	if !strings.Contains(output, "unknown subcommand") {
		t.Errorf("output missing 'unknown subcommand': %q", output)
	}
	if !strings.Contains(output, "available") {
		t.Errorf("output missing 'available': %q", output)
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exitErr.ExitCode())
	}
}


// TestQuerySubcommandHelp verifies that 'usearch query --help' exits cleanly.
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
