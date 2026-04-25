// Package llm — cost extraction and budget cap enforcement.
// REQ-LLM-006: Parse x-litellm-response-cost header; emit counter; handle missing/malformed.
// NFR-LLM-003: Post-flight budget cap; ErrBudgetExceeded returned alongside Response.
package llm

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/elymas/universal-search/internal/obs/metrics"
	"github.com/elymas/universal-search/internal/obs/reqid"
	"go.opentelemetry.io/otel/trace"

	"github.com/prometheus/client_golang/prometheus"
)

// costHeaderKey is the LiteLLM response header carrying USD cost per request.
const costHeaderKey = "x-litellm-response-cost"

// costMiddlewareFunc is the signature consumed by openai-go's option.WithMiddleware.
// It captures the x-litellm-response-cost header and stores it in the request context.
//
// @MX:WARN: [AUTO] Context mutation via pointer; request context is replaced
// @MX:REASON: openai-go middleware does not allow replacing the request pointer directly;
//
//	we use a context pointer trick — see implementation note below.
func newCostMiddlewareRoundTripper(next http.RoundTripper, costPtr *float64, logger *slog.Logger) http.RoundTripper {
	return costRoundTripper{next: next, costPtr: costPtr, logger: logger}
}

type costRoundTripper struct {
	next    http.RoundTripper
	costPtr *float64
	logger  *slog.Logger
}

func (c costRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.next.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}

	raw := resp.Header.Get(costHeaderKey)
	if raw == "" {
		if c.logger != nil {
			c.logger.DebugContext(req.Context(), "cost header missing",
				slog.String("provider", req.Host))
		}
		return resp, nil
	}

	v, parseErr := strconv.ParseFloat(raw, 64)
	if parseErr != nil {
		if c.logger != nil {
			c.logger.WarnContext(req.Context(), "malformed cost header",
				slog.String("raw", raw),
				slog.String("err", parseErr.Error()))
		}
		return resp, nil
	}

	*c.costPtr = v
	return resp, nil
}

// emitCostMetric records cost on the LLMCost counter and LLMLatency exemplar.
// When ctx carries an active OTel span and a request ID, it emits a Prometheus exemplar.
func emitCostMetric(ctx context.Context, reg *metrics.Registry, provider, model string, cost float64) {
	if reg == nil || reg.LLMCost == nil {
		return
	}
	if cost > 0 {
		reg.LLMCost.WithLabelValues(provider, model).Add(cost)
	}

	// Exemplar on LLMLatency histogram for trace→metric linking (REQ-LLM-007).
	if reg.LLMLatency == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	rid := reqid.FromContext(ctx)
	if span.SpanContext().IsValid() && rid != "" {
		obs := reg.LLMLatency.WithLabelValues(provider, model)
		if o, ok := obs.(prometheus.ExemplarObserver); ok {
			o.ObserveWithExemplar(0, prometheus.Labels{
				"trace_id":   span.SpanContext().TraceID().String(),
				"request_id": rid,
			})
		}
	}
}

// checkBudget compares cost against cap and returns ErrBudgetExceeded if exceeded.
// cap == 0 means unlimited.
func checkBudget(cost, cap float64) error {
	if cap <= 0 {
		return nil
	}
	if cost > cap {
		return fmt.Errorf("%w: cost %.6f exceeds cap %.6f", ErrBudgetExceeded, cost, cap)
	}
	return nil
}
