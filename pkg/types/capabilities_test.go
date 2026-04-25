// Package types_test — Capabilities and DocType shape tests.
package types_test

import (
	"reflect"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestCapabilitiesStructFields verifies Capabilities declares the ten
// required fields with the specified types, per REQ-CORE-002.
func TestCapabilitiesStructFields(t *testing.T) {
	t.Parallel()

	c := types.Capabilities{}
	rt := reflect.TypeOf(c)
	if rt.Kind() != reflect.Struct {
		t.Fatalf("Capabilities is %v, want Struct", rt.Kind())
	}

	wantFields := map[string]reflect.Type{
		"SourceID":          reflect.TypeOf(""),
		"DisplayName":       reflect.TypeOf(""),
		"DocTypes":          reflect.TypeOf([]types.DocType{}),
		"SupportedLangs":    reflect.TypeOf([]string{}),
		"SupportsSince":     reflect.TypeOf(false),
		"RequiresAuth":      reflect.TypeOf(false),
		"AuthEnvVars":       reflect.TypeOf([]string{}),
		"RateLimitPerMin":   reflect.TypeOf(0),
		"DefaultMaxResults": reflect.TypeOf(0),
		"Notes":             reflect.TypeOf(""),
	}

	if got, want := rt.NumField(), len(wantFields); got != want {
		t.Errorf("Capabilities NumField = %d, want %d", got, want)
	}

	for name, wantT := range wantFields {
		f, ok := rt.FieldByName(name)
		if !ok {
			t.Errorf("Capabilities missing field %q", name)
			continue
		}
		if f.Type != wantT {
			t.Errorf("Capabilities.%s type = %v, want %v", name, f.Type, wantT)
		}
	}
}

// TestDocTypeEnumComplete verifies all eight DocType constants are declared
// and have the expected canonical string form.
// REQ-CORE-002.
func TestDocTypeEnumComplete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		dt   types.DocType
		want string
	}{
		{types.DocTypeArticle, "article"},
		{types.DocTypePost, "post"},
		{types.DocTypePaper, "paper"},
		{types.DocTypeVideo, "video"},
		{types.DocTypeRepo, "repo"},
		{types.DocTypeIssue, "issue"},
		{types.DocTypeSocial, "social"},
		{types.DocTypeOther, "other"},
	}

	seen := make(map[string]bool)
	for _, tc := range cases {
		if string(tc.dt) != tc.want {
			t.Errorf("DocType(%v) = %q, want %q", tc.dt, string(tc.dt), tc.want)
		}
		if seen[string(tc.dt)] {
			t.Errorf("duplicate DocType value %q", string(tc.dt))
		}
		seen[string(tc.dt)] = true
	}
	if len(seen) != 8 {
		t.Errorf("DocType enum size = %d, want 8", len(seen))
	}
}
