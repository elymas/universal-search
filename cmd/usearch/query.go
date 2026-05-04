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
	"os"
	"strings"
	"time"
	"unicode"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/adapters/hn"
	"github.com/elymas/universal-search/internal/adapters/reddit"
	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/internal/synthesis"
	"github.com/elymas/universal-search/pkg/types"
)

const (
	defaultTimeout = 30 * time.Second
	maxTimeout     = 5 * time.Minute
)

// synthResult is the internal representation of a synthesis result.
// Maps to internal/synthesis.Result but avoids a hard import for testability.
type synthResult struct {
	Text      string
	Citations []synthCitation
}

// synthCitation is a single citation from the synthesis client.
type synthCitation struct {
	Marker int
	DocID  string
	URL    string
	Title  string
}

// synthClientIface is the interface used by the CLI to call the synthesis client.
// The real client (internal/synthesis.Client) and the nopSynthClient both satisfy this.
type synthClientIface interface {
	Synthesize(ctx context.Context, query, lang string, docs []types.NormalizedDoc) (synthResult, error)
}

// errSynthUnavailable is a sentinel for the nop synthesis client (REQ-CLI-009).
var errSynthUnavailable = errors.New("synthesis: client unavailable")

// nopSynthClient is a test-only no-op implementation of synthClientIface.
// Used to verify REQ-CLI-009 degraded mode behavior.
type nopSynthClient struct{}

func (n *nopSynthClient) Synthesize(_ context.Context, _, _ string, _ []types.NormalizedDoc) (synthResult, error) {
	return synthResult{}, errSynthUnavailable
}

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
}

// withRegistry injects a custom adapter registry for testing.
func withRegistry(reg *adapters.Registry) executeOption {
	return func(c *executeConfig) { c.registry = reg }
}

// withSynth injects a custom synthesis client for testing.
func withSynth(s synthClientIface) executeOption {
	return func(c *executeConfig) { c.synth = s }
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

	// Parse flags.
	flags, prompt, err := parseQueryFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUserError
	}

	// Validate prompt (REQ-CLI-007).
	if strings.TrimFunc(prompt, unicode.IsSpace) == "" {
		fmt.Fprintln(stderr, "usearch query: prompt argument required")
		return ExitUserError
	}

	// Validate format (REQ-CLI-004).
	if flags.Format != "text" && flags.Format != "json" {
		fmt.Fprintf(stderr, "usearch query: unsupported format %q; valid: text, json\n", flags.Format)
		return ExitUserError
	}

	// Validate timeout (REQ-CLI-005).
	if flags.Timeout > maxTimeout {
		fmt.Fprintf(stderr, "usearch query: --timeout exceeds maximum %s\n", maxTimeout)
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
		reg = buildProductionRegistry()
	}

	// Validate source filter against registry (REQ-CLI-003).
	if len(flags.Source) > 0 {
		for _, src := range flags.Source {
			if _, ok := reg.Get(src); !ok {
				fmt.Fprintf(stderr, "usearch query: unknown adapter %q\n", src)
				return ExitUserError
			}
		}
	}

	// Build router.
	rtr, routerErr := buildRouter(reg)
	if routerErr != nil {
		fmt.Fprintf(stderr, "usearch query: router init failed: %v\n", routerErr)
		return ExitSystemError
	}

	// Classify query (REQ-CLI-001).
	decision, classifyErr := rtr.Classify(spanCtx, router.RouterQuery{
		Query: types.Query{Text: prompt},
	})
	if classifyErr != nil {
		fmt.Fprintf(stderr, "usearch query: classify failed: %v\n", classifyErr)
		return ExitUserError
	}

	// Intersect source filter with router decision (REQ-CLI-003).
	effectiveSet := intersectSources(decision.AdapterSet, flags.Source)
	if len(effectiveSet) == 0 {
		if len(flags.Source) > 0 {
			// Source was specified but router produced empty intersection.
			fmt.Fprintln(stderr, "usearch query: no adapters matched the source filter and routing decision")
			return ExitSystemError
		}
		fmt.Fprintln(stderr, "usearch query: no adapters matched for this query")
		return ExitSystemError
	}

	span.SetAttributes(attribute.String("cli.adapter_set", strings.Join(effectiveSet, ",")))

	// Emit router progress (REQ-CLI-006).
	prog := newProgressEmitter(flags.Format, stderr)
	prog.Emit("router", fmt.Sprintf("classified as %s (lang=%s, adapters=%s)",
		decision.Category, decision.Lang, strings.Join(effectiveSet, ",")))

	// Fanout to adapters (REQ-CLI-005: context timeout propagated).
	prog.Emit("fanout", fmt.Sprintf("querying %d adapters", len(effectiveSet)))
	docs, adapterErrs := runFanout(spanCtx, effectiveSet, reg, prompt)

	// Emit adapter error warnings (REQ-CLI-006).
	for name, aerr := range adapterErrs {
		fmt.Fprintf(stderr, "usearch query: adapter %q error: %v\n", name, aerr)
	}

	// Check for context timeout (REQ-CLI-005).
	if spanCtx.Err() != nil {
		fmt.Fprintln(stderr, "usearch query: timeout: fanout stage — pipeline deadline exceeded")
		span.SetAttributes(attribute.Int("cli.exit_code", ExitSystemError))
		return ExitSystemError
	}

	// Check for all-adapters-failed (REQ-CLI-008).
	if len(docs) == 0 && len(adapterErrs) > 0 {
		fmt.Fprintln(stderr, "usearch query: all adapters failed")
		span.SetAttributes(attribute.Int("cli.exit_code", ExitSystemError))
		return ExitSystemError
	}

	// Synthesize (REQ-CLI-008, REQ-CLI-009).
	synth := cfg.synth
	if synth == nil {
		synth = buildProductionSynth()
	}

	prog.Emit("synthesis", fmt.Sprintf("synthesizing from %d docs", len(docs)))
	synthResp, synthErr := synth.Synthesize(spanCtx, prompt, decision.Lang, docs)

	// Build response struct.
	resp := buildQueryResponse(prompt, decision, effectiveSet, docs, synthResp, synthErr, rid)

	// Determine exit code (REQ-CLI-008).
	exitCode := determineExitCode(docs, adapterErrs, synthResp, synthErr)

	// Emit synthesis warning on nop client (REQ-CLI-009).
	if errors.Is(synthErr, errSynthUnavailable) {
		fmt.Fprintln(stderr, "[synthesis: unavailable]")
	} else if synthErr != nil {
		fmt.Fprintf(stderr, "usearch query: synthesis failed: %v\n", synthErr)
	}

	// Format and write output to stdout (REQ-CLI-006).
	var fmtErr error
	if flags.Format == "json" {
		fmtErr = formatJSON(stdout, resp)
	} else {
		fmtErr = formatText(stdout, resp)
	}
	if fmtErr != nil {
		fmt.Fprintf(stderr, "usearch query: format output error: %v\n", fmtErr)
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
	fs.StringVar(&format, "format", "text", "output format: text (default) or json")
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

// runFanout dispatches the query to each adapter in effectiveSet concurrently.
// Returns the merged docs slice and a map of adapter name -> error for adapters that failed.
//
// @MX:ANCHOR: [AUTO] CLI-internal fanout; replacement target when SPEC-FAN-001 lands.
// @MX:REASON: fan_in >= 2 (Execute + tests); clean function boundary for future swap.
// @MX:SPEC: SPEC-CLI-001
//
// @MX:WARN: [AUTO] runFanout spawns one goroutine per adapter using errgroup.
// @MX:REASON: goroutine cancellation discipline is load-bearing for NFR-CLI-002
// (goleak zero-leak requirement); all goroutines must select on ctx.Done().
// @MX:SPEC: SPEC-CLI-001
func runFanout(ctx context.Context, names []string, reg *adapters.Registry, prompt string) (
	docs []types.NormalizedDoc, errs map[string]error,
) {
	type result struct {
		name string
		docs []types.NormalizedDoc
		err  error
	}

	results := make([]result, len(names))
	eg, egCtx := errgroup.WithContext(ctx)

	for i, name := range names {
		i, name := i, name // capture loop vars
		eg.Go(func() error {
			ad, ok := reg.Get(name)
			if !ok {
				results[i] = result{name: name, err: fmt.Errorf("adapter %q not found in registry", name)}
				return nil
			}
			docs, err := ad.Search(egCtx, types.Query{Text: prompt})
			results[i] = result{name: name, docs: docs, err: err}
			return nil // never return error to eg; collect individually
		})
	}

	// Wait for all goroutines. Context cancellation propagates via egCtx.
	_ = eg.Wait()

	errs = make(map[string]error)
	for _, r := range results {
		if r.err != nil {
			errs[r.name] = r.err
		} else {
			docs = append(docs, r.docs...)
		}
	}

	// If context was cancelled (timeout), return context error.
	if ctx.Err() != nil {
		return docs, errs
	}

	return docs, errs
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

// buildProductionRegistry constructs the adapter registry for production use.
//
// Registers Reddit (SPEC-ADP-001) and Hacker News (SPEC-ADP-002). Both
// adapters have RequiresAuth=false so no env-var preconditions apply.
// In tests, withRegistry() injects an alternative registry.
//
// Env-var overrides (used by NFR-CLI-001 integration tests):
//   - REDDIT_BASE_URL: redirects Reddit adapter to a stub HTTP server
//   - HN_BASE_URL: redirects HN adapter to a stub HTTP server
// Empty values fall back to the adapter's compiled-in defaults.
//
// @MX:NOTE: [AUTO] Production adapter wiring per SPEC-CLI-001 §2.1(m).
// New M3 adapters are registered here; auth-gated adapters check
// Capabilities.AuthEnvVars before Register.
// @MX:SPEC: SPEC-CLI-001
func buildProductionRegistry() *adapters.Registry {
	reg := adapters.NewRegistry(nil)

	if a, err := reddit.New(reddit.Options{
		BaseURL: os.Getenv("REDDIT_BASE_URL"),
	}); err == nil {
		_ = reg.Register(a)
	}
	if a, err := hn.New(hn.Options{
		BaseURL: os.Getenv("HN_BASE_URL"),
	}); err == nil {
		_ = reg.Register(a)
	}

	return reg
}

// buildRouter constructs the Intent Router from a registry.
// The production path uses env-configured LLM client.
func buildRouter(reg *adapters.Registry) (*router.Router, error) {
	return router.New(router.Options{
		Registry: reg,
	})
}

// buildProductionSynth constructs the synthesis client for production use.
//
// Wires the real synthesis.Client (SPEC-SYN-001) using RESEARCHER_BASE_URL
// and RESEARCHER_REQUEST_TIMEOUT_SECONDS env vars. Falls back to
// nopSynthClient if config load fails or client construction errors;
// the nop fallback satisfies REQ-CLI-009 degraded-mode behavior.
//
// obs is nil here: REQ-SYN-006 guarantees the synthesis.Client is nil-safe
// across obs.Obs, individual collectors, and obs.Logger.
func buildProductionSynth() synthClientIface {
	cfg, err := synthesis.LoadConfig()
	if err != nil {
		return &nopSynthClient{}
	}
	client, err := synthesis.New(cfg, nil)
	if err != nil {
		return &nopSynthClient{}
	}
	return &productionSynthAdapter{client: client}
}

// productionSynthAdapter bridges *synthesis.Client to synthClientIface.
//
// @MX:NOTE: [AUTO] Type adapter — synthesis.Result -> synthResult mapping
// to keep cmd/usearch decoupled from internal/synthesis concrete types.
// @MX:SPEC: SPEC-CLI-001
type productionSynthAdapter struct {
	client *synthesis.Client
}

func (a *productionSynthAdapter) Synthesize(ctx context.Context, query, lang string, docs []types.NormalizedDoc) (synthResult, error) {
	res, err := a.client.Synthesize(ctx, query, lang, docs)
	if err != nil {
		return synthResult{}, err
	}
	citations := make([]synthCitation, len(res.Citations))
	for i, c := range res.Citations {
		citations[i] = synthCitation{
			Marker: c.Marker,
			DocID:  c.DocID,
			URL:    c.URL,
			Title:  c.Title,
		}
	}
	return synthResult{Text: res.Text, Citations: citations}, nil
}
