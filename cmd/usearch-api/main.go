// Package main is the stub entrypoint for the usearch REST/gRPC API server.
// Full implementation lands in SPEC-IR-001.
//
// SPEC-SYN-004: Registers POST /query/stream (SSE streaming synthesis endpoint)
// as an additive change; all other server scaffolding remains in SPEC-IR-001.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/elymas/universal-search/cmd/usearch-api/handlers"
	"github.com/elymas/universal-search/internal/adapters"
	adminapi "github.com/elymas/universal-search/internal/api/admin"
	"github.com/elymas/universal-search/internal/obs"
	vver "github.com/elymas/universal-search/internal/version"
)

func main() {
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

	// Register SPEC-SYN-004 streaming endpoint.
	// Full server mux registration lives in SPEC-IR-001; this is an additive stub.
	mux := http.NewServeMux()
	mux.Handle("/query/stream", handlers.NewSynthesisHandler(nil, handlers.DefaultConfig()))

	// SPEC-UI-002 Phase A: Admin route group under /api/admin/.
	// All admin endpoints are protected by loopback-only middleware (REQ-LH-001).
	reg := adapters.NewRegistry(nil) // populated by SPEC-IR-001 in full implementation
	registerAdminRoutes(mux, reg)

	_ = mux // mux is registered but server.ListenAndServe is owned by SPEC-IR-001.

	fmt.Fprintln(os.Stderr, "usearch-api: not implemented (see SPEC-IR-001)")
	os.Exit(0)
}

func adminAddr() string {
	if v := os.Getenv("USEARCH_ADMIN_PORT"); v != "" {
		return "127.0.0.1:" + v
	}
	return ""
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
