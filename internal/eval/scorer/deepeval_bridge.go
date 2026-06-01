// Package scorer is the Go→Python bridge to the DeepEval faithfulness judge.
//
// REQ-EVAL1-005: marshals a synthesis.Result into the judge's claim schema,
// POSTs to the Python /judge/faithfulness endpoint, and returns per-claim
// scores. Claim segmentation is locale-aware: English uses the live
// structural checker's ASCII rule, Korean uses EVAL-001's own CJK-aware
// segmentation because the structural checker provides none (HISTORY D3).
package scorer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/elymas/universal-search/internal/synthesis"
)

// markerRE matches inline [N] citation markers, mirroring the live structural
// checker's _MARKER_RE = \[(\d+)\] (faithfulness_endpoint.py).
var markerRE = regexp.MustCompile(`\[(\d+)\]`)

// errUnavailable wraps judge-availability failures (timeout, 5xx, conn refused)
// so the runner can distinguish "could not score" from "scored zero".
// REQ-EVAL1-006.
var errUnavailable = errors.New("scorer: judge unavailable")

// IsUnavailable reports whether err is a judge-availability error.
func IsUnavailable(err error) bool { return errors.Is(err, errUnavailable) }

// WrapUnavailable tags err as a judge-availability error so callers (and the
// runner) treat the query as unscoreable (null) rather than zero.
// REQ-EVAL1-006.
func WrapUnavailable(err error) error {
	return fmt.Errorf("%w: %v", errUnavailable, err)
}

// JudgeRequest is the POST body sent to /judge/faithfulness.
type JudgeRequest struct {
	QueryID string         `json:"query_id"`
	Claims  []ClaimInput   `json:"claims"`
	Corpus  map[string]any `json:"corpus"`
}

// ClaimInput is one segmented claim plus the doc IDs it cites.
type ClaimInput struct {
	Text        string   `json:"text"`
	CitedDocIDs []string `json:"cited_doc_ids"`
}

// JudgeResponse is the parsed response from /judge/faithfulness.
type JudgeResponse struct {
	QueryID           string       `json:"query_id"`
	ClaimScores       []ClaimScore `json:"claim_scores"`
	FaithfulnessScore float64      `json:"faithfulness_score"`
	TotalClaims       int          `json:"total_claims"`
	SupportedClaims   int          `json:"supported_claims"`
}

// ClaimScore is the judge's verdict for a single claim.
type ClaimScore struct {
	Text           string `json:"text"`
	Supported      bool   `json:"supported"`
	JudgeRationale string `json:"judge_rationale"`
}

// Result is the bridge's per-query output handed back to the runner.
type Result struct {
	Score    float64
	PerClaim []ClaimScore
}

// Bridge is an HTTP client for the Python judge endpoint.
type Bridge struct {
	endpoint string
	client   *http.Client
	timeout  time.Duration
}

// NewBridge constructs a Bridge for the given judge base URL with a per-query
// timeout (NFR-EVAL1-002: 30s).
func NewBridge(baseURL string, timeout time.Duration) *Bridge {
	return &Bridge{
		endpoint: strings.TrimRight(baseURL, "/") + "/judge/faithfulness",
		client:   &http.Client{Timeout: timeout},
		timeout:  timeout,
	}
}

// @MX:NOTE: [AUTO] Locale-aware claim segmentation. EN mirrors the live
// structural checker's (?<=[.!?])\s+; KO is EVAL-001's own concern because
// faithfulness_endpoint.py has no CJK segmentation (HISTORY D3).
// @MX:SPEC: SPEC-EVAL-001 REQ-EVAL1-005

// SegmentClaims splits synthesized text into claims using locale-aware rules.
//   - locale "en": split after . ! ? followed by whitespace.
//   - locale "ko": split after . ! ? OR CJK punctuation 。！？, with or without
//     trailing whitespace (Korean text frequently has no space after a period).
func SegmentClaims(text, locale string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if locale == "ko" {
		return segmentKorean(text)
	}
	return segmentASCII(text)
}

// segmentASCII splits on sentence-final punctuation followed by whitespace,
// equivalent to Python's (?<=[.!?])\s+.
func segmentASCII(text string) []string {
	var claims []string
	var b strings.Builder
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		b.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			// Lookahead: whitespace or end-of-string ends the sentence.
			if i+1 >= len(runes) || isSpace(runes[i+1]) {
				flush(&claims, &b)
				// Skip following whitespace.
				for i+1 < len(runes) && isSpace(runes[i+1]) {
					i++
				}
			}
		}
	}
	flush(&claims, &b)
	return claims
}

// segmentKorean splits on ASCII or CJK sentence-final punctuation, NOT
// requiring trailing whitespace (Korean endings like 다. often have none).
func segmentKorean(text string) []string {
	var claims []string
	var b strings.Builder
	for _, r := range text {
		b.WriteRune(r)
		switch r {
		case '.', '!', '?', '。', '！', '？':
			flush(&claims, &b)
		}
	}
	flush(&claims, &b)
	return claims
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

func flush(claims *[]string, b *strings.Builder) {
	s := strings.TrimSpace(b.String())
	if s != "" {
		*claims = append(*claims, s)
	}
	b.Reset()
}

// CitedDocIDs extracts the doc IDs cited by a claim by resolving its trailing
// [N] markers against the synthesis citation table.
// REQ-EVAL1-005(b).
func CitedDocIDs(claim string, citations []synthesis.Citation) []string {
	byMarker := make(map[int]string, len(citations))
	for _, c := range citations {
		byMarker[c.Marker] = c.DocID
	}
	seen := make(map[string]bool)
	var out []string
	for _, m := range markerRE.FindAllStringSubmatch(claim, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if docID, ok := byMarker[n]; ok && !seen[docID] {
			seen[docID] = true
			out = append(out, docID)
		}
	}
	return out
}

// Score segments the synthesis output, marshals the claims into the judge
// schema, POSTs to the judge endpoint, and returns the per-query result.
// REQ-EVAL1-005.
func (b *Bridge) Score(ctx context.Context, queryID, locale string, result synthesis.Result, corpus map[string]string) (Result, error) {
	claims := SegmentClaims(result.Text, locale)
	inputs := make([]ClaimInput, 0, len(claims))
	for _, c := range claims {
		inputs = append(inputs, ClaimInput{Text: c, CitedDocIDs: CitedDocIDs(c, result.Citations)})
	}

	// REQ-EVAL1-005(c): pass only the docs the synthesis actually cited.
	corpusBody := make(map[string]any)
	for _, in := range inputs {
		for _, docID := range in.CitedDocIDs {
			if body, ok := corpus[docID]; ok {
				corpusBody[docID] = body
			}
		}
	}

	payload := JudgeRequest{QueryID: queryID, Claims: inputs, Corpus: corpusBody}
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, fmt.Errorf("scorer: marshal request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, b.endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("scorer: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		// Connection refused, timeout, context deadline → unavailable.
		return Result{}, fmt.Errorf("%w: %v", errUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 500 {
		respBody, _ := io.ReadAll(resp.Body)
		return Result{}, fmt.Errorf("%w: server error %d: %s", errUnavailable, resp.StatusCode, string(respBody))
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return Result{}, fmt.Errorf("scorer: client error %d: %s", resp.StatusCode, string(respBody))
	}

	var jr JudgeResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return Result{}, fmt.Errorf("scorer: decode response: %w", err)
	}

	return Result{Score: jr.FaithfulnessScore, PerClaim: jr.ClaimScores}, nil
}
