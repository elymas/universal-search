package tenancy

import "testing"

func TestDedicatedCollectionNameFormat(t *testing.T) {
	t.Parallel()
	name := DedicatedCollectionName("team-MEGA")
	if len(name) < len("usearch_docs__team_")+16 {
		t.Errorf("collection name too short: %q", name)
	}
	// Should start with usearch_docs__team_
	prefix := "usearch_docs__team_"
	if name[:len(prefix)] != prefix {
		t.Errorf("collection name should start with %q, got %q", prefix, name)
	}
}

func TestDedicatedCollectionNameDeterministic(t *testing.T) {
	t.Parallel()
	a := DedicatedCollectionName("team-MEGA")
	b := DedicatedCollectionName("team-MEGA")
	if a != b {
		t.Errorf("not deterministic: %q != %q", a, b)
	}
}

func TestDedicatedCollectionNameDifferentTeams(t *testing.T) {
	t.Parallel()
	a := DedicatedCollectionName("team-A")
	b := DedicatedCollectionName("team-B")
	if a == b {
		t.Error("different teams should produce different collection names")
	}
}

func TestTierConfigIsDedicatedTeam(t *testing.T) {
	t.Parallel()
	tc := &TierConfig{DedicatedTeams: []string{"team-MEGA", "team-ENTERPRISE"}}
	if !tc.IsDedicatedTeam("team-MEGA") {
		t.Error("team-MEGA should be dedicated")
	}
	if tc.IsDedicatedTeam("team-SMALL") {
		t.Error("team-SMALL should not be dedicated")
	}
}

func TestTierConfigCollectionForTeamDedicated(t *testing.T) {
	t.Parallel()
	tc := &TierConfig{DedicatedTeams: []string{"team-MEGA"}}
	name := tc.CollectionForTeam("team-MEGA", "usearch_docs")
	if name != DedicatedCollectionName("team-MEGA") {
		t.Errorf("dedicated team got %q, want dedicated collection", name)
	}
}

func TestTierConfigCollectionForTeamDefault(t *testing.T) {
	t.Parallel()
	tc := &TierConfig{DedicatedTeams: []string{"team-MEGA"}}
	name := tc.CollectionForTeam("team-SMALL", "usearch_docs")
	if name != "usearch_docs" {
		t.Errorf("non-dedicated team got %q, want 'usearch_docs'", name)
	}
}

func TestTierConfigEmptyList(t *testing.T) {
	t.Parallel()
	tc := &TierConfig{}
	if tc.IsDedicatedTeam("any-team") {
		t.Error("empty tier config should not have any dedicated teams")
	}
}
