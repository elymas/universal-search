// Package types_test — Query struct shape tests.
package types_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestQueryStructFields verifies the Query type declares the six required
// fields with the specified types, per REQ-CORE-002.
func TestQueryStructFields(t *testing.T) {
	t.Parallel()

	q := types.Query{}
	rt := reflect.TypeOf(q)
	if rt.Kind() != reflect.Struct {
		t.Fatalf("Query is %v, want Struct", rt.Kind())
	}

	wantFields := map[string]reflect.Type{
		"Text":       reflect.TypeOf(""),
		"Lang":       reflect.TypeOf(""),
		"MaxResults": reflect.TypeOf(0),
		"Filters":    reflect.TypeOf([]types.Filter{}),
		"Cursor":     reflect.TypeOf(""),
		"Deadline":   reflect.TypeOf(time.Time{}),
	}

	if got, want := rt.NumField(), len(wantFields); got != want {
		t.Errorf("Query NumField = %d, want %d", got, want)
	}

	for name, wantT := range wantFields {
		f, ok := rt.FieldByName(name)
		if !ok {
			t.Errorf("Query missing field %q", name)
			continue
		}
		if f.Type != wantT {
			t.Errorf("Query.%s type = %v, want %v", name, f.Type, wantT)
		}
	}
}

// TestFilterStructFields verifies the Filter type has Key/Value strings.
func TestFilterStructFields(t *testing.T) {
	t.Parallel()

	f := types.Filter{Key: "date_from", Value: "2026-01-01"}
	if f.Key != "date_from" {
		t.Errorf("Filter.Key = %q, want %q", f.Key, "date_from")
	}
	if f.Value != "2026-01-01" {
		t.Errorf("Filter.Value = %q, want %q", f.Value, "2026-01-01")
	}
}
