package deepagent

// Prompt templates for each agent role.
// @MX:NOTE: [AUTO] Prompt templates — semantic intent per role

// ResearcherSystemPrompt returns the system prompt for the Researcher agent.
// @MX:NOTE: [AUTO] Researcher's task is to extract key claims from documents
func ResearcherSystemPrompt() string {
	return `You are a research analyst. Given a set of documents, extract the key claims and evidence. For each claim, identify the supporting source documents. Focus on factual claims that can be verified against the provided evidence.`
}

// ReviewerSystemPrompt returns the system prompt for the Reviewer agent.
// @MX:NOTE: [AUTO] Reviewer's task is to critique claims for quality and support
func ReviewerSystemPrompt() string {
	return `You are a critical reviewer. Given a set of claims extracted from documents, critique each claim for factual accuracy, completeness, and strength of supporting evidence. Identify unsupported assertions, logical gaps, and areas needing clarification.`
}

// WriterSystemPrompt returns the system prompt for the Writer agent.
// @MX:NOTE: [AUTO] Writer's task is to synthesize claims and critique into a coherent draft with citations
func WriterSystemPrompt() string {
	return `You are a technical writer. Given claims, evidence, and reviewer critique, produce a well-structured draft report. Every factual statement must include a citation marker [N] referencing the source document. Citation markers are 1-indexed. Do not include uncited factual claims.`
}

// VerifierSystemPrompt returns the system prompt for the Verifier agent.
// @MX:NOTE: [AUTO] Verifier does not use LLM — calls CheckFaithfulness directly
func VerifierSystemPrompt() string {
	// Verifier uses SYN-002 binary gate, not an LLM prompt.
	// Kept for symmetry with other agents but not used in LLM calls.
	return ""
}
