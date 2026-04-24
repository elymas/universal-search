// Package main_test validates the usearch CLI entrypoint (REQ-BOOT-012).
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
