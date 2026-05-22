package idx5

import (
	"strings"
	"testing"
)

func TestCacheDocIDIncludesTeamID(t *testing.T) {
	// REQ-IDX5-007: doc_id must include team_id to prevent cross-tenant collision
	queryHash := "abc123def456"
	teamT := "team-T"
	teamU := "team-U"

	idT := CacheDocID(queryHash, teamT)
	idU := CacheDocID(queryHash, teamU)

	if idT == idU {
		t.Errorf("same queryHash with different team_id produced identical doc_id: %q", idT)
	}
}

func TestCacheDocIDFormat(t *testing.T) {
	// REQ-IDX5-006: doc_id format = "answer-cache:<hash>:<team_id>"
	queryHash := "abc123"
	teamID := "team-T"

	id := CacheDocID(queryHash, teamID)

	if !strings.HasPrefix(id, "answer-cache:") {
		t.Errorf("doc_id %q should start with 'answer-cache:'", id)
	}
	if !strings.HasSuffix(id, ":"+teamID) {
		t.Errorf("doc_id %q should end with ':%s'", id, teamID)
	}
}

func TestCacheDocIDDeterministic(t *testing.T) {
	// REQ-IDX5-007: same inputs must always produce the same doc_id
	queryHash := "hash123"
	teamID := "team-X"

	id1 := CacheDocID(queryHash, teamID)
	id2 := CacheDocID(queryHash, teamID)

	if id1 != id2 {
		t.Errorf("CacheDocID is not deterministic: %q != %q", id1, id2)
	}
}
