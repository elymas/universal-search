// Package main — cobra root command for the usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-001: Migrates from stdlib flag to github.com/spf13/cobra.
// Preserves v0 invocation contract: usearch query "..." works identically.
// New subcommands are added via cobra.Command registration.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/elymas/universal-search/internal/llm"
	llmconfig "github.com/elymas/universal-search/internal/llm/config"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/version"
)

// rootCmd is the cobra root command for the usearch CLI.
// @MX:ANCHOR: [AUTO] Root cobra command — all subcommands and global flags register here.
// @MX:REASON: fan_in >= 3 — main.go, tests, and future subcommand additions all reference this.
// @MX:SPEC: SPEC-CLI-002
var rootCmd = &cobra.Command{
	Use:     "usearch",
	Short:   "Universal Search CLI",
	Version: version.Short(),
	// No RunE — when invoked with zero args and no subcommand, cobra shows help
	// (unless REPL mode is active, which is handled in PersistentPostRun).
}

func init() {
	// Preserve v0 version output format: "usearch v<semver>"
	rootCmd.SetVersionTemplate("usearch v{{.Version}}\n")
}

// newRootCmd creates a fresh root command for testing.
// Each test gets its own cobra tree to avoid shared state.
func newRootCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "usearch",
		Short:         "Universal Search CLI",
		Version:       version.Short(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	// Preserve v0 version output format: "usearch v<semver>"
	cmd.SetVersionTemplate("usearch v{{.Version}}\n")

	// Register subcommands.
	registerSubcommands(cmd)

	return cmd
}

// registerSubcommands adds all subcommands to the root command.
func registerSubcommands(root *cobra.Command) {
	// query subcommand — wraps v0 Execute().
	root.AddCommand(newQueryCmd())

	// config subcommand tree — SPEC-CLI-002 REQ-CLI2-012.
	root.AddCommand(newConfigCmd())

	// history subcommand tree — SPEC-CLI-002 REQ-CLI2-011.
	root.AddCommand(newHistoryCmd())

	// REPL subcommand — SPEC-CLI-002 REQ-CLI2-008.
	registerREPL(root)

	// deep subcommand — SPEC-CLI-002 REQ-CLI2-003.
	root.AddCommand(newDeepCmd())

	// sources subcommand tree — SPEC-CLI-002 REQ-CLI2-004.
	root.AddCommand(newSourcesCmd())

	// login subcommand tree — SPEC-CLI-002 REQ-CLI2-007.
	root.AddCommand(newLoginCmd())

	// version is handled by cobra's built-in --version flag.
	// No need for a separate version subcommand.
}

// newQueryCmd creates the cobra command for the query subcommand.
// Wraps the v0 Execute() function as a cobra RunE.
func newQueryCmd() *cobra.Command {
	var (
		source  string
		format  string
		timeout string
		noLLM   bool
		noObs   bool
	)

	cmd := &cobra.Command{
		Use:   "query [flags] <prompt>",
		Short: "Search and synthesize results",
		Long: `Query the universal search system.

Routes the prompt through intent classification, fan-out to adapters,
and LLM synthesis. Output goes to stdout; progress/errors to stderr.`,
		Example: `  usearch query "latest AI research"
  usearch query --format json --source reddit "golang generics"
  usearch query --timeout 10s "quick lookup"`,
		Args: cobra.RangeArgs(1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Collect flags into the format expected by Execute().
			var flagArgs []string
			if source != "" {
				flagArgs = append(flagArgs, "--source", source)
			}
			if format != "" {
				flagArgs = append(flagArgs, "--format", format)
			}
			if timeout != "" {
				flagArgs = append(flagArgs, "--timeout", timeout)
			}
			if noObs {
				flagArgs = append(flagArgs, "--no-obs")
			}

			// Combine flags with the positional prompt.
			allArgs := append(flagArgs, args...)

			// If --no-llm is set, strip it and set the flag for obs init.
			filtered := make([]string, 0, len(allArgs))
			for _, a := range allArgs {
				if a == "--no-llm" || a == "-no-llm" {
					noLLM = true
					continue
				}
				filtered = append(filtered, a)
			}
			allArgs = filtered

			// Init observability (unless --no-obs).
			if !noObs {
				o, shutdown, err := obs.Init(ctx, obs.Config{
					ServiceName:    "usearch",
					ServiceVersion: version.Short(),
					LogLevel:       os.Getenv("LOG_LEVEL"),
					AdminAddr:      adminAddr(),
					OTLPEndpoint:   os.Getenv("OTLP_ENDPOINT"),
				})
				if err != nil {
					return exitError{code: ExitSystemError, err: fmt.Errorf("obs.Init: %w", err)}
				}
				defer func() { _ = shutdown(ctx) }()

				_ = o // obs bundle available for future registry wiring

				// Initialise LLM client when LITELLM_MASTER_KEY is set and --no-llm is absent.
				if !noLLM && os.Getenv("LITELLM_MASTER_KEY") != "" {
					cfg, cfgErr := llmconfig.Load()
					if cfgErr != nil {
						return exitError{code: ExitSystemError, err: fmt.Errorf("llm config: %w", cfgErr)}
					}
					llmClient, llmErr := llm.New(cfg, o)
					if llmErr != nil {
						return exitError{code: ExitSystemError, err: fmt.Errorf("llm init: %w", llmErr)}
					}
					defer func() { _ = llmClient.Close() }()
				}
			}

			code := Execute(ctx, allArgs, cmd.OutOrStdout(), cmd.ErrOrStderr())
			if code != 0 {
				return exitError{code: code}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "comma-separated adapter names (empty = all enabled)")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text (default), json, or markdown")
	cmd.Flags().StringVar(&timeout, "timeout", "30s", "total pipeline deadline (max 5m)")
	cmd.Flags().BoolVar(&noLLM, "no-llm", false, "skip LLM client initialisation")
	cmd.Flags().BoolVar(&noObs, "no-obs", false, "disable observability init (test flag)")

	return cmd
}

// exitError wraps an exit code into an error so cobra RunE can return it.
// The caller (runCobra) extracts the exit code.
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit code %d", e.code)
}

// runCobra executes the cobra root command and returns the exit code.
func runCobra(cmd *cobra.Command, args []string) int {
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil {
		return ExitSuccess
	}
	// Check if it's our exitError.
	var exitErr exitError
	if errors.As(err, &exitErr) {
		return exitErr.code
	}
	// Cobra errors (unknown command, etc.) map to ExitUserError.
	// @MX:NOTE: [AUTO] Cobra error -> exit code mapping.
	return ExitUserError
}

// dispatch provides backward-compatible routing for tests that call it directly.
// Routes through cobra internally.
func dispatch(args []string) int {
	cmd := newRootCmd(os.Stderr, os.Stderr)
	return runCobra(cmd, args)
}

// adminAddr returns the admin server bind address from the environment.
func adminAddr() string {
	if v := os.Getenv("USEARCH_ADMIN_PORT"); v != "" {
		return "127.0.0.1:" + v
	}
	return ""
}
