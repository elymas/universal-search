package deepagent

import "github.com/elymas/universal-search/internal/deepreport"

// Agent is an enum-like type representing the four pipeline agents.
// @MX:NOTE: [AUTO] Enum-like type; bounded label values for Prometheus cardinality safety (NFR-DEEP2-002)
type Agent string

const (
	// AgentResearcher retrieves documents via fanout and extracts claims.
	AgentResearcher Agent = "researcher"
	// AgentReviewer critiques Researcher claims without retrieval.
	AgentReviewer Agent = "reviewer"
	// AgentWriter produces the final draft with sentence-level citations.
	AgentWriter Agent = "writer"
	// AgentVerifier checks draft faithfulness via SYN-002 binary gate.
	AgentVerifier Agent = "verifier"
)

// AllAgents is the bounded set of all pipeline agents.
// Used for pre-declaration and iteration.
var AllAgents = []Agent{AgentResearcher, AgentReviewer, AgentWriter, AgentVerifier}

// PipelineRequest is the input to the multi-agent pipeline.
type PipelineRequest struct {
	RequestID string
	Query     string
	Lang      string
}

// PipelineResult is the output of the multi-agent pipeline.
type PipelineResult struct {
	RequestID string
	AgentLog  []AgentLogEntry
	Draft     *WriterDraft
	IsEmpty   bool
}

// AgentLogEntry records a single agent's execution in the pipeline.
type AgentLogEntry struct {
	Agent      Agent
	Outcome    string // "success", "error", "empty_corpus"
	DurationMs int64
	CostUSD    float64
	Model      string
	Error      string
}

// AgentOutcome records per-agent execution metadata.
type AgentOutcome struct {
	Agent      Agent
	Model      string
	Provider   string
	CostUSD    float64
	DurationMs int64
}

// ResearcherOutput is the output of the Researcher agent.
type ResearcherOutput struct {
	Claims   []Claim
	Evidence []deepreport.NormalizedDocPayload
	IsEmpty  bool
}

// Claim represents a key claim extracted by the Researcher from documents.
type Claim struct {
	ID      string
	Text    string
	Sources []string // doc IDs supporting this claim
}

// CritiqueNote is a single critique from the Reviewer.
type CritiqueNote struct {
	ClaimID     string `json:"claim_id"`
	ConcernType string `json:"concern_type"`
	Severity    string `json:"severity"`
}

// ReviewerCritique is the output of the Reviewer agent.
type ReviewerCritique struct {
	Notes []CritiqueNote
}

// WriterDraft is the output of the Writer agent.
type WriterDraft struct {
	Sections  []DraftSection
	Citations []DraftCitation
	CostUSD   float64
	Model     string
	Provider  string
}

// DraftSection is a single section in the WriterDraft.
type DraftSection struct {
	SectionIndex    int
	Heading         string
	Text            string
	CitationMarkers []int // 1-indexed citation markers in this section
}

// DraftCitation maps a numeric marker to a source document.
type DraftCitation struct {
	Marker int
	DocID  string
	URL    string
	Title  string
}

// VerifierResult is the output of the Verifier agent.
type VerifierResult struct {
	Pass     bool
	Feedback *VerifierFeedback
}

// VerifierFeedback contains details when Verifier rejects a draft.
type VerifierFeedback struct {
	UncitedCount     int
	UncitedSentences []string
}
