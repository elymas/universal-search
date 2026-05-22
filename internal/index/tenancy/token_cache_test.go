package tenancy

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestTokenCacheGetOrIssueCachesToken(t *testing.T) {
	defer goleak.VerifyNone(t)

	var calls atomic.Int32
	cache := NewTokenCache(TokenCacheConfig{
		IssueToken: func(ctx context.Context, teamID, userID, apiKeyUID string) (string, time.Time, error) {
			calls.Add(1)
			return "token-" + teamID, time.Now().Add(15 * time.Minute), nil
		},
	})

	entry, err := cache.GetOrIssue(context.Background(), "team-T", "alice", "uid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Token != "token-team-T" {
		t.Errorf("token = %q, want 'token-team-T'", entry.Token)
	}
	if calls.Load() != 1 {
		t.Errorf("issueToken called %d times, want 1", calls.Load())
	}

	// Second call should use cache.
	entry2, err := cache.GetOrIssue(context.Background(), "team-T", "alice", "uid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry2.Token != "token-team-T" {
		t.Errorf("cached token = %q, want 'token-team-T'", entry2.Token)
	}
	if calls.Load() != 1 {
		t.Errorf("issueToken called %d times after cache hit, want 1", calls.Load())
	}
}

func TestTokenCacheDifferentKeysDifferentTokens(t *testing.T) {
	defer goleak.VerifyNone(t)

	cache := NewTokenCache(TokenCacheConfig{
		IssueToken: func(ctx context.Context, teamID, userID, apiKeyUID string) (string, time.Time, error) {
			return "token-" + teamID, time.Now().Add(15 * time.Minute), nil
		},
	})

	entry1, _ := cache.GetOrIssue(context.Background(), "team-T", "alice", "uid-1")
	entry2, _ := cache.GetOrIssue(context.Background(), "team-U", "bob", "uid-1")

	if entry1.Token == entry2.Token {
		t.Error("different keys should produce different tokens")
	}
}

func TestTokenCacheReissuesOnExpiry(t *testing.T) {
	defer goleak.VerifyNone(t)

	var calls atomic.Int32
	cache := NewTokenCache(TokenCacheConfig{
		IssueToken: func(ctx context.Context, teamID, userID, apiKeyUID string) (string, time.Time, error) {
			calls.Add(1)
			return "token-v2", time.Now().Add(15 * time.Minute), nil
		},
	})

	// Issue with short TTL.
	cache.mu.Lock()
	cache.entries[cacheKey{TeamID: "team-T", UserID: "alice", KeyUID: "uid-1"}] = &TokenEntry{
		Token:     "token-v1",
		ExpiresAt: time.Now().Add(-1 * time.Second), // already expired
	}
	cache.mu.Unlock()

	entry, err := cache.GetOrIssue(context.Background(), "team-T", "alice", "uid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Token != "token-v2" {
		t.Errorf("reissued token = %q, want 'token-v2'", entry.Token)
	}
}

func TestTokenCacheConcurrency50x100(t *testing.T) {
	defer goleak.VerifyNone(t)

	var calls atomic.Int32
	cache := NewTokenCache(TokenCacheConfig{
		IssueToken: func(ctx context.Context, teamID, userID, apiKeyUID string) (string, time.Time, error) {
			calls.Add(1)
			time.Sleep(10 * time.Millisecond) // simulate work
			return "token-concurrent", time.Now().Add(15 * time.Minute), nil
		},
	})

	var wg sync.WaitGroup
	const goroutines = 50
	const callsPerGoroutine = 100

	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range callsPerGoroutine {
				_, err := cache.GetOrIssue(context.Background(), "team-T", "alice", "uid-1")
				if err != nil {
					t.Errorf("unexpected error in goroutine %d: %v", g, err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if calls.Load() != 1 {
		t.Errorf("issueToken called %d times, want exactly 1 (sync.Once per triplet)", calls.Load())
	}
}

func TestTokenCacheShutdownStopsWorker(t *testing.T) {
	defer goleak.VerifyNone(t)

	cache := NewTokenCache(TokenCacheConfig{
		IssueToken: func(ctx context.Context, teamID, userID, apiKeyUID string) (string, time.Time, error) {
			return "token", time.Now().Add(15 * time.Minute), nil
		},
	})

	cache.StartRefreshWorker(context.Background())
	time.Sleep(100 * time.Millisecond)
	cache.Shutdown()
	// goleak.VerifyNone will confirm no leaked goroutines
}

func TestTokenCacheRevoke(t *testing.T) {
	defer goleak.VerifyNone(t)

	cache := NewTokenCache(TokenCacheConfig{
		IssueToken: func(ctx context.Context, teamID, userID, apiKeyUID string) (string, time.Time, error) {
			return "token-" + teamID, time.Now().Add(15 * time.Minute), nil
		},
	})

	cache.GetOrIssue(context.Background(), "team-T", "alice", "uid-1")
	if cache.Size() != 1 {
		t.Errorf("cache size = %d, want 1", cache.Size())
	}

	cache.Revoke("team-T", "alice", "uid-1")
	if cache.Size() != 0 {
		t.Errorf("cache size after revoke = %d, want 0", cache.Size())
	}
}

func TestTokenEntryIsExpired(t *testing.T) {
	expired := &TokenEntry{ExpiresAt: time.Now().Add(-1 * time.Second)}
	if !expired.IsExpired() {
		t.Error("expired entry should report as expired")
	}
	valid := &TokenEntry{ExpiresAt: time.Now().Add(1 * time.Hour)}
	if valid.IsExpired() {
		t.Error("valid entry should not report as expired")
	}
}

func TestTokenEntryNeedsRefresh(t *testing.T) {
	entry := &TokenEntry{ExpiresAt: time.Now().Add(30 * time.Second)}
	if !entry.NeedsRefresh(60 * time.Second) {
		t.Error("entry expiring within 60s should need refresh")
	}
	if entry.NeedsRefresh(10 * time.Second) {
		t.Error("entry expiring in 30s should not need refresh with 10s margin")
	}
}

func TestTokenCacheRefreshExpiringDirectly(t *testing.T) {
	defer goleak.VerifyNone(t)

	var issues atomic.Int32
	cache := NewTokenCache(TokenCacheConfig{
		RefreshMargin: 2 * time.Second,
		IssueToken: func(ctx context.Context, teamID, userID, apiKeyUID string) (string, time.Time, error) {
			n := issues.Add(1)
			return fmt.Sprintf("token-v%d", n), time.Now().Add(15 * time.Minute), nil
		},
	})

	// Insert a token that expires in 1 second (within refresh margin of 2s).
	cache.mu.Lock()
	cache.entries[cacheKey{TeamID: "team-T", UserID: "alice", KeyUID: "uid-1"}] = &TokenEntry{
		Token:     "token-v0",
		ExpiresAt: time.Now().Add(1 * time.Second),
		KeyUID:    "uid-1",
	}
	cache.mu.Unlock()

	// Call refreshExpiring directly (bypassing the ticker).
	cache.refreshExpiring(context.Background())

	cache.mu.RLock()
	entry := cache.entries[cacheKey{TeamID: "team-T", UserID: "alice", KeyUID: "uid-1"}]
	cache.mu.RUnlock()

	if entry == nil {
		t.Fatal("entry should exist after refresh")
	}
	if entry.Token == "token-v0" {
		t.Error("token should have been refreshed from v0")
	}
	if issues.Load() != 1 {
		t.Errorf("expected 1 issuance during refresh, got %d", issues.Load())
	}
}
