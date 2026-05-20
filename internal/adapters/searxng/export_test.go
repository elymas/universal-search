// Package searxng — test-only export shims.
// This file exposes unexported functions under alias names for the external
// (black-box) test package searxng_test.
package searxng

import (
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// ExportedParseSearch is the test-package alias for the internal parseSearch.
// Used by the external test package searxng_test.
var ExportedParseSearch = func(body []byte, retrievedAt time.Time, currentPage int) ([]types.NormalizedDoc, string, error) {
	return parseSearch(body, retrievedAt, currentPage)
}
