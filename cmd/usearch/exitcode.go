// Package main — exit code constants and error classifier for the query subcommand.
//
// REQ-CLI-008: Exit code contract for usearch query pipeline stages.
//
// @MX:NOTE: [AUTO] Exit-code mapping is the load-bearing UX contract; behaviour
// change ripples to CI scripts and downstream SPEC-CLI-002 integrations.
// @MX:SPEC: SPEC-CLI-001
package main

import (
	"errors"
)

// Exit code constants used throughout the query subcommand.
const (
	// ExitSuccess indicates full success: >= 1 doc + non-empty summary. (REQ-CLI-008)
	ExitSuccess = 0
	// ExitUserError indicates a user-supplied input error:
	// empty prompt, invalid format, unknown adapter, parse failure. (REQ-CLI-008)
	ExitUserError = 1
	// ExitSystemError indicates a system/infrastructure error:
	// timeout, all adapters failed, unknown subcommand. (REQ-CLI-008)
	ExitSystemError = 2
	// ExitPartial indicates partial success:
	// synthesis failed OR some adapters errored but >= 1 succeeded. (REQ-CLI-008)
	ExitPartial = 3
)

// errUserInput is a sentinel wrapped around user-facing input errors.
var errUserInput = errors.New("user input error")

// classifyError maps a pipeline error to an exit code.
// Returns ExitSuccess (0) when err is nil.
//
// @MX:NOTE: [AUTO] classifyError is the sole mapping from pipeline errors to
// CLI exit codes; all error paths in runQuery must route through this function.
// @MX:SPEC: SPEC-CLI-001
func classifyError(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if errors.Is(err, errUserInput) {
		return ExitUserError
	}
	return ExitSystemError
}
