// Package main is the entrypoint for the usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-001: Migrated from stdlib flag to github.com/spf13/cobra.
// The v0 invocation contract is preserved — usearch query "..." works identically.
// SPEC-CLI-001: Original subcommand dispatcher now routes through cobra.
package main

import (
	"os"
)

// Version is the current release version of usearch.
// Format: semver, e.g. "0.1.0-dev".
const Version = "0.1.0-dev"

func main() {
	cmd := newRootCmd(os.Stdout, os.Stderr)
	code := runCobra(cmd, os.Args[1:])
	os.Exit(code)
}
