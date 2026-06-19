// Package main is the entrypoint for the usearch HTTP API server.
//
// SPEC-API-001: Makes the usearch-api server functional by wiring the production
// search pipeline and serving the frontend's HTTP contract.
//
// REQ-API-001: Server starts and calls ListenAndServe.
// REQ-API-003: Graceful shutdown on SIGINT/SIGTERM.
// REQ-API-004: --healthcheck flag for Dockerfile HEALTHCHECK.
// REQ-API-015: Corrected stale "SPEC-IR-001" references.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elymas/universal-search/cmd/usearch-api/handlers"
	"github.com/elymas/universal-search/internal/adapters"
	adminapi "github.com/elymas/universal-search/internal/api/admin"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/internal/pipeline"
	vver "github.com/elymas/universal-search/internal/version"
)

func main() {
	// REQ-API-004: --healthcheck flag for Docker HEALTHCHECK.
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		runHealthcheck()
		return
	}

	ctx := context.Background()
	o, shutdown, err := obs.Init(ctx, obs.Config{
		ServiceName:    "usearch-api",
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

	o.Logger.Info("usearch-api starting", "version", vver.Short(), "admin_addr", o.AdminAddr)

	// REQ-API-001: Build production search pipeline.
	asm, err := pipeline.BuildProductionAssembly()
	if err != nil {
		o.Logger.Error("failed to build pipeline assembly", "error", err)
		os.Exit(1)
	}

	// Wire HTTP handlers.
	mux := http.NewServeMux()

	// SPEC-API-001: Search API endpoints.
	svc := newProdSearchService(asm)
	apiHandler := NewHandler(svc)
	mux.Handle("/", apiHandler)

	// SPEC-SYN-004: Streaming synthesis endpoint (POST, internal).
	mux.Handle("/query/stream", handlers.NewSynthesisHandler(nil, handlers.DefaultConfig()))

	// SPEC-UI-002: Admin route group under /api/admin/.
	registerAdminRoutes(mux, asm.Registry)

	// REQ-API-001: Determine listen address.
	addr := apiAddr()
	o.Logger.Info("usearch-api listening", "addr", addr)

	// REQ-API-003: Graceful shutdown on SIGINT/SIGTERM.
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine.
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			o.Logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	o.Logger.Info("shutting down", "signal", sig)

	// Graceful shutdown with timeout.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		o.Logger.Error("server forced to shutdown", "error", err)
	}

	o.Logger.Info("usearch-api stopped")
}

// apiAddr returns the listen address from USEARCH_API_PORT (default :8080).
func apiAddr() string {
	if v := os.Getenv("USEARCH_API_PORT"); v != "" {
		if v[0] != ':' {
			return ":" + v
		}
		return v
	}
	return ":8080"
}

// adminAddr returns the admin listen address from USEARCH_ADMIN_PORT.
func adminAddr() string {
	if v := os.Getenv("USEARCH_ADMIN_PORT"); v != "" {
		return "127.0.0.1:" + v
	}
	return ""
}

// runHealthcheck probes the server's /healthz endpoint and exits appropriately.
// REQ-API-004: Used by Dockerfile HEALTHCHECK directive.
func runHealthcheck() {
	addr := apiAddr()
	url := "http://localhost" + addr + "/healthz"

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: failed to connect to %s: %v\n", url, err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: unexpected status %d\n", resp.StatusCode)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "healthcheck: ok")
	os.Exit(0)
}

// registerAdminRoutes mounts all /api/admin/* endpoints on mux with loopback
// middleware. The registry is used by handlers to read adapter state.
//
// @MX:NOTE: [AUTO] Admin route group. Loopback middleware is applied per-route
// so Go's default mux pattern matching does not bypass it.
// @MX:SPEC: SPEC-UI-002 REQ-LH-002
func registerAdminRoutes(mux *http.ServeMux, reg *adapters.Registry) {
	// SPEC-UI-002 Phase A: adapter listing.
	adaptersHandler := adminapi.LoopbackOnly(adminapi.NewAdaptersHandler(reg))
	mux.Handle("/api/admin/adapters", adaptersHandler)

	// SPEC-EVAL-002 REQ-EVAL2-010b: adapter health surface on the SAME mux,
	// behind the same LoopbackOnly middleware (no new server/port).
	mux.Handle("/api/admin/adapters/health", adminapi.LoopbackOnly(adminapi.NewAdaptersHealthHandler(reg)))

	// SPEC-UI-002 Phase B: adapter actions.
	mux.Handle("POST /api/admin/adapters/{id}/resync", adminapi.LoopbackOnly(adminapi.NewResyncHandler(reg)))
	mux.Handle("POST /api/admin/adapters/{id}/toggle", adminapi.LoopbackOnly(adminapi.NewToggleHandler(reg)))

	// SPEC-UI-002 Phase B: audit queries.
	// TODO: wire a real AuditQuerier implementation when audit store gains a
	// QueryEntries method. Until then, the handler returns empty results.
	mux.Handle("/api/admin/audit/queries", adminapi.LoopbackOnly(adminapi.NewAuditHandler(nil)))
}
