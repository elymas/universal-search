// Package types_test — Adapter interface shape verification.
package types_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestAdapterInterfaceShape confirms the Adapter interface declares exactly
// the four documented methods with the expected signatures.
// REQ-CORE-002.
func TestAdapterInterfaceShape(t *testing.T) {
	t.Parallel()

	rt := reflect.TypeOf((*types.Adapter)(nil)).Elem()
	if rt.Kind() != reflect.Interface {
		t.Fatalf("Adapter is %v, want Interface", rt.Kind())
	}
	if got, want := rt.NumMethod(), 4; got != want {
		t.Fatalf("Adapter NumMethod = %d, want %d", got, want)
	}

	stringT := reflect.TypeOf("")
	errT := reflect.TypeOf((*error)(nil)).Elem()
	ctxT := reflect.TypeOf((*context.Context)(nil)).Elem()
	docsT := reflect.TypeOf([]types.NormalizedDoc{})
	queryT := reflect.TypeOf(types.Query{})
	capsT := reflect.TypeOf(types.Capabilities{})

	wantMethods := map[string]struct {
		in  []reflect.Type
		out []reflect.Type
	}{
		"Name":         {in: nil, out: []reflect.Type{stringT}},
		"Search":       {in: []reflect.Type{ctxT, queryT}, out: []reflect.Type{docsT, errT}},
		"Healthcheck":  {in: []reflect.Type{ctxT}, out: []reflect.Type{errT}},
		"Capabilities": {in: nil, out: []reflect.Type{capsT}},
	}

	for name, want := range wantMethods {
		m, ok := rt.MethodByName(name)
		if !ok {
			t.Errorf("Adapter missing method %s", name)
			continue
		}
		ft := m.Type
		if got := ft.NumIn(); got != len(want.in) {
			t.Errorf("%s NumIn = %d, want %d", name, got, len(want.in))
			continue
		}
		for i, in := range want.in {
			if ft.In(i) != in {
				t.Errorf("%s.In(%d) = %v, want %v", name, i, ft.In(i), in)
			}
		}
		if got := ft.NumOut(); got != len(want.out) {
			t.Errorf("%s NumOut = %d, want %d", name, got, len(want.out))
			continue
		}
		for i, out := range want.out {
			if ft.Out(i) != out {
				t.Errorf("%s.Out(%d) = %v, want %v", name, i, ft.Out(i), out)
			}
		}
	}
}
