package deepagent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elymas/universal-search/internal/deepreport"
	"github.com/elymas/universal-search/internal/fanout"
	"github.com/elymas/universal-search/internal/llm"
	"github.com/elymas/universal-search/internal/streamsynth"
	"github.com/elymas/universal-search/pkg/types"
)

// FanoutFn is the function signature for fanout dispatch.
// Abstracted for testability — production callers pass a wrapper around fanout.Fanout.Dispatch.
type FanoutFn func(ctx context.Context, query string) (*fanout.Result, error)

// Researcher performs retrieval via fanout and extracts claims from documents.
// REQ-DEEP2-005: Calls fanoutFn EXACTLY once. No other retrieval mechanism.
func Researcher(ctx context.Context, cfg Config, llmClient llm.Client, req PipelineRequest, fanoutFn FanoutFn) (ResearcherOutput, error) {
	// Step 1: Call fanout exactly once.
	result, err := fanoutFn(ctx, req.Query)
	if err != nil {
		return ResearcherOutput{}, fmt.Errorf("researcher: fanout dispatch: %w", err)
	}

	// Step 2: Check for empty corpus (REQ-DEEP2-012).
	if len(result.Docs) == 0 {
		return ResearcherOutput{IsEmpty: true}, nil
	}

	// Step 3: Convert NormalizedDoc to NormalizedDocPayload for LLM context.
	payloads := make([]deepreport.NormalizedDocPayload, len(result.Docs))
	for i, doc := range result.Docs {
		payloads[i] = normalizedDocToPayload(doc)
	}

	// Step 4: Call LLM to extract claims from documents.
	docsJSON, err := json.Marshal(payloads)
	if err != nil {
		return ResearcherOutput{}, fmt.Errorf("researcher: marshal docs: %w", err)
	}

	userMsg := fmt.Sprintf("Query: %s\n\nDocuments:\n%s", req.Query, string(docsJSON))
	llmResp, err := llmClient.Complete(ctx, llm.Request{
		Class:    "rag",
		System:   ResearcherSystemPrompt(),
		Override: cfg.ResearcherModel,
		Messages: []llm.Message{
			{Role: "user", Content: userMsg},
		},
	})
	if err != nil {
		return ResearcherOutput{}, fmt.Errorf("researcher: LLM call: %w", err)
	}

	// Step 5: Parse claims from LLM response.
	var claimResp struct {
		Claims []Claim `json:"claims"`
	}
	if err := json.Unmarshal([]byte(llmResp.Text), &claimResp); err != nil {
		// If JSON parsing fails, create a single claim from the raw text.
		claimResp.Claims = []Claim{
			{ID: "c1", Text: llmResp.Text, Sources: []string{}},
		}
	}

	// Copy payloads to ensure immutability (separate backing array from input docs).
	evidence := make([]deepreport.NormalizedDocPayload, len(payloads))
	copy(evidence, payloads)

	return ResearcherOutput{
		Claims:   claimResp.Claims,
		Evidence: evidence,
		IsEmpty:  false,
	}, nil
}

// normalizedDocToPayload converts a types.NormalizedDoc to a deepreport.NormalizedDocPayload.
func normalizedDocToPayload(doc types.NormalizedDoc) deepreport.NormalizedDocPayload {
	return deepreport.NormalizedDocPayload{
		ID:          doc.ID,
		SourceID:    doc.SourceID,
		URL:         doc.URL,
		Title:       doc.Title,
		Body:        doc.Body,
		Snippet:     doc.Snippet,
		PublishedAt: doc.PublishedAt.Format(time.RFC3339),
		RetrievedAt: doc.RetrievedAt.Format(time.RFC3339),
		Author:      doc.Author,
		Score:       doc.Score,
		Lang:        doc.Lang,
		Citations:   doc.Citations,
		Metadata:    doc.Metadata,
		Hash:        doc.Hash,
	}
}

// Reviewer critiques Researcher claims without any retrieval.
// REQ-DEEP2-002: Reviewer does NOT call fanout. Critique-only.
// Note: The function signature intentionally excludes FanoutFn to enforce
// at compile time that Reviewer cannot perform retrieval.
func Reviewer(ctx context.Context, cfg Config, llmClient llm.Client, research ResearcherOutput) (ReviewerCritique, error) {
	// Build LLM context from claims and evidence.
	claimsJSON, err := json.Marshal(research.Claims)
	if err != nil {
		return ReviewerCritique{}, fmt.Errorf("reviewer: marshal claims: %w", err)
	}
	evidenceJSON, err := json.Marshal(research.Evidence)
	if err != nil {
		return ReviewerCritique{}, fmt.Errorf("reviewer: marshal evidence: %w", err)
	}

	userMsg := fmt.Sprintf("Claims:\n%s\n\nEvidence:\n%s", string(claimsJSON), string(evidenceJSON))
	llmResp, err := llmClient.Complete(ctx, llm.Request{
		Class:    "rag",
		System:   ReviewerSystemPrompt(),
		Override: cfg.ReviewerModel,
		Messages: []llm.Message{
			{Role: "user", Content: userMsg},
		},
	})
	if err != nil {
		return ReviewerCritique{}, fmt.Errorf("reviewer: LLM call: %w", err)
	}

	// Parse critique notes from LLM response.
	var critiqueResp struct {
		Notes []CritiqueNote `json:"notes"`
	}
	if err := json.Unmarshal([]byte(llmResp.Text), &critiqueResp); err != nil {
		// If JSON parsing fails, return empty critique (allow pipeline to continue).
		return ReviewerCritique{}, nil
	}

	return ReviewerCritique{Notes: critiqueResp.Notes}, nil
}

// Writer produces the final draft with sentence-level citations from research and critique.
// REQ-DEEP2-002: Writer does NOT call fanout. Draft-only.
// Note: The function signature intentionally excludes FanoutFn to enforce
// at compile time that Writer cannot perform retrieval.
func Writer(ctx context.Context, cfg Config, llmClient llm.Client, research ResearcherOutput, critique ReviewerCritique, retryHint *VerifierFeedback) (WriterDraft, error) {
	// Build LLM context from claims, evidence, and critique.
	claimsJSON, err := json.Marshal(research.Claims)
	if err != nil {
		return WriterDraft{}, fmt.Errorf("writer: marshal claims: %w", err)
	}
	evidenceJSON, err := json.Marshal(research.Evidence)
	if err != nil {
		return WriterDraft{}, fmt.Errorf("writer: marshal evidence: %w", err)
	}
	critiqueJSON, err := json.Marshal(critique.Notes)
	if err != nil {
		return WriterDraft{}, fmt.Errorf("writer: marshal critique: %w", err)
	}

	userMsg := fmt.Sprintf("Claims:\n%s\n\nEvidence:\n%s\n\nCritique:\n%s", string(claimsJSON), string(evidenceJSON), string(critiqueJSON))

	// Append retry hint context if present (for Writer retry loop).
	if retryHint != nil {
		hintJSON, err := json.Marshal(retryHint)
		if err != nil {
			return WriterDraft{}, fmt.Errorf("writer: marshal retry hint: %w", err)
		}
		userMsg = fmt.Sprintf("%s\n\nRetry Feedback (uncited sentences to fix):\n%s", userMsg, string(hintJSON))
	}

	llmResp, err := llmClient.Complete(ctx, llm.Request{
		Class:    "rag",
		System:   WriterSystemPrompt(),
		Override: cfg.WriterModel,
		Messages: []llm.Message{
			{Role: "user", Content: userMsg},
		},
	})
	if err != nil {
		return WriterDraft{}, fmt.Errorf("writer: LLM call: %w", err)
	}

	// Parse draft sections and citations from LLM response.
	var draftResp struct {
		Sections  []DraftSection  `json:"sections"`
		Citations []DraftCitation `json:"citations"`
	}
	if err := json.Unmarshal([]byte(llmResp.Text), &draftResp); err != nil {
		// Return empty draft if JSON parsing fails.
		return WriterDraft{
			CostUSD:  llmResp.CostUSD,
			Model:    llmResp.Model,
			Provider: llmResp.Provider,
		}, nil
	}

	return WriterDraft{
		Sections:  draftResp.Sections,
		Citations: draftResp.Citations,
		CostUSD:   llmResp.CostUSD,
		Model:     llmResp.Model,
		Provider:  llmResp.Provider,
	}, nil
}

// Compile-time check: WriterDraft satisfies streamsynth.LongFormSource.
var _ streamsynth.LongFormSource = WriterDraft{}

// SourceSections implements streamsynth.LongFormSource.
func (d WriterDraft) SourceSections() []streamsynth.SourceSection {
	sections := make([]streamsynth.SourceSection, len(d.Sections))
	for i, s := range d.Sections {
		sections[i] = streamsynth.SourceSection{
			SectionIndex: s.SectionIndex,
			Heading:      s.Heading,
			Level:        1, // default level for deepagent drafts
			Text:         s.Text,
			Markers:      s.CitationMarkers,
		}
	}
	return sections
}

// SourceCitations implements streamsynth.LongFormSource.
func (d WriterDraft) SourceCitations() []streamsynth.SourceCitation {
	citations := make([]streamsynth.SourceCitation, len(d.Citations))
	for i, c := range d.Citations {
		citations[i] = streamsynth.SourceCitation{
			Marker: c.Marker,
			DocID:  c.DocID,
			URL:    c.URL,
			Title:  c.Title,
		}
	}
	return citations
}

// SourceMetadata implements streamsynth.LongFormSource.
func (d WriterDraft) SourceMetadata() streamsynth.SourceMetadata {
	return streamsynth.SourceMetadata{
		Model:    d.Model,
		Provider: d.Provider,
		CostUSD:  d.CostUSD,
	}
}

// CheckFaithfulnessFn is the function signature for faithfulness checking.
// Abstracted for testability — production callers pass a wrapper around synthesis.CheckFaithfulness.
type CheckFaithfulnessFn func(ctx context.Context, text string, citations []string, docs []string) (VerifierResult, error)

// VerifierWithChecker checks draft faithfulness using the provided checker function.
// REQ-DEEP2-006: Binary gate — PASS iff UncitedSentencesCount == 0.
// The checker function handles the actual HTTP call to the sidecar.
func VerifierWithChecker(ctx context.Context, cfg Config, draft WriterDraft, docs []deepreport.NormalizedDocPayload, checkFn CheckFaithfulnessFn) (VerifierResult, error) {
	// Serialize draft text from sections.
	var text string
	for _, sec := range draft.Sections {
		text += sec.Text + " "
	}

	// Serialize citations.
	var citations []string
	for _, c := range draft.Citations {
		citations = append(citations, fmt.Sprintf("[%d] %s %s", c.Marker, c.Title, c.URL))
	}

	// Serialize docs.
	var docTexts []string
	for _, d := range docs {
		docTexts = append(docTexts, d.Body)
	}

	return checkFn(ctx, text, citations, docTexts)
}

// DeepTreeMode determines the execution mode for a /deep request.
// It checks tree configuration and request parameters to decide whether
// to use tree-mode expansion or fall back to DEEP-002 single-shot.
//
// REQ-DEEP3-005: Tree mode requires cfg.Enabled AND valid breadth/depth.
// Fallback: breadth=0 or depth=0 → single-shot with X-Deep-Tree-Fallback header.
//
// @MX:NOTE: [AUTO] Tree-mode routing logic; agents.go is the HTTP handler boundary
type DeepTreeMode int

const (
	// DeepTreeModeNone indicates tree mode is not active (DEEP-002 single-shot).
	DeepTreeModeNone DeepTreeMode = iota
	// DeepTreeModeActive indicates tree-mode expansion should proceed.
	DeepTreeModeActive
	// DeepTreeModeFallbackBreadthZero indicates breadth=0 fallback to single-shot.
	DeepTreeModeFallbackBreadthZero
	// DeepTreeModeFallbackDepthZero indicates depth=0 fallback to single-shot.
	DeepTreeModeFallbackDepthZero
)

// DetermineTreeMode returns the execution mode based on tree config and request parameters.
// REQ-DEEP3-005: Returns fallback mode when breadth or depth is zero.
func DetermineTreeMode(treeCfg TreeConfigExtra, requestBreadth, requestDepth int) DeepTreeMode {
	if !treeCfg.Enabled {
		return DeepTreeModeNone
	}

	// Check fallback conditions (REQ-DEEP3-005).
	if requestBreadth == 0 {
		return DeepTreeModeFallbackBreadthZero
	}
	if requestDepth == 0 {
		return DeepTreeModeFallbackDepthZero
	}

	return DeepTreeModeActive
}

// FallbackHeaderValue returns the HTTP header value for the given tree mode.
// Returns empty string when no fallback applies.
func FallbackHeaderValue(mode DeepTreeMode) string {
	switch mode {
	case DeepTreeModeFallbackBreadthZero:
		return "breadth_zero"
	case DeepTreeModeFallbackDepthZero:
		return "depth_zero"
	default:
		return ""
	}
}
