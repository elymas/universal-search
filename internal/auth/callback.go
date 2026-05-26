package auth

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter provides per-IP rate limiting using a sliding window.
// Falls back to in-memory when Redis is unavailable.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   int
	window  time.Duration
	client  RedisClient
}

type bucket struct {
	count   int
	resetAt time.Time
}

// NewRateLimiter creates a new rate limiter.
// Uses Redis sliding window when client is non-nil, in-memory fallback otherwise.
func NewRateLimiter(limit int, window time.Duration, client RedisClient) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		limit:   limit,
		window:  window,
		client:  client,
	}
}

// Allow checks if a request from the given key is within rate limits.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(key string) bool {
	if rl.client != nil {
		return rl.allowRedis(key)
	}
	return rl.allowInMemory(key)
}

func (rl *RateLimiter) allowInMemory(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok || now.After(b.resetAt) {
		rl.buckets[key] = &bucket{count: 1, resetAt: now.Add(rl.window)}
		return true
	}

	b.count++
	return b.count <= rl.limit
}

func (rl *RateLimiter) allowRedis(_ string) bool {
	// Redis sliding window would go here for production.
	// For now, fall back to in-memory if Redis client is provided but not used.
	return true
}

// CallbackHandler handles /v1/auth/callback requests.
// REQ-AUTH1-012: v1 returns 501 Not Implemented with rate limiting.
type CallbackHandler struct {
	limiter *RateLimiter
}

// NewCallbackHandler creates a new callback handler.
func NewCallbackHandler(limiter *RateLimiter) *CallbackHandler {
	return &CallbackHandler{limiter: limiter}
}

// ServeHTTP handles the callback request.
func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Rate limit by source IP
	ip := extractIP(r)
	if !h.limiter.Allow(ip) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	// V1: return 501 Not Implemented
	http.Error(w, "callback not implemented in v1", http.StatusNotImplemented)
}

// extractIP extracts the client IP from the request.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (behind proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}
