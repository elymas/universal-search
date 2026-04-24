// Package main is the stub entrypoint for the usearch REST/gRPC API server.
// Full implementation lands in SPEC-IR-001.
package main

import (
	"context"
	"fmt"
	"os"

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
	fmt.Fprintln(os.Stderr, "usearch-api: not implemented (see SPEC-IR-001)")
	os.Exit(0)
}

func adminAddr() string {
	if v := os.Getenv("USEARCH_ADMIN_PORT"); v != "" {
		return "127.0.0.1:" + v
	}
	return ""
}
