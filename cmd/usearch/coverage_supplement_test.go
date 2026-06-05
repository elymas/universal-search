// Package main — additional tests to meet 80% coverage target (SPEC-CLI-001 NFR-CLI-003).
//
// These tests cover production-path helpers and output formatters that are not
// exercised by the primary Execute() test suite (which uses injected test doubles).
package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/pipeline"
	"github.com/elymas/universal-search/pkg/types"
)

// --- pipeline.BuildProductionRegistry / pipeline.BuildProductionSynth ---

// TestBuildProductionRegistryReturnsRegistry verifies the production registry
// constructor does not panic, returns a non-nil registry, and registers the
// expected adapters (Reddit + Hacker News) per SPEC-CLI-001.
// Reddit now requires REDDIT_CLIENT_ID + REDDIT_CLIENT_SECRET (ADP-001a).
func TestBuildProductionRegistryReturnsRegistry(t *testing.T) {
	// Set Reddit OAuth credentials so the adapter registers.
	t.Setenv("REDDIT_CLIENT_ID", "test_coverage_id")
	t.Setenv("REDDIT_CLIENT_SECRET", "test_coverage_secret")

	reg := pipeline.BuildProductionRegistry()
	if reg == nil {
		t.Fatal("BuildProductionRegistry() returned nil")
	}
	for _, name := range []string{"reddit", "hackernews"} {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("expected adapter %q to be registered", name)
		}
	}
}

// TestBuildProductionSynthReturnsClient verifies the production synth
// constructor returns a non-nil client.
func TestBuildProductionSynthReturnsClient(t *testing.T) {
	s := pipeline.BuildProductionSynth()
	if s == nil {
		t.Fatal("BuildProductionSynth() returned nil")
	}
	// Calling Synthesize against an unreachable sidecar (or the nop fallback)
	// must return an error (not panic). Use a short-cancelled context so the
	// real client does not actually open a TCP connection that would leak.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := s.Synthesize(ctx, "query", "en", nil)
	if err == nil {
		t.Error("expected error from synth client (cancelled ctx or nop), got nil")
	}
	_ = res
}

// --- adminAddr ---

// TestAdminAddrEmpty verifies that adminAddr returns empty string when
// USEARCH_ADMIN_PORT is not set.
func TestAdminAddrEmpty(t *testing.T) {
	t.Setenv("USEARCH_ADMIN_PORT", "")
	got := adminAddr()
	if got != "" {
		t.Errorf("adminAddr() = %q, want empty", got)
	}
}

// TestAdminAddrWithPort verifies that adminAddr formats the bind address correctly.
func TestAdminAddrWithPort(t *testing.T) {
	t.Setenv("USEARCH_ADMIN_PORT", "9090")
	got := adminAddr()
	if got != "127.0.0.1:9090" {
		t.Errorf("adminAddr() = %q, want %q", got, "127.0.0.1:9090")
	}
}

// --- cobra help text ---

// TestHelpTextContainsQuery verifies that cobra help mentions the query subcommand.
func TestHelpTextContainsQuery(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, io.Discard)
	cmd.SetArgs([]string{"--help"})
	// Execute help and capture output.
	_ = cmd.Execute()
	helpTxt := buf.String()
	if !strings.Contains(helpTxt, "query") {
		t.Errorf("cobra help does not mention 'query': %q", helpTxt)
	}
	if !strings.Contains(helpTxt, "version") {
		t.Errorf("cobra help does not mention 'version': %q", helpTxt)
	}
}

// --- progressEmitter: both implementations ---

// TestJSONProgressEmitIsNoop verifies that jsonProgress.Emit writes nothing.
func TestJSONProgressEmitIsNoop(t *testing.T) {
	jp := &jsonProgress{}
	// Emit must not panic and must have no observable side-effect.
	jp.Emit("stage", "message")
}

// TestHumanProgressEmitWrites verifies that humanProgress.Emit writes to its writer.
func TestHumanProgressEmitWrites(t *testing.T) {
	var buf strings.Builder
	hp := &humanProgress{w: &buf}
	hp.Emit("fetch", "downloading results")
	got := buf.String()
	if !strings.Contains(got, "[fetch]") {
		t.Errorf("humanProgress.Emit missing [fetch]: %q", got)
	}
	if !strings.Contains(got, "downloading results") {
		t.Errorf("humanProgress.Emit missing message: %q", got)
	}
}

// --- dispatch: help and version paths ---

// TestDispatchHelpExitsSuccess verifies --help and help aliases return 0.
func TestDispatchHelpExitsSuccess(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		code := dispatch([]string{arg})
		if code != ExitSuccess {
			t.Errorf("dispatch(%q) = %d, want %d (ExitSuccess)", arg, code, ExitSuccess)
		}
	}
}

// TestDispatchVersionExitsSuccess verifies -v returns 0.
func TestDispatchVersionExitsSuccess(t *testing.T) {
	code := dispatch([]string{"-v"})
	if code != ExitSuccess {
		t.Errorf("dispatch(-v) = %d, want %d (ExitSuccess)", code, ExitSuccess)
	}
}

// TestDispatchNoArgsShowsHelp verifies empty args shows help and exits 0.
// SPEC-CLI-002 REQ-CLI2-008: zero args + non-TTY shows help, exit 0.
// (v0 exited 2; v1 updated to exit 0 per cobra default behavior.)
func TestDispatchNoArgsShowsHelp(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(&buf, &buf)
	cmd.SetArgs([]string{})
	code := runCobra(cmd, []string{})
	if code != ExitSuccess {
		t.Errorf("dispatch([]) = %d, want %d (ExitSuccess)", code, ExitSuccess)
	}
	output := buf.String()
	if !strings.Contains(output, "usearch") {
		t.Errorf("help output does not mention 'usearch': %q", output)
	}
}

// --- sourceFromDocs ---

// TestSourceFromDocsFound verifies that sourceFromDocs returns the SourceID for a matching doc.
func TestSourceFromDocsFound(t *testing.T) {
	docs := []types.NormalizedDoc{
		{ID: "abc", SourceID: "reddit"},
		{ID: "xyz", SourceID: "hackernews"},
	}
	got := sourceFromDocs("abc", docs)
	if got != "reddit" {
		t.Errorf("sourceFromDocs(abc) = %q, want %q", got, "reddit")
	}
}

// TestSourceFromDocsNotFound verifies that sourceFromDocs returns empty string when not matched.
func TestSourceFromDocsNotFound(t *testing.T) {
	docs := []types.NormalizedDoc{
		{ID: "abc", SourceID: "reddit"},
	}
	got := sourceFromDocs("missing", docs)
	if got != "" {
		t.Errorf("sourceFromDocs(missing) = %q, want empty", got)
	}
}

// --- formatText: snippet-empty falls back to Title ---

// TestFormatTextDegradedTitleFallback verifies the Title fallback when Snippet is empty.
func TestFormatTextDegradedTitleFallback(t *testing.T) {
	resp := &queryResponse{
		Docs: []types.NormalizedDoc{
			{Title: "Only a title, no snippet"},
		},
	}
	var buf bytes.Buffer
	if err := formatText(&buf, resp); err != nil {
		t.Fatalf("formatText error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Only a title, no snippet") {
		t.Errorf("title fallback not rendered: %q", out)
	}
}

// --- determineExitCode direct unit tests ---

// TestDetermineExitCodeSuccess verifies ExitSuccess when docs+synth ok and no adapter errors.
func TestDetermineExitCodeSuccess(t *testing.T) {
	docs := []types.NormalizedDoc{{ID: "1"}}
	code := determineExitCode(docs, map[string]error{}, synthResult{Text: "summary"}, nil)
	if code != ExitSuccess {
		t.Errorf("determineExitCode = %d, want ExitSuccess", code)
	}
}

// TestDetermineExitCodePartialWithAdapterError verifies ExitPartial when adapter error.
func TestDetermineExitCodePartialWithAdapterError(t *testing.T) {
	docs := []types.NormalizedDoc{{ID: "1"}}
	errs := map[string]error{"reddit": errSynthUnavailable}
	code := determineExitCode(docs, errs, synthResult{Text: "summary"}, nil)
	if code != ExitPartial {
		t.Errorf("determineExitCode(partial) = %d, want ExitPartial", code)
	}
}

// TestDetermineExitCodeSystemErrorNoDocs verifies ExitSystemError when no docs.
func TestDetermineExitCodeSystemErrorNoDocs(t *testing.T) {
	code := determineExitCode(nil, map[string]error{}, synthResult{}, nil)
	if code != ExitSystemError {
		t.Errorf("determineExitCode(no docs) = %d, want ExitSystemError", code)
	}
}

// (TestExecuteRouterWithNoAdaptersExitsTwo removed: buildProductionRegistry()
// now registers Reddit + HN per SPEC-CLI-001 §2.1(m), so the test premise
// ("empty registry") no longer holds. The "all adapters fail" path is still
// covered by TestExitTwoOnAllAdaptersFail in query_test.go using mock adapters.)

// --- formatText error paths via failing writer ---

// errWriter always returns an error on any Write call.
type errWriter struct{}

func (e *errWriter) Write(_ []byte) (int, error) {
	return 0, bytes.ErrTooLarge
}

// TestFormatTextWriteErrorSummaryPropagated verifies that formatText propagates
// write errors when writing the summary line.
func TestFormatTextWriteErrorSummaryPropagated(t *testing.T) {
	resp := &queryResponse{Summary: "some summary"}
	err := formatText(&errWriter{}, resp)
	if err == nil {
		t.Error("expected error from formatText when writer fails, got nil")
	}
}

// TestFormatTextWriteErrorDegradedPropagated verifies error propagation in degraded mode.
func TestFormatTextWriteErrorDegradedPropagated(t *testing.T) {
	resp := &queryResponse{
		Docs: []types.NormalizedDoc{{Snippet: "snippet"}},
	}
	err := formatText(&errWriter{}, resp)
	if err == nil {
		t.Error("expected error from formatText in degraded mode when writer fails, got nil")
	}
}
