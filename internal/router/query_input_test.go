// Package router_test validates RouterQuery wrapping + validation.
package router_test

import (
	"errors"
	"testing"

	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// TestRouterQueryAcceptsPopulated asserts a populated RouterQuery validates
// successfully (REQ-IR-001).
func TestRouterQueryAcceptsPopulated(t *testing.T) {
	t.Parallel()

	q := router.RouterQuery{
		Query: types.Query{Text: "transformer paper", MaxResults: 10},
	}
	if err := q.Validate(); err != nil {
		t.Errorf("Validate populated query: %v", err)
	}
}

// TestRouterQueryEmptyTextFails asserts empty Text returns ErrInvalidQuery
// (REQ-IR-005).
func TestRouterQueryEmptyTextFails(t *testing.T) {
	t.Parallel()
	q := router.RouterQuery{Query: types.Query{Text: ""}}
	if err := q.Validate(); !errors.Is(err, router.ErrInvalidQuery) {
		t.Errorf("Validate empty: got %v, want ErrInvalidQuery", err)
	}
}

// TestRouterQueryWhitespaceOnly asserts Unicode-whitespace-only Text returns
// ErrInvalidQuery (REQ-IR-005 acceptance S-4).
func TestRouterQueryWhitespaceOnly(t *testing.T) {
	t.Parallel()

	cases := []string{"   ", "\t\n", "\t\n  \r", " ", " \r ", " ", "　"}
	for _, txt := range cases {
		txt := txt
		t.Run(txt, func(t *testing.T) {
			t.Parallel()
			q := router.RouterQuery{Query: types.Query{Text: txt}}
			if err := q.Validate(); !errors.Is(err, router.ErrInvalidQuery) {
				t.Errorf("Validate whitespace %q: got %v, want ErrInvalidQuery", txt, err)
			}
		})
	}
}

// TestRouterQueryLangOverrideExposed asserts the optional Lang hint is
// reachable through the Query.Lang field (REQ-IR-004).
func TestRouterQueryLangOverrideExposed(t *testing.T) {
	t.Parallel()
	q := router.RouterQuery{Query: types.Query{Text: "hi", Lang: "ja"}}
	if q.Lang != "ja" {
		t.Errorf("Lang override: got %q, want %q", q.Lang, "ja")
	}
}
