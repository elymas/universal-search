// Package main — history subcommand tree for the usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-011: usearch history {list,show,search,clear}.
// Manages query/deep invocation history via the configured backend.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/elymas/universal-search/internal/usearch/config"
	"github.com/elymas/universal-search/internal/usearch/history"
)

// newHistoryCmd creates the cobra command tree for the history subcommand.
func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Manage query history",
		Long: `Manage query and deep invocation history.

History is persisted to a JSONL file by default. Entries include prompt,
summary, adapters used, latency, and cost.`,
		Example: `  usearch history list
  usearch history list --limit 5
  usearch history list --since 7d
  usearch history show abc123
  usearch history search "golang"
  usearch history clear
  usearch history clear --confirm
  usearch history clear --since 30d --confirm`,
	}

	cmd.AddCommand(newHistoryListCmd())
	cmd.AddCommand(newHistoryShowCmd())
	cmd.AddCommand(newHistorySearchCmd())
	cmd.AddCommand(newHistoryClearCmd())

	return cmd
}

// parseSince parses a duration string like "7d", "24h", "30m" into a time.Duration.
func parseSince(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %q", s)
	}
	unit := s[len(s)-1]
	value := s[:len(s)-1]
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number: %q", value)
	}
	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %c (use d, h, or m)", unit)
	}
}

// openHistoryBackend creates a history backend from the current config.
func openHistoryBackend() (*history.JSONLBackend, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	historyPath := config.HistoryPath(cfg.History.Backend)
	return history.NewJSONLBackend(historyPath, cfg.History.MaxEntries, cfg.History.RetentionDays)
}

// newHistoryListCmd returns the history list subcommand.
func newHistoryListCmd() *cobra.Command {
	var limit int
	var since string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent history entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, err := openHistoryBackend()
			if err != nil {
				return exitError{code: ExitSystemError, err: err}
			}

			entries, err := backend.List(limit)
			if err != nil {
				return exitError{code: ExitSystemError, err: err}
			}

			// Filter by --since if provided.
			if since != "" {
				dur, parseErr := parseSince(since)
				if parseErr != nil {
					return exitError{code: ExitUserError, err: parseErr}
				}
				cutoff := time.Now().Add(-dur)
				var filtered []history.Entry
				for _, e := range entries {
					if e.Timestamp.After(cutoff) {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}

			if len(entries) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No history entries.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, e := range entries {
				prompt := e.Prompt
				if len(prompt) > 60 {
					prompt = prompt[:57] + "..."
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.ID, e.Timestamp.Format("2006-01-02 15:04:05"), e.Command, prompt)
			}
			_ = w.Flush()
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of entries to show")
	cmd.Flags().StringVar(&since, "since", "", "show entries from the last N duration (e.g. 7d, 24h)")

	return cmd
}

// newHistoryShowCmd returns the history show subcommand.
func newHistoryShowCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of a history entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, err := openHistoryBackend()
			if err != nil {
				return exitError{code: ExitSystemError, err: err}
			}

			entry, err := backend.Get(args[0])
			if err != nil {
				return exitError{code: ExitUserError, err: err}
			}

			switch format {
			case "json":
				data, jsonErr := json.MarshalIndent(entry, "", "  ")
				if jsonErr != nil {
					return exitError{code: ExitSystemError, err: jsonErr}
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			default:
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ID:       %s\n", entry.ID)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Time:     %s\n", entry.Timestamp.Format(time.RFC3339))
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Command:  %s\n", entry.Command)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Prompt:   %s\n", entry.Prompt)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Category: %s\n", entry.Category)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Adapters: %s\n", strings.Join(entry.Adapters, ", "))
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Summary:  %s\n", entry.Summary)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Latency:  %dms\n", entry.LatencyMs)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Cost:     $%.4f\n", entry.CostUSD)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Exit:     %d\n", entry.ExitCode)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text (default) or json")

	return cmd
}

// newHistorySearchCmd returns the history search subcommand.
func newHistorySearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <prompt>",
		Short: "Search history by prompt substring",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, err := openHistoryBackend()
			if err != nil {
				return exitError{code: ExitSystemError, err: err}
			}

			results, err := backend.Search(args[0])
			if err != nil {
				return exitError{code: ExitSystemError, err: err}
			}

			if len(results) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No matching entries.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, e := range results {
				prompt := e.Prompt
				if len(prompt) > 60 {
					prompt = prompt[:57] + "..."
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", e.ID, e.Timestamp.Format("2006-01-02 15:04:05"), prompt)
			}
			_ = w.Flush()
			return nil
		},
	}

	return cmd
}

// newHistoryClearCmd returns the history clear subcommand.
func newHistoryClearCmd() *cobra.Command {
	var confirm bool
	var since string

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear history entries",
		Long:  "Clear history entries. Requires --confirm for non-interactive use. Use --since to clear only entries older than the cutoff.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if stdin is a TTY.
			isTTY := isTerminal(os.Stdin)

			if !isTTY && !confirm {
				return exitError{
					code: ExitUserError,
					err:  fmt.Errorf("usearch history clear: --confirm required for non-interactive use"),
				}
			}

			if isTTY && !confirm {
				// Interactive prompt.
				fmt.Fprint(os.Stderr, "Are you sure you want to clear history? [y/N] ")
				var response string
				_, _ = fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
					return nil
				}
			}

			backend, err := openHistoryBackend()
			if err != nil {
				return exitError{code: ExitSystemError, err: err}
			}

			var sinceTime time.Time
			if since != "" {
				dur, parseErr := parseSince(since)
				if parseErr != nil {
					return exitError{code: ExitUserError, err: parseErr}
				}
				sinceTime = time.Now().Add(-dur)
			}

			if err := backend.Clear(sinceTime); err != nil {
				return exitError{code: ExitSystemError, err: err}
			}

			if sinceTime.IsZero() {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "History cleared.")
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Cleared entries older than %s.\n", sinceTime.Format("2006-01-02"))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "confirm clearing (required for non-interactive)")
	cmd.Flags().StringVar(&since, "since", "", "clear only entries older than this duration (e.g. 30d)")

	return cmd
}

// isTerminal checks if the file is a terminal (TTY).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
