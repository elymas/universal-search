// Package main is the entrypoint for the usearch CLI.
// It supports --version / -v flags per REQ-BOOT-012.
// SPEC-LLM-001: LLM client is initialised when LITELLM_MASTER_KEY is set
// (or --no-llm is absent). Use --no-llm to skip LLM init for testing.
// SPEC-CLI-001: Subcommand dispatcher routes to query subcommand.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/elymas/universal-search/internal/llm"
	llmconfig "github.com/elymas/universal-search/internal/llm/config"
	"github.com/elymas/universal-search/internal/obs"
)

// Version is the current release version of usearch.
// Format: semver, e.g. "0.1.0-dev".
const Version = "0.1.0-dev"

func main() {
	args := os.Args[1:]

	// @MX:NOTE: [AUTO] Subcommand dispatcher inspects the first non-flag argument.
	// --version / -v are pre-dispatched before subcommand routing to preserve
	// existing REQ-BOOT-012 semantics without obs.Init side-effects.
	// @MX:SPEC: SPEC-CLI-001
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usearch: no command given. Use --version for version info or 'query' to search.")
		os.Exit(2)
	}

	code := dispatch(args)
	os.Exit(code)
}

// dispatch routes args to the correct subcommand handler.
// Returns the exit code. Never calls os.Exit itself.
func dispatch(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usearch: no command given. Use --version for version info.")
		fmt.Fprintln(os.Stderr, "available: query, --version")
		return ExitSystemError
	}

	switch args[0] {
	case "--version", "-v":
		fmt.Printf("usearch v%s\n", Version)
		return ExitSuccess

	case "query":
		return runQueryWithObs(args[1:])

	case "--help", "-h", "help":
		fmt.Fprintln(os.Stderr, usageText())
		return ExitSuccess

	default:
		fmt.Fprintf(os.Stderr, "usearch: unknown subcommand %q; available: query, --version\n", args[0])
		return ExitSystemError
	}
}

// usageText returns the help text for the usearch binary.
func usageText() string {
	return `usearch - Universal Search CLI

Usage:
  usearch query [flags] <prompt>   Search and synthesize results
  usearch --version                Print version and exit
  usearch --help                   Show this help

Query flags:
  --source string    Comma-separated adapter names (empty = all)
  --format string    Output format: text (default) or json
  --timeout duration Pipeline deadline (default 30s, max 5m)
`
}

// runQueryWithObs runs the query subcommand with full observability initialisation.
// In production, obs.Init wires slog/prometheus/otel. For unit tests, --no-obs is
// passed via Execute() options instead.
func runQueryWithObs(args []string) int {
	ctx := context.Background()

	// Check for --no-llm early so we can configure obs before handing off.
	noLLMFlag := flag.NewFlagSet("pre", flag.ContinueOnError)
	noLLM := noLLMFlag.Bool("no-llm", false, "skip LLM")
	_ = noLLMFlag.Parse(args)

	o, shutdown, err := obs.Init(ctx, obs.Config{
		ServiceName:    "usearch",
		ServiceVersion: Version,
		LogLevel:       os.Getenv("LOG_LEVEL"),
		AdminAddr:      adminAddr(),
		OTLPEndpoint:   os.Getenv("OTLP_ENDPOINT"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "obs.Init: %v\n", err)
		return ExitSystemError
	}
	defer func() { _ = shutdown(ctx) }()

	_ = o // obs bundle available for future registry wiring

	// Initialise LLM client when LITELLM_MASTER_KEY is set and --no-llm is absent.
	if !*noLLM && os.Getenv("LITELLM_MASTER_KEY") != "" {
		cfg, cfgErr := llmconfig.Load()
		if cfgErr != nil {
			o.Logger.Error("llm config load failed", slog.String("err", cfgErr.Error()))
			return ExitSystemError
		}
		llmClient, llmErr := llm.New(cfg, o)
		if llmErr != nil {
			o.Logger.Error("llm client init failed", slog.String("err", llmErr.Error()))
			return ExitSystemError
		}
		defer func() { _ = llmClient.Close() }()
		o.Logger.Info("llm client ready",
			slog.String("base_url", cfg.BaseURL),
			slog.Float64("budget_usd", cfg.PerRequestCapUSD),
		)
	} else if !*noLLM {
		o.Logger.Warn("llm disabled: LITELLM_MASTER_KEY not set; use --no-llm to suppress this warning")
	}

	return Execute(ctx, args, os.Stdout, os.Stderr)
}

// adminAddr returns the admin server bind address from the environment.
func adminAddr() string {
	if v := os.Getenv("USEARCH_ADMIN_PORT"); v != "" {
		return "127.0.0.1:" + v
	}
	return ""
}
