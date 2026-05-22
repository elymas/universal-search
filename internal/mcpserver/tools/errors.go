// Error types for MCP server, shared across packages.
// Defined here to avoid import cycles between mcpserver and mcpserver/tools.
package tools

import (
	"encoding/json"
	"fmt"
)

// Error codes in the JSON-RPC server-defined range -32099..-32000.
// FROZEN per REQ-MCP-016: codes MUST NOT be renumbered or renamed within V1.x.
const (
	ErrCodeCapExceeded      = -32000
	ErrCodeDeepNotWarranted = -32001
	ErrCodeUnauthorized     = -32002
	ErrCodeNoAdaptersMatched = -32003
	ErrCodeAllAdaptersFailed = -32004
	ErrCodeSynthesisDegraded = -32005
	ErrCodeTimeout           = -32006
	ErrCodeCitationNotFound  = -32007
)

// MCPError is a structured error mapped from internal errors to JSON-RPC errors.
type MCPError struct {
	Code      int            `json:"code"`
	Message   string         `json:"message"`
	Namespace string         `json:"-"` // placed in data.namespace
	Data      map[string]any `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP error %d: %s (%s)", e.Code, e.Message, e.Namespace)
}

// ToJSONRPC returns a JSON-RPC error object.
func (e *MCPError) ToJSONRPC() map[string]any {
	obj := map[string]any{
		"code":    e.Code,
		"message": e.Message,
	}
	if e.Namespace != "" || len(e.Data) > 0 {
		data := make(map[string]any)
		for k, v := range e.Data {
			data[k] = v
		}
		if e.Namespace != "" {
			data["namespace"] = e.Namespace
		}
		obj["data"] = data
	}
	return obj
}

// MarshalJSON implements json.Marshaler for MCPError.
func (e *MCPError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.ToJSONRPC())
}

// MapError maps an internal error to an MCPError per REQ-MCP-016.
func MapError(err error) *MCPError {
	if err == nil {
		return nil
	}
	return &MCPError{
		Code:      -32603,
		Message:   "Internal error",
		Namespace: "usearch.internal",
		Data:      map[string]any{"reason": err.Error()},
	}
}

// Specific error constructors for each REQ-MCP-016 mapping.

func CapExceededError(dimension string, remaining int, resetAt string, retryAfterS int) *MCPError {
	return &MCPError{Code: ErrCodeCapExceeded, Message: "Cap exceeded", Namespace: "usearch.cap_exceeded",
		Data: map[string]any{"namespace": "usearch.cap_exceeded", "dimension": dimension, "remaining": remaining, "reset_at": resetAt, "retry_after_s": retryAfterS}}
}

func DeepNotWarrantedError(screenScore float64, suggestedMode, rationale string) *MCPError {
	return &MCPError{Code: ErrCodeDeepNotWarranted, Message: "Deep research not warranted", Namespace: "usearch.deep_not_warranted",
		Data: map[string]any{"namespace": "usearch.deep_not_warranted", "screen_score": screenScore, "suggested_mode": suggestedMode, "rationale": rationale}}
}

func UnauthorizedError(reason string) *MCPError {
	return &MCPError{Code: ErrCodeUnauthorized, Message: "Unauthorized", Namespace: "usearch.unauthorized",
		Data: map[string]any{"namespace": "usearch.unauthorized", "reason": reason}}
}

func NoAdaptersMatchedError(queryCategory string) *MCPError {
	return &MCPError{Code: ErrCodeNoAdaptersMatched, Message: "No adapters matched", Namespace: "usearch.no_adapters_matched",
		Data: map[string]any{"namespace": "usearch.no_adapters_matched", "query_category": queryCategory}}
}

func AllAdaptersFailedError(errors []string) *MCPError {
	return &MCPError{Code: ErrCodeAllAdaptersFailed, Message: "All adapters failed", Namespace: "usearch.all_adapters_failed",
		Data: map[string]any{"namespace": "usearch.all_adapters_failed", "errors": errors}}
}

func SynthesisDegradedError(reason string) *MCPError {
	return &MCPError{Code: ErrCodeSynthesisDegraded, Message: "Synthesis degraded", Namespace: "usearch.synthesis_degraded",
		Data: map[string]any{"namespace": "usearch.synthesis_degraded", "degraded_reason": reason}}
}

func TimeoutError(stage string, deadlineMs int) *MCPError {
	return &MCPError{Code: ErrCodeTimeout, Message: "Timeout", Namespace: "usearch.timeout",
		Data: map[string]any{"namespace": "usearch.timeout", "stage": stage, "deadline_ms": deadlineMs}}
}

func CitationNotFoundError(docID string) *MCPError {
	return &MCPError{Code: ErrCodeCitationNotFound, Message: "Citation not found", Namespace: "usearch.citation_not_found",
		Data: map[string]any{"namespace": "usearch.citation_not_found", "doc_id": docID}}
}

func ToolNotEnabledError(toolName string) *MCPError {
	return &MCPError{Code: -32601, Message: "Method not found", Data: map[string]any{"tool_name": toolName}}
}

func InputSchemaViolationError(field, reason string) *MCPError {
	return &MCPError{Code: -32602, Message: "Invalid params", Data: map[string]any{"field": field, "reason": reason}}
}
