// Package main is the entrypoint for the usearch MCP server.
//
// SPEC-MCP-001: Replaces the SPEC-BOOT-001 stub with a functional MCP server
// exposing search, deep_research, list_sources, and get_citation tools over
// stdio (default) and Streamable HTTP (opt-in) transports.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/mcpserver"
	"github.com/elymas/universal-search/internal/obs"
	vver "github.com/elymas/universal-search/internal/version"
)

func main() {
	// Parse flags.
	showVersion := flag.Bool("version", false, "print version and exit")
	transport := flag.String("transport", "", "transport mode: stdio (default) or http")
	flag.Parse()

	if *showVersion {
		fmt.Printf("usearch-mcp %s\n", vver.Short())
		os.Exit(0)
	}

	ctx := context.Background()
	o, shutdown, err := obs.Init(ctx, obs.Config{
		ServiceName:    "usearch-mcp",
		ServiceVersion: vver.Short(),
		LogLevel:       os.Getenv("LOG_LEVEL"),
		AdminAddr:      adminAddr(),
		OTLPEndpoint:   os.Getenv("OTLP_ENDPOINT"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "obs.Init: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(ctx) }()

	// Build config with defaults, then apply flag overrides.
	cfg := mcpserver.DefaultConfig()
	if env := os.Getenv("MCP_TRANSPORT"); env != "" {
		cfg.Transport = env
	}
	if *transport != "" {
		cfg.Transport = *transport
	}

	// Build adapter registry (reuses production registry from cmd/usearch).
	reg := buildProductionRegistry()

	srv := mcpserver.New(cfg, o, reg, nil, nil)
	if err := srv.Start(ctx); err != nil {
		o.Logger.Error("usearch-mcp failed", "error", err)
		os.Exit(1)
	}
}

func adminAddr() string {
	if v := os.Getenv("USEARCH_ADMIN_PORT"); v != "" {
		return "127.0.0.1:" + v
	}
	return ""
}

// buildProductionRegistry constructs the adapter registry for the MCP server.
// Mirrors cmd/usearch/query.go::buildProductionRegistry.
func buildProductionRegistry() *adapters.Registry {
	return adapters.NewRegistry(nil)
}
