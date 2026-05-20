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
	"github.com/elymas/universal-search/internal/obs"
)

const version = "0.1.0-dev"

func main() {
	ctx := context.Background()
	o, shutdown, err := obs.Init(ctx, obs.Config{
		ServiceName:    "usearch-api",
		ServiceVersion: version,
		LogLevel:       os.Getenv("LOG_LEVEL"),
		AdminAddr:      adminAddr(),
		OTLPEndpoint:   os.Getenv("OTLP_ENDPOINT"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "obs.Init: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(ctx) }()

	o.Logger.Info("usearch-api starting", "version", version, "admin_addr", o.AdminAddr)

	// Register SPEC-SYN-004 streaming endpoint.
	// Full server mux registration lives in SPEC-IR-001; this is an additive stub.
	mux := http.NewServeMux()
	mux.Handle("/query/stream", handlers.NewSynthesisHandler(nil, handlers.DefaultConfig()))

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
