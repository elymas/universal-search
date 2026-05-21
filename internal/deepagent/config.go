// Package deepagent provides the multi-agent /deep pipeline
// (Researcher/Reviewer/Writer/Verifier) for SPEC-DEEP-002.
//
// REQ-DEEP2-004: Each agent uses env-var resolved model alias.
// All DEEP_AGENT_* env-vars are loaded here; individual agents
// must not call os.Getenv directly.
package deepagent

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the deep agent pipeline.
// @MX:NOTE: [AUTO] All DEEP_AGENT_* env-vars loaded here; agents must not call os.Getenv (REQ-DEEP2-004)
type Config struct {
	// ResearcherModel is the LiteLLM model alias for the Researcher agent.
	ResearcherModel string
	// ReviewerModel is the LiteLLM model alias for the Reviewer agent.
	ReviewerModel string
	// WriterModel is the LiteLLM model alias for the Writer agent.
	WriterModel string
	// VerifierModel is the LiteLLM model alias for the Verifier agent.
	VerifierModel string
	// MaxRetries is the maximum number of Writer retries on Verifier rejection.
	// Total attempts = MaxRetries + 1 (default: 2 retries = 3 total attempts).
	MaxRetries int
	// WriterRetryDelayMs is the backoff delay between Writer retries in milliseconds.
	WriterRetryDelayMs int
	// VerifierTimeoutMs is the timeout for the faithfulness check call in milliseconds.
	VerifierTimeoutMs int
	// FaithfulnessURL is the URL for the researcher sidecar faithfulness endpoint.
	FaithfulnessURL string
}

// DefaultConfig returns a Config with production-safe defaults
// per SPEC-DEEP-002 research.md Section 4.
func DefaultConfig() Config {
	return Config{
		ResearcherModel:    "claude-3-5-haiku-20241022",
		ReviewerModel:      "claude-3-5-haiku-20241022",
		WriterModel:        "claude-3-5-sonnet-20241022",
		VerifierModel:      "claude-3-5-sonnet-20241022",
		MaxRetries:         2,
		WriterRetryDelayMs: 500,
		VerifierTimeoutMs:  30000,
		FaithfulnessURL:    "http://localhost:8001/faithfulness_check",
	}
}

// NewConfigFromEnv reads all DEEP_AGENT_* environment variables.
// Unset values fall back to DefaultConfig values.
func NewConfigFromEnv() (Config, error) {
	cfg := DefaultConfig()

	if v := os.Getenv("DEEP_AGENT_RESEARCHER_MODEL"); v != "" {
		cfg.ResearcherModel = v
	}
	if v := os.Getenv("DEEP_AGENT_REVIEWER_MODEL"); v != "" {
		cfg.ReviewerModel = v
	}
	if v := os.Getenv("DEEP_AGENT_WRITER_MODEL"); v != "" {
		cfg.WriterModel = v
	}
	if v := os.Getenv("DEEP_AGENT_VERIFIER_MODEL"); v != "" {
		cfg.VerifierModel = v
	}
	if v := os.Getenv("DEEP_AGENT_MAX_RETRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("deepagent config: invalid DEEP_AGENT_MAX_RETRIES %q: %w", v, err)
		}
		cfg.MaxRetries = n
	}
	if v := os.Getenv("DEEP_AGENT_WRITER_RETRY_DELAY_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("deepagent config: invalid DEEP_AGENT_WRITER_RETRY_DELAY_MS %q: %w", v, err)
		}
		cfg.WriterRetryDelayMs = n
	}
	if v := os.Getenv("DEEP_AGENT_VERIFIER_TIMEOUT_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("deepagent config: invalid DEEP_AGENT_VERIFIER_TIMEOUT_MS %q: %w", v, err)
		}
		cfg.VerifierTimeoutMs = n
	}
	if v := os.Getenv("DEEP_AGENT_FAITHFULNESS_URL"); v != "" {
		cfg.FaithfulnessURL = v
	}

	return cfg, nil
}

// TreeConfigExtra holds tree-mode configuration for SPEC-DEEP-003.
// REQ-DEEP3-005: Tree mode is disabled by default; opt-in via DEEP_TREE_ENABLED.
// @MX:NOTE: [AUTO] All DEEP_TREE_* env-vars loaded here; no direct os.Getenv elsewhere
type TreeConfigExtra struct {
	// Enabled controls whether tree-mode expansion is active.
	Enabled bool
	// DefaultBreadth is the default number of child nodes per expansion.
	DefaultBreadth int
	// DefaultDepth is the maximum tree depth.
	DefaultDepth int
	// TokenBudget is the maximum total tokens for the entire tree.
	TokenBudget int64
	// RootTokenEstimate is the seed token estimate for the root node.
	RootTokenEstimate int64
	// NodeTimeoutMs is the per-node timeout in milliseconds.
	NodeTimeoutMs int
	// DecomposeModel is the LiteLLM model alias for the decompose call.
	DecomposeModel string
	// PersistenceDir is the directory for tree.json storage.
	PersistenceDir string
	// DecomposeModelPricePerToken is the per-token price for the decompose model.
	DecomposeModelPricePerToken float64
}

// DefaultTreeConfig returns a TreeConfigExtra with production-safe defaults.
func DefaultTreeConfig() TreeConfigExtra {
	return TreeConfigExtra{
		Enabled:                     false,
		DefaultBreadth:              4,
		DefaultDepth:                3,
		TokenBudget:                 60000,
		RootTokenEstimate:           5000,
		NodeTimeoutMs:               30000,
		DecomposeModel:              "claude-3-5-haiku-20241022",
		PersistenceDir:              ".moai/runs",
		DecomposeModelPricePerToken: 0.0000008,
	}
}

// NewTreeConfigFromEnv reads all DEEP_TREE_* environment variables.
// Unset values fall back to DefaultTreeConfig values.
func NewTreeConfigFromEnv() (TreeConfigExtra, error) {
	cfg := DefaultTreeConfig()

	if v := os.Getenv("DEEP_TREE_ENABLED"); v == "true" || v == "1" {
		cfg.Enabled = true
	}
	if v := os.Getenv("DEEP_TREE_DEFAULT_BREADTH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return TreeConfigExtra{}, fmt.Errorf("deepagent tree config: invalid DEEP_TREE_DEFAULT_BREADTH %q: %w", v, err)
		}
		cfg.DefaultBreadth = n
	}
	if v := os.Getenv("DEEP_TREE_DEFAULT_DEPTH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return TreeConfigExtra{}, fmt.Errorf("deepagent tree config: invalid DEEP_TREE_DEFAULT_DEPTH %q: %w", v, err)
		}
		cfg.DefaultDepth = n
	}
	if v := os.Getenv("DEEP_TREE_TOKEN_BUDGET"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return TreeConfigExtra{}, fmt.Errorf("deepagent tree config: invalid DEEP_TREE_TOKEN_BUDGET %q: %w", v, err)
		}
		cfg.TokenBudget = n
	}
	if v := os.Getenv("DEEP_TREE_ROOT_TOKEN_ESTIMATE"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return TreeConfigExtra{}, fmt.Errorf("deepagent tree config: invalid DEEP_TREE_ROOT_TOKEN_ESTIMATE %q: %w", v, err)
		}
		cfg.RootTokenEstimate = n
	}
	if v := os.Getenv("DEEP_TREE_NODE_TIMEOUT_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return TreeConfigExtra{}, fmt.Errorf("deepagent tree config: invalid DEEP_TREE_NODE_TIMEOUT_MS %q: %w", v, err)
		}
		cfg.NodeTimeoutMs = n
	}
	if v := os.Getenv("DEEP_TREE_DECOMPOSE_MODEL"); v != "" {
		cfg.DecomposeModel = v
	}
	if v := os.Getenv("DEEP_TREE_PERSISTENCE_DIR"); v != "" {
		cfg.PersistenceDir = v
	}

	return cfg, nil
}

// FallbackHeader returns the X-Deep-Tree-Fallback header value for a given
// breadth and depth configuration. Returns empty string when no fallback applies.
//
// REQ-DEEP3-005: breadth=0 → "breadth_zero", depth=0 → "depth_zero".
// Priority: breadth_zero takes precedence when both are zero.
func FallbackHeader(breadth, depth int) string {
	if breadth == 0 {
		return "breadth_zero"
	}
	if depth == 0 {
		return "depth_zero"
	}
	return ""
}
