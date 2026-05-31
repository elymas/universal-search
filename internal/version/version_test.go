// Package version_test validates version package behavior per SPEC-REL-001 REQ-REL-001.
package version_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/version"
)

// TestVersionDefault validates that the default Version value is set correctly.
func TestVersionDefault(t *testing.T) {
	if version.Version != "0.1.0-dev" {
		t.Errorf("Version = %q, want %q", version.Version, "0.1.0-dev")
	}
}

// TestVersionMatchesSemverRegex ensures Version conforms to the actual
// TestVersionFlag regex from cmd/usearch/main_test.go:12 per REQ-REL-002.
// The real regex is: ^usearch v\d+\.\d+\.\d+  (prefix-anchored, no $, no prerelease group)
func TestVersionMatchesSemverRegex(t *testing.T) {
	// Use the actual semver pattern from cmd/usearch/main_test.go:12
	semverPattern := regexp.MustCompile(`^usearch v\d+\.\d+\.\d+`)

	// Test with default version (0.1.0-dev)
	defaultOutput := "usearch v" + version.Version
	if !semverPattern.MatchString(defaultOutput) {
		t.Errorf("default version %q does not match pattern %q",
			defaultOutput, semverPattern.String())
	}

	// Test with a release version (simulating ldflags injection)
	releaseOutput := "usearch v1.0.0"
	if !semverPattern.MatchString(releaseOutput) {
		t.Errorf("release version %q does not match pattern %q",
			releaseOutput, semverPattern.String())
	}
}

// TestStringFormat validates the String() output format.
func TestStringFormat(t *testing.T) {
	output := version.String()
	if !strings.HasPrefix(output, "usearch v") {
		t.Errorf("String() = %q, want prefix %q", output, "usearch v")
	}
	if !strings.Contains(output, version.Version) {
		t.Errorf("String() = %q, missing Version %q", output, version.Version)
	}
	if !strings.Contains(output, "built") {
		t.Errorf("String() = %q, missing 'built' keyword", output)
	}
}

// TestShortReturnsVersionOnly validates that Short() returns only the version string.
func TestShortReturnsVersionOnly(t *testing.T) {
	output := version.Short()
	if output != version.Version {
		t.Errorf("Short() = %q, want %q", output, version.Version)
	}
	// Ensure Short() doesn't contain extra formatting
	if strings.Contains(output, "built") {
		t.Errorf("Short() = %q, should not contain extra formatting", output)
	}
}

// TestCommitDefault validates that Commit has a sensible default.
func TestCommitDefault(t *testing.T) {
	if version.Commit == "" {
		t.Errorf("Commit is empty, expected at least 'unknown'")
	}
}

// TestBuildDateDefault validates that BuildDate has a sensible default.
func TestBuildDateDefault(t *testing.T) {
	if version.BuildDate == "" {
		t.Errorf("BuildDate is empty, expected at least 'unknown'")
	}
}
