package idx5

import (
	"testing"
	"time"
)

func TestStalenessFresh(t *testing.T) {
	// REQ-IDX5-003: age < 0.5 * TTL → fresh
	// category="web", TTL=3600s, soft boundary at 1800s
	ca := &CachedAnswer{
		TTLSeconds: 3600,
		CreatedAt:  time.Now().Add(-500 * time.Second), // 500s ago, well under 1800s
		ForceStale: false,
	}
	s := EvaluateStaleness(ca, time.Now(), defaultCategoryTTLs)
	if s != Fresh {
		t.Errorf("EvaluateStaleness(fresh) = %v, want Fresh", s)
	}
}

func TestStalenessSoftStale(t *testing.T) {
	// REQ-IDX5-003: 0.5 * TTL <= age < TTL → soft-stale
	ca := &CachedAnswer{
		TTLSeconds: 3600,
		CreatedAt:  time.Now().Add(-3000 * time.Second), // 3000s ago, 1800 <= 3000 < 3600
		ForceStale: false,
	}
	s := EvaluateStaleness(ca, time.Now(), defaultCategoryTTLs)
	if s != SoftStale {
		t.Errorf("EvaluateStaleness(soft_stale) = %v, want SoftStale", s)
	}
}

func TestStalenessHardStale(t *testing.T) {
	// REQ-IDX5-003: age >= TTL → hard-stale
	ca := &CachedAnswer{
		TTLSeconds: 3600,
		CreatedAt:  time.Now().Add(-5400 * time.Second), // 5400s ago, > 3600
		ForceStale: false,
	}
	s := EvaluateStaleness(ca, time.Now(), defaultCategoryTTLs)
	if s != HardStale {
		t.Errorf("EvaluateStaleness(hard_stale) = %v, want HardStale", s)
	}
}

func TestStalenessForceStaleOverride(t *testing.T) {
	// REQ-IDX5-005: force_stale=true → always hard-stale regardless of age
	ca := &CachedAnswer{
		TTLSeconds: 3600,
		CreatedAt:  time.Now().Add(-100 * time.Second), // very fresh
		ForceStale: true,
	}
	s := EvaluateStaleness(ca, time.Now(), defaultCategoryTTLs)
	if s != HardStale {
		t.Errorf("EvaluateStaleness(force_stale) = %v, want HardStale", s)
	}
}

func TestStalenessTTLBoundaryExactly(t *testing.T) {
	// Edge2: age = exactly TTL → hard-stale (>= comparison)
	now := time.Now()
	ttl := 3600
	ca := &CachedAnswer{
		TTLSeconds: ttl,
		CreatedAt:  now.Add(-time.Duration(ttl) * time.Second), // exactly at TTL
		ForceStale: false,
	}
	s := EvaluateStaleness(ca, now, defaultCategoryTTLs)
	if s != HardStale {
		t.Errorf("EvaluateStaleness(exactly TTL) = %v, want HardStale", s)
	}
}

func TestStalenessJustBelowTTLBoundary(t *testing.T) {
	// Edge2: age = TTL - 1ms → soft-stale (still < TTL)
	now := time.Now()
	ttl := 3600
	ca := &CachedAnswer{
		TTLSeconds: ttl,
		CreatedAt:  now.Add(-time.Duration(ttl-1) * time.Second), // 3599s ago
		ForceStale: false,
	}
	s := EvaluateStaleness(ca, now, defaultCategoryTTLs)
	if s != SoftStale {
		t.Errorf("EvaluateStaleness(3599s of 3600s TTL) = %v, want SoftStale", s)
	}
}

func TestStalenessPerCategoryTTL(t *testing.T) {
	// REQ-IDX5-003 D2: per-category TTL (web=1h, social=30m, academic=30d, korean=1h, mixed/unknown=2h)
	now := time.Now()

	cases := []struct {
		category  string
		age       time.Duration
		want      Staleness
	}{
		{"web", 20 * time.Minute, Fresh},
		{"web", 40 * time.Minute, SoftStale},
		{"web", 70 * time.Minute, HardStale},
		{"social", 10 * time.Minute, Fresh},
		{"social", 20 * time.Minute, SoftStale},
		{"social", 40 * time.Minute, HardStale},
		{"academic", 10 * 24 * time.Hour, Fresh},      // 10 days, TTL=30d
		{"academic", 20 * 24 * time.Hour, SoftStale},   // 20 days, 15d <= 20d < 30d
		{"korean", 20 * time.Minute, Fresh},
		{"unknown", 30 * time.Minute, Fresh},            // TTL=2h, 30m < 1h
		{"unknown", 90 * time.Minute, SoftStale},        // 1h <= 90m < 2h
		{"unknown", 150 * time.Minute, HardStale},       // 150m >= 2h
	}

	for _, tc := range cases {
		ca := &CachedAnswer{
			Category:   tc.category,
			CreatedAt:  now.Add(-tc.age),
			TTLSeconds: 0, // should be overridden by category TTL
			ForceStale: false,
		}
		s := EvaluateStaleness(ca, now, defaultCategoryTTLs)
		if s != tc.want {
			t.Errorf("category=%q age=%v: got %v, want %v", tc.category, tc.age, s, tc.want)
		}
	}
}
