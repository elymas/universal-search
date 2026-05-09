// Package streamsynth orchestrates streaming synthesis output over SSE.
//
// REQ-SYN4-001c: Un-cited sentences are never emitted (SYN-002 invariant preserved).
// REQ-SYN4-002:  Per-sentence SSE emission with citation payloads.
// REQ-SYN4-003:  Heartbeat keepalive via sse.RunHeartbeat.
// REQ-SYN4-004:  Client-disconnect context cancellation propagation.
//
// @MX:ANCHOR: [AUTO] Stream synthesis entry point; callers: handlers.SynthesisHandler, tests
// @MX:REASON: fan_in >= 3; all SSE-mode synthesis flows through StreamSynthesize
package streamsynth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/elymas/universal-search/internal/sse"
	"github.com/elymas/universal-search/internal/synthesis"
)

// sentenceRegex is the canonical sentence-boundary regex from SPEC-SYN-002
// REQ-SYN2-001. Matches punctuation followed by whitespace OR at end of string.
var sentenceRegex = regexp.MustCompile(`[.!?。！？]\s+|[.!?。！？]$`)

// markerRegex extracts [N] citation markers from sentence text.
var markerRegex = regexp.MustCompile(`\[(\d+)\]`)

// StreamRequest holds all parameters for a single streaming synthesis call.
type StreamRequest struct {
	RequestID   string
	SynthResult synthesis.Result
}

// StreamStats summarises the outcome of a StreamSynthesize call.
type StreamStats struct {
	SentencesEmitted int
	LatencyMs        float64
}

// CitationRef is the per-sentence citation payload shape (REQ-SYN4-002).
type CitationRef struct {
	Marker int    `json:"marker"`
	DocID  string `json:"doc_id"`
	URL    string `json:"url"`
	Title  string `json:"title"`
}

// SentencePayload is the JSON payload for an `event: sentence` SSE event.
type SentencePayload struct {
	RequestID     string        `json:"request_id"`
	SentenceIndex int           `json:"sentence_index"`
	Text          string        `json:"text"`
	Citations     []CitationRef `json:"citations"`
	SchemaVersion int           `json:"schema_version"`
}

// DonePayload is the JSON payload for an `event: done` SSE event.
type DonePayload struct {
	RequestID      string  `json:"request_id"`
	TotalSentences int     `json:"total_sentences"`
	LatencyMs      float64 `json:"latency_ms"`
	Model          string  `json:"model"`
	Provider       string  `json:"provider"`
	CostUSD        float64 `json:"cost_usd"`
	SchemaVersion  int     `json:"schema_version"`
}

// ErrorPayload is the JSON payload for an `event: error` SSE event.
type ErrorPayload struct {
	RequestID               string `json:"request_id"`
	ErrorCode               string `json:"error_code"`
	ErrorMessage            string `json:"error_message"`
	PartialSentencesEmitted int    `json:"partial_sentences_emitted"`
	SchemaVersion           int    `json:"schema_version"`
}

// flushWriter combines http.ResponseWriter and http.Flusher.
type flushWriter interface {
	http.ResponseWriter
}

// StreamSynthesize reads result from req.SynthResult, segments into sentences,
// validates citations, and emits one `event: sentence` per cited sentence.
// It emits `event: done` on success or `event: error` on failure.
//
// @MX:WARN: [AUTO] Goroutine coordination — main writer exits on ctx cancel or write error
// @MX:REASON: cancel on client disconnect; all goroutines must release within cancellation deadline
func StreamSynthesize(ctx context.Context, w flushWriter, req StreamRequest) (StreamStats, error) {
	start := time.Now()
	sw := sse.NewWriter(w)

	result := req.SynthResult
	sentences := segmentSentences(result.Text)

	citationMap := buildCitationMap(result.Citations)

	var emitted int
	for i, sentence := range sentences {
		// Check context before emitting each sentence.
		select {
		case <-ctx.Done():
			return StreamStats{SentencesEmitted: emitted}, ctx.Err()
		default:
		}

		// Resolve [N] markers; skip uncited sentences (REQ-SYN4-001c).
		refs := resolveCitations(sentence, citationMap)
		if len(refs) == 0 {
			continue
		}

		payload := SentencePayload{
			RequestID:     req.RequestID,
			SentenceIndex: i,
			Text:          sentence,
			Citations:     refs,
			SchemaVersion: 1,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return StreamStats{SentencesEmitted: emitted}, fmt.Errorf("marshal sentence: %w", err)
		}
		if err := sw.WriteEvent("sentence", data); err != nil {
			return StreamStats{SentencesEmitted: emitted}, err
		}
		if err := sw.Flush(); err != nil {
			return StreamStats{SentencesEmitted: emitted}, err
		}
		emitted++
	}

	// Emit done event.
	donePayload := DonePayload{
		RequestID:      req.RequestID,
		TotalSentences: emitted,
		LatencyMs:      float64(time.Since(start).Milliseconds()),
		Model:          result.Model,
		Provider:       result.Provider,
		CostUSD:        result.CostUSD,
		SchemaVersion:  1,
	}
	data, err := json.Marshal(donePayload)
	if err != nil {
		return StreamStats{SentencesEmitted: emitted}, fmt.Errorf("marshal done: %w", err)
	}
	if err := sw.WriteEvent("done", data); err != nil {
		return StreamStats{SentencesEmitted: emitted}, err
	}
	_ = sw.Flush()

	return StreamStats{SentencesEmitted: emitted, LatencyMs: float64(time.Since(start).Milliseconds())}, nil
}

// segmentSentences splits text into sentences using the SYN-002 canonical regex.
func segmentSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// Find all sentence boundary positions.
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
	// Capture any trailing text not terminated by punctuation.
	if prev < len(text) {
		trailing := strings.TrimSpace(text[prev:])
		if trailing != "" {
			sentences = append(sentences, trailing)
		}
	}
	return sentences
}

// buildCitationMap indexes citations by marker number for O(1) lookup.
func buildCitationMap(citations []synthesis.Citation) map[int]synthesis.Citation {
	m := make(map[int]synthesis.Citation, len(citations))
	for _, c := range citations {
		m[c.Marker] = c
	}
	return m
}

// resolveCitations extracts [N] markers from sentence and returns CitationRef
// slices for each resolved marker. Returns nil if no markers are found or all
// markers are unresolved (caller should skip the sentence — REQ-SYN4-001c).
func resolveCitations(sentence string, citationMap map[int]synthesis.Citation) []CitationRef {
	matches := markerRegex.FindAllStringSubmatch(sentence, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[int]bool)
	var refs []CitationRef
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err != nil || seen[n] {
			continue
		}
		seen[n] = true
		if c, ok := citationMap[n]; ok {
			refs = append(refs, CitationRef{
				Marker: c.Marker,
				DocID:  c.DocID,
				URL:    c.URL,
				Title:  c.Title,
			})
		}
	}
	return refs
}
