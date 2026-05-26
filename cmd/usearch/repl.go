// Package main — interactive REPL for the usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-008: Zero-args enters interactive query loop
// when stdin is a TTY. --repl flag forces REPL regardless of TTY.
// Slash commands: /help, /quit, /sources, /history, /config.
// Each query is saved to the history backend.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"

	"github.com/elymas/universal-search/internal/usearch/config"
	"github.com/elymas/universal-search/internal/usearch/history"
)

const replPrompt = "usearch> "

// slashCommands defines the built-in REPL slash commands and their help text.
var slashCommands = []struct {
	Name string
	Help string
}{
	{"/help", "Show this help message"},
	{"/quit", "Exit the REPL"},
	{"/sources", "List available search adapters"},
	{"/history", "Show recent query history"},
	{"/config", "Show current configuration"},
}

// runREPL executes the interactive REPL loop using the provided input/output.
// This is the testable entry point that accepts custom io.Reader/io.Writer.
func runREPL(stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	cmd := newRootCmd(stdout, stderr)
	cmd.SetArgs(args)

	// Check if --repl is in the args.
	hasRepl := false
	for _, a := range args {
		if a == "--repl" {
			hasRepl = true
			break
		}
	}
	if !hasRepl {
		return runCobra(cmd, args)
	}

	// Enter REPL mode.
	return replLoop(stdin, stdout, stderr, nil)
}

// runREPLWithHistory executes the REPL with a specific home directory
// (for testing with isolated history/config).
func runREPLWithHistory(stdin io.Reader, stdout, stderr io.Writer, args []string, homeDir string) int {
	// Home dir is set by the test via t.Setenv; just run replLoop directly.
	_ = homeDir // env is already set by caller
	return replLoop(stdin, stdout, stderr, nil)
}

// dispatchWithStdin routes zero-args without --repl.
// Without TTY and without --repl, shows cobra help.
func dispatchWithStdin(stdin io.Reader, stdout, stderr io.Writer, args []string) int {
	cmd := newRootCmd(stdout, stderr)
	cmd.SetArgs(args)
	return runCobra(cmd, args)
}

// replLoop reads lines from stdin, executes queries or slash commands,
// and writes output to stdout.
func replLoop(stdin io.Reader, stdout, stderr io.Writer, _ []string) int {
	_, _ = fmt.Fprintln(stdout, "Universal Search REPL. Type /help for commands, /quit to exit.")

	scanner := bufio.NewScanner(stdin)
	for {
		_, _ = fmt.Fprint(stdout, replPrompt)

		if !scanner.Scan() {
			break // EOF
		}
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		// Handle slash commands.
		if strings.HasPrefix(line, "/") {
			if handleSlashCommand(line, stdin, stdout, stderr) {
				return ExitSuccess // /quit
			}
			continue
		}

		// Regular query: save to history.
		if err := saveREPLHistory(line); err != nil {
			_, _ = fmt.Fprintf(stderr, "Warning: failed to save history: %v\n", err)
		}

		// For now, delegate to query subcommand.
		// In a full implementation, this would run the pipeline inline.
		_, _ = fmt.Fprintf(stdout, "Query: %s\n", line)
		_, _ = fmt.Fprintln(stdout, "(pipeline execution not yet wired in REPL)")
	}

	return ExitSuccess
}

// handleSlashCommand processes a slash command. Returns true if the REPL should exit.
func handleSlashCommand(line string, stdin io.Reader, stdout, stderr io.Writer) bool {
	parts := strings.Fields(line)
	cmd := parts[0]

	switch cmd {
	case "/quit", "/q", "/exit":
		_, _ = fmt.Fprintln(stdout, "Goodbye!")
		return true
	case "/help", "/h", "/?":
		_, _ = fmt.Fprintln(stdout, "Available commands:")
		for _, c := range slashCommands {
			_, _ = fmt.Fprintf(stdout, "  %-12s %s\n", c.Name, c.Help)
		}
	case "/sources":
		handleSourcesCommand(stdout, stderr)
	case "/history":
		handleHistoryCommand(stdout, stderr)
	case "/config":
		handleConfigCommand(stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stdout, "Unknown command: %s. Type /help for available commands.\n", cmd)
	}
	return false
}

// handleSourcesCommand displays available adapters.
func handleSourcesCommand(stdout, stderr io.Writer) {
	_, _ = fmt.Fprintln(stdout, "Sources command — adapter listing:")
	_, _ = fmt.Fprintln(stdout, "  (Use 'usearch sources list' for full details)")
	// In a full implementation, this would read from adapters.Registry.
	// For now, list known adapters from the codebase.
	adapters := []string{"arxiv", "github", "hn", "koreanews", "naver", "reddit", "searxng", "youtube"}
	for _, a := range adapters {
		_, _ = fmt.Fprintf(stdout, "  - %s\n", a)
	}
}

// handleHistoryCommand shows recent history entries.
func handleHistoryCommand(stdout, stderr io.Writer) {
	backend, err := openHistoryBackend()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: could not open history: %v\n", err)
		return
	}

	entries, err := backend.List(10)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: could not read history: %v\n", err)
		return
	}

	if len(entries) == 0 {
		_, _ = fmt.Fprintln(stdout, "No history entries.")
		return
	}

	_, _ = fmt.Fprintln(stdout, "Recent history:")
	for _, e := range entries {
		prompt := e.Prompt
		if len(prompt) > 50 {
			prompt = prompt[:47] + "..."
		}
		_, _ = fmt.Fprintf(stdout, "  %s  %s  %s\n", e.ID, e.Timestamp.Format("2006-01-02 15:04"), prompt)
	}
}

// handleConfigCommand shows current configuration.
func handleConfigCommand(stdout, stderr io.Writer) {
	cfg, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: could not load config: %v\n", err)
		return
	}

	_, _ = fmt.Fprintln(stdout, "Current configuration:")
	_, _ = fmt.Fprintf(stdout, "  Config path: %s\n", config.ConfigPath())
	_, _ = fmt.Fprintf(stdout, "  History backend: %s\n", cfg.History.Backend)
	_, _ = fmt.Fprintf(stdout, "  Max entries: %d\n", cfg.History.MaxEntries)
	_, _ = fmt.Fprintf(stdout, "  Retention days: %d\n", cfg.History.RetentionDays)
}

// saveREPLHistory writes a REPL query entry to the history backend.
func saveREPLHistory(prompt string) error {
	// Ensure data directory exists before writing.
	if err := ensureDataDir(); err != nil {
		return err
	}

	backend, err := openHistoryBackend()
	if err != nil {
		return err
	}

	entry := history.Entry{
		ID:            generateID(),
		Timestamp:     time.Now(),
		Command:       "repl",
		Prompt:        prompt,
		Category:      "interactive",
		SchemaVersion: 1,
	}
	return backend.Write(entry)
}

// generateID creates a unique ID for REPL history entries using ULID.
func generateID() string {
	return "repl-" + ulid.Make().String()
}

// newREPLCmd creates the --repl flag handler for the root command.
// REQ-CLI2-008: --repl forces interactive REPL entry.
func newREPLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repl",
		Short: "Start interactive REPL",
		Long: `Start an interactive read-eval-print loop.

Type queries to search, or use slash commands:
  /help    Show available commands
  /quit    Exit the REPL
  /sources List available search adapters
  /history Show recent query history
  /config  Show current configuration

Each query is automatically saved to history.`,
		Aliases: []string{"interactive", "i"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return replLoopWithCobra(cmd)
		},
	}

	return cmd
}

// replLoopWithCobra runs the REPL loop using cobra's IO streams.
func replLoopWithCobra(cmd *cobra.Command) error {
	code := replLoop(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), nil)
	if code != 0 {
		return exitError{code: code}
	}
	return nil
}

// registerREPL adds REPL support to the root command.
// REQ-CLI2-008: --repl flag and zero-args TTY detection.
func registerREPL(root *cobra.Command) {
	// Add --repl persistent flag.
	root.PersistentFlags().Bool("repl", false, "force interactive REPL mode")

	// Add repl subcommand.
	root.AddCommand(newREPLCmd())

	// Override root RunE to enter REPL on zero args with TTY.
	origRunE := root.RunE
	root.RunE = func(cmd *cobra.Command, args []string) error {
		replFlag, _ := cmd.Flags().GetBool("repl")

		// Enter REPL only if:
		// 1. --repl flag is explicitly set, OR
		// 2. Zero args AND stdin is a terminal AND stdin is not nil
		// The stdin nil check prevents REPL entry in test contexts where
		// stdin is not connected.
		if replFlag {
			return replLoopWithCobra(cmd)
		}
		if len(args) == 0 && isTerminal(os.Stdin) && os.Stdin != nil {
			return replLoopWithCobra(cmd)
		}

		// Fall through to original behavior (show help).
		if origRunE != nil {
			return origRunE(cmd, args)
		}
		return nil
	}

	// Ensure zero-args doesn't trigger cobra's default "required args" error.
	root.Args = cobra.NoArgs
}

// ensureDataDir creates the XDG data directory if it doesn't exist.
func ensureDataDir() error {
	dataDir := config.DataPath()
	if dataDir == "" || dataDir == "." {
		return nil
	}
	return os.MkdirAll(dataDir, 0o755)
}
