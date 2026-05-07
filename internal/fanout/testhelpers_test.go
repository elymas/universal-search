// testhelpers_test.go — shared test utilities for the fanout package.
// These helpers are in package fanout_test and compiled only during testing.
package fanout_test

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// --------------------------------------------------------------------------
// stubAdapter — synchronous adapter for unit tests.
// --------------------------------------------------------------------------

type stubAdapter struct {
	name     string
	docs     []types.NormalizedDoc
	err      error
	latency  time.Duration // optional sleep before returning
	doPanic  bool          // if true, Search panics
	inflight *atomic.Int32 // optional shared counter (for MaxParallel tests)
}

func (s *stubAdapter) Name() string { return s.name }
func (s *stubAdapter) Capabilities() types.Capabilities {
	return types.Capabilities{SourceID: s.name}
}
func (s *stubAdapter) Healthcheck(_ context.Context) error { return nil }
func (s *stubAdapter) Search(ctx context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
	if s.inflight != nil {
		s.inflight.Add(1)
		defer s.inflight.Add(-1)
	}
	if s.latency > 0 {
		select {
		case <-time.After(s.latency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.doPanic {
		panic("oops")
	}
	return s.docs, s.err
}

// --------------------------------------------------------------------------
// Builder helpers.
// --------------------------------------------------------------------------

// buildTestRegistry creates a registry with the given stub adapters (SkipAuthCheck=true).
func buildTestRegistry(stubs ...*stubAdapter) *adapters.Registry {
	reg := adapters.NewRegistry(nil)
	for _, s := range stubs {
		if err := reg.RegisterWithOptions(s, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
			panic("buildTestRegistry: " + err.Error())
		}
	}
	return reg
}

// makeDocs returns n unique NormalizedDocs tagged with the given adapterName.
func makeDocs(adapterName string, n int) []types.NormalizedDoc {
	now := time.Now()
	docs := make([]types.NormalizedDoc, n)
	for i := range n {
		docs[i] = types.NormalizedDoc{
			ID:          adapterName + "_" + itoa(i),
			SourceID:    adapterName,
			URL:         "https://example.com/" + adapterName + "/" + itoa(i),
			Title:       adapterName + " title " + itoa(i),
			Body:        "body",
			RetrievedAt: now,
			Score:       0.5,
		}
	}
	return docs
}

// makeDecision constructs a RoutingDecision with the given adapter names.
func makeDecision(names ...string) router.RoutingDecision {
	return router.RoutingDecision{
		Category:   router.CategoryWeb,
		AdapterSet: names,
	}
}

// itoa converts a non-negative int to a string.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
