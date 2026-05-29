// Package prompt provides heuristic LLM prompt-injection sanitization for
// SPEC-SEC-001 (REQ-SEC-015). It wraps untrusted indexed-document content in
// EVIDENCE blocks and detects known injection patterns before the content
// reaches the synthesis/citation-faithfulness flow (SPEC-SYN-002).
package prompt
