package searxng_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/adapters/searxng"
	"github.com/elymas/universal-search/pkg/types"
)

// testDialTimeout is the context timeout for Healthcheck tests dialing
// unreachable addresses. Keep short to avoid slow CI.
const testDialTimeout = 200 * time.Millisecond

// asSourceError attempts to unwrap err as *types.SourceError via errors.As.
// Returns true and fills target on success.
func asSourceError(err error, target **types.SourceError) bool {
	return errors.As(err, target)
}

// testCtx returns a background context that is cancelled when the test ends.
func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

// exportedParseSearch is a local alias for the exported test shim.
var exportedParseSearch = searxng.ExportedParseSearch
