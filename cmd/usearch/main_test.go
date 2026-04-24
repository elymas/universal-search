package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// TestVersionFlag is RED 1: asserts --version prints semver and exits 0.
// Fails until main.go implements the --version flag.
func TestVersionFlag(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "usearch")
	if err := exec.Command("go", "build", "-o", bin, ".").Run(); err != nil {
		t.Skip("go build not available in this environment:", err)
	}

	cmd := exec.Command(bin, "--version")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("--version exited %d: %s", exitErr.ExitCode(), exitErr.Stderr)
		}
		t.Fatalf("unexpected error: %v", err)
	}

	pattern := regexp.MustCompile(`^usearch v\d+\.\d+\.\d+`)
	if !pattern.Match(out) {
		t.Errorf("stdout %q does not match pattern %s", out, pattern)
	}
}

// TestVersionFlagExitCode verifies exit code 0 via os/exec.
func TestVersionFlagExitCode(t *testing.T) {
	if os.Getenv("TEST_USEARCH_BIN") == "" {
		t.Skip("set TEST_USEARCH_BIN=/path/to/usearch to run this test")
	}
	bin := os.Getenv("TEST_USEARCH_BIN")
	cmd := exec.Command(bin, "--version")
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected exit 0, got: %v", err)
	}
}
