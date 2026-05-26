package costguard

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/cap_check.lua
var capCheckLuaScript string

// RedisClient abstracts the Redis operations needed by cap-check.
// This allows testing with miniredis or mocks.
type RedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

// CapChecker performs atomic cap evaluation using a Redis Lua script.
// REQ-DEEP4-009: single Redis call covers eval + increment + TTL refresh.
type CapChecker struct {
	redis     RedisClient
	cfg       Config
	ttl       time.Duration
	once      sync.Once
	scriptSHA string
}

// NewCapChecker creates a new CapChecker with the given Redis client and config.
func NewCapChecker(redis RedisClient, cfg Config) *CapChecker {
	return &CapChecker{
		redis: redis,
		cfg:   cfg,
		ttl:   24 * time.Hour,
	}
}

// EvaluateAtomic performs an atomic cap-check via Redis Lua script.
// @MX:ANCHOR: [AUTO] Atomic cap evaluation; callers: middleware, tests, integration
// @MX:REASON: fan_in >= 3; every /deep request flows through this function
//
// Returns CapResult with allowed status and remaining counts.
func (c *CapChecker) EvaluateAtomic(ctx context.Context, tenantID, userID string, costUSD float64) (CapResult, error) {
	keys := c.buildKeys(tenantID, userID)
	args := c.buildArgs(costUSD)

	val, err := c.redis.Eval(ctx, capCheckLuaScript, keys, args...).Result()
	if err != nil {
		return CapResult{}, fmt.Errorf("cap check lua eval: %w", err)
	}

	return c.parseResult(val)
}

// buildKeys constructs the Redis keys for the Lua script.
func (c *CapChecker) buildKeys(tenantID, userID string) []string {
	userCapKeys := ""
	userCapUSD := ""
	if c.cfg.User.Enabled && userID != "" && userID != "anonymous" {
		userCapKeys = fmt.Sprintf("costguard:calls:user:%s", userID)
		userCapUSD = fmt.Sprintf("costguard:window:user:%s", userID)
	}
	return []string{
		fmt.Sprintf("costguard:calls:tenant:%s", tenantID),
		fmt.Sprintf("costguard:window:tenant:%s", tenantID),
		userCapKeys,
		userCapUSD,
	}
}

// buildArgs constructs the arguments for the Lua script.
func (c *CapChecker) buildArgs(costUSD float64) []interface{} {
	userMaxCalls := 0
	userMaxUSD := 0.0
	if c.cfg.User.Enabled {
		userMaxCalls = c.cfg.User.MaxCallsPerDay
		userMaxUSD = c.cfg.User.MaxUSDPerDay
	}
	return []interface{}{
		c.cfg.Tenant.MaxCallsPerDay,
		c.cfg.Tenant.MaxUSDPerDay,
		userMaxCalls,
		userMaxUSD,
		costUSD,
		int(c.ttl.Seconds()),
	}
}

// parseResult converts the Lua script return value into a CapResult.
func (c *CapChecker) parseResult(val interface{}) (CapResult, error) {
	arr, ok := val.([]interface{})
	if !ok || len(arr) < 6 {
		return CapResult{}, fmt.Errorf("unexpected lua result type: %T", val)
	}

	allowed := toInt(arr[0]) == 1
	dimCode := toInt(arr[1])
	remainingCalls := toInt(arr[4])
	remainingUSD := toFloat(arr[5])

	var exceeded CapDimension
	switch dimCode {
	case 1:
		exceeded = DimensionCalls
	case 2:
		exceeded = DimensionUSD
	default:
		exceeded = DimensionNone
	}

	return CapResult{
		Allowed:        allowed,
		Exceeded:       exceeded,
		RemainingCalls: remainingCalls,
		RemainingUSD:   remainingUSD,
	}, nil
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case string:
		f, _ := parseFloat(string(n))
		return f
	default:
		return 0
	}
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
