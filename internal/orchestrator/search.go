// Package orchestrator provides the shared search pipeline used by both the
// CLI (cmd/usearch) and the MCP server (internal/mcpserver).
//
// SPEC-MCP-001 REQ-MCP-008: The search tool MUST NOT duplicate the
// orchestration logic; both surfaces depend on this single implementation.
package orchestrator

import (
	"context"
	"strings"
	"time"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// SearchParams contains the input parameters for a search operation.
type SearchParams struct {
	// Query is the search text.
	Query string
	// Sources is an optional list of adapter names to restrict to.
	Sources []string
	// Timeout is the pipeline deadline. Zero means no explicit deadline.
	Timeout time.Duration
}

// SearchResult contains the output of a search operation.
type SearchResult struct {
	// Summary is the synthesized text (empty when synthesis fails).
	Summary string
	// Citations contains the extracted citations.
	Citations []Citation
	// Docs are the raw documents from fanout.
	Docs []types.NormalizedDoc
	// AdapterErrors maps adapter names to their errors.
	AdapterErrors map[string]error
	// Category is the routing classification.
	Category string
	// Lang is the detected/specified language.
	Lang string
	// AdapterSet is the effective set of adapters used.
	AdapterSet []string
	// RequestID is the unique request identifier.
	RequestID string
}

// Citation represents a single citation in search results.
type Citation struct {
	DocID  string
	Title  string
	URL    string
	Source string
	Marker int
}

// SynthFunc is the function signature for synthesis. Abstracted so both
// CLI (synthesis.Client) and tests (stubs) can provide it.
type SynthFunc func(ctx context.Context, query, lang string, docs []types.NormalizedDoc) (string, []Citation, error)

// Search executes the shared search pipeline: router -> fanout -> synthesis.
//
// @MX:ANCHOR: [AUTO] Shared search pipeline; callers: CLI, MCP server, tests
// @MX:REASON: fan_in >= 2; single source of truth for basic-mode pipeline;
// both surfaces depend on identical behaviour.
// @MX:SPEC: SPEC-MCP-001
func Search(ctx context.Context, reg *adapters.Registry, params SearchParams, synth SynthFunc) (*SearchResult, error) {
	// Build router.
	rtr, err := router.New(router.Options{Registry: reg})
	if err != nil {
		return nil, err
	}

	// Apply timeout if specified.
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}

	// Classify query.
	decision, err := rtr.Classify(ctx, router.RouterQuery{
		Query: types.Query{Text: params.Query},
	})
	if err != nil {
		return nil, err
	}

	// Intersect source filter with router decision.
	effectiveSet := intersectSources(decision.AdapterSet, params.Sources)
	if len(effectiveSet) == 0 {
		return &SearchResult{
			Category:      string(decision.Category),
			Lang:          decision.Lang,
			AdapterErrors: map[string]error{},
		}, ErrNoAdaptersMatched
	}

	// Build fanout dispatcher.
	f, err := fanout.New(fanout.Options{Registry: reg})
	if err != nil {
		return nil, err
	}

	// Fanout to adapters.
	fanoutDecision := router.RoutingDecision{
		Category:   decision.Category,
		AdapterSet: effectiveSet,
		Lang:       decision.Lang,
	}
	fanoutResult, _ := f.Dispatch(ctx, fanoutDecision, types.Query{Text: params.Query})

	// Check for all adapters failed. Per-adapter details are surfaced via
	// SearchResult.AdapterErrors; callers format the breakdown as needed.
	if len(fanoutResult.Docs) == 0 && len(fanoutResult.AdapterErrors) > 0 {
		return &SearchResult{
			Docs:          fanoutResult.Docs,
			AdapterErrors: fanoutResult.AdapterErrors,
			Category:      string(decision.Category),
			Lang:          decision.Lang,
			AdapterSet:    effectiveSet,
		}, ErrAllAdaptersFailed
	}

	// Synthesize.
	var summary string
	var citations []Citation
	var synthErr error
	if synth != nil {
		summary, citations, synthErr = synth(ctx, params.Query, decision.Lang, fanoutResult.Docs)
	}

	return &SearchResult{
		Summary:       summary,
		Citations:     citations,
		Docs:          fanoutResult.Docs,
		AdapterErrors: fanoutResult.AdapterErrors,
		Category:      string(decision.Category),
		Lang:          decision.Lang,
		AdapterSet:    effectiveSet,
	}, synthErr
}

// intersectSources returns the subset of adapterSet matching sourceFilter.
func intersectSources(adapterSet, sourceFilter []string) []string {
	if len(sourceFilter) == 0 {
		return adapterSet
	}
	filterMap := make(map[string]bool, len(sourceFilter))
	for _, s := range sourceFilter {
		filterMap[strings.TrimSpace(s)] = true
	}
	var result []string
	for _, name := range adapterSet {
		if filterMap[name] {
			result = append(result, name)
		}
	}
	return result
}
