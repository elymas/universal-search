package tenancy

import (
	"strings"
	"testing"

	pb "github.com/qdrant/go-client/qdrant"
)

// Test 27: TenancyFilter adds team_id must condition.
func TestQdrantFilterAddsTeamIDCondition(t *testing.T) {
	t.Parallel()
	f, err := TenancyFilter("team-T", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	if len(f.Must) != 1 {
		t.Fatalf("expected 1 must condition, got %d", len(f.Must))
	}
	if len(f.Should) != 0 {
		t.Fatalf("expected 0 should conditions, got %d", len(f.Should))
	}
}

// Test 28: TenancyFilter adds user_id should for private visibility.
func TestQdrantFilterAddsUserIDForPrivateVisibility(t *testing.T) {
	t.Parallel()
	f, err := TenancyFilter("team-T", "alice@example.com", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// must: team_id + nested user_id filter
	if len(f.Must) != 2 {
		t.Fatalf("expected 2 must conditions, got %d", len(f.Must))
	}
	// Second must should be a nested filter with user_id should
	nested := f.Must[1].GetFilter()
	if nested == nil {
		t.Fatal("expected nested Filter condition for user_id")
	}
	if len(nested.Should) != 2 {
		t.Fatalf("expected 2 should conditions in nested user_id filter, got %d", len(nested.Should))
	}
}

// Test 29: TenancyFilter adds __public__ should when IncludePublic.
func TestQdrantFilterAddsPublicOnIncludeFlag(t *testing.T) {
	t.Parallel()
	f, err := TenancyFilter("team-T", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// should: [team_id == team-T, team_id == __public__]
	if len(f.Should) != 2 {
		t.Fatalf("expected 2 should conditions, got %d", len(f.Should))
	}
}

// Test 30: TenancyFilter rejects __public__ as team_id input.
func TestQdrantFilterRejectsPublicSentinelAsUserInput(t *testing.T) {
	t.Parallel()
	_, err := TenancyFilter("__public__", "", false)
	if err == nil {
		t.Error("expected error for __public__ team_id input")
	}
}

// Test 31: TenancyFilter returns nil for empty team_id.
func TestQdrantFilterReturnsNilForEmptyTeamID(t *testing.T) {
	t.Parallel()
	f, err := TenancyFilter("", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != nil {
		t.Errorf("expected nil filter for empty team_id, got %v", f)
	}
}

// Test: combined filter with team + user + public
func TestQdrantFilterCombinedTeamUserPublic(t *testing.T) {
	t.Parallel()
	f, err := TenancyFilter("team-T", "alice@example.com", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Should) != 2 {
		t.Fatalf("expected 2 should conditions (public), got %d", len(f.Should))
	}
	if len(f.Must) != 1 {
		t.Fatalf("expected 1 must condition (user_id nested filter), got %d", len(f.Must))
	}
}

// --- Meili filter tests ---

func TestMeiliFilterAddsTeamID(t *testing.T) {
	t.Parallel()
	f, err := BuildMeiliTenancyFilter("team-T", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != `team_id = "team-T"` {
		t.Errorf("got %q, want team_id filter", f)
	}
}

func TestMeiliFilterWithPublic(t *testing.T) {
	t.Parallel()
	f, err := BuildMeiliTenancyFilter("team-T", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(f, "__public__") {
		t.Errorf("expected __public__ in filter, got %q", f)
	}
	if !strings.Contains(f, "team-T") {
		t.Errorf("expected team-T in filter, got %q", f)
	}
}

func TestMeiliFilterWithUserPrivate(t *testing.T) {
	t.Parallel()
	f, err := BuildMeiliTenancyFilter("team-T", "alice@example.com", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(f, `user_id = "alice@example.com"`) {
		t.Errorf("expected user_id in filter, got %q", f)
	}
	if !strings.Contains(f, `user_id = ""`) {
		t.Errorf("expected empty user_id fallback in filter, got %q", f)
	}
}

func TestMeiliFilterRejectsPublicSentinel(t *testing.T) {
	t.Parallel()
	_, err := BuildMeiliTenancyFilter("__public__", "", false)
	if err == nil {
		t.Error("expected error for __public__ team_id input")
	}
}

func TestMeiliFilterReturnsEmptyForEmptyTeamID(t *testing.T) {
	t.Parallel()
	f, err := BuildMeiliTenancyFilter("", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != "" {
		t.Errorf("expected empty filter, got %q", f)
	}
}

// Test: Qdrant should conditions use public sentinel correctly
func TestQdrantFilterPublicShouldConditions(t *testing.T) {
	t.Parallel()
	f, err := TenancyFilter("team-T", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify one of the should conditions has __public__
	found := false
	for _, c := range f.Should {
		fc := c.GetField()
		if fc != nil {
			kw := fc.Match.GetKeyword()
			if kw == "__public__" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected __public__ in should conditions")
	}
}

// Test that the TenancyFilter function produces correct pb.Filter structure
func TestQdrantFilterMustContainsTeamIDMatch(t *testing.T) {
	t.Parallel()
	f, err := TenancyFilter("team-T", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the must condition contains team_id = team-T
	fc := f.Must[0].GetField()
	if fc == nil {
		t.Fatal("must condition is not a Field condition")
	}
	if fc.Key != "team_id" {
		t.Errorf("must condition key = %q, want 'team_id'", fc.Key)
	}
	kw := fc.Match.GetKeyword()
	if kw != "team-T" {
		t.Errorf("must condition value = %q, want 'team-T'", kw)
	}
}

// Verify the pb.Condition type is correct for field conditions
func TestQdrantFilterFieldConditionType(t *testing.T) {
	t.Parallel()
	f, err := TenancyFilter("team-T", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	switch f.Must[0].ConditionOneOf.(type) {
	case *pb.Condition_Field:
		// expected
	default:
		t.Error("team_id condition should be Field type")
	}
}
