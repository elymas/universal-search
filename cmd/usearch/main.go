package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/elymas/universal-search/internal/meta"
)

func main() {
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Printf("usearch %s\n", meta.Version)
		os.Exit(0)
	}

	fmt.Fprintln(os.Stderr, "usearch: no command given. Run with --help for usage.")
	os.Exit(1)
}
