// Package router — RouterQuery wraps pkg/types.Query for classification.
// SPEC-IR-001: REQ-IR-001, REQ-IR-004, REQ-IR-005.
package router

import (
	"strings"
	"unicode"

	"github.com/elymas/universal-search/pkg/types"
)

// RouterQuery is the input passed to Router.Classify. It embeds the canonical
// pkg/types.Query so callers can populate every existing field, plus carries
// IR-only optional hints (currently piggy-backing on Query.Lang for the
// REQ-IR-004 override).
type RouterQuery struct {
	types.Query
}

// Validate reports ErrInvalidQuery when the underlying Text is empty or
// composed entirely of Unicode whitespace runes (REQ-IR-005).
func (q RouterQuery) Validate() error {
	if strings.TrimFunc(q.Text, unicode.IsSpace) == "" {
		return ErrInvalidQuery
	}
	return nil
}
