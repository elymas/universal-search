package costguard

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupMiniredis creates an in-memory Redis for testing.
func setupMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})
	return mr, client
}

// adaptRedisClient wraps a *redis.Client to implement the RedisClient interface.
type adaptRedisClient struct {
	*redis.Client
}

func (a adaptRedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	return a.Client.Eval(ctx, script, keys, args...)
}

// ---------------------------------------------------------------------------
// Phase B: Redis Cap-Check Primitive + Lua Script
// ---------------------------------------------------------------------------

// TestCapCheckTenantCalls verifies that when tenant calls reach the daily max,
// the cap-check returns exceeded.
// REQ-DEEP4-009: tenant call-count cap enforcement.
func TestCapCheckTenantCalls(t *testing.T) {
	t.Parallel()

	_, client := setupMiniredis(t)
	cfg := DefaultConfig()
	cfg.Tenant.MaxCallsPerDay = 20
	cfg.Tenant.MaxUSDPerDay = 100.00

	checker := NewCapChecker(adaptRedisClient{client}, cfg)
	ctx := context.Background()

	// First 20 calls should pass.
	for i := 0; i < 20; i++ {
		result, err := checker.EvaluateAtomic(ctx, "default", "anonymous", 0.01)
		if err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("call %d: expected allowed, got exceeded=%s", i+1, result.Exceeded)
		}
	}

	// 21st call should be rejected.
	result, err := checker.EvaluateAtomic(ctx, "default", "anonymous", 0.01)
	if err != nil {
		t.Fatalf("call 21: %v", err)
	}
	if result.Allowed {
		t.Error("call 21: expected cap exceeded (calls), got allowed")
	}
	if result.Exceeded != DimensionCalls {
		t.Errorf("call 21: expected dimension=calls, got %s", result.Exceeded)
	}
}

// TestCapCheckTenantUSD verifies that when tenant USD cost reaches the daily max,
// the cap-check returns exceeded.
// REQ-DEEP4-009: tenant USD cap enforcement.
func TestCapCheckTenantUSD(t *testing.T) {
	t.Parallel()

	_, client := setupMiniredis(t)
	cfg := DefaultConfig()
	cfg.Tenant.MaxCallsPerDay = 1000
	cfg.Tenant.MaxUSDPerDay = 5.00

	checker := NewCapChecker(adaptRedisClient{client}, cfg)
	ctx := context.Background()

	// Accumulate $4.95 across 5 calls ($0.99 each).
	for i := 0; i < 5; i++ {
		result, err := checker.EvaluateAtomic(ctx, "default", "anonymous", 0.99)
		if err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("call %d: expected allowed, got exceeded=%s", i+1, result.Exceeded)
		}
	}

	// Next call costing $0.10 should push past $5.00 ($4.95 + $0.10 = $5.05) and be rejected.
	result, err := checker.EvaluateAtomic(ctx, "default", "anonymous", 0.10)
	if err != nil {
		t.Fatalf("over-budget call: %v", err)
	}
	if result.Allowed {
		t.Error("expected cap exceeded (usd), got allowed")
	}
	if result.Exceeded != DimensionUSD {
		t.Errorf("expected dimension=usd, got %s", result.Exceeded)
	}
}

// TestCapCheckUserCapWhenEnabled verifies that per-user cap enforcement only
// activates when cfg.User.Enabled is true.
// REQ-DEEP4-009: user-level cap enforcement.
func TestCapCheckUserCapWhenEnabled(t *testing.T) {
	t.Parallel()

	_, client := setupMiniredis(t)
	ctx := context.Background()

	// When User.Enabled = false (default), user cap should not be checked.
	cfgDisabled := DefaultConfig()
	cfgDisabled.User.Enabled = false
	cfgDisabled.Tenant.MaxCallsPerDay = 1000
	cfgDisabled.User.MaxCallsPerDay = 5

	checkerDisabled := NewCapChecker(adaptRedisClient{client}, cfgDisabled)
	for i := 0; i < 10; i++ {
		result, err := checkerDisabled.EvaluateAtomic(ctx, "default", "alice", 0.01)
		if err != nil {
			t.Fatalf("disabled user cap call %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Errorf("disabled user cap call %d: expected allowed since user cap disabled", i+1)
		}
	}

	// Reset Redis state.
	client.FlushAll(ctx).Err()

	// When User.Enabled = true, user cap should enforce.
	cfgEnabled := DefaultConfig()
	cfgEnabled.User.Enabled = true
	cfgEnabled.Tenant.MaxCallsPerDay = 1000
	cfgEnabled.User.MaxCallsPerDay = 5

	checkerEnabled := NewCapChecker(adaptRedisClient{client}, cfgEnabled)
	for i := 0; i < 5; i++ {
		result, err := checkerEnabled.EvaluateAtomic(ctx, "default", "alice", 0.01)
		if err != nil {
			t.Fatalf("enabled user cap call %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("enabled user cap call %d: expected allowed", i+1)
		}
	}

	// 6th call should be rejected by user cap.
	result, err := checkerEnabled.EvaluateAtomic(ctx, "default", "alice", 0.01)
	if err != nil {
		t.Fatalf("user cap exceeded call: %v", err)
	}
	if result.Allowed {
		t.Error("expected user cap exceeded, got allowed")
	}
}

// TestCapCheckLuaScriptAtomic verifies that the Lua script performs evaluation
// + counter increment + TTL refresh in a single atomic call.
// REQ-DEEP4-009, NFR-DEEP4-004.
func TestCapCheckLuaScriptAtomic(t *testing.T) {
	t.Parallel()

	mr, client := setupMiniredis(t)
	cfg := DefaultConfig()
	cfg.Tenant.MaxCallsPerDay = 20
	cfg.Tenant.MaxUSDPerDay = 100.00

	checker := NewCapChecker(adaptRedisClient{client}, cfg)
	ctx := context.Background()

	result, err := checker.EvaluateAtomic(ctx, "default", "anonymous", 0.05)
	if err != nil {
		t.Fatalf("EvaluateAtomic: %v", err)
	}
	if !result.Allowed {
		t.Error("first call should be allowed")
	}

	// Verify Redis state was mutated atomically.
	callsKey := "costguard:calls:tenant:default"
	usdKey := "costguard:window:tenant:default"

	calls, err := client.Get(ctx, callsKey).Int()
	if err != nil {
		t.Fatalf("get calls: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls counter: got %d, want 1", calls)
	}

	usd, err := client.Get(ctx, usdKey).Float64()
	if err != nil {
		t.Fatalf("get usd: %v", err)
	}
	// Redis INCRBYFLOAT may have minor floating-point variance.
	if usd < 0.04 || usd > 0.06 {
		t.Errorf("usd counter: got %f, want ~0.05", usd)
	}

	// Verify TTL was set.
	ttl := client.TTL(ctx, callsKey).Val()
	if ttl <= 0 {
		t.Error("expected TTL to be set on calls key")
	}
	_ = mr // keep reference
}

// TestCapCheckConcurrent100RequestsNoRace verifies that 100 concurrent
// requests against a cap of 20 result in exactly 20 passes and 80 rejections.
// NFR-DEEP4-004: atomic cap-check with no race conditions.
func TestCapCheckConcurrent100RequestsNoRace(t *testing.T) {
	// Not t.Parallel() — uses shared miniredis state.

	_, client := setupMiniredis(t)
	cfg := DefaultConfig()
	cfg.Tenant.MaxCallsPerDay = 20
	cfg.Tenant.MaxUSDPerDay = 10000.00

	checker := NewCapChecker(adaptRedisClient{client}, cfg)
	ctx := context.Background()

	var allowed, rejected atomic.Int64

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := checker.EvaluateAtomic(ctx, "default", "anonymous", 0.01)
			if err != nil {
				t.Errorf("concurrent call: %v", err)
				return
			}
			if result.Allowed {
				allowed.Add(1)
			} else {
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()

	a := allowed.Load()
	r := rejected.Load()

	if a != 20 {
		t.Errorf("allowed: got %d, want exactly 20", a)
	}
	if r != 80 {
		t.Errorf("rejected: got %d, want exactly 80", r)
	}
}
