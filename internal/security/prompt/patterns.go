package prompt

import "regexp"

// PatternClass enumerates the heuristic prompt-injection categories detected by
// Sanitize. Detection is heuristic regex/string matching only — there is NO LLM
// classifier in V1 (reserved for post-V1).
type PatternClass string

const (
	// PatternOverrideAttempt: "ignore previous instructions" style overrides.
	PatternOverrideAttempt PatternClass = "override_attempt"
	// PatternRoleInjection: attempts to assign the model a new system role.
	PatternRoleInjection PatternClass = "role_injection"
	// PatternTagBreak: attempts to close/forge structural tags (e.g. </EVIDENCE>).
	PatternTagBreak PatternClass = "tag_break"
	// PatternPersonaSwap: "you are now ..." persona-swap attempts.
	PatternPersonaSwap PatternClass = "persona_swap"
	// PatternFormatBreak: fake system/assistant turn markers or fences.
	PatternFormatBreak PatternClass = "format_break"
)

// detector pairs a PatternClass with its compiled heuristic regex.
type detector struct {
	class PatternClass
	re    *regexp.Regexp
}

// detectors is the ordered heuristic detector set. Patterns are
// case-insensitive. Order is stable so multi-match results are deterministic.
var detectors = []detector{
	{PatternOverrideAttempt, regexp.MustCompile(`(?i)\b(ignore|disregard|forget|override)\b[^.\n]{0,40}\b(previous|prior|above|earlier|all)\b[^.\n]{0,20}\b(instruction|prompt|rule|context|message)s?\b`)},
	{PatternRoleInjection, regexp.MustCompile(`(?i)\b(system|developer)\s*(prompt|message|role)\s*[:=]`)},
	{PatternTagBreak, regexp.MustCompile(`(?i)</?\s*(EVIDENCE|system|assistant|user|instruction)\s*>`)},
	{PatternPersonaSwap, regexp.MustCompile(`(?i)\byou\s+are\s+(now|no\s+longer)\b|\bact\s+as\s+(a|an|the)\b|\bpretend\s+to\s+be\b`)},
	{PatternFormatBreak, regexp.MustCompile(`(?i)^\s*(###\s*)?(system|assistant|user|human)\s*:|<\|.*?\|>`)},
}

// DetectPatterns returns the set of PatternClass values matched in input, in
// detector declaration order (deterministic). An empty slice means no match.
func DetectPatterns(input string) []PatternClass {
	var found []PatternClass
	for _, d := range detectors {
		if d.re.MatchString(input) {
			found = append(found, d.class)
		}
	}
	return found
}
