// Package main — query subcommand orchestrator.
//
// REQ-CLI-001: usearch binary dispatches on the first positional argument.
// REQ-CLI-002: query subcommand parses flags via a per-subcommand *flag.FlagSet.
// REQ-CLI-005: total-pipeline deadline from --timeout (default 30s, max 5m).
// REQ-CLI-006: payload to stdout ONLY; progress/errors to stderr ONLY.
// REQ-CLI-008: exit codes 0/1/2/3 per pipeline outcome.
// REQ-CLI-010: root OTel span "usearch.cli.query" with standard attributes.
// REQ-CLI-011: request ID via reqid.New() attached to context.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/oklog/ulid/v2"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/internal/pipeline"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/internal/usearch/history"
	"github.com/elymas/universal-search/pkg/types"
)

const (
	defaultTimeout = 30 * time.Second
	maxTimeout     = 5 * time.Minute
)

// synthResult is the internal representation of a synthesis result.
// Maps to internal/synthesis.Result but avoids a hard import for testability.
type synthResult = pipeline.SynthResult

// synthCitation is a single citation from the synthesis client.
type synthCitation = pipeline.SynthCitation

// synthClientIface is the interface used by the CLI to call the synthesis client.
// The real client (internal/synthesis.Client) and the nopSynthClient both satisfy this.
type synthClientIface = pipeline.SynthClient

// errSynthUnavailable is a sentinel for the nop synthesis client (REQ-CLI-009).
var errSynthUnavailable = pipeline.ErrSynthUnavailable

// queryFlags holds the parsed flags for the query subcommand.
type queryFlags struct {
	Source  []string
	Format  string
	Timeout time.Duration
}

// executeOption allows test injection of registry and synth client.
type executeOption func(*executeConfig)

type executeConfig struct {
	registry *adapters.Registry
	synth    synthClientIface
	history  history.Backend
}

// withRegistry injects a custom adapter registry for testing.
func withRegistry(reg *adapters.Registry) executeOption {
	return func(c *executeConfig) { c.registry = reg }
}

// withSynth injects a custom synthesis client for testing.
func withSynth(s synthClientIface) executeOption {
	return func(c *executeConfig) { c.synth = s }
}

// withHistory wires a history backend so each query persists an entry.
// When nil (the default), history recording is skipped — keeping unit tests
// free of filesystem side effects.
func withHistory(b history.Backend) executeOption {
	return func(c *executeConfig) { c.history = b }
}

// Execute is the public entry point for the query subcommand.
//
// @MX:ANCHOR: [AUTO] Sole CLI entry boundary for the query subcommand.
// @MX:REASON: fan_in >= 3 — called from main.go dispatcher, query_test.go, and
// future SPEC-CLI-002 cobra wrapper. Signature stability is critical for migration.
// @MX:SPEC: SPEC-CLI-001
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer, opts ...executeOption) int {
	cfg := &executeConfig{}
	for _, o := range opts {
		o(cfg)
	}

	start := time.Now()

	// Parse flags.
	flags, prompt, err := parseQueryFlags(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return ExitUserError
	}

	// Validate prompt (REQ-CLI-007).
	if strings.TrimFunc(prompt, unicode.IsSpace) == "" {
		_, _ = fmt.Fprintln(stderr, "usearch query: prompt argument required")
		return ExitUserError
	}

	// Validate format (REQ-CLI-004, REQ-CLI2-006).
	if flags.Format != "text" && flags.Format != "json" && flags.Format != "markdown" {
		_, _ = fmt.Fprintf(stderr, "usearch query: unsupported format %q; valid: text, json, markdown\n", flags.Format)
		return ExitUserError
	}

	// Validate timeout (REQ-CLI-005).
	if flags.Timeout > maxTimeout {
		_, _ = fmt.Fprintf(stderr, "usearch query: --timeout exceeds maximum %s\n", maxTimeout)
		return ExitUserError
	}

	// Attach request ID (REQ-CLI-011).
	rid := reqid.New()
	ctx = reqid.WithContext(ctx, rid)

	// Apply pipeline deadline (REQ-CLI-005).
	ctx, cancel := context.WithTimeout(ctx, flags.Timeout)
	defer cancel()

	// Start root OTel span (REQ-CLI-010).
	tracer := otel.Tracer("usearch.cli")
	spanCtx, span := tracer.Start(ctx, "usearch.cli.query")
	defer span.End()

	sourceFilterCount := len(flags.Source)
	span.SetAttributes(
		attribute.Int("cli.prompt_length", len(prompt)),
		attribute.String("cli.format", flags.Format),
		attribute.Int("cli.source_filter_count", sourceFilterCount),
	)

	// Build adapter registry (use injected or production registry).
	reg := cfg.registry
	if reg == nil {
		reg = pipeline.BuildProductionRegistry()
	}

	// Validate source filter against registry (REQ-CLI-003).
	if len(flags.Source) > 0 {
		for _, src := range flags.Source {
			if _, ok := reg.Get(src); !ok {
				_, _ = fmt.Fprintf(stderr, "usearch query: unknown adapter %q\n", src)
				return ExitUserError
			}
		}
	}

	// Build router.
	rtr, routerErr := pipeline.BuildRouter(reg)
	if routerErr != nil {
		_, _ = fmt.Fprintf(stderr, "usearch query: router init failed: %v\n", routerErr)
		return ExitSystemError
	}

	// Classify query (REQ-CLI-001).
	decision, classifyErr := rtr.Classify(spanCtx, router.RouterQuery{
		Query: types.Query{Text: prompt},
	})
	if classifyErr != nil {
		_, _ = fmt.Fprintf(stderr, "usearch query: classify failed: %v\n", classifyErr)
		return ExitUserError
	}

	// Intersect source filter with router decision (REQ-CLI-003).
	effectiveSet := intersectSources(decision.AdapterSet, flags.Source)
	if len(effectiveSet) == 0 {
		if len(flags.Source) > 0 {
			// Source was specified but router produced empty intersection.
			_, _ = fmt.Fprintln(stderr, "usearch query: no adapters matched the source filter and routing decision")
			return ExitSystemError
		}
		_, _ = fmt.Fprintln(stderr, "usearch query: no adapters matched for this query")
		return ExitSystemError
	}

	span.SetAttributes(attribute.String("cli.adapter_set", strings.Join(effectiveSet, ",")))

	// Emit router progress (REQ-CLI-006).
	prog := newProgressEmitter(flags.Format, stderr)
	prog.Emit("router", fmt.Sprintf("classified as %s (lang=%s, adapters=%s)",
		decision.Category, decision.Lang, strings.Join(effectiveSet, ",")))

	// Build Fanout dispatcher (SPEC-FAN-001).
	// Decision.AdapterSet is the effective set already narrowed by source filter above.
	fanoutDecision := router.RoutingDecision{
		Category:   decision.Category,
		AdapterSet: effectiveSet,
		Lang:       decision.Lang,
	}
	f, fanoutInitErr := fanout.New(fanout.Options{Registry: reg})
	if fanoutInitErr != nil {
		_, _ = fmt.Fprintf(stderr, "usearch query: fanout init failed: %v\n", fanoutInitErr)
		return ExitSystemError
	}

	// Fanout to adapters (REQ-CLI-005: context timeout propagated via spanCtx).
	prog.Emit("fanout", fmt.Sprintf("querying %d adapters", len(effectiveSet)))
	fanoutResult, _ := f.Dispatch(spanCtx, fanoutDecision, types.Query{Text: prompt})
	docs := fanoutResult.Docs
	adapterErrs := fanoutResult.AdapterErrors

	// Emit adapter error warnings (REQ-CLI-006).
	for name, aerr := range adapterErrs {
		_, _ = fmt.Fprintf(stderr, "usearch query: adapter %q error: %v\n", name, aerr)
	}

	// Check for context timeout (REQ-CLI-005).
	if spanCtx.Err() != nil {
		_, _ = fmt.Fprintln(stderr, "usearch query: timeout: fanout stage — pipeline deadline exceeded")
		span.SetAttributes(attribute.Int("cli.exit_code", ExitSystemError))
		return ExitSystemError
	}

	// Check for all-adapters-failed (REQ-CLI-008).
	if len(docs) == 0 && len(adapterErrs) > 0 {
		_, _ = fmt.Fprintln(stderr, "usearch query: all adapters failed")
		span.SetAttributes(attribute.Int("cli.exit_code", ExitSystemError))
		return ExitSystemError
	}

	// Synthesize (REQ-CLI-008, REQ-CLI-009).
	synth := cfg.synth
	if synth == nil {
		synth = pipeline.BuildProductionSynth()
	}

	prog.Emit("synthesis", fmt.Sprintf("synthesizing from %d docs", len(docs)))
	synthResp, synthErr := synth.Synthesize(spanCtx, prompt, decision.Lang, docs)

	// Build response struct.
	resp := buildQueryResponse(prompt, decision, effectiveSet, docs, synthResp, synthErr, rid)

	// Determine exit code (REQ-CLI-008).
	exitCode := determineExitCode(docs, adapterErrs, synthResp, synthErr)

	// Persist a history entry (best-effort; warn but never fail the query).
	if cfg.history != nil {
		entry := history.Entry{
			ID:            "query-" + ulid.Make().String(),
			Timestamp:     start,
			Command:       "query",
			Prompt:        prompt,
			Category:      string(decision.Category),
			Adapters:      effectiveSet,
			Summary:       synthResp.Text,
			Citations:     len(resp.Citations),
			ExitCode:      exitCode,
			LatencyMs:     time.Since(start).Milliseconds(),
			RequestID:     rid,
			SchemaVersion: 1,
		}
		if werr := cfg.history.Write(entry); werr != nil {
			_, _ = fmt.Fprintf(stderr, "usearch query: warning: failed to save history: %v\n", werr)
		}
	}

	// Emit synthesis warning on nop client (REQ-CLI-009).
	if errors.Is(synthErr, errSynthUnavailable) {
		_, _ = fmt.Fprintln(stderr, "[synthesis: unavailable]")
	} else if synthErr != nil {
		_, _ = fmt.Fprintf(stderr, "usearch query: synthesis failed: %v\n", synthErr)
	}

	// Format and write output to stdout (REQ-CLI-006, REQ-CLI2-006).
	var fmtErr error
	switch flags.Format {
	case "json":
		fmtErr = formatJSON(stdout, resp)
	case "markdown":
		fmtErr = formatMarkdown(stdout, resp)
	default:
		fmtErr = formatText(stdout, resp)
	}
	if fmtErr != nil {
		_, _ = fmt.Fprintf(stderr, "usearch query: format output error: %v\n", fmtErr)
		return ExitSystemError
	}

	span.SetAttributes(attribute.Int("cli.exit_code", exitCode))
	return exitCode
}

// parseQueryFlags parses args for the query subcommand.
// Returns queryFlags, the positional prompt, and any parse error.
// Uses flag.ContinueOnError so usage errors return an error rather than calling os.Exit.
func parseQueryFlags(args []string) (queryFlags, string, error) {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // suppress default usage output; we handle errors ourselves

	var sourceStr string
	var format string
	var timeout time.Duration
	var noObs bool

	fs.StringVar(&sourceStr, "source", "", "comma-separated adapter names (empty = all enabled)")
	fs.StringVar(&format, "format", "text", "output format: text (default), json, or markdown")
	fs.DurationVar(&timeout, "timeout", defaultTimeout, "total pipeline deadline (max 5m)")
	fs.BoolVar(&noObs, "no-obs", false, "disable observability init (test flag)")

	if err := fs.Parse(args); err != nil {
		return queryFlags{}, "", fmt.Errorf("%w: %s", errUserInput, err.Error())
	}

	positionals := fs.Args()
	if len(positionals) == 0 {
		return queryFlags{}, "", fmt.Errorf("%w: usearch query: prompt argument required", errUserInput)
	}
	if len(positionals) > 1 {
		return queryFlags{}, "", fmt.Errorf("%w: usearch query: exactly one positional argument expected, got %d", errUserInput, len(positionals))
	}

	var sources []string
	if strings.TrimSpace(sourceStr) != "" {
		for _, s := range strings.Split(sourceStr, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				sources = append(sources, s)
			}
		}
	}

	return queryFlags{
		Source:  sources,
		Format:  format,
		Timeout: timeout,
	}, positionals[0], nil
}

// intersectSources returns the subset of adapterSet that matches sourceFilter.
// When sourceFilter is empty, adapterSet is returned unchanged (all adapters).
func intersectSources(adapterSet, sourceFilter []string) []string {
	if len(sourceFilter) == 0 {
		return adapterSet
	}
	filterMap := make(map[string]bool, len(sourceFilter))
	for _, s := range sourceFilter {
		filterMap[s] = true
	}
	var result []string
	for _, name := range adapterSet {
		if filterMap[name] {
			result = append(result, name)
		}
	}
	return result
}

// newProgressEmitter returns the appropriate progress emitter for the format.
// JSON format uses a no-op emitter (structured logging handles progress).
func newProgressEmitter(format string, stderr io.Writer) progressEmitter {
	if format == "json" {
		return &jsonProgress{}
	}
	return &humanProgress{w: stderr}
}

// buildQueryResponse assembles the queryResponse from pipeline outputs.
func buildQueryResponse(
	prompt string,
	decision router.RoutingDecision,
	adapters []string,
	docs []types.NormalizedDoc,
	synth synthResult,
	synthErr error,
	requestID string,
) *queryResponse {
	var citations []queryCitation
	if synthErr == nil {
		for _, c := range synth.Citations {
			citations = append(citations, queryCitation{
				Index:  c.Marker,
				Title:  c.Title,
				URL:    c.URL,
				Source: sourceFromDocs(c.DocID, docs),
				DocID:  c.DocID,
			})
		}
	}

	synthOK := synthErr == nil && synth.Text != ""
	return &queryResponse{
		Query:     prompt,
		Category:  string(decision.Category),
		Lang:      decision.Lang,
		Adapters:  adapters,
		Summary:   synth.Text,
		Citations: citations,
		Stats: queryStats{
			RequestID:    requestID,
			AdapterCount: len(adapters),
			DocCount:     len(docs),
			SynthOK:      synthOK,
		},
		Docs: docs,
	}
}

// sourceFromDocs looks up the SourceID of a doc by ID.
func sourceFromDocs(docID string, docs []types.NormalizedDoc) string {
	for _, d := range docs {
		if d.ID == docID {
			return d.SourceID
		}
	}
	return ""
}

// determineExitCode computes the exit code from pipeline outcomes (REQ-CLI-008).
func determineExitCode(
	docs []types.NormalizedDoc,
	adapterErrs map[string]error,
	synth synthResult,
	synthErr error,
) int {
	hasDocs := len(docs) > 0
	hasAdapterErrs := len(adapterErrs) > 0
	synthOK := synthErr == nil && synth.Text != ""

	if hasDocs && synthOK && !hasAdapterErrs {
		return ExitSuccess
	}
	if hasDocs && (synthErr != nil || hasAdapterErrs) {
		return ExitPartial
	}
	return ExitSystemError
}
