package tenancy

import "testing"

func TestIsPublicSentinel(t *testing.T) {
	t.Parallel()
	if !IsPublicSentinel("__public__") {
		t.Error("IsPublicSentinel('__public__') should be true")
	}
	if IsPublicSentinel("team-T") {
		t.Error("IsPublicSentinel('team-T') should be false")
	}
}

func TestValidateTeamIDRejectsSentinel(t *testing.T) {
	t.Parallel()
	if err := ValidateTeamID("__public__"); err == nil {
		t.Error("ValidateTeamID('__public__') should return error")
	}
}

func TestValidateTeamIDAcceptsNormal(t *testing.T) {
	t.Parallel()
	if err := ValidateTeamID("team-T"); err != nil {
		t.Errorf("ValidateTeamID('team-T') returned unexpected error: %v", err)
	}
}

func TestHashTeamIDReturns8HexChars(t *testing.T) {
	t.Parallel()
	got := HashTeamID("team-T")
	if len(got) != 8 {
		t.Errorf("HashTeamID('team-T') = %q, want 8 hex chars", got)
	}
}

func TestHashTeamIDPublicSentinelPassthrough(t *testing.T) {
	t.Parallel()
	got := HashTeamID("__public__")
	if got != "__public__" {
		t.Errorf("HashTeamID('__public__') = %q, want '__public__'", got)
	}
}

func TestHashTeamIDDeterministic(t *testing.T) {
	t.Parallel()
	a := HashTeamID("team-T")
	b := HashTeamID("team-T")
	if a != b {
		t.Errorf("HashTeamID is not deterministic: %q != %q", a, b)
	}
}

func TestHashTeamIDDifferentInputsDifferentOutputs(t *testing.T) {
	t.Parallel()
	a := HashTeamID("team-T")
	b := HashTeamID("team-U")
	if a == b {
		t.Errorf("HashTeamID('team-T') == HashTeamID('team-U'), expected different")
	}
}

func TestVisibilityString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		v    Visibility
		want string
	}{
		{VisibilityTeamShared, "team_shared"},
		{VisibilityUserPrivate, "user_private"},
		{VisibilityPublic, "public"},
	}
	for _, tt := range tests {
		if got := tt.v.String(); got != tt.want {
			t.Errorf("Visibility(%d).String() = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestVisibilityFromString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  Visibility
	}{
		{"user_private", VisibilityUserPrivate},
		{"public", VisibilityPublic},
		{"team_shared", VisibilityTeamShared},
		{"unknown", VisibilityTeamShared},
		{"", VisibilityTeamShared},
	}
	for _, tt := range tests {
		if got := VisibilityFromString(tt.input); got != tt.want {
			t.Errorf("VisibilityFromString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
