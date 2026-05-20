// Package main — text formatter for the query subcommand output.
//
// REQ-CLI-004: --format text (default) writes the synthesized paragraph followed
// by a Citations block with numbered [N] <Title> — <URL> entries.
package main

import (
	"fmt"
	"io"
)

// formatText writes the human-readable answer + citation list to w (stdout).
//
// Output format:
//
//	<summary paragraph>
//
//	Citations:
//	[1] <Title> — <URL>
//	[2] <Title> — <URL>
//	...
//
// When summary is empty (degraded mode), raw doc snippets are numbered instead.
func formatText(w io.Writer, resp *queryResponse) error {
	if resp.Summary != "" {
		_, err := fmt.Fprintln(w, resp.Summary)
		if err != nil {
			return err
		}
	} else {
		// Degraded mode: print raw doc snippets with manual numbering.
		for i, doc := range resp.Docs {
			snippet := doc.Snippet
			if snippet == "" {
				snippet = doc.Title
			}
			if _, err := fmt.Fprintf(w, "[%d] %s\n", i+1, snippet); err != nil {
				return err
			}
		}
	}

	if len(resp.Citations) > 0 {
		if _, err := fmt.Fprintln(w, "\nCitations:"); err != nil {
			return err
		}
		for _, c := range resp.Citations {
			if _, err := fmt.Fprintf(w, "[%d] %s — %s\n", c.Index, c.Title, c.URL); err != nil {
				return err
			}
		}
	}
	return nil
}
