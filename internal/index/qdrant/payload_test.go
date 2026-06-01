package qdrant

// Behavior tests for payloadValue: maps each Qdrant protobuf Value kind to its
// Go-native representation. Used when converting search-hit payloads back into
// NormalizedDoc fields.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"testing"

	qdrantpb "github.com/qdrant/go-client/qdrant"
)

func TestPayloadValue(t *testing.T) {
	tests := []struct {
		name string
		in   *qdrantpb.Value
		want any
	}{
		{"nil value", nil, nil},
		{
			"string",
			&qdrantpb.Value{Kind: &qdrantpb.Value_StringValue{StringValue: "hello"}},
			"hello",
		},
		{
			"integer",
			&qdrantpb.Value{Kind: &qdrantpb.Value_IntegerValue{IntegerValue: 42}},
			int64(42),
		},
		{
			"double",
			&qdrantpb.Value{Kind: &qdrantpb.Value_DoubleValue{DoubleValue: 3.14}},
			3.14,
		},
		{
			"bool",
			&qdrantpb.Value{Kind: &qdrantpb.Value_BoolValue{BoolValue: true}},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := payloadValue(tt.in)
			if got != tt.want {
				t.Errorf("payloadValue(%s) = %v (%T), want %v (%T)", tt.name, got, got, tt.want, tt.want)
			}
		})
	}
}

// TestPayloadValue_UnknownKindFallsBackToString verifies the default branch
// returns the (empty) string-value representation for an unhandled kind.
func TestPayloadValue_UnknownKindFallsBackToString(t *testing.T) {
	// A Value with a list kind is not handled explicitly; the default branch
	// calls GetStringValue() which returns "" for non-string kinds.
	v := &qdrantpb.Value{Kind: &qdrantpb.Value_ListValue{ListValue: &qdrantpb.ListValue{}}}
	if got := payloadValue(v); got != "" {
		t.Errorf("payloadValue(list) = %v, want empty string fallback", got)
	}
}
