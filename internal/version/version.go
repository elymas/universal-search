// Package version provides the single-source version information for all usearch binaries.
//
// SPEC-REL-001 REQ-REL-001: Consolidates scattered version constants
// (cmd/usearch/main.go, cmd/usearch-api/main.go, cmd/usearch-mcp/main.go)
// into a single shared package consumed via ldflags injection at release time.
//
// @MX:ANCHOR: fan_in = 3 binaries + tests (high-touch release artifact)
// @MX:SPEC: SPEC-REL-001 REQ-REL-001
package version

import (
	"fmt"
	"runtime"
)

// @MX:NOTE: These variables are injected at build time via go build -ldflags.
// Default values are suitable for development; release builds override via:
//  -X github.com/elymas/universal-search/internal/version.Version=1.0.0
//  -X github.com/elymas/universal-search/internal/version.Commit=<sha>
//  -X github.com/elymas/universal-search/internal/version.BuildDate=<iso8601>
var (
	Version   = "0.1.0-dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// String returns a detailed version string: "usearch v<Version> (<Commit>, built <BuildDate>, <GoVersion>)".
// Used in verbose logging and debugging contexts.
func String() string {
	return fmt.Sprintf("usearch v%s (%s, built %s, %s)",
		Version, Commit, BuildDate, runtime.Version())
}

// Short returns the semantic version string only (e.g., "0.1.0-dev" or "1.0.0").
// Used in `--version` flag output and obs.Config ServiceVersion fields.
func Short() string {
	return Version
}
