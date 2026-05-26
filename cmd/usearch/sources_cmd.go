// Package main — sources subcommand for the usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-004: usearch sources {list,status,show}.
// Wraps adapters.Registry for source discovery.
package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// knownAdapters lists the adapters available in the codebase.
// In a full implementation, this reads from adapters.Registry.List().
var knownAdapters = []adapterInfo{
	{name: "arxiv", category: "academic", desc: "arXiv preprint repository"},
	{name: "github", category: "code", desc: "GitHub repositories and issues"},
	{name: "hn", category: "social", desc: "Hacker News stories and comments"},
	{name: "koreanews", category: "news", desc: "Korean news sources"},
	{name: "naver", category: "search", desc: "Naver search API"},
	{name: "reddit", category: "social", desc: "Reddit posts and comments"},
	{name: "searxng", category: "meta", desc: "SearXNG meta-search engine"},
	{name: "youtube", category: "video", desc: "YouTube video search"},
}

type adapterInfo struct {
	name     string
	category string
	desc     string
}

// newSourcesCmd creates the cobra command tree for the sources subcommand.
func newSourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage search sources",
		Long:  `View and manage available search adapters.`,
		Example: `  usearch sources list
	  usearch sources status
	  usearch sources show reddit`,
	}

	cmd.AddCommand(newSourcesListCmd())
	cmd.AddCommand(newSourcesStatusCmd())
	cmd.AddCommand(newSourcesShowCmd())

	return cmd
}

// newSourcesListCmd creates the sources list subcommand.
func newSourcesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tCATEGORY\tDESCRIPTION")
			for _, a := range knownAdapters {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", a.name, a.category, a.desc)
			}
			_ = w.Flush()
			return nil
		},
	}

	return cmd
}

// newSourcesStatusCmd creates the sources status subcommand.
func newSourcesStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show source health status",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Source health check not yet implemented.")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Available sources:")
			for _, a := range knownAdapters {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", a.name, "unknown")
			}
			return nil
		},
	}

	return cmd
}

// newSourcesShowCmd creates the sources show subcommand.
func newSourcesShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show detailed source information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			for _, a := range knownAdapters {
				if a.name == name {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Name:     %s\n", a.name)
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Category: %s\n", a.category)
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", a.desc)
					return nil
				}
			}

			return exitError{
				code: ExitUserError,
				err:  fmt.Errorf("unknown source: %q", name),
			}
		},
	}

	return cmd
}
