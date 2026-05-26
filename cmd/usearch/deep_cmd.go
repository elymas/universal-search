// Package main — deep subcommand for the usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-003: usearch deep "..." invokes the deep agent pipeline.
// Shows progress stages: Researcher → Reviewer → Writer → Verifier.
// Supports --budget flag for cost limit override.
// Real OIDC deferred to SPEC-AUTH-001.
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newDeepCmd creates the cobra command for the deep subcommand.
func newDeepCmd() *cobra.Command {
	var budget float64

	cmd := &cobra.Command{
		Use:   "deep [flags] <prompt>",
		Short: "Run deep research pipeline",
		Long: `Run the multi-agent deep research pipeline.

Invokes the full pipeline: Researcher → Reviewer → Writer → Verifier.
Shows progress for each stage. Supports cost limit via --budget flag.`,
		Example: `  usearch deep "comprehensive analysis of AI safety"
	  usearch deep --budget 0.50 "latest quantum computing breakthroughs"`,
		Args: cobra.RangeArgs(1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := args[0]

			// @MX:TODO: [AUTO] Wire to deepagent.RunPipeline when LLM client available.
			// For now, show what would happen.
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Deep research: %q\n", prompt)
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Budget: $%.2f\n", budget)
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Pipeline stages:")
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "  [1/4] Researcher  — gathering sources...")
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "  [2/4] Reviewer    — evaluating quality...")
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "  [3/4] Writer      — composing answer...")
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "  [4/4] Verifier    — checking faithfulness...")

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Deep research pipeline not yet wired (requires LLM client).")
			return exitError{code: ExitSystemError}
		},
	}

	cmd.Flags().Float64Var(&budget, "budget", 1.0, "cost limit in USD (default: $1.00)")

	return cmd
}
