// Package main — markdown formatter for the query subcommand output.
//
// SPEC-CLI-002 REQ-CLI2-006: --format markdown renders response as Markdown
// with citation links.
package main

import (
	"fmt"
	"io"
)

// formatMarkdown writes the response as Markdown with citation links.
//
// Output format:
//
//	## Answer
//
//	<summary paragraph>
//
//	## Sources
//
//	1. [Title](URL)
//	2. [Title](URL)
//	...
//
// When summary is empty (degraded mode), raw doc snippets are listed instead.
func formatMarkdown(w io.Writer, resp *queryResponse) error {
	fmt.Fprintln(w, "## Answer")
	fmt.Fprintln(w)

	if resp.Summary != "" {
		fmt.Fprintln(w, resp.Summary)
	} else {
		// Degraded mode: list raw doc snippets.
		for i, doc := range resp.Docs {
			snippet := doc.Snippet
			if snippet == "" {
				snippet = doc.Title
			}
			fmt.Fprintf(w, "%d. %s\n", i+1, snippet)
		}
	}

	if len(resp.Citations) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "## Sources")
		fmt.Fprintln(w)
		for _, c := range resp.Citations {
			fmt.Fprintf(w, "%d. [%s](%s)\n", c.Index, c.Title, c.URL)
		}
	}

	return nil
}
