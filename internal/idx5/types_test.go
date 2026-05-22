package idx5

import (
	"testing"
	"time"
)

func TestDocTypeCachedAnswerEnumValue(t *testing.T) {
	// REQ-IDX5-006: DocTypeCachedAnswer = "cached_answer" must be a valid DocType value
	want := "cached_answer"
	if string(DocTypeCachedAnswer) != want {
		t.Errorf("DocTypeCachedAnswer = %q, want %q", string(DocTypeCachedAnswer), want)
	}
}

func TestStalenessEnumValues(t *testing.T) {
	// REQ-IDX5-003: Fresh, SoftStale, HardStale enum values
	cases := []struct {
		s    Staleness
		want string
	}{
		{Fresh, "fresh"},
		{SoftStale, "soft_stale"},
		{HardStale, "hard_stale"},
	}
	for _, tc := range cases {
		if string(tc.s) != tc.want {
			t.Errorf("Staleness(%v) = %q, want %q", tc.s, string(tc.s), tc.want)
		}
	}
}

func TestCachedAnswerStruct(t *testing.T) {
	// REQ-IDX5-006: CachedAnswer struct must have all required fields
	now := time.Now()
	ca := CachedAnswer{
		DocID:        "answer-cache:abc123:team-T",
		TeamID:       "team-T",
		QueryHash:    "abc123",
		QueryText:    "test query",
		Category:     "web",
		ResponseJSON: `{"text":"hello"}`,
		Similarity:   0.94,
		TTLSeconds:   3600,
		CreatedAt:    now,
		LastServedAt: now,
		HitCount:     5,
		ForceStale:   false,
	}
	if ca.DocID == "" {
		t.Error("CachedAnswer.DocID should not be empty")
	}
	if ca.TeamID != "team-T" {
		t.Errorf("TeamID = %q, want %q", ca.TeamID, "team-T")
	}
	if ca.TTLSeconds != 3600 {
		t.Errorf("TTLSeconds = %d, want 3600", ca.TTLSeconds)
	}
}

func TestLookupResultOutcomes(t *testing.T) {
	// REQ-IDX5-002: LookupOutcome enum values
	cases := []struct {
		o    LookupOutcome
		want string
	}{
		{OutcomeHit, "hit"},
		{OutcomeSoftHit, "soft_hit"},
		{OutcomeMiss, "miss"},
		{OutcomeHardStale, "hard_stale"},
		{OutcomeBypassed, "bypassed"},
	}
	for _, tc := range cases {
		if string(tc.o) != tc.want {
			t.Errorf("LookupOutcome(%v) = %q, want %q", tc.o, string(tc.o), tc.want)
		}
	}
}

func TestLookupResultStruct(t *testing.T) {
	// REQ-IDX5-002: LookupResult carries outcome, cached answer, and score
	now := time.Now()
	lr := LookupResult{
		Outcome:  OutcomeHit,
		Cached:   nil,
		Score:    0.94,
		Duration: 120 * time.Millisecond,
	}
	if lr.Outcome != OutcomeHit {
		t.Errorf("Outcome = %v, want %v", lr.Outcome, OutcomeHit)
	}

	// With a cached answer
	ca := &CachedAnswer{
		DocID:      "answer-cache:abc123:team-T",
		TeamID:     "team-T",
		QueryHash:  "abc123",
		Category:   "web",
		CreatedAt:  now,
		TTLSeconds: 3600,
	}
	lr2 := LookupResult{
		Outcome: OutcomeSoftHit,
		Cached:  ca,
		Score:   0.93,
	}
	if lr2.Cached == nil {
		t.Error("LookupResult.Cached should not be nil when set")
	}
	if lr2.Cached.TeamID != "team-T" {
		t.Errorf("Cached.TeamID = %q, want %q", lr2.Cached.TeamID, "team-T")
	}
}
