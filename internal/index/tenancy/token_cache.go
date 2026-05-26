package tenancy

import (
	"context"
	"sync"
	"time"
)

// TokenEntry represents a cached tenant token with its expiry time.
type TokenEntry struct {
	Token     string
	ExpiresAt time.Time
	KeyUID    string
}

// IsExpired returns true if the token has expired.
func (e *TokenEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// NeedsRefresh returns true if the token will expire within the given duration.
func (e *TokenEntry) NeedsRefresh(refreshMargin time.Duration) bool {
	return time.Now().Add(refreshMargin).After(e.ExpiresAt)
}

// TokenCache provides in-process caching of Meili tenant tokens.
// REQ-IDX4-009: sync.Map + per-entry expires_at + refresh worker + sync.Once per triplet.
type TokenCache struct {
	mu      sync.RWMutex
	entries map[cacheKey]*TokenEntry
	// onceMap ensures single issuance per (team, user, keyUID) triplet.
	onceMap sync.Map // cacheKey -> *sync.Once
	// refreshMargin is how long before expiry the refresh worker triggers.
	refreshMargin time.Duration
	// issueToken is the function that actually issues a token.
	issueToken func(ctx context.Context, teamID, userID, apiKeyUID string) (token string, expiresAt time.Time, err error)
	// cancel is the context cancel function for the refresh worker.
	cancel context.CancelFunc
}

type cacheKey struct {
	TeamID string
	UserID string
	KeyUID string
}

// TokenCacheConfig configures the token cache.
type TokenCacheConfig struct {
	// RefreshMargin is how long before token expiry to trigger refresh.
	// Default: 60 seconds.
	RefreshMargin time.Duration
	// IssueToken is the function that issues tenant tokens.
	IssueToken func(ctx context.Context, teamID, userID, apiKeyUID string) (token string, expiresAt time.Time, err error)
}

// NewTokenCache creates a new token cache.
func NewTokenCache(cfg TokenCacheConfig) *TokenCache {
	margin := cfg.RefreshMargin
	if margin == 0 {
		margin = 60 * time.Second
	}
	return &TokenCache{
		entries:       make(map[cacheKey]*TokenEntry),
		refreshMargin: margin,
		issueToken:    cfg.IssueToken,
	}
}

// GetOrIssue returns a cached token or issues a new one using sync.Once per triplet.
// REQ-IDX4-009: same (team, user, keyUID) → single issuance under concurrency.
func (tc *TokenCache) GetOrIssue(ctx context.Context, teamID, userID, apiKeyUID string) (*TokenEntry, error) {
	key := cacheKey{TeamID: teamID, UserID: userID, KeyUID: apiKeyUID}

	// Fast path: check cache with read lock.
	tc.mu.RLock()
	if entry, ok := tc.entries[key]; ok && !entry.IsExpired() {
		tc.mu.RUnlock()
		return entry, nil
	}
	tc.mu.RUnlock()

	// Ensure single issuance via sync.Once.
	onceVal, _ := tc.onceMap.LoadOrStore(key, &sync.Once{})
	once := onceVal.(*sync.Once)

	var result *TokenEntry
	var err error

	once.Do(func() {
		// Double-check after acquiring once.
		tc.mu.RLock()
		if entry, ok := tc.entries[key]; ok && !entry.IsExpired() {
			tc.mu.RUnlock()
			result = entry
			return
		}
		tc.mu.RUnlock()

		token, expiresAt, issueErr := tc.issueToken(ctx, teamID, userID, apiKeyUID)
		if issueErr != nil {
			err = issueErr
			return
		}

		entry := &TokenEntry{
			Token:     token,
			ExpiresAt: expiresAt,
			KeyUID:    apiKeyUID,
		}

		tc.mu.Lock()
		tc.entries[key] = entry
		tc.mu.Unlock()

		result = entry
	})

	if err != nil {
		// Reset once so next call retries.
		tc.onceMap.Delete(key)
		return nil, err
	}

	if result == nil {
		// once.Do already ran by another goroutine; read from cache.
		tc.mu.RLock()
		result = tc.entries[key]
		tc.mu.RUnlock()
	}

	return result, nil
}

// StartRefreshWorker starts a background goroutine that refreshes tokens before expiry.
// REQ-IDX4-009: refresh 60s before expiry.
func (tc *TokenCache) StartRefreshWorker(ctx context.Context) {
	workerCtx, cancel := context.WithCancel(ctx)
	tc.cancel = cancel

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				tc.refreshExpiring(workerCtx)
			}
		}
	}()
}

// refreshExpiring refreshes tokens that will expire within the refresh margin.
func (tc *TokenCache) refreshExpiring(ctx context.Context) {
	tc.mu.RLock()
	keys := make([]cacheKey, 0, len(tc.entries))
	for key, entry := range tc.entries {
		if entry.NeedsRefresh(tc.refreshMargin) {
			keys = append(keys, key)
		}
	}
	tc.mu.RUnlock()

	for _, key := range keys {
		// Reset the once so a new issuance can occur.
		tc.onceMap.Delete(key)

		// Issue fresh token.
		token, expiresAt, err := tc.issueToken(ctx, key.TeamID, key.UserID, key.KeyUID)
		if err != nil {
			continue
		}

		entry := &TokenEntry{
			Token:     token,
			ExpiresAt: expiresAt,
			KeyUID:    key.KeyUID,
		}

		tc.mu.Lock()
		tc.entries[key] = entry
		tc.mu.Unlock()
	}
}

// Shutdown stops the refresh worker goroutine.
func (tc *TokenCache) Shutdown() {
	if tc.cancel != nil {
		tc.cancel()
	}
}

// Revoke removes a cached token entry (hook point for AUTH-002 team.member.removed).
func (tc *TokenCache) Revoke(teamID, userID, apiKeyUID string) {
	key := cacheKey{TeamID: teamID, UserID: userID, KeyUID: apiKeyUID}
	tc.mu.Lock()
	delete(tc.entries, key)
	tc.mu.Unlock()
	tc.onceMap.Delete(key)
}

// Size returns the number of cached tokens (for testing).
func (tc *TokenCache) Size() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.entries)
}
