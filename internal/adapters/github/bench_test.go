// Package github — benchmark + TestMain goroutine-leak guard.
// NFR-ADP4-001: parse throughput ≤ 5ms median for 25 results.
// NFR-ADP4-003: goleak.VerifyTestMain — package-level goroutine leak check.
package github

import (
	"context"
	"testing"

	"go.uber.org/goleak"
)

// TestMain installs goleak as the package-level goroutine leak check.
// Any test that leaves a goroutine running after the package tests finish
// will be reported here.
//
// net/http.setRequestCancel.func4 goroutines are created by the HTTP client
// when requests are cancelled (e.g. in TestSearchNoGoroutineLeakOnCancel).
// They exit asynchronously once the underlying connection drains; they are
// not genuine leaks.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("net/http.setRequestCancel.func4"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	)
}

// BenchmarkParseGitHubResponse25Results benchmarks the full Search path
// through a local httptest stub serving 25 repository results.
// NFR-ADP4-001: median ≤ 5ms; allocs/op ≤ 625.
func BenchmarkParseGitHubResponse25Results(b *testing.B) {
	srv := newRepoStubServer(b, "search_repos_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(b, srv.URL)

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		q := testQuery("golang", "repos", 25, "")
		docs, err := a.Search(context.Background(), q)
		if err != nil {
			b.Fatalf("Search: %v", err)
		}
		if len(docs) != 25 {
			b.Fatalf("expected 25 docs, got %d", len(docs))
		}
	}
}
