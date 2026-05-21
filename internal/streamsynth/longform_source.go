package streamsynth

// LongFormSource is the interface that any report-like type must implement
// to be streamed via StreamLongFormReport or a similar SSE emitter.
// REQ-DEEP2-008: decouples deepagent.WriterDraft from deepreport.Report.
//
// @MX:ANCHOR: [AUTO] LongFormSource interface; implemented by deepreport.Report, deepagent.WriterDraft
// @MX:REASON: consumer-defined interface per Go convention; enables multi-source SSE streaming
type LongFormSource interface {
	// SourceSections returns the ordered list of sections to stream.
	SourceSections() []SourceSection

	// SourceCitations returns the flat list of citations referenced by sections.
	SourceCitations() []SourceCitation

	// SourceMetadata returns model, provider, and cost for the done event.
	SourceMetadata() SourceMetadata
}

// SourceSection represents a single section in a LongFormSource.
type SourceSection struct {
	SectionIndex int
	Heading      string
	Level        int
	Text         string
	Markers      []int            // citation markers referenced in this section
	Sentences    []SourceSentence // sentence-level data for streaming
}

// SourceSentence represents a single sentence within a section.
type SourceSentence struct {
	SentenceIndex int
	Text          string
	Markers       []int // citation markers referenced in this sentence
}

// SourceCitation represents a single citation in a LongFormSource.
type SourceCitation struct {
	Marker int
	DocID  string
	URL    string
	Title  string
}

// SourceMetadata carries model/provider/cost from the source.
type SourceMetadata struct {
	Model    string
	Provider string
	CostUSD  float64
}
