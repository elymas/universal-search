package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// DedupChecker checks whether a spend log entry has already been processed.
// Interface-based for mock testing.
type DedupChecker interface {
	ExistsInCostLedger(ctx context.Context, requestID string) (bool, error)
	ExistsInAuditEvents(ctx context.Context, litellmRequestID string) (bool, error)
}

// Reconciler performs LiteLLM cost reconciliation.
// REQ-AUTH3-003: poll LiteLLM GET /spend/logs, dedup, INSERT remaining as cost.reconciled.
type Reconciler struct {
	client  LiteLLMClient
	emitter *Emitter
	dedup   DedupChecker
	metrics *Metrics
	cfg     Config
}

// NewReconciler creates a new LiteLLM reconciliation job handler.
func NewReconciler(client LiteLLMClient, emitter *Emitter, dedup DedupChecker, metrics *Metrics, cfg Config) *Reconciler {
	return &Reconciler{
		client:  client,
		emitter: emitter,
		dedup:   dedup,
		metrics: metrics,
		cfg:     cfg,
	}
}

// Reconcile executes one reconciliation cycle.
// REQ-AUTH3-003: poll LiteLLM /spend/logs, dedup, INSERT as cost.reconciled.
// cost_ledger SHALL NOT be mutated.
func (r *Reconciler) Reconcile(ctx context.Context) error {
	if r.client == nil {
		slog.Warn("audit: reconcile skipped, no LiteLLM client configured")
		return nil
	}

	now := time.Now().UTC()
	// 1-minute overlap for window gap protection.
	startDate := now.Add(-time.Duration(r.cfg.ReconcileIntervalMinutes+1) * time.Minute)
	endDate := now

	logs, err := r.client.FetchSpendLogs(ctx, startDate, endDate)
	if err != nil {
		if r.metrics != nil {
			r.metrics.ReconcilePollsTotal.WithLabelValues("error").Inc()
		}
		return fmt.Errorf("audit: reconcile fetch: %w", err)
	}

	inserted := 0
	skipped := 0

	for _, log := range logs {
		// Dedup: check cost_ledger first.
		if r.dedup != nil {
			exists, err := r.dedup.ExistsInCostLedger(ctx, log.RequestID)
			if err != nil {
				slog.Warn("audit: dedup cost_ledger check failed", "request_id", log.RequestID, "error", err)
				continue
			}
			if exists {
				skipped++
				continue
			}

			// Dedup: check audit_events.
			exists, err = r.dedup.ExistsInAuditEvents(ctx, log.RequestID)
			if err != nil {
				slog.Warn("audit: dedup audit_events check failed", "request_id", log.RequestID, "error", err)
				continue
			}
			if exists {
				skipped++
				continue
			}
		}

		// INSERT as cost.reconciled.
		// REQ-AUTH3-003: cost_ledger SHALL NOT be mutated.
		payload := map[string]interface{}{
			"litellm_request_id":  log.RequestID,
			"model":               log.Model,
			"prompt_tokens":       log.PromptTokens,
			"completion_tokens":   log.CompletionTokens,
			"spend_usd":           log.Spend,
			"call_type":           log.CallType,
		}
		if log.Metadata != nil {
			payload["metadata"] = log.Metadata
		}

		evt := AuditEvent{
			EventType: EventCostReconciled,
			Decision:  DecisionNone,
			UserID:    "anonymous", // LiteLLM may not provide user identity
			TenantID:  "default",
			RequestID: log.RequestID,
			Source:    SourcePython,
			Payload:   payload,
		}

		if err := r.emitter.EmitEvent(ctx, evt); err != nil {
			slog.Warn("audit: reconcile emit failed", "request_id", log.RequestID, "error", err)
			continue
		}
		inserted++
	}

	// Update lag gauge.
	if r.metrics != nil {
		r.metrics.ReconcilePollsTotal.WithLabelValues("success").Inc()
		r.metrics.ReconcileLagSeconds.Set(time.Since(startDate).Seconds())
	}

	slog.Info("audit: reconciliation complete",
		"fetched", len(logs),
		"inserted", inserted,
		"skipped", skipped,
	)

	return nil
}
