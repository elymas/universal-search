package audit

import (
	"context"
	"fmt"
	"time"
)

// ReplayRequest is the input for a replay operation.
type ReplayRequest struct {
	RequestID string `json:"request_id"`
}

// ReplayResponse is the output of a successful replay.
type ReplayResponse struct {
	NewRequestID string `json:"new_request_id"`
	Status       string `json:"status"`
}

// ReplayError represents a replay failure with a specific reason.
type ReplayError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *ReplayError) Error() string {
	return fmt.Sprintf("replay error %d: %s", e.Code, e.Message)
}

// EventFetcher retrieves audit events by request_id.
type EventFetcher interface {
	FetchByRequestID(ctx context.Context, requestID string) (*AuditEvent, error)
}

// ReplayRateLimiter tracks per-user replay rate limits.
type ReplayRateLimiter interface {
	Allow(userID string) bool
}

// ReplayHandler handles query replay logic.
// REQ-AUTH3-004: JWT auth, RBAC, rate limit, fetch, validate, re-execute.
// @MX:WARN: [AUTO] Admin can re-execute arbitrary queries
// @MX:REASON: If AUTH-002 permission validation is bypassed, admin could re-execute all queries in audit_events. Verify RBAC check order when changing this function.
type ReplayHandler struct {
	emitter   *Emitter
	fetcher   EventFetcher
	rateLimit ReplayRateLimiter
	metrics   *Metrics
	cfg       Config
}

// NewReplayHandler creates a new replay handler.
func NewReplayHandler(emitter *Emitter, fetcher EventFetcher, rateLimit ReplayRateLimiter, metrics *Metrics, cfg Config) *ReplayHandler {
	return &ReplayHandler{
		emitter:   emitter,
		fetcher:   fetcher,
		rateLimit: rateLimit,
		metrics:   metrics,
		cfg:       cfg,
	}
}

// Replay executes a query replay.
// REQ-AUTH3-004: fetch event, validate replayable, emit replay event.
// REQ-AUTH3-006: reject if query text is masked.
func (h *ReplayHandler) Replay(ctx context.Context, adminUserID string, req ReplayRequest) (*ReplayResponse, error) {
	// Rate limit check.
	if h.rateLimit != nil && !h.rateLimit.Allow(adminUserID) {
		if h.metrics != nil {
			h.metrics.ReplayRequestsTotal.WithLabelValues("error").Inc()
		}
		// Emit deny event for rate limit.
		_ = h.emitter.EmitEvent(ctx, AuditEvent{
			EventType: EventAdminReplay,
			Decision:  DecisionDeny,
			UserID:    adminUserID,
			Payload: map[string]interface{}{
				"original_request_id": req.RequestID,
				"denied_reason":       "rate_limit_exceeded",
			},
		})
		return nil, &ReplayError{Code: 429, Message: "rate_limit_exceeded", Detail: "retry after 1 minute"}
	}

	// Fetch the original event.
	if h.fetcher == nil {
		return nil, &ReplayError{Code: 400, Message: "unknown_request_id"}
	}

	original, err := h.fetcher.FetchByRequestID(ctx, req.RequestID)
	if err != nil || original == nil {
		// Emit deny event for unknown request.
		_ = h.emitter.EmitEvent(ctx, AuditEvent{
			EventType: EventAdminReplay,
			Decision:  DecisionDeny,
			UserID:    adminUserID,
			Payload: map[string]interface{}{
				"original_request_id": req.RequestID,
				"denied_reason":       "unknown_request_id",
			},
		})
		return nil, &ReplayError{Code: 400, Message: "unknown_request_id"}
	}

	// Check if the event type is replayable.
	if !original.EventType.IsReplayable() {
		_ = h.emitter.EmitEvent(ctx, AuditEvent{
			EventType: EventAdminReplay,
			Decision:  DecisionDeny,
			UserID:    adminUserID,
			Payload: map[string]interface{}{
				"original_request_id": req.RequestID,
				"denied_reason":       "event_not_replayable",
			},
		})
		return nil, &ReplayError{Code: 400, Message: "event_not_replayable"}
	}

	// Check for PII-masked query text (REQ-AUTH3-006).
	if query, ok := original.Payload["query"].(map[string]interface{}); ok {
		if _, hasText := query["text"]; !hasText {
			if _, hasSHA := query["text_sha256"]; hasSHA {
				_ = h.emitter.EmitEvent(ctx, AuditEvent{
					EventType: EventAdminReplay,
					Decision:  DecisionDeny,
					UserID:    adminUserID,
					Payload: map[string]interface{}{
						"original_request_id": req.RequestID,
						"denied_reason":       "query_text_masked",
					},
				})
				return nil, &ReplayError{Code: 400, Message: "query_text_masked", Detail: "original query unavailable for replay"}
			}
		}
	}

	// Generate new request ID.
	newRequestID := fmt.Sprintf("req_replay_%d", time.Now().UnixNano())

	// Emit admin.replay event with cross-reference.
	_ = h.emitter.EmitEvent(ctx, AuditEvent{
		EventType: EventAdminReplay,
		Decision:  DecisionAllow,
		UserID:    adminUserID,
		Payload: map[string]interface{}{
			"original_request_id": req.RequestID,
			"new_request_id":      newRequestID,
			"replay_actor":        adminUserID,
		},
	})

	// Emit new query.submit event with admin actor identity (NOT original user).
	// REQ-AUTH3-004: admin actor identity, NOT original user.
	replayPayload := map[string]interface{}{}
	for k, v := range original.Payload {
		replayPayload[k] = v
	}
	replayPayload["replayed_from"] = req.RequestID
	replayPayload["replayed_by"] = adminUserID

	_ = h.emitter.EmitEvent(ctx, AuditEvent{
		EventType: original.EventType,
		Decision:  DecisionAllow,
		UserID:    adminUserID,
		TenantID:  original.TenantID,
		TeamID:    original.TeamID,
		RequestID: newRequestID,
		Source:    SourceGo,
		Payload:   replayPayload,
	})

	if h.metrics != nil {
		h.metrics.ReplayRequestsTotal.WithLabelValues("allowed").Inc()
	}

	return &ReplayResponse{
		NewRequestID: newRequestID,
		Status:       "submitted",
	}, nil
}
