package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// RedisClient is a minimal interface for token revocation operations.
// Production implementations wrap go-redis or similar; tests use RedisMock.
type RedisClient interface {
	// SAdd adds a member to a Redis set. Returns error on connection failure.
	SAdd(ctx context.Context, key string, members ...interface{}) error
	// Expire sets a TTL on a key. Returns error on connection failure.
	Expire(ctx context.Context, key string, ttl time.Duration) error
	// Exists checks if a key exists. Returns (count, error).
	Exists(ctx context.Context, key string) (int64, error)
}

// RevocationChecker performs token revocation checks via Redis.
type RevocationChecker struct {
	client      RedisClient
	enabled     bool
	failureMode RevocationFailureMode
	logger      *slog.Logger
	metrics     *AuthMetrics
}

// NewRevocationChecker creates a new revocation checker.
func NewRevocationChecker(client RedisClient, enabled bool, failureMode RevocationFailureMode, logger *slog.Logger, metrics *AuthMetrics) *RevocationChecker {
	return &RevocationChecker{
		client:      client,
		enabled:     enabled,
		failureMode: failureMode,
		logger:      logger,
		metrics:     metrics,
	}
}

// CheckRevoked checks if the token identified by jti has been revoked.
// Returns true if the token is revoked, false otherwise.
// REQ-AUTH1-010: fail-open (default) skips check on Redis failure;
// fail-closed returns error on Redis failure.
func (rc *RevocationChecker) CheckRevoked(ctx context.Context, jti string) (bool, error) {
	if !rc.enabled || jti == "" {
		return false, nil
	}

	key := fmt.Sprintf("auth:revoked:%s", jti)
	count, err := rc.client.Exists(ctx, key)
	if err != nil {
		if rc.failureMode == RevocationFailClosed {
			rc.metrics.Failures.WithLabelValues(string(ReasonRevocationUnavailable)).Inc()
			return false, fmt.Errorf("%w: revocation check failed", ErrRevocationUnavailable)
		}
		// fail-open: log warning and allow request
		if rc.logger != nil {
			rc.logger.Warn("revocation check unavailable, fail-open",
				"jti", jti,
				"error", err,
			)
		}
		return false, nil
	}

	return count > 0, nil
}

// RevokeToken adds a token's jti to the revocation set in Redis.
// REQ-AUTH1-009: SADD + EXPIRE with remaining TTL.
func (rc *RevocationChecker) RevokeToken(ctx context.Context, jti string, exp time.Time) error {
	if !rc.enabled || jti == "" {
		return nil
	}

	key := fmt.Sprintf("auth:revoked:%s", jti)

	if err := rc.client.SAdd(ctx, key, 1); err != nil {
		return fmt.Errorf("revocation SADD failed: %w", err)
	}

	ttl := time.Until(exp)
	if ttl > 0 {
		if err := rc.client.Expire(ctx, key, ttl); err != nil {
			return fmt.Errorf("revocation EXPIRE failed: %w", err)
		}
	}

	return nil
}
