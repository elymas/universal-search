package deepagent

import (
	"context"
	"fmt"
	"time"

	"github.com/elymas/universal-search/internal/llm"
)

// RunPipeline executes the multi-agent pipeline: Researcher -> Reviewer -> Writer -> Verifier (with retry loop).
// REQ-DEEP2-002: Agents run strictly sequential. Orchestrator checks ctx.Err() before each agent.
// REQ-DEEP2-003: Only Verifier rejection triggers Writer retry. MaxRetries+1 bound.
//
// @MX:ANCHOR: [AUTO] Single entry for 4-agent pipeline orchestration
// @MX:REASON: All /deep?mode=agents requests funnel here; agent ordering invariant (Researcher -> Reviewer -> Writer -> Verifier) enforced; retry loop bounded
// @MX:SPEC: SPEC-DEEP-002 REQ-DEEP2-002, REQ-DEEP2-003
func RunPipeline(ctx context.Context, cfg Config, llmClient llm.Client, req PipelineRequest, fanoutFn FanoutFn) (PipelineResult, error) {
	// Default: no explicit verifier checker (backward compat with M2/M3 tests).
	return RunPipelineWithVerifier(ctx, cfg, llmClient, req, fanoutFn, nil)
}

// RunPipelineWithVerifier executes the full 4-agent pipeline with an explicit faithfulness checker.
// REQ-DEEP2-003: Retry loop bounded by cfg.MaxRetries + 1 total Writer attempts.
func RunPipelineWithVerifier(ctx context.Context, cfg Config, llmClient llm.Client, req PipelineRequest, fanoutFn FanoutFn, checkFn CheckFaithfulnessFn) (PipelineResult, error) {
	result := PipelineResult{
		RequestID: req.RequestID,
	}

	// --- Phase 1: Researcher ---
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("pipeline: context cancelled before researcher: %w", err)
	}

	researchStart := time.Now()
	research, err := Researcher(ctx, cfg, llmClient, req, fanoutFn)
	researchDuration := time.Since(researchStart).Milliseconds()

	if err != nil {
		result.AgentLog = append(result.AgentLog, AgentLogEntry{
			Agent:      AgentResearcher,
			Outcome:    "error",
			DurationMs: researchDuration,
			Error:      err.Error(),
		})
		return result, fmt.Errorf("pipeline: researcher failed: %w", err)
	}

	// Handle empty corpus (REQ-DEEP2-012).
	if research.IsEmpty {
		result.IsEmpty = true
		result.AgentLog = append(result.AgentLog, AgentLogEntry{
			Agent:      AgentResearcher,
			Outcome:    "empty_corpus",
			DurationMs: researchDuration,
		})
		return result, nil
	}

	result.AgentLog = append(result.AgentLog, AgentLogEntry{
		Agent:      AgentResearcher,
		Outcome:    "success",
		DurationMs: researchDuration,
	})

	// --- Phase 2: Reviewer ---
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("pipeline: context cancelled before reviewer: %w", err)
	}

	reviewStart := time.Now()
	critique, err := Reviewer(ctx, cfg, llmClient, research)
	reviewDuration := time.Since(reviewStart).Milliseconds()

	if err != nil {
		result.AgentLog = append(result.AgentLog, AgentLogEntry{
			Agent:      AgentReviewer,
			Outcome:    "error",
			DurationMs: reviewDuration,
			Error:      err.Error(),
		})
		return result, fmt.Errorf("pipeline: reviewer failed: %w", err)
	}

	result.AgentLog = append(result.AgentLog, AgentLogEntry{
		Agent:      AgentReviewer,
		Outcome:    "success",
		DurationMs: reviewDuration,
	})

	// --- Phase 3 & 4: Writer + Verifier with retry loop ---
	// REQ-DEEP2-003: MaxRetries + 1 total attempts (default 3).
	// Only Verifier rejection triggers retry.
	maxAttempts := cfg.MaxRetries + 1

	// @MX:WARN: [AUTO] Bounded retry creates cost amplification surface
	// @MX:REASON: Each retry invokes Writer (Sonnet tier) — without SPEC-DEEP-004 quota enforcement, max 3 Sonnet calls per request is the only cost ceiling
	// @MX:SPEC: SPEC-DEEP-002 REQ-DEEP2-003, NFR-DEEP2-004
	var draft WriterDraft
	var retryHint *VerifierFeedback

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("pipeline: context cancelled before writer attempt %d: %w", attempt+1, err)
		}

		// --- Phase 3: Writer ---
		writerStart := time.Now()
		draft, err = Writer(ctx, cfg, llmClient, research, critique, retryHint)
		writerDuration := time.Since(writerStart).Milliseconds()

		if err != nil {
			result.AgentLog = append(result.AgentLog, AgentLogEntry{
				Agent:      AgentWriter,
				Outcome:    "error",
				DurationMs: writerDuration,
				Error:      err.Error(),
			})
			return result, fmt.Errorf("pipeline: writer failed on attempt %d: %w", attempt+1, err)
		}

		result.AgentLog = append(result.AgentLog, AgentLogEntry{
			Agent:      AgentWriter,
			Outcome:    "success",
			DurationMs: writerDuration,
		})

		// --- Phase 4: Verifier (if checkFn provided) ---
		if checkFn == nil {
			// No verifier configured — accept draft as-is.
			result.Draft = &draft
			return result, nil
		}

		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("pipeline: context cancelled before verifier attempt %d: %w", attempt+1, err)
		}

		verifierStart := time.Now()
		vResult, err := VerifierWithChecker(ctx, cfg, draft, research.Evidence, checkFn)
		verifierDuration := time.Since(verifierStart).Milliseconds()

		if err != nil {
			result.AgentLog = append(result.AgentLog, AgentLogEntry{
				Agent:      AgentVerifier,
				Outcome:    "error",
				DurationMs: verifierDuration,
				Error:      err.Error(),
			})
			return result, fmt.Errorf("pipeline: verifier failed on attempt %d: %w", attempt+1, err)
		}

		if vResult.Pass {
			result.AgentLog = append(result.AgentLog, AgentLogEntry{
				Agent:      AgentVerifier,
				Outcome:    "success",
				DurationMs: verifierDuration,
			})
			result.Draft = &draft
			return result, nil
		}

		// Verifier rejected — prepare retry hint.
		result.AgentLog = append(result.AgentLog, AgentLogEntry{
			Agent:      AgentVerifier,
			Outcome:    "error",
			DurationMs: verifierDuration,
			Error:      "verifier_rejection",
		})

		retryHint = vResult.Feedback

		// If this was the last attempt, return max-retry exhaustion error.
		if attempt+1 >= maxAttempts {
			return result, fmt.Errorf("pipeline: max retry exhausted after %d attempts: verifier_rejection_exhausted", maxAttempts)
		}
	}

	return result, nil
}
