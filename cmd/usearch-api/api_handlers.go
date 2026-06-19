// Package main — HTTP API handlers for the usearch-api server.
//
// SPEC-API-001: HTTP endpoints serving the Next.js frontend search contract.
// REQ-API-006: Buffered search (GET /api/query)
// REQ-API-008: Streaming search (GET /api/query/stream)
// REQ-API-009: SSE event translation (sentence/citation/complete/error)
// REQ-API-011: Adapter listing (GET /api/sources)
// REQ-API-012: Empty history (GET /api/history)
// REQ-API-005: Health check (GET /healthz)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/internal/pipeline"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/internal/sse"
	"github.com/elymas/universal-search/pkg/types"
)

// SearchResult is the JSON response shape for buffered search (REQ-API-006).
// Matches the frontend's SearchResult interface in api.ts.
type SearchResult struct {
	Answer      string     `json:"answer"`
	Citations   []Citation `json:"citations"`
	Query       string     `json:"query"`
	SourcesUsed []string   `json:"sources_used"`
	ElapsedMs   int64      `json:"elapsed_ms"`
}

// Citation is a single citation in the search response (REQ-API-006, REQ-API-009).
// Matches the frontend's Citation interface in api.ts.
type Citation struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Source  string `json:"source"`
}

// SourceInfo describes an adapter for the /api/sources response (REQ-API-011).
// Matches the frontend's AdapterInfo interface in api.ts.
type SourceInfo struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Enabled  bool   `json:"enabled"`
}

// SearchService is the interface for running searches (REQ-API-017).
// Allows test doubles to replace the full pipeline.
type SearchService interface {
	// Search runs the full pipeline and returns a buffered result.
	Search(ctx context.Context, query string, sources []string) (*SearchResult, error)
	// ListSources returns adapter metadata for /api/sources.
	ListSources() []SourceInfo
}

// Handler serves the HTTP API endpoints.
type Handler struct {
	svc SearchService
	mux *http.ServeMux
}

// NewHandler creates a Handler and registers all routes.
func NewHandler(svc SearchService) *Handler {
	h := &Handler{svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.handleHealthz)
	mux.HandleFunc("GET /api/query", h.handleQuery)
	mux.HandleFunc("GET /api/query/stream", h.handleStreamQuery)
	mux.HandleFunc("GET /api/sources", h.handleSources)
	mux.HandleFunc("GET /api/history", h.handleHistory)
	h.mux = mux
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// handleHealthz returns 200 OK with {"status":"ok"} (REQ-API-005).
func (h *Handler) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleQuery runs a buffered search and returns JSON (REQ-API-006).
func (h *Handler) handleQuery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, `{"error":"missing query parameter 'q'"}`, http.StatusBadRequest)
		return
	}

	var sources []string
	if s := r.URL.Query().Get("sources"); s != "" {
		for _, src := range strings.Split(s, ",") {
			src = strings.TrimSpace(src)
			if src != "" {
				sources = append(sources, src)
			}
		}
	}

	// Attach request ID (REQ-API-013).
	rid := reqid.New()
	ctx := reqid.WithContext(r.Context(), rid)
	w.Header().Set("X-Request-Id", rid)

	result, err := h.svc.Search(ctx, q, sources)
	if err != nil {
		// REQ-API-010: degraded mode
		if isUnknownSource(err) {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
			return
		}
		slog.WarnContext(ctx, "search error", "error", err, "request_id", rid)
		// Return degraded result rather than 500
		result = &SearchResult{
			Answer:      "",
			Query:       q,
			SourcesUsed: sources,
			ElapsedMs:   0,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// handleStreamQuery runs a streaming search and returns SSE (REQ-API-008).
//
// @MX:WARN: [AUTO] SSE event translation — sentence/citation/complete/error
// @MX:REASON: REQ-API-009 mandates deriving citation events from CitationRef and
// renaming done→complete; incorrect translation breaks frontend SSE listeners.
func (h *Handler) handleStreamQuery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, `{"error":"missing query parameter 'q'"}`, http.StatusBadRequest)
		return
	}

	var sources []string
	if s := r.URL.Query().Get("sources"); s != "" {
		for _, src := range strings.Split(s, ",") {
			src = strings.TrimSpace(src)
			if src != "" {
				sources = append(sources, src)
			}
		}
	}

	// Attach request ID (REQ-API-013).
	rid := reqid.New()
	ctx := reqid.WithContext(r.Context(), rid)

	// Set SSE headers.
	sw := sse.NewWriter(w)
	sw.SetHeaders()
	w.WriteHeader(http.StatusOK)

	// Run the full pipeline.
	result, err := h.svc.Search(ctx, q, sources)
	if err != nil {
		// REQ-API-010: emit error event
		errData, _ := json.Marshal(map[string]string{"message": err.Error()})
		_ = sw.WriteEvent("error", errData)
		_ = sw.Flush()
		return
	}

	start := time.Now()

	// REQ-API-009: Segment the answer into sentences and emit SSE events.
	sentences := segmentSentences(result.Answer)

	// Build citation map from result citations.
	citationMap := make(map[int]Citation, len(result.Citations))
	for _, c := range result.Citations {
		citationMap[c.Index] = c
	}

	emitted := 0
	for _, sentence := range sentences {
		// Check for context cancellation.
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Extract citation markers [N] from the sentence.
		markers := extractMarkers(sentence)
		if len(markers) == 0 {
			// Emit sentence without citations (REQ-SYN4-001c says skip uncited,
			// but for the API endpoint we emit all sentences for completeness).
			data, _ := json.Marshal(map[string]interface{}{
				"text":      sentence,
				"citations": []interface{}{},
			})
			_ = sw.WriteEvent("sentence", data)
			_ = sw.Flush()
			emitted++
			continue
		}

		// Resolve citations from the result.
		var citedRefs []map[string]interface{}
		for _, m := range markers {
			if c, ok := citationMap[m]; ok {
				citedRefs = append(citedRefs, map[string]interface{}{
					"marker": m,
					"doc_id": "",
					"url":    c.URL,
					"title":  c.Title,
				})
			}
		}

		// Emit sentence event.
		data, _ := json.Marshal(map[string]interface{}{
			"text":      sentence,
			"citations": citedRefs,
		})
		_ = sw.WriteEvent("sentence", data)
		_ = sw.Flush()

		// REQ-API-009: Derive citation events for each distinct marker.
		for _, m := range markers {
			if c, ok := citationMap[m]; ok {
				citData, _ := json.Marshal(c)
				_ = sw.WriteEvent("citation", citData)
			}
		}
		_ = sw.Flush()
		emitted++
	}

	// REQ-API-009: done → complete
	completeData, _ := json.Marshal(map[string]interface{}{
		"elapsed_ms": time.Since(start).Milliseconds(),
	})
	_ = sw.WriteEvent("complete", completeData)
	_ = sw.Flush()
}

// handleSources returns the adapter listing (REQ-API-011).
func (h *Handler) handleSources(w http.ResponseWriter, _ *http.Request) {
	sources := h.svc.ListSources()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sources)
}

// handleHistory returns an empty array (REQ-API-012, Decision Point D4).
func (h *Handler) handleHistory(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("[]"))
}

// --- SSE helpers ---

// sentenceRegex matches sentence boundaries (from streamsynth).
var sentenceRegex = regexp.MustCompile(`[.!?。！？]\s+|[.!?。！？]$`)

// markerRegex extracts [N] citation markers.
var markerRegex = regexp.MustCompile(`\[(\d+)\]`)

// segmentSentences splits text into sentences.
func segmentSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var sentences []string
	prev := 0
	matches := sentenceRegex.FindAllStringIndex(text, -1)
	for _, m := range matches {
		end := m[1]
		sentence := strings.TrimSpace(text[prev:end])
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
		prev = end
	}
	if prev < len(text) {
		trailing := strings.TrimSpace(text[prev:])
		if trailing != "" {
			sentences = append(sentences, trailing)
		}
	}
	return sentences
}

// extractMarkers returns unique citation marker numbers from text.
func extractMarkers(text string) []int {
	matches := markerRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[int]bool)
	var result []int
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err != nil || seen[n] {
			continue
		}
		seen[n] = true
		result = append(result, n)
	}
	return result
}

// isUnknownSource checks if the error is an unknown source error.
func isUnknownSource(err error) bool {
	if _, ok := err.(*unknownSourceError); ok {
		return true
	}
	return false
}

// --- Production SearchService ---

// prodSearchService implements SearchService using the real pipeline.
type prodSearchService struct {
	reg   *adapters.Registry
	rtr   *router.Router
	fan   *fanout.Fanout
	synth pipeline.SynthClient
}

// newProdSearchService creates a production search service from an Assembly.
func newProdSearchService(asm *pipeline.Assembly) *prodSearchService {
	return &prodSearchService{
		reg:   asm.Registry,
		rtr:   asm.Router,
		fan:   asm.Fanout,
		synth: asm.Synth,
	}
}

// Search runs the full search pipeline (REQ-API-006, REQ-API-007).
func (p *prodSearchService) Search(ctx context.Context, query string, sources []string) (*SearchResult, error) {
	start := time.Now()

	// Validate source filter against registry (REQ-API-007).
	if len(sources) > 0 {
		for _, s := range sources {
			if _, ok := p.reg.Get(s); !ok {
				return nil, &unknownSourceError{name: s}
			}
		}
	}

	// Classify query.
	decision, err := p.rtr.Classify(ctx, router.RouterQuery{
		Query: types.Query{Text: query},
	})
	if err != nil {
		return nil, err
	}

	// Intersect source filter with router decision (REQ-API-007).
	effectiveSet := intersectSources(decision.AdapterSet, sources)
	if len(effectiveSet) == 0 {
		return &SearchResult{
			Query:       query,
			SourcesUsed: []string{},
			ElapsedMs:   time.Since(start).Milliseconds(),
		}, nil
	}

	// Dispatch fanout.
	fanoutDecision := router.RoutingDecision{
		Category:   decision.Category,
		AdapterSet: effectiveSet,
		Lang:       decision.Lang,
	}
	fanoutResult, _ := p.fan.Dispatch(ctx, fanoutDecision, types.Query{Text: query})
	docs := fanoutResult.Docs

	// Synthesize.
	synthResp, synthErr := p.synth.Synthesize(ctx, query, decision.Lang, docs)

	// Build citation list (REQ-API-006).
	var citations []Citation
	if synthErr == nil {
		for _, c := range synthResp.Citations {
			snippet := ""
			source := ""
			// Resolve source and snippet via doc_id lookup.
			for _, d := range docs {
				if d.ID == c.DocID {
					source = d.SourceID
					snippet = d.Snippet
					break
				}
			}
			citations = append(citations, Citation{
				Index:   c.Marker,
				Title:   c.Title,
				URL:     c.URL,
				Snippet: snippet,
				Source:  source,
			})
		}
	}

	answer := synthResp.Text
	if synthErr != nil {
		answer = "" // Degraded: empty answer (REQ-API-010)
	}

	return &SearchResult{
		Answer:      answer,
		Citations:   citations,
		Query:       query,
		SourcesUsed: effectiveSet,
		ElapsedMs:   time.Since(start).Milliseconds(),
	}, nil
}

// ListSources returns adapter metadata from registry (REQ-API-011).
func (p *prodSearchService) ListSources() []SourceInfo {
	names := p.reg.List()
	out := make([]SourceInfo, 0, len(names))
	for _, name := range names {
		a, ok := p.reg.Get(name)
		if !ok {
			continue
		}
		caps := a.Capabilities()
		cat := ""
		if len(caps.DocTypes) > 0 {
			cat = string(caps.DocTypes[0])
		}
		out = append(out, SourceInfo{
			Name:     name,
			Category: cat,
			Enabled:  true, // REQ-API-011a: all registered adapters are enabled in v0
		})
	}
	return out
}

// intersectSources returns the subset of adapterSet matching sourceFilter.
// When sourceFilter is empty, adapterSet is returned unchanged.
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

// unknownSourceError is returned when a source name is not in the registry.
type unknownSourceError struct {
	name string
}

func (e *unknownSourceError) Error() string {
	return "unknown source: " + e.name
}
