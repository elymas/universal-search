// Package main is the entrypoint for the usearch CLI.
// It supports --version / -v flags per REQ-BOOT-012.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/elymas/universal-search/internal/obs"
)

// Version is the current release version of usearch.
// Format: semver, e.g. "0.1.0-dev".
const Version = "0.1.0-dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.BoolVar(showVersion, "v", false, "print version and exit (shorthand)")
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
