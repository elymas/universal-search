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
	"github.com/elymas/universal-search/internal/adapters/meta"
	"github.com/elymas/universal-search/internal/adapters/social"
	"github.com/elymas/universal-search/internal/mcpserver"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/security/secretstore"
	vver "github.com/elymas/universal-search/internal/version"
)

// xEnabledLookup resolves the USEARCH_X_ENABLED config flag. It is a seam over
// os.Getenv so tests can drive the flag deterministically without mutating the
// process environment (which is unsafe under -race). The flag is config, not a
// secret, so it stays off the secretstore.Resolver path.
var xEnabledLookup = os.Getenv

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

// buildProductionRegistry constructs the adapter registry for the MCP server
// using the default (env) Resolver. Thin backward-compatible wrapper over
// buildProductionRegistryWithResolver.
func buildProductionRegistry() *adapters.Registry {
	return buildProductionRegistryWithResolver(nil)
}

// buildProductionRegistryWithResolver constructs the adapter registry for the
// MCP server, resolving credentialed adapters' SECRETS through the injected
// secretstore.Resolver (SPEC-SEC-002 cred-02). When resolver is nil, an
// EnvResolver is used, preserving prior env-only behavior. The non-secret
// USEARCH_X_ENABLED flag stays on the env path (config, not a secret).
//
// Credentialed adapters (Threads, X) declare RequiresAuth + AuthEnvVars, so
// they MUST be registered with SkipAuthCheck: true — otherwise the registry's
// env-only auth gate would reject a token sourced from a non-env backend.
//
// REQ-ADP10-001: Threads is registered only when THREADS_ACCESS_TOKEN resolves.
// REQ-ADP10-008: Facebook is NOT registered (no viable search path).
func buildProductionRegistryWithResolver(r secretstore.Resolver) *adapters.Registry {
	if r == nil {
		r = secretstore.NewEnvResolver()
	}
	ctx := context.Background()
	reg := adapters.NewRegistry(nil)

	// Threads: secret-gated registration via Resolver (SPEC-ADP-010 D1,
	// SPEC-SEC-002 cred-02). Missing creds (incl. vault ErrNotImplemented) →
	// silent skip, matching prior optional-adapter behavior.
	if token, err := r.Get(ctx, "THREADS_ACCESS_TOKEN"); err == nil && token != "" {
		if a, err := meta.NewThreads(meta.ThreadsOptions{AccessToken: token}); err == nil {
			_ = reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true})
		}
	}

	// X (Twitter): env-gated registration (SPEC-ADP-006-XENABLE). The enable
	// flag is config (os.Getenv); the bearer token is a secret (Resolver).
	// @MX:WARN: [AUTO] ToS + secret gate for a ToS-grey source.
	// @MX:REASON: registering X without the env + ToS-ack gates violates tech.md:147.
	// @MX:SPEC: SPEC-ADP-006-XENABLE
	if xEnabledLookup("USEARCH_X_ENABLED") == "true" {
		if prov, ok := buildXProvider(ctx, r); ok {
			if a, err := social.NewX(social.XOptions{Provider: prov}); err == nil {
				_ = reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true})
			}
		}
	}

	return reg
}

// buildXProvider constructs an X provider, resolving the bearer token via the
// injected Resolver (SPEC-SEC-002 cred-02). Returns (provider, true) when the
// token resolves; (nil, false) otherwise (incl. resolver error / empty token).
// SPEC-ADP-006-XENABLE: Option A (X official API v2) is the default provider.
func buildXProvider(ctx context.Context, r secretstore.Resolver) (social.XProvider, bool) {
	bearerToken, err := r.Get(ctx, "X_BEARER_TOKEN")
	if err != nil || bearerToken == "" {
		return nil, false
	}
	prov, err := social.NewXOfficialProvider(social.XOfficialOptions{
		BearerToken: bearerToken,
	})
	if err != nil {
		return nil, false
	}
	return prov, true
}
