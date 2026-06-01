package scorer_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/eval/scorer"
	"github.com/elymas/universal-search/internal/synthesis"
)

// TestSegmentClaimsEnglish verifies ASCII sentence splitting equivalent to the
// live structural checker's (?<=[.!?])\s+ rule.
// REQ-EVAL1-005(a).
func TestSegmentClaimsEnglish(t *testing.T) {
	t.Parallel()
	text := "Google claimed supremacy [1]. IBM disputed the result [2]. The debate continues [1]."
	claims := scorer.SegmentClaims(text, "en")
	if len(claims) != 3 {
		t.Fatalf("expected 3 EN claims, got %d: %#v", len(claims), claims)
	}
	if claims[0] != "Google claimed supremacy [1]." {
		t.Errorf("first claim = %q", claims[0])
	}
}

// TestSegmentClaimsKorean verifies Korean-aware segmentation handles 다./요.
// endings without trailing whitespace and CJK punctuation. The structural
// checker provides NO CJK segmentation, so this is EVAL-001's own concern.
// REQ-EVAL1-005(a); HISTORY D3.
func TestSegmentClaimsKorean(t *testing.T) {
	t.Parallel()
	// Korean text with no spaces after sentence-final periods (real corpus style).
	text := "구글은 200초 만에 작업을 완료했다[1].고전 컴퓨터로는 1만 년이 걸린다[2].이 결과는 널리 인용된다[1]."
	claims := scorer.SegmentClaims(text, "ko")
	if len(claims) != 3 {
		t.Fatalf("expected 3 KO claims, got %d: %#v", len(claims), claims)
	}
	// Each claim must retain its citation marker.
	for i, c := range claims {
		if c == "" {
			t.Errorf("claim %d is empty", i)
		}
	}
}

// TestSegmentClaimsKoreanCJKPunctuation verifies 。！？ boundaries split.
func TestSegmentClaimsKoreanCJKPunctuation(t *testing.T) {
	t.Parallel()
	text := "첫 번째 문장입니다[1]。두 번째 문장입니다[2]！세 번째입니까[1]？"
	claims := scorer.SegmentClaims(text, "ko")
	if len(claims) != 3 {
		t.Fatalf("expected 3 KO claims with CJK punctuation, got %d: %#v", len(claims), claims)
	}
}

// TestBridgeExtractsCitations verifies cited doc IDs are resolved from the
// trailing [N] markers via the synthesis.Result.Citations array.
// REQ-EVAL1-005(b).
func TestBridgeExtractsCitations(t *testing.T) {
	t.Parallel()
	cites := []synthesis.Citation{
		{Marker: 1, DocID: "doc-001"},
		{Marker: 2, DocID: "doc-002"},
	}
	got := scorer.CitedDocIDs("Claim with two markers [1] and [2].", cites)
	if len(got) != 2 {
		t.Fatalf("expected 2 cited doc ids, got %d: %v", len(got), got)
	}
	want := map[string]bool{"doc-001": true, "doc-002": true}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unexpected doc id %q", id)
		}
	}
}

// TestBridgeMarshalsClaims verifies the bridge POSTs the judge schema and
// parses the per-claim response.
// REQ-EVAL1-005(d,e).
func TestBridgeMarshalsClaims(t *testing.T) {
	t.Parallel()
	var gotBody scorer.JudgeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := scorer.JudgeResponse{
			QueryID: gotBody.QueryID,
			ClaimScores: []scorer.ClaimScore{
				{Text: "c1", Supported: true, JudgeRationale: "ok"},
				{Text: "c2", Supported: false, JudgeRationale: "not entailed"},
			},
			FaithfulnessScore: 0.5,
			TotalClaims:       2,
			SupportedClaims:   1,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	b := scorer.NewBridge(srv.URL, 30*time.Second)
	result := synthesis.Result{
		Text:      "Claim one supported [1]. Claim two unsupported [2].",
		Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-001"}, {Marker: 2, DocID: "doc-002"}},
	}
	corpus := map[string]string{"doc-001": "body one", "doc-002": "body two"}
	out, err := b.Score(context.Background(), "EVAL-001-Q001", "en", result, corpus)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if gotBody.QueryID != "EVAL-001-Q001" {
		t.Errorf("judge received query_id %q", gotBody.QueryID)
	}
	if len(gotBody.Claims) != 2 {
		t.Errorf("judge received %d claims, want 2", len(gotBody.Claims))
	}
	if out.Score != 0.5 {
		t.Errorf("score = %v, want 0.5", out.Score)
	}
	if len(out.PerClaim) != 2 {
		t.Errorf("per-claim len = %d, want 2", len(out.PerClaim))
	}
}

// TestBridgeReturnsPerClaimRationale verifies the unsupported claim's rationale
// is surfaced to the runner.
// REQ-EVAL1-005(e), REQ-EVAL1-007.
func TestBridgeReturnsPerClaimRationale(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := scorer.JudgeResponse{
			ClaimScores: []scorer.ClaimScore{
				{Text: "c1", Supported: false, JudgeRationale: "cited doc is about classical computing"},
			},
			FaithfulnessScore: 0.0,
			TotalClaims:       1,
			SupportedClaims:   0,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	b := scorer.NewBridge(srv.URL, 30*time.Second)
	result := synthesis.Result{Text: "Wrong claim [1].", Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-001"}}}
	out, err := b.Score(context.Background(), "Q", "en", result, map[string]string{"doc-001": "body"})
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if len(out.PerClaim) != 1 || out.PerClaim[0].Supported {
		t.Fatalf("expected one unsupported claim, got %#v", out.PerClaim)
	}
	if out.PerClaim[0].JudgeRationale == "" {
		t.Error("expected non-empty judge rationale for unsupported claim")
	}
}

// TestBridgeTimeoutEnforced verifies the bridge honours the per-query timeout
// and classifies the error as a timeout/unavailability.
// REQ-EVAL1-005, NFR-EVAL1-002, REQ-EVAL1-006.
func TestBridgeTimeoutEnforced(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	b := scorer.NewBridge(srv.URL, 20*time.Millisecond)
	result := synthesis.Result{Text: "Claim [1].", Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-001"}}}
	_, err := b.Score(context.Background(), "Q", "en", result, map[string]string{"doc-001": "b"})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !scorer.IsUnavailable(err) {
		t.Errorf("expected error classified as unavailable, got %v", err)
	}
}

// TestWrapUnavailableRoundTrip verifies WrapUnavailable tags an error so that
// IsUnavailable recognises it while still preserving the original message.
// REQ-EVAL1-006.
func TestWrapUnavailableRoundTrip(t *testing.T) {
	t.Parallel()
	base := errors.New("judge sidecar down")
	wrapped := scorer.WrapUnavailable(base)
	if !scorer.IsUnavailable(wrapped) {
		t.Error("WrapUnavailable output must be classified as unavailable")
	}
	if !strings.Contains(wrapped.Error(), "judge sidecar down") {
		t.Errorf("wrapped error %q must preserve the original message", wrapped.Error())
	}
}

// TestBridgeServerErrorIsUnavailable verifies 5xx maps to the unavailability class.
// REQ-EVAL1-006.
func TestBridgeServerErrorIsUnavailable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	b := scorer.NewBridge(srv.URL, 30*time.Second)
	result := synthesis.Result{Text: "Claim [1].", Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-001"}}}
	_, err := b.Score(context.Background(), "Q", "en", result, map[string]string{"doc-001": "b"})
	if !scorer.IsUnavailable(err) {
		t.Errorf("expected 503 classified as unavailable, got %v", err)
	}
}
