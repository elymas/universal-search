// Package streamsynth provides long-form SSE streaming for deep reports.
//
// SPEC-DEEP-001 REQ-DEEP1-005: section-aware SSE emission.
// SPEC-DEEP-001 REQ-DEEP1-006: buffered JSON fallback (handler responsibility).
//
// Event sequence per section:
//
//	section_start -> sentence xN -> section_done
//
// Terminal event: done with totals.
// Inherits SYN-004 heartbeat + disconnect via sse.Writer.
//
// @MX:ANCHOR: [AUTO] Long-form SSE stream entry; callers: handler deep endpoint, tests
// @MX:REASON: fan_in >= 3; all long-form SSE flows through StreamLongFormReport
package streamsynth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elymas/universal-search/internal/sse"
)

// SectionStartPayload is the JSON payload for an `event: section_start` SSE event.
type SectionStartPayload struct {
	RequestID     string `json:"request_id"`
	SectionIndex  int    `json:"section_index"`
	Heading       string `json:"heading"`
	Level         int    `json:"level"`
	SchemaVersion int    `json:"schema_version"`
}

// LongFormSentencePayload is the JSON payload for an `event: sentence` SSE event
// in the long-form streaming context. Extends the single-paragraph SentencePayload
// with section_index.
type LongFormSentencePayload struct {
	RequestID     string        `json:"request_id"`
	SectionIndex  int           `json:"section_index"`
	SentenceIndex int           `json:"sentence_index"`
	Text          string        `json:"text"`
	Citations     []CitationRef `json:"citations"`
	SchemaVersion int           `json:"schema_version"`
}

// SectionDonePayload is the JSON payload for an `event: section_done` SSE event.
type SectionDonePayload struct {
	RequestID        string `json:"request_id"`
	SectionIndex     int    `json:"section_index"`
	SentencesEmitted int    `json:"sentences_emitted"`
	SchemaVersion    int    `json:"schema_version"`
}

// LongFormDonePayload is the JSON payload for the terminal `event: done` SSE event
// in the long-form streaming context. Extends the single-paragraph DonePayload
// with total_sections.
type LongFormDonePayload struct {
	RequestID      string  `json:"request_id"`
	TotalSections  int     `json:"total_sections"`
	TotalSentences int     `json:"total_sentences"`
	LatencyMs      float64 `json:"latency_ms"`
	Model          string  `json:"model"`
	Provider       string  `json:"provider"`
	CostUSD        float64 `json:"cost_usd"`
	SchemaVersion  int     `json:"schema_version"`
}

// StreamLongFormReport streams section-aware SSE events from any LongFormSource.
// Per section: section_start -> sentence xN -> section_done.
// Terminal: done with totals.
//
// Sentences whose citation markers do not resolve to source citations are skipped,
// preserving the SYN-001c citation invariant.
//
// The caller is responsible for:
//   - Setting SSE response headers via w.SetHeaders()
//   - Starting heartbeat goroutine via sse.RunHeartbeat()
//   - Providing a context derived from the HTTP request for disconnect detection
//
// @MX:WARN: [AUTO] Context check per section — must respect cancellation
// @MX:REASON: client disconnect must stop emission; ctx checked at section and sentence boundaries
func StreamLongFormReport(ctx context.Context, w *sse.Writer, requestID string, src LongFormSource) (StreamStats, error) {
	start := time.Now()

	// measureLatency returns elapsed time in milliseconds with nanosecond precision.
	measureLatency := func() float64 {
		return float64(time.Since(start).Nanoseconds()) / 1e6
	}

	sections := src.SourceSections()
	citations := src.SourceCitations()
	meta := src.SourceMetadata()

	citationMap := buildSourceCitationMap(citations)

	var totalSentences int

	for _, section := range sections {
		// Check context before each section.
		select {
		case <-ctx.Done():
			return StreamStats{SentencesEmitted: totalSentences}, ctx.Err()
		default:
		}

		// Emit section_start.
		startPayload := SectionStartPayload{
			RequestID:     requestID,
			SectionIndex:  section.SectionIndex,
			Heading:       section.Heading,
			Level:         section.Level,
			SchemaVersion: 1,
		}
		if err := writeAndFlush(w, "section_start", startPayload); err != nil {
			return StreamStats{SentencesEmitted: totalSentences}, err
		}

		// Emit sentences within the section.
		var sentencesEmitted int
		for _, sentence := range section.Sentences {
			select {
			case <-ctx.Done():
				return StreamStats{SentencesEmitted: totalSentences}, ctx.Err()
			default:
			}

			refs := resolveSourceCitations(sentence.Markers, citationMap)
			if len(refs) == 0 {
				continue
			}

			sentencePayload := LongFormSentencePayload{
				RequestID:     requestID,
				SectionIndex:  section.SectionIndex,
				SentenceIndex: sentence.SentenceIndex,
				Text:          sentence.Text,
				Citations:     refs,
				SchemaVersion: 1,
			}
			if err := writeAndFlush(w, "sentence", sentencePayload); err != nil {
				return StreamStats{SentencesEmitted: totalSentences}, err
			}
			sentencesEmitted++
			totalSentences++
		}

		// Emit section_done.
		donePayload := SectionDonePayload{
			RequestID:        requestID,
			SectionIndex:     section.SectionIndex,
			SentencesEmitted: sentencesEmitted,
			SchemaVersion:    1,
		}
		if err := writeAndFlush(w, "section_done", donePayload); err != nil {
			return StreamStats{SentencesEmitted: totalSentences}, err
		}
	}

	// Emit terminal done event.
	terminalPayload := LongFormDonePayload{
		RequestID:      requestID,
		TotalSections:  len(sections),
		TotalSentences: totalSentences,
		LatencyMs:      measureLatency(),
		Model:          meta.Model,
		Provider:       meta.Provider,
		CostUSD:        meta.CostUSD,
		SchemaVersion:  1,
	}
	if err := writeAndFlush(w, "done", terminalPayload); err != nil {
		return StreamStats{SentencesEmitted: totalSentences}, err
	}

	return StreamStats{
		SentencesEmitted: totalSentences,
		LatencyMs:        measureLatency(),
	}, nil
}

// writeAndFlush marshals payload to JSON, writes it as an SSE event, and flushes.
func writeAndFlush(w *sse.Writer, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", eventType, err)
	}
	if err := w.WriteEvent(eventType, data); err != nil {
		return err
	}
	return w.Flush()
}

// buildSourceCitationMap indexes SourceCitation by marker number for O(1) lookup.
func buildSourceCitationMap(citations []SourceCitation) map[int]SourceCitation {
	m := make(map[int]SourceCitation, len(citations))
	for _, c := range citations {
		m[c.Marker] = c
	}
	return m
}

// resolveSourceCitations maps marker numbers to CitationRef slices.
func resolveSourceCitations(markers []int, citationMap map[int]SourceCitation) []CitationRef {
	if len(markers) == 0 {
		return nil
	}
	var refs []CitationRef
	for _, marker := range markers {
		if c, ok := citationMap[marker]; ok {
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
