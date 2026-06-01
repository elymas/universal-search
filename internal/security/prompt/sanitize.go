package prompt

import (
	"context"
	"strings"

	"github.com/elymas/universal-search/internal/security/events"
)

// evidenceOpen / evidenceClose delimit untrusted indexed-document content so a
// downstream LLM treats it as quoted evidence, not instructions.
const (
	evidenceOpen  = "<EVIDENCE>"
	evidenceClose = "</EVIDENCE>"
)

// Result is the outcome of sanitizing one untrusted input.
type Result struct {
	// Sanitized is the input wrapped in an EVIDENCE block with injection
	// markers neutralized. Always safe to embed in a prompt.
	Sanitized string
	// Detected lists the heuristic injection pattern classes found in the raw
	// input (before neutralization). Empty means the input looked clean.
	Detected []PatternClass
}

// EventEmitter is the minimal slice of *events.Emitter Sanitize needs.
type EventEmitter interface {
	Emit(ctx context.Context, ev events.Event) error
}

// Sanitize wraps untrusted content in an <EVIDENCE>...</EVIDENCE> block and
// neutralizes injection markers so the content cannot escape the block or be
// interpreted as instructions. Detection is heuristic (see patterns.go); there
// is NO LLM classifier in V1.
//
// @MX:NOTE: [AUTO] SPEC-SYN-002 integration point — every indexed-document body
// passes through here before synthesis.CheckFaithfulness so untrusted corpus
// text is fenced as evidence, not instructions. Detection is heuristic, not an
// LLM classifier (post-V1).
// @MX:SPEC: SPEC-SEC-001 (REQ-SEC-015)
func Sanitize(input string) Result {
	detected := DetectPatterns(input)
	neutral := neutralize(input)
	return Result{
		Sanitized: evidenceOpen + "\n" + neutral + "\n" + evidenceClose,
		Detected:  detected,
	}
}

// SanitizeAndEmit runs Sanitize and, when any injection pattern is detected,
// emits a low-severity prompt.sanitized event through emitter. The emitter may
// be nil (no-op). The Result is returned regardless of emission.
func SanitizeAndEmit(ctx context.Context, emitter EventEmitter, input string) Result {
	res := Sanitize(input)
	if emitter != nil && len(res.Detected) > 0 {
		_ = emitter.Emit(ctx, events.Event{
			Type:     events.TypePromptSanitized,
			Severity: events.SeverityLow,
		})
	}
	return res
}

// neutralize defangs structural tokens that could break out of the EVIDENCE
// fence or forge a conversational turn. It does NOT delete content — it makes
// the markers inert so the evidence remains readable to a human reviewer.
func neutralize(s string) string {
	replacer := strings.NewReplacer(
		evidenceClose, "<​EVIDENCE>", // zero-width break inside the tag
		evidenceOpen, "<​EVIDENCE>",
		"<|", "<​|",
		"|>", "|​>",
	)
	return replacer.Replace(s)
}
