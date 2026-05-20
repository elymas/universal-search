// Package main — TDD acceptance tests for REQ-CLI-001..011 + NFR-CLI-001..004.
//
// TestMain uses goleak to enforce zero goroutine leaks (NFR-CLI-002).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/pkg/types"
	"go.uber.org/goleak"
)

// TestMain enforces zero goroutine leaks via goleak (NFR-CLI-002).
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// --- Mock types for in-process testing ---

// mockAdapter is a test double for types.Adapter that returns fixed results.
type mockAdapter struct {
	name    string
	docs    []types.NormalizedDoc
	err     error
	delay   time.Duration
}

func (m *mockAdapter) Name() string { return m.name }
func (m *mockAdapter) Healthcheck(_ context.Context) error { return nil }
func (m *mockAdapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:       m.name,
		DisplayName:    m.name,
		DocTypes:       []types.DocType{types.DocTypePost},
		SupportedLangs: []string{},
		DefaultMaxResults: 10,
	}
}
func (m *mockAdapter) Search(ctx context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}
	return m.docs, m.err
}

// makeDocs creates n NormalizedDoc values for testing.
func makeDocs(n int, source string) []types.NormalizedDoc {
	now := time.Now()
	docs := make([]types.NormalizedDoc, n)
	for i := range docs {
		docs[i] = types.NormalizedDoc{
			ID:          fmt.Sprintf("%s-doc-%d", source, i+1),
			SourceID:    source,
			URL:         fmt.Sprintf("https://example.com/%s/%d", source, i+1),
			Title:       fmt.Sprintf("%s title %d", source, i+1),
			Snippet:     fmt.Sprintf("%s snippet %d", source, i+1),
			RetrievedAt: now,
		}
	}
	return docs
}

// mockSynthClient is a test double for the synthesis client.
type mockSynthClient struct {
	summary string
	err     error
}

func (m *mockSynthClient) Synthesize(
	_ context.Context, query, _ string, docs []types.NormalizedDoc,
) (synthResult, error) {
	if m.err != nil {
		return synthResult{}, m.err
	}
	citations := make([]synthCitation, len(docs))
	for i, d := range docs {
		citations[i] = synthCitation{
			Marker: i + 1,
			DocID:  d.ID,
			URL:    d.URL,
			Title:  d.Title,
		}
	}
	return synthResult{Text: m.summary, Citations: citations}, nil
}

// buildTestRegistry creates a Registry with the given adapters, skipping auth checks.
func buildTestRegistry(adaps ...types.Adapter) *adapters.Registry {
	reg := adapters.NewRegistry(nil)
	for _, a := range adaps {
		if err := reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
			panic(fmt.Sprintf("buildTestRegistry: %v", err))
		}
	}
	return reg
}

// --- REQ-CLI-007: Empty prompt rejection ---

func TestEmptyPromptExitsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	reg := buildTestRegistry()
	// Empty string as positional: flags precede the positional.
	code := Execute(context.Background(), []string{"--no-obs", ""}, &stdout, &stderr,
		withRegistry(reg), withSynth(&mockSynthClient{}))
	if code != ExitUserError {
		t.Errorf("empty prompt: exit %d, want %d (stderr: %s)", code, ExitUserError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "prompt argument required") {
		t.Errorf("stderr missing 'prompt argument required': %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty on empty prompt: %q", stdout.String())
	}
}

func TestWhitespacePromptExitsOne(t *testing.T) {
	cases := []string{"   ", "\t", "  \t  "}
	for _, prompt := range cases {
		t.Run(fmt.Sprintf("prompt=%q", prompt), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			reg := buildTestRegistry()
			code := Execute(context.Background(), []string{"--no-obs", prompt}, &stdout, &stderr,
				withRegistry(reg), withSynth(&mockSynthClient{}))
			if code != ExitUserError {
				t.Errorf("whitespace prompt %q: exit %d, want %d", prompt, code, ExitUserError)
			}
			if !strings.Contains(stderr.String(), "prompt argument required") {
				t.Errorf("stderr missing message for %q: %s", prompt, stderr.String())
			}
		})
	}
}

// --- REQ-CLI-002: Flag parsing ---

func TestQueryParsesPositionalAndFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	redditDocs := makeDocs(2, "reddit")
	reg := buildTestRegistry(&mockAdapter{name: "reddit", docs: redditDocs})
	// Note: flags must precede positional args for stdlib flag.FlagSet.
	code := Execute(
		context.Background(),
		[]string{"--format", "json", "--timeout", "10s", "--source", "reddit", "--no-obs", "hello world"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{summary: "test summary"}),
	)
	// Should be 0 (full success) or 3 (partial)
	if code != ExitSuccess && code != ExitPartial {
		t.Errorf("exit %d, want 0 or 3 (stderr: %s)", code, stderr.String())
	}
}

func TestQueryRejectsZeroPositional(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), []string{"--no-obs", "--format", "text"}, &stdout, &stderr)
	if code != ExitUserError {
		t.Errorf("zero positional: exit %d, want %d (stderr: %s)", code, ExitUserError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "prompt argument required") {
		t.Errorf("stderr missing message: %s", stderr.String())
	}
}

func TestQueryRejectsTwoPositionals(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Flags must come before positionals for stdlib flag parsing.
	code := Execute(context.Background(), []string{"--no-obs", "a", "b"}, &stdout, &stderr)
	if code != ExitUserError {
		t.Errorf("two positionals: exit %d, want %d (stderr: %s)", code, ExitUserError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "exactly one positional argument") {
		t.Errorf("stderr missing message: %s", stderr.String())
	}
}

// --- REQ-CLI-004: Format flag ---

func TestFormatInvalidExitsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), []string{"--format", "yaml", "--no-obs", "hello"}, &stdout, &stderr)
	if code != ExitUserError {
		t.Errorf("invalid format: exit %d, want %d (stderr: %s)", code, ExitUserError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unsupported format") {
		t.Errorf("stderr missing 'unsupported format': %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "yaml") {
		t.Errorf("stderr missing format name 'yaml': %s", stderr.String())
	}
}

func TestFormatDefaultIsText(t *testing.T) {
	var stdout, stderr bytes.Buffer
	redditDocs := makeDocs(2, "reddit")
	reg := buildTestRegistry(&mockAdapter{name: "reddit", docs: redditDocs})
	code := Execute(
		context.Background(),
		[]string{"--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{summary: "default text output"}),
	)
	if code != ExitSuccess && code != ExitPartial {
		t.Errorf("default format: exit %d, want 0 or 3", code)
	}
	// Text format should NOT be valid JSON on its own
	var dummy map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &dummy); err == nil {
		t.Errorf("default format stdout should not be JSON, got: %s", stdout.String())
	}
}

func TestFormatJSONOutputShape(t *testing.T) {
	var stdout, stderr bytes.Buffer
	redditDocs := makeDocs(2, "reddit")
	reg := buildTestRegistry(&mockAdapter{name: "reddit", docs: redditDocs})
	code := Execute(
		context.Background(),
		[]string{"--format", "json", "--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{summary: "json test summary"}),
	)
	if code != ExitSuccess && code != ExitPartial {
		t.Errorf("json format: exit %d, want 0 or 3 (stderr: %s)", code, stderr.String())
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout.String())
	}

	required := []string{"schema_version", "query", "category", "lang", "adapters", "summary", "citations", "stats"}
	for _, k := range required {
		if _, ok := out[k]; !ok {
			t.Errorf("missing JSON key: %q", k)
		}
	}

	var sv string
	_ = json.Unmarshal(out["schema_version"], &sv)
	if sv != "1" {
		t.Errorf("schema_version = %q, want \"1\"", sv)
	}
}

// --- REQ-CLI-005: Timeout ---

func TestTimeoutExceedsMaxRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(),
		[]string{"--timeout", "10m", "--no-obs", "hello"},
		&stdout, &stderr)
	if code != ExitUserError {
		t.Errorf("max timeout exceeded: exit %d, want %d (stderr: %s)", code, ExitUserError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "exceeds maximum") {
		t.Errorf("stderr missing 'exceeds maximum': %s", stderr.String())
	}
}

func TestTimeoutCancelsFanout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	slowAdapter := &mockAdapter{name: "slow", delay: 60 * time.Second, docs: nil}
	reg := buildTestRegistry(slowAdapter)
	start := time.Now()
	code := Execute(
		context.Background(),
		[]string{"--timeout", "150ms", "--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{}),
	)
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Errorf("fanout should be cancelled quickly, elapsed=%v", elapsed)
	}
	if code != ExitSystemError {
		t.Errorf("timeout: exit %d, want %d", code, ExitSystemError)
	}
	if !strings.Contains(stderr.String(), "timeout") {
		t.Errorf("stderr missing 'timeout': %s", stderr.String())
	}
}

// --- REQ-CLI-008: Exit codes ---

func TestExitZeroOnFullSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	docs := makeDocs(3, "reddit")
	reg := buildTestRegistry(&mockAdapter{name: "reddit", docs: docs})
	code := Execute(
		context.Background(),
		[]string{"--no-obs", "hello world"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{summary: "full success summary"}),
	)
	if code != ExitSuccess {
		t.Errorf("full success: exit %d, want %d (stderr: %s)", code, ExitSuccess, stderr.String())
	}
}

func TestExitThreeOnSynthesisFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	docs := makeDocs(2, "reddit")
	reg := buildTestRegistry(&mockAdapter{name: "reddit", docs: docs})
	code := Execute(
		context.Background(),
		[]string{"--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{err: fmt.Errorf("synthesis failed")}),
	)
	if code != ExitPartial {
		t.Errorf("synth failure: exit %d, want %d", code, ExitPartial)
	}
	// stdout should contain degraded output (docs)
	if stdout.Len() == 0 {
		t.Error("stdout should have degraded output when synthesis fails")
	}
}

func TestExitThreeOnPartialAdapterFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	docs := makeDocs(2, "reddit")
	reg := buildTestRegistry(
		&mockAdapter{name: "reddit", docs: docs},
		&mockAdapter{name: "hackernews", err: fmt.Errorf("hn error")},
	)
	code := Execute(
		context.Background(),
		[]string{"--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{summary: "partial success"}),
	)
	if code != ExitPartial {
		t.Errorf("partial failure: exit %d, want %d (stderr: %s)", code, ExitPartial, stderr.String())
	}
}

func TestExitTwoOnAllAdaptersFail(t *testing.T) {
	var stdout, stderr bytes.Buffer
	reg := buildTestRegistry(
		&mockAdapter{name: "reddit", err: fmt.Errorf("reddit error")},
		&mockAdapter{name: "hackernews", err: fmt.Errorf("hn error")},
	)
	code := Execute(
		context.Background(),
		[]string{"--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{}),
	)
	if code != ExitSystemError {
		t.Errorf("all fail: exit %d, want %d (stderr: %s)", code, ExitSystemError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "all adapters failed") {
		t.Errorf("stderr missing 'all adapters failed': %s", stderr.String())
	}
}

// --- REQ-CLI-009: Nop synthesis (degraded mode) ---

func TestNopSynthesisProducesDegradedOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	docs := makeDocs(2, "reddit")
	reg := buildTestRegistry(&mockAdapter{name: "reddit", docs: docs})
	// NopClient returns ErrSynthesisUnavailable
	code := Execute(
		context.Background(),
		[]string{"--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&nopSynthClient{}),
	)
	if code != ExitPartial {
		t.Errorf("nop synth: exit %d, want %d", code, ExitPartial)
	}
	// stdout must have numbered doc list
	out := stdout.String()
	if !strings.Contains(out, "[1]") {
		t.Errorf("nop degraded output missing [1]: %q", out)
	}
}

func TestNopSynthesisStderrCarriesWarning(t *testing.T) {
	var stdout, stderr bytes.Buffer
	docs := makeDocs(2, "reddit")
	reg := buildTestRegistry(&mockAdapter{name: "reddit", docs: docs})
	_ = Execute(
		context.Background(),
		[]string{"--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&nopSynthClient{}),
	)
	if !strings.Contains(stderr.String(), "[synthesis: unavailable]") {
		t.Errorf("stderr missing '[synthesis: unavailable]': %s", stderr.String())
	}
}

// --- REQ-CLI-006: stdout/stderr separation ---

func TestStdoutContainsOnlyPayload(t *testing.T) {
	var stdout, stderr bytes.Buffer
	docs := makeDocs(2, "reddit")
	reg := buildTestRegistry(&mockAdapter{name: "reddit", docs: docs})
	code := Execute(
		context.Background(),
		[]string{"--format", "json", "--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{summary: "test"}),
	)
	if code != ExitSuccess && code != ExitPartial {
		t.Errorf("exit %d", code)
	}
	// stdout must be parseable as exactly one JSON object
	var out map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Errorf("stdout is not a single JSON object: %v\nstdout: %s", err, stdout.String())
	}
}

// --- REQ-CLI-011: Request ID ---

func TestRequestIDInJSONStats(t *testing.T) {
	var stdout, stderr bytes.Buffer
	docs := makeDocs(1, "reddit")
	reg := buildTestRegistry(&mockAdapter{name: "reddit", docs: docs})
	code := Execute(
		context.Background(),
		[]string{"--format", "json", "--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{summary: "test"}),
	)
	if code != ExitSuccess && code != ExitPartial {
		t.Fatalf("exit %d", code)
	}

	var out struct {
		Stats struct {
			RequestID string `json:"request_id"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("JSON parse: %v\nstdout: %s", err, stdout.String())
	}
	if len(out.Stats.RequestID) != 26 {
		t.Errorf("request_id length = %d, want 26: %q", len(out.Stats.RequestID), out.Stats.RequestID)
	}
}

// --- REQ-CLI-003: Source flag ---

func TestSourceFlagUnknownAdapterRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	reg := buildTestRegistry(&mockAdapter{name: "reddit", docs: makeDocs(1, "reddit")})
	code := Execute(
		context.Background(),
		[]string{"--source", "nosuchadapter", "--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{}),
	)
	if code != ExitUserError {
		t.Errorf("unknown adapter: exit %d, want %d (stderr: %s)", code, ExitUserError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown adapter") {
		t.Errorf("stderr missing 'unknown adapter': %s", stderr.String())
	}
}

func TestSourceFlagFiltersAdapters(t *testing.T) {
	var stdout, stderr bytes.Buffer
	redditDocs := makeDocs(2, "reddit")
	hnCalled := false
	hnAdapter := &callTrackingAdapter{
		mockAdapter: &mockAdapter{name: "hackernews", docs: makeDocs(1, "hackernews")},
		onSearch:    func() { hnCalled = true },
	}
	reg := buildTestRegistry(
		&mockAdapter{name: "reddit", docs: redditDocs},
		hnAdapter,
	)
	Execute(
		context.Background(),
		[]string{"--source", "reddit", "--no-obs", "hello"},
		&stdout, &stderr,
		withRegistry(reg),
		withSynth(&mockSynthClient{summary: "filtered"}),
	)
	if hnCalled {
		t.Error("hackernews adapter should not be called when --source reddit")
	}
}

// callTrackingAdapter wraps mockAdapter to track Search calls.
type callTrackingAdapter struct {
	*mockAdapter
	onSearch func()
}

func (c *callTrackingAdapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	c.onSearch()
	return c.mockAdapter.Search(ctx, q)
}

// --- NFR-CLI-004: Human-readable errors ---

func TestErrorMessagesNoStackTrace(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{"empty prompt", []string{"--no-obs", ""}},
		{"invalid format", []string{"--format", "yaml", "--no-obs", "hello"}},
		{"max timeout", []string{"--timeout", "10m", "--no-obs", "hello"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			Execute(context.Background(), tc.args, &stdout, &stderr)
			errOutput := stderr.String()
			for _, line := range strings.Split(errOutput, "\n") {
				if strings.Contains(line, "goroutine ") {
					t.Errorf("stderr contains goroutine stack trace: %q", line)
				}
				if strings.HasPrefix(line, "\t") && strings.Contains(line, ".go:") {
					t.Errorf("stderr contains file path in stack trace: %q", line)
				}
			}
		})
	}
}

func TestErrorMessagesUnder200Chars(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Execute(context.Background(), []string{"--format", "yaml", "--no-obs", "hello"}, &stdout, &stderr)
	for _, line := range strings.Split(stderr.String(), "\n") {
		if len(line) > 200 {
			t.Errorf("error line exceeds 200 chars (%d): %q", len(line), line)
		}
	}
}
