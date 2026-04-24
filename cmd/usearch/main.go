// Package main is the entrypoint for the usearch CLI.
// It supports --version / -v flags per REQ-BOOT-012.
package main

import (
	"flag"
	"fmt"
	"os"
)

// Version is the current release version of usearch.
// Format: semver, e.g. "0.1.0-dev".
const Version = "0.1.0-dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.BoolVar(showVersion, "v", false, "print version and exit (shorthand)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("usearch v%s\n", Version)
		os.Exit(0)
	}

	fmt.Fprintln(os.Stderr, "usearch: no command given. Use --version for version info.")
	os.Exit(1)
}
