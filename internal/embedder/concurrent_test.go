package embedder_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/embedder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientEmbedConcurrent verifies NFR-IDX-005: 50 goroutines × 100 calls = 5,000 invocations
// under go test -race with no data races or panics.
func TestClientEmbedConcurrent(t *testing.T) {
	t.Parallel()

	const goroutines = 50
	const callsPerGoroutine = 100

	var totalCalls int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&totalCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cannedResponse(t, "req-concurrent", 1, 0, 1))
	}))
	t.Cleanup(srv.Close)

	cfg := embedder.Config{
		BaseURL:        srv.URL,
		RequestTimeout: 5 * time.Second,
	}
	c, err := embedder.New(cfg, nil)
	require.NoError(t, err)

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*callsPerGoroutine)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < callsPerGoroutine; i++ {
				req := embedder.Request{
					RequestID:   "req-concurrent",
					Texts:       []string{"hello"},
					ReturnDense: true,
					BatchSize:   32,
				}
				_, err := c.Embed(context.Background(), req)
				if err != nil {
					errCh <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	assert.Empty(t, errs, "concurrent calls should not produce errors")
	assert.Equal(t, int64(goroutines*callsPerGoroutine), atomic.LoadInt64(&totalCalls),
		"all 5000 calls should reach the server")
}
