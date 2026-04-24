// Package main is the entrypoint for the usearch CLI.
// It supports --version / -v flags per REQ-BOOT-012.
// SPEC-LLM-001: LLM client is initialised when LITELLM_MASTER_KEY is set
// (or --no-llm is absent). Use --no-llm to skip LLM init for testing.
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
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.BoolVar(showVersion, "v", false, "print version and exit (shorthand)")
	noLLM := flag.Bool("no-llm", false, "skip LLM client initialisation")
	flag.Parse()

	// --version exits before obs.Init to avoid any side-effects.
	if *showVersion {
		fmt.Printf("usearch v%s\n", Version)
		os.Exit(0)
	}

	ctx := context.Background()
	o, shutdown, err := obs.Init(ctx, obs.Config{
		ServiceName:    "usearch",
		ServiceVersion: Version,
		LogLevel:       os.Getenv("LOG_LEVEL"),
		AdminAddr:      adminAddr(),
		OTLPEndpoint:   os.Getenv("OTLP_ENDPOINT"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "obs.Init: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(ctx) }()

	o.Logger.Info("usearch starting", "version", Version, "admin_addr", o.AdminAddr)

	// Initialise LLM client when LITELLM_MASTER_KEY is set and --no-llm is absent.
	// SPEC-LLM-001: REQ-LLM-001
	if !*noLLM && os.Getenv("LITELLM_MASTER_KEY") != "" {
		cfg, cfgErr := llmconfig.Load()
		if cfgErr != nil {
			o.Logger.Error("llm config load failed", slog.String("err", cfgErr.Error()))
			os.Exit(1)
		}
		llmClient, llmErr := llm.New(cfg, o)
		if llmErr != nil {
			o.Logger.Error("llm client init failed", slog.String("err", llmErr.Error()))
			os.Exit(1)
		}
		defer func() { _ = llmClient.Close() }()
		o.Logger.Info("llm client ready",
			slog.String("base_url", cfg.BaseURL),
			slog.Float64("budget_usd", cfg.PerRequestCapUSD),
		)
	} else if !*noLLM {
		o.Logger.Warn("llm disabled: LITELLM_MASTER_KEY not set; use --no-llm to suppress this warning")
	}

	fmt.Fprintln(os.Stderr, "usearch: no command given. Use --version for version info.")
	os.Exit(1)
}

// adminAddr returns the admin server bind address from the environment.
func adminAddr() string {
	if v := os.Getenv("USEARCH_ADMIN_PORT"); v != "" {
		return "127.0.0.1:" + v
	}
	return ""
}
