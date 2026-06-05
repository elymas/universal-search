// Package main — sources subcommand for the usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-004: usearch sources {list,status,show}.
// SPEC-CLI-003: Registry-backed listing, live health status, derived columns.
// Wraps adapters.Registry for source discovery. Single source of truth:
// the SAME registry as the query path (pipeline.BuildProductionRegistry).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/pkg/types"
)

// validSourcesFormats lists accepted --format values for sources subcommands.
// SPEC-CLI-003 REQ-CLI3-004.
var validSourcesFormats = map[string]string{
	"human":    "human",
	"text":     "human",
	"json":     "json",
	"markdown": "markdown",
	"md":       "markdown",
}

// sourcesSchemaVersion is the JSON output schema version for sources commands.
// SPEC-CLI-003 §2.5: string "1" (matching output_json.go:19).
const sourcesSchemaVersion = "1"

// validateSourcesFormat validates the --format flag value and returns the
// canonical format name. Returns an error for unsupported values.
// SPEC-CLI-003 REQ-CLI3-004: shared helper with canonical message.
func validateSourcesFormat(format string) (string, error) {
	canonical, ok := validSourcesFormats[format]
	if !ok {
		return "", fmt.Errorf("unsupported format %q; valid: human, text, json, markdown, md", format)
	}
	return canonical, nil
}

// deriveCategory maps DocTypes to a single category string using the fixed
// priority table from SPEC-CLI-003 §2.5.
// @MX:NOTE: [AUTO] DocTypes→category mapping per SPEC-CLI-003 §2.5.
// This is the ONLY permitted hand-maintained constant (NFR-CLI3-004 exception).
func deriveCategory(docTypes []types.DocType) string {
	priority := []struct {
		match types.DocType
		cat   string
	}{
		{types.DocTypePaper, "academic"},
		{types.DocTypeRepo, "code"},
		{types.DocTypeIssue, "code"},
		{types.DocTypeVideo, "video"},
		{types.DocTypePost, "social"},
		{types.DocTypeSocial, "social"},
		{types.DocTypeArticle, "news"},
	}

	for _, p := range priority {
		for _, dt := range docTypes {
			if dt == p.match {
				return p.cat
			}
		}
	}
	return "other"
}

// deriveLang folds SupportedLangs to a single display string per §2.5.
// empty → "*" (language-agnostic), single → that code, multiple → first+.
func deriveLang(langs []string) string {
	switch len(langs) {
	case 0:
		return "*"
	case 1:
		return langs[0]
	default:
		return langs[0] + "+"
	}
}

// authRequired returns "y" if the adapter requires auth, "n" otherwise.
func authRequired(caps types.Capabilities) string {
	if caps.RequiresAuth {
		return "y"
	}
	return "n"
}

// keySet reports whether all required auth env vars are present.
// Mirrors the logic in registry.go:266-277.
func keySet(caps types.Capabilities) bool {
	if !caps.RequiresAuth || len(caps.AuthEnvVars) == 0 {
		return true
	}
	for _, ev := range caps.AuthEnvVars {
		if _, ok := os.LookupEnv(ev); !ok {
			return false
		}
	}
	return true
}

// adapterStatus represents the classified status of one adapter.
type adapterStatus struct {
	Name   string
	Status string // connected, unhealthy, disabled, not-configured
	KeySet bool
	Error  string // non-empty for unhealthy adapters
}

// probeResult is the per-adapter result collected by the concurrent probe fan-out.
type probeResult struct {
	Name    string
	Status  string
	KeySet  bool
	ErrText string
}

// classifyAdapters runs the 4-state classification algorithm per SPEC-CLI-003 §5.3.
// @MX:WARN: [AUTO] Concurrent Healthcheck fan-out with per-adapter timeout and recover().
// @MX:REASON: Uses plain sync.WaitGroup (NOT errgroup.WithContext) so one slow adapter
// does not cancel siblings per REQ-CLI3-003. Each probe has its own context.WithTimeout.
func classifyAdapters(ctx context.Context, reg *adapters.Registry, timeout time.Duration) []adapterStatus {
	// Pre-probe: build disabled set from SnapshotForAdmin (§2.6).
	disabledSet := make(map[string]bool)
	for _, v := range reg.SnapshotForAdmin() {
		if v.Status == "disabled" {
			disabledSet[v.ID] = true
		}
	}

	ids := reg.List()
	if len(ids) == 0 {
		return nil
	}

	results := make([]probeResult, len(ids))

	// Determine which adapters need probing vs pre-probe classification.
	type probeJob struct {
		index    int
		id       string
		adapter  types.Adapter
		caps     types.Capabilities
	}
	var jobs []probeJob

	for i, id := range ids {
		adapter, _ := reg.Get(id)
		caps := adapter.Capabilities()

		pr := probeResult{Name: id, KeySet: keySet(caps)}

		if disabledSet[id] {
			pr.Status = "disabled"
			results[i] = pr
			continue
		}

		if caps.RequiresAuth && len(caps.AuthEnvVars) > 0 {
			allSet := true
			for _, ev := range caps.AuthEnvVars {
				if _, ok := os.LookupEnv(ev); !ok {
					allSet = false
					break
				}
			}
			if !allSet {
				pr.Status = "not-configured"
				results[i] = pr
				continue
			}
		}

		jobs = append(jobs, probeJob{index: i, id: id, adapter: adapter, caps: caps})
	}

	// Fan-out: probe remaining adapters concurrently (non-cancelling).
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		go func(j probeJob) {
			defer wg.Done()
			pr := probeResult{Name: j.id, KeySet: keySet(j.caps)}

			// Per-adapter timeout context.
			probeCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Recover from panicking Healthcheck (§2.7).
			func() {
				defer func() {
					if r := recover(); r != nil {
						pr.Status = "unhealthy"
						pr.ErrText = fmt.Sprintf("panic: %v", r)
					}
				}()
				if err := j.adapter.Healthcheck(probeCtx); err != nil {
					pr.Status = "unhealthy"
					pr.ErrText = err.Error()
				} else {
					pr.Status = "connected"
				}
			}()

			results[j.index] = pr
		}(job)
	}
	wg.Wait()

	// Build final status list.
	statuses := make([]adapterStatus, len(results))
	for i, pr := range results {
		statuses[i] = adapterStatus{
			Name:   pr.Name,
			Status: pr.Status,
			KeySet: pr.KeySet,
			Error:  pr.ErrText,
		}
	}
	return statuses
}

// newSourcesCmd creates the cobra command tree for the sources subcommand.
// regFactory builds a fresh adapter registry per invocation.
func newSourcesCmd(regFactory func() *adapters.Registry) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage search sources",
		Long:  `View and manage available search adapters.`,
		Example: `  usearch sources list
  usearch sources status
  usearch sources show reddit`,
	}

	cmd.AddCommand(newSourcesListCmd(regFactory))
	cmd.AddCommand(newSourcesStatusCmd(regFactory))
	cmd.AddCommand(newSourcesShowCmd(regFactory))

	return cmd
}

// newSourcesListCmd creates the sources list subcommand.
// SPEC-CLI-003 REQ-CLI3-001, REQ-CLI3-004.
func newSourcesListCmd(regFactory func() *adapters.Registry) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			canonical, err := validateSourcesFormat(format)
			if err != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "usearch sources:", err)
				return exitError{code: ExitUserError, err: err}
			}

			reg := regFactory()
			ids := reg.List()

			switch canonical {
			case "json":
				return formatSourcesListJSON(cmd.OutOrStdout(), reg, ids)
			case "markdown":
				return formatSourcesListMarkdown(cmd.OutOrStdout(), reg, ids)
			default:
				return formatSourcesListHuman(cmd.OutOrStdout(), reg, ids)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "human", "output format: human, text, json, markdown, md")
	return cmd
}

// newSourcesStatusCmd creates the sources status subcommand.
// SPEC-CLI-003 REQ-CLI3-002, REQ-CLI3-003, REQ-CLI3-005.
func newSourcesStatusCmd(regFactory func() *adapters.Registry) *cobra.Command {
	var format string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show source health status",
		RunE: func(cmd *cobra.Command, args []string) error {
			canonical, fmtErr := validateSourcesFormat(format)
			if fmtErr != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "usearch sources:", fmtErr)
				return exitError{code: ExitUserError, err: fmtErr}
			}

			if timeout <= 0 {
				err := fmt.Errorf("invalid timeout %s; must be positive", timeout)
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "usearch sources:", err)
				return exitError{code: ExitUserError, err: err}
			}

			reg := regFactory()
			ctx := context.Background()
			statuses := classifyAdapters(ctx, reg, timeout)

			switch canonical {
			case "json":
				return formatSourcesStatusJSON(cmd.OutOrStdout(), statuses)
			case "markdown":
				return formatSourcesStatusMarkdown(cmd.OutOrStdout(), statuses)
			default:
				return formatSourcesStatusHuman(cmd.OutOrStdout(), statuses)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "human", "output format: human, text, json, markdown, md")
	cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "per-adapter health check timeout")
	return cmd
}

// newSourcesShowCmd creates the sources show subcommand.
// SPEC-CLI-003 REQ-CLI3-001.
func newSourcesShowCmd(regFactory func() *adapters.Registry) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show detailed source information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			reg := regFactory()

			adapter, ok := reg.Get(name)
			if !ok {
				err := fmt.Errorf("usearch sources: unknown adapter %q", name)
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), err)
				return exitError{code: ExitUserError, err: err}
			}

			caps := adapter.Capabilities()
			out := cmd.OutOrStdout()

			_, _ = fmt.Fprintf(out, "Name:           %s\n", caps.SourceID)
			_, _ = fmt.Fprintf(out, "Display Name:   %s\n", caps.DisplayName)
			_, _ = fmt.Fprintf(out, "Category:       %s\n", deriveCategory(caps.DocTypes))
			_, _ = fmt.Fprintf(out, "Languages:      %s\n", formatLangsSlice(caps.SupportedLangs))
			_, _ = fmt.Fprintf(out, "Doc Types:      %s\n", formatDocTypesSlice(caps.DocTypes))
			_, _ = fmt.Fprintf(out, "Auth Required:  %s\n", authRequired(caps))
			_, _ = fmt.Fprintf(out, "Auth Env Vars:  %s\n", strings.Join(caps.AuthEnvVars, ", "))
			_, _ = fmt.Fprintf(out, "Key Set:        %s\n", boolToStr(keySet(caps)))
			if caps.RateLimitPerMin > 0 {
				_, _ = fmt.Fprintf(out, "Rate Limit:     %d/min\n", caps.RateLimitPerMin)
			}
			return nil
		},
	}

	return cmd
}

// --- Output formatters ---

func formatSourcesListHuman(w io.Writer, reg *adapters.Registry, ids []string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tCATEGORY\tLANG\tAUTH")
	for _, id := range ids {
		adapter, _ := reg.Get(id)
		caps := adapter.Capabilities()
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			caps.SourceID,
			deriveCategory(caps.DocTypes),
			deriveLang(caps.SupportedLangs),
			authRequired(caps),
		)
	}
	return tw.Flush()
}

func formatSourcesListJSON(w io.Writer, reg *adapters.Registry, ids []string) error {
	type sourceEntry struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Lang     string `json:"lang"`
		Auth     string `json:"auth_required"`
	}

	entries := make([]sourceEntry, 0, len(ids))
	for _, id := range ids {
		adapter, _ := reg.Get(id)
		caps := adapter.Capabilities()
		entries = append(entries, sourceEntry{
			Name:     caps.SourceID,
			Category: deriveCategory(caps.DocTypes),
			Lang:     deriveLang(caps.SupportedLangs),
			Auth:     authRequired(caps),
		})
	}

	out := struct {
		SchemaVersion string        `json:"schema_version"`
		Sources       []sourceEntry `json:"sources"`
	}{
		SchemaVersion: sourcesSchemaVersion,
		Sources:       entries,
	}
	if out.Sources == nil {
		out.Sources = []sourceEntry{}
	}

	enc := json.NewEncoder(w)
	return enc.Encode(out)
}

func formatSourcesListMarkdown(w io.Writer, reg *adapters.Registry, ids []string) error {
	_, _ = fmt.Fprintln(w, "| NAME | CATEGORY | LANG | AUTH |")
	_, _ = fmt.Fprintln(w, "|------|----------|------|------|")
	for _, id := range ids {
		adapter, _ := reg.Get(id)
		caps := adapter.Capabilities()
		_, _ = fmt.Fprintf(w, "| %s | %s | %s | %s |\n",
			caps.SourceID,
			deriveCategory(caps.DocTypes),
			deriveLang(caps.SupportedLangs),
			authRequired(caps),
		)
	}
	return nil
}

func formatSourcesStatusHuman(w io.Writer, statuses []adapterStatus) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tSTATUS\tKEYS")
	for _, s := range statuses {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Name, s.Status, boolToStr(s.KeySet))
	}
	return tw.Flush()
}

func formatSourcesStatusJSON(w io.Writer, statuses []adapterStatus) error {
	type jsonSource struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		KeySet bool   `json:"key_set"`
		Error  string `json:"error,omitempty"`
	}

	sources := make([]jsonSource, 0, len(statuses))
	for _, s := range statuses {
		sources = append(sources, jsonSource{
			Name:   s.Name,
			Status: s.Status,
			KeySet: s.KeySet,
			Error:  s.Error,
		})
	}

	out := struct {
		SchemaVersion string       `json:"schema_version"`
		Sources       []jsonSource `json:"sources"`
	}{
		SchemaVersion: sourcesSchemaVersion,
		Sources:       sources,
	}
	if out.Sources == nil {
		out.Sources = []jsonSource{}
	}

	enc := json.NewEncoder(w)
	return enc.Encode(out)
}

func formatSourcesStatusMarkdown(w io.Writer, statuses []adapterStatus) error {
	_, _ = fmt.Fprintln(w, "| NAME | STATUS | KEYS |")
	_, _ = fmt.Fprintln(w, "|------|--------|------|")
	for _, s := range statuses {
		_, _ = fmt.Fprintf(w, "| %s | %s | %s |\n", s.Name, s.Status, boolToStr(s.KeySet))
	}
	return nil
}

// --- Helpers ---

func formatLangsSlice(langs []string) string {
	if len(langs) == 0 {
		return "* (language-agnostic)"
	}
	return strings.Join(langs, ", ")
}

func formatDocTypesSlice(docTypes []types.DocType) string {
	if len(docTypes) == 0 {
		return "(none)"
	}
	strs := make([]string, len(docTypes))
	for i, dt := range docTypes {
		strs[i] = string(dt)
	}
	return strings.Join(strs, ", ")
}

func boolToStr(b bool) string {
	if b {
		return "y"
	}
	return "n"
}
