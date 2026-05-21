package deepreport

import "github.com/elymas/universal-search/internal/streamsynth"

// Compile-time check: Report satisfies streamsynth.LongFormSource.
var _ streamsynth.LongFormSource = Report{}

// SourceSections implements streamsynth.LongFormSource.
func (r Report) SourceSections() []streamsynth.SourceSection {
	if len(r.Sections) == 0 {
		return nil
	}
	sections := make([]streamsynth.SourceSection, len(r.Sections))
	for i, s := range r.Sections {
		sentences := make([]streamsynth.SourceSentence, len(s.Sentences))
		for j, sent := range s.Sentences {
			sentences[j] = streamsynth.SourceSentence{
				SentenceIndex: sent.SentenceIndex,
				Text:          sent.Text,
				Markers:       sent.Markers,
			}
		}
		var markers []int
		for _, sent := range s.Sentences {
			markers = append(markers, sent.Markers...)
		}
		sections[i] = streamsynth.SourceSection{
			SectionIndex: s.SectionIndex,
			Heading:      s.Heading,
			Level:        s.Level,
			Text:         s.Text,
			Markers:      markers,
			Sentences:    sentences,
		}
	}
	return sections
}

// SourceCitations implements streamsynth.LongFormSource.
func (r Report) SourceCitations() []streamsynth.SourceCitation {
	if len(r.Citations) == 0 {
		return nil
	}
	citations := make([]streamsynth.SourceCitation, len(r.Citations))
	for i, c := range r.Citations {
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
func (r Report) SourceMetadata() streamsynth.SourceMetadata {
	return streamsynth.SourceMetadata{
		Model:    r.Model,
		Provider: r.Provider,
		CostUSD:  r.CostUSD,
	}
}
