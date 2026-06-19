// Package main — tests for the sources subcommand.
// SPEC-CLI-003: Registry-backed listing, live health status, derived columns.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/pkg/types"
)

// --- Fake adapters for testing ---

// fakeAdapter implements types.Adapter for testing.
type fakeAdapter struct {
	name     string
	caps     types.Capabilities
	hcResult error // nil = healthy, non-nil = error
	hcCalls  int32 // atomic counter for healthcheck calls
	hcDelay  time.Duration
	hcPanic  bool
}

func (f *fakeAdapter) Name() string                     { return f.name }
func (f *fakeAdapter) Capabilities() types.Capabilities { return f.caps }
func (f *fakeAdapter) Healthcheck(ctx context.Context) error {
	atomic.AddInt32(&f.hcCalls, 1)
	if f.hcPanic {
		panic("adapter healthcheck panicked")
	}
	if f.hcDelay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.hcDelay):
		}
	}
	return f.hcResult
}
func (f *fakeAdapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	return nil, nil
}

// hcCallCount returns the number of Healthcheck calls made on this fake.
func (f *fakeAdapter) hcCallCount() int {
	return int(atomic.LoadInt32(&f.hcCalls))
}

// makeFakeCaps creates Capabilities with sensible defaults.
func makeFakeCaps(name string, opts ...func(*types.Capabilities)) types.Capabilities {
	caps := types.Capabilities{
		SourceID:    name,
		DisplayName: name + " display",
		DocTypes:    []types.DocType{types.DocTypeArticle},
	}
	for _, o := range opts {
		o(&caps)
	}
	return caps
}

// withDocTypes sets DocTypes on Capabilities.
func withDocTypes(dt ...types.DocType) func(*types.Capabilities) {
	return func(c *types.Capabilities) { c.DocTypes = dt }
}

// withAuth sets auth requirements on Capabilities.
func withAuth(envVars ...string) func(*types.Capabilities) {
	return func(c *types.Capabilities) {
		c.RequiresAuth = true
		c.AuthEnvVars = envVars
	}
}

// withLangs sets SupportedLangs on Capabilities.
func withLangs(langs ...string) func(*types.Capabilities) {
	return func(c *types.Capabilities) { c.SupportedLangs = langs }
}

// buildSourcesTestRegistry creates a registry with the given fake adapters.
func buildSourcesTestRegistry(fakes ...*fakeAdapter) (*adapters.Registry, []*fakeAdapter) {
	reg := adapters.NewRegistry(nil)
	for _, f := range fakes {
		_ = reg.RegisterWithOptions(f, adapters.RegisterOptions{SkipAuthCheck: true})
	}
	return reg, fakes
}

// registryFactory returns a function that always returns the given registry.
func registryFactory(reg *adapters.Registry) func() *adapters.Registry {
	return func() *adapters.Registry { return reg }
}

// --- REQ-CLI3-001: Registry-backed listing, single source of truth ---

func TestSourcesListReflectsRegistry(t *testing.T) {
	fakes := []*fakeAdapter{
		{name: "alpha", caps: makeFakeCaps("alpha")},
		{name: "bravo", caps: makeFakeCaps("bravo")},
	}
	reg, _ := buildSourcesTestRegistry(fakes...)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "list"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources list failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "alpha") {
		t.Errorf("output missing adapter 'alpha': %s", output)
	}
	if !strings.Contains(output, "bravo") {
		t.Errorf("output missing adapter 'bravo': %s", output)
	}
}

func TestSourcesListOmitsUnregistered(t *testing.T) {
	// Only register one adapter; ensure others don't appear.
	fakes := []*fakeAdapter{
		{name: "only-me", caps: makeFakeCaps("only-me")},
	}
	reg, _ := buildSourcesTestRegistry(fakes...)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "list"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources list failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "only-me") {
		t.Errorf("output missing 'only-me': %s", output)
	}
	// Should NOT contain other names.
	if strings.Contains(output, "alpha") || strings.Contains(output, "reddit") {
		t.Errorf("output contains unexpected adapter: %s", output)
	}
}

func TestSourcesNoStaticKnownAdapters(t *testing.T) {
	// Verify the static knownAdapters slice is gone by checking that
	// a registry with zero adapters produces empty output.
	reg := adapters.NewRegistry(nil)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "list"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources list with empty registry: %v", err)
	}

	output := buf.String()
	// Should have header only (no adapter rows).
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("expected header-only output for empty registry, got %d lines: %s", len(lines), output)
	}
	if !strings.Contains(lines[0], "NAME") {
		t.Errorf("expected header row, got: %s", lines[0])
	}
}

// --- REQ-CLI3-002: Live concurrent healthcheck + classification ---

func TestSourcesStatusLiveHealthcheck(t *testing.T) {
	healthy := &fakeAdapter{name: "healthy-adapter", caps: makeFakeCaps("healthy-adapter"), hcResult: nil}
	sick := &fakeAdapter{name: "sick-adapter", caps: makeFakeCaps("sick-adapter"), hcResult: fmt.Errorf("connection refused")}
	reg, _ := buildSourcesTestRegistry(healthy, sick)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--timeout", "5s"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources status failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "healthy-adapter") {
		t.Errorf("output missing 'healthy-adapter': %s", output)
	}
	if !strings.Contains(output, "sick-adapter") {
		t.Errorf("output missing 'sick-adapter': %s", output)
	}
	if !strings.Contains(output, "connected") {
		t.Errorf("output missing 'connected' status: %s", output)
	}
	if !strings.Contains(output, "unhealthy") {
		t.Errorf("output missing 'unhealthy' status: %s", output)
	}
}

func TestSourcesStatusConcurrentProbe(t *testing.T) {
	// Verify healthchecks run concurrently by checking all are called.
	fakes := []*fakeAdapter{
		{name: "a1", caps: makeFakeCaps("a1"), hcResult: nil},
		{name: "a2", caps: makeFakeCaps("a2"), hcResult: nil},
		{name: "a3", caps: makeFakeCaps("a3"), hcResult: nil},
	}
	reg, _ := buildSourcesTestRegistry(fakes...)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--timeout", "5s"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources status failed: %v", err)
	}

	for _, f := range fakes {
		count := f.hcCallCount()
		if count != 1 {
			t.Errorf("adapter %q: expected 1 healthcheck call, got %d", f.name, count)
		}
	}
}

func TestSourcesStatusClassifiesDisabled(t *testing.T) {
	f := &fakeAdapter{name: "disabled-adapter", caps: makeFakeCaps("disabled-adapter"), hcResult: nil}
	reg, _ := buildSourcesTestRegistry(f)

	// Disable the adapter.
	_, _ = reg.ToggleEnabled(context.Background(), "disabled-adapter")

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--timeout", "5s"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources status failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "disabled") {
		t.Errorf("expected 'disabled' in output: %s", output)
	}
	// Disabled adapter should NOT have been probed.
	if f.hcCallCount() != 0 {
		t.Errorf("disabled adapter should not be probed, got %d calls", f.hcCallCount())
	}
}

func TestSourcesStatusClassifiesNotConfigured(t *testing.T) {
	// Register an auth-requiring adapter with SkipAuthCheck, but don't set the env var.
	f := &fakeAdapter{
		name:     "nokey-adapter",
		caps:     makeFakeCaps("nokey-adapter", withAuth("MISSING_KEY_12345")),
		hcResult: nil,
	}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--timeout", "5s"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources status failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "not-configured") {
		t.Errorf("expected 'not-configured' in output: %s", output)
	}
	// Not-configured adapter should NOT be probed.
	if f.hcCallCount() != 0 {
		t.Errorf("not-configured adapter should not be probed, got %d calls", f.hcCallCount())
	}
}

func TestSourcesStatusReportsKeySet(t *testing.T) {
	// One adapter with no auth (key_set=true), one with auth+env set (key_set=true),
	// one with auth but env unset (key_set=false).
	noAuth := &fakeAdapter{name: "noauth", caps: makeFakeCaps("noauth")}
	authSet := &fakeAdapter{
		name: "auth-set",
		caps: makeFakeCaps("auth-set", withAuth("TEST_KEY_THAT_EXISTS_999")),
	}
	authUnset := &fakeAdapter{
		name: "auth-unset",
		caps: makeFakeCaps("auth-unset", withAuth("MISSING_KEY_99999")),
	}

	// Set the env var for auth-set adapter.
	os.Setenv("TEST_KEY_THAT_EXISTS_999", "test-value")
	defer os.Unsetenv("TEST_KEY_THAT_EXISTS_999")

	reg, _ := buildSourcesTestRegistry(noAuth, authSet, authUnset)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--format", "json", "--timeout", "5s"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources status failed: %v", err)
	}

	var result struct {
		Sources []struct {
			Name   string `json:"name"`
			KeySet bool   `json:"key_set"`
		} `json:"sources"`
	}
	if parseErr := json.Unmarshal(buf.Bytes(), &result); parseErr != nil {
		t.Fatalf("parse JSON: %v", parseErr)
	}

	keySetMap := map[string]bool{}
	for _, s := range result.Sources {
		keySetMap[s.Name] = s.KeySet
	}

	if !keySetMap["noauth"] {
		t.Error("noauth adapter should have key_set=true")
	}
	if !keySetMap["auth-set"] {
		t.Error("auth-set adapter should have key_set=true")
	}
	if keySetMap["auth-unset"] {
		t.Error("auth-unset adapter should have key_set=false")
	}
}

func TestSourcesStatusPanicClassifiedUnhealthy(t *testing.T) {
	f := &fakeAdapter{
		name:    "panicker",
		caps:    makeFakeCaps("panicker"),
		hcPanic: true,
	}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--timeout", "5s"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources status should not fail on panic: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "unhealthy") {
		t.Errorf("panicking adapter should be classified 'unhealthy': %s", output)
	}
}

// --- REQ-CLI3-003: Per-adapter timeout ---

func TestSourcesStatusTimeoutClassifiesUnhealthy(t *testing.T) {
	// Adapter that blocks for longer than the timeout.
	f := &fakeAdapter{
		name:    "slow",
		caps:    makeFakeCaps("slow"),
		hcDelay: 5 * time.Second,
	}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--timeout", "100ms"})

	start := time.Now()
	err := cmd.Execute()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("sources status failed: %v", err)
	}

	if elapsed > 2*time.Second {
		t.Errorf("command took too long: %v (expected ~100ms)", elapsed)
	}

	output := buf.String()
	if !strings.Contains(output, "unhealthy") {
		t.Errorf("timed-out adapter should be 'unhealthy': %s", output)
	}
}

func TestSourcesStatusOneSlowDoesNotBlockOthers(t *testing.T) {
	fast := &fakeAdapter{name: "fast", caps: makeFakeCaps("fast"), hcResult: nil}
	slow := &fakeAdapter{name: "slow", caps: makeFakeCaps("slow"), hcDelay: 2 * time.Second}
	reg, _ := buildSourcesTestRegistry(fast, slow)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--timeout", "200ms"})

	start := time.Now()
	err := cmd.Execute()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("sources status failed: %v", err)
	}

	// Should complete in ~200ms (timeout), not 2s (slow adapter's delay).
	if elapsed > 1*time.Second {
		t.Errorf("command took too long: %v (one slow should not block)", elapsed)
	}

	output := buf.String()
	if !strings.Contains(output, "fast") {
		t.Errorf("output missing 'fast' adapter: %s", output)
	}
}

// --- REQ-CLI3-004: --format flag + derived columns ---

func TestSourcesListCategoryLangDerivation(t *testing.T) {
	tests := []struct {
		name     string
		docTypes []types.DocType
		langs    []string
		wantCat  string
		wantLang string
	}{
		{"paper", []types.DocType{types.DocTypePaper}, []string{"en"}, "academic", "en"},
		{"repo", []types.DocType{types.DocTypeRepo}, []string{}, "code", "*"},
		{"video", []types.DocType{types.DocTypeVideo}, []string{"en", "ko"}, "video", "en+"},
		{"social", []types.DocType{types.DocTypePost, types.DocTypeSocial}, []string{"en"}, "social", "en"},
		{"news", []types.DocType{types.DocTypeArticle}, []string{"ko"}, "news", "ko"},
		{"other", []types.DocType{types.DocTypeOther}, []string{}, "other", "*"},
		{"empty", []types.DocType{}, []string{}, "other", "*"},
		{"issue", []types.DocType{types.DocTypeIssue}, []string{"en"}, "code", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCat := deriveCategory(tt.docTypes)
			if gotCat != tt.wantCat {
				t.Errorf("deriveCategory(%v) = %q, want %q", tt.docTypes, gotCat, tt.wantCat)
			}
			gotLang := deriveLang(tt.langs)
			if gotLang != tt.wantLang {
				t.Errorf("deriveLang(%v) = %q, want %q", tt.langs, gotLang, tt.wantLang)
			}
		})
	}
}

func TestSourcesListJSONFormat(t *testing.T) {
	f := &fakeAdapter{
		name: "test-source",
		caps: makeFakeCaps("test-source", withDocTypes(types.DocTypePaper), withLangs("en")),
	}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "list", "--format", "json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources list --format json failed: %v", err)
	}

	var result struct {
		SchemaVersion string `json:"schema_version"`
		Sources       []struct {
			Name     string `json:"name"`
			Category string `json:"category"`
			Lang     string `json:"lang"`
			Auth     string `json:"auth_required"`
		} `json:"sources"`
	}
	if parseErr := json.Unmarshal(buf.Bytes(), &result); parseErr != nil {
		t.Fatalf("parse JSON: %v", parseErr)
	}

	if result.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want %q", result.SchemaVersion, "1")
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(result.Sources))
	}
	s := result.Sources[0]
	if s.Name != "test-source" {
		t.Errorf("name = %q, want %q", s.Name, "test-source")
	}
	if s.Category != "academic" {
		t.Errorf("category = %q, want %q", s.Category, "academic")
	}
	if s.Lang != "en" {
		t.Errorf("lang = %q, want %q", s.Lang, "en")
	}
}

func TestSourcesListMarkdownFormat(t *testing.T) {
	f := &fakeAdapter{name: "test-source", caps: makeFakeCaps("test-source")}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "list", "--format", "markdown"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources list --format markdown failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "| NAME |") {
		t.Errorf("missing markdown table header: %s", output)
	}
	if !strings.Contains(output, "| test-source |") {
		t.Errorf("missing markdown table row: %s", output)
	}
}

func TestSourcesStatusJSONSchema(t *testing.T) {
	f := &fakeAdapter{name: "test-adapter", caps: makeFakeCaps("test-adapter")}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--format", "json", "--timeout", "5s"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources status --format json failed: %v", err)
	}

	var result struct {
		SchemaVersion string `json:"schema_version"`
		Sources       []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			KeySet bool   `json:"key_set"`
			Error  string `json:"error,omitempty"`
		} `json:"sources"`
	}
	if parseErr := json.Unmarshal(buf.Bytes(), &result); parseErr != nil {
		t.Fatalf("parse JSON: %v", parseErr)
	}

	// Schema version MUST be string "1".
	if result.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want string %q", result.SchemaVersion, "1")
	}
	if len(result.Sources) != 1 || result.Sources[0].Name != "test-adapter" {
		t.Errorf("unexpected sources: %+v", result.Sources)
	}
	if result.Sources[0].Status != "connected" {
		t.Errorf("status = %q, want %q", result.Sources[0].Status, "connected")
	}
}

func TestSourcesStatusMarkdownTable(t *testing.T) {
	f := &fakeAdapter{name: "test-adapter", caps: makeFakeCaps("test-adapter")}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--format", "md", "--timeout", "5s"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources status --format md failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "| NAME |") {
		t.Errorf("missing markdown table header: %s", output)
	}
	if !strings.Contains(output, "connected") {
		t.Errorf("missing 'connected' status in markdown: %s", output)
	}
}

func TestSourcesFormatInvalidRejected(t *testing.T) {
	reg := adapters.NewRegistry(nil)

	var stderr bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"sources", "list", "--format", "xml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid format")
	}

	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "unsupported format") {
		t.Errorf("expected canonical error message in stderr: %s", stderrStr)
	}
	if !strings.Contains(stderrStr, "xml") {
		t.Errorf("error message should mention the invalid value: %s", stderrStr)
	}
}

func TestSourcesFormatHelperShared(t *testing.T) {
	// Both list and status should use the same validation path.
	tests := []struct {
		subcmd string
	}{
		{"list"},
		{"status"},
	}
	for _, tt := range tests {
		t.Run(tt.subcmd, func(t *testing.T) {
			reg := adapters.NewRegistry(nil)
			var stderr bytes.Buffer
			args := []string{"sources", tt.subcmd, "--format", "bogus"}
			if tt.subcmd == "status" {
				args = append(args, "--timeout", "3s")
			}
			cmd := newRootCmdWithSources(reg)
			cmd.SetErr(&stderr)
			cmd.SetArgs(args)

			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error for invalid format")
			}
			if !strings.Contains(stderr.String(), "unsupported format") {
				t.Errorf("subcommand %s: expected canonical message, got: %s", tt.subcmd, stderr.String())
			}
		})
	}
}

func TestSourcesFormatAliases(t *testing.T) {
	tests := []struct {
		format string
	}{
		{"text"},
		{"md"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			f := &fakeAdapter{name: "x", caps: makeFakeCaps("x")}
			reg, _ := buildSourcesTestRegistry(f)
			var buf bytes.Buffer
			cmd := newRootCmdWithSources(reg)
			cmd.SetOut(&buf)
			cmd.SetArgs([]string{"sources", "list", "--format", tt.format})

			err := cmd.Execute()
			if err != nil {
				t.Errorf("format alias %q should be accepted: %v", tt.format, err)
			}
		})
	}
}

// --- REQ-CLI3-005: Exit code semantics ---

func TestSourcesStatusExitsZeroWithUnhealthy(t *testing.T) {
	f := &fakeAdapter{name: "sick", caps: makeFakeCaps("sick"), hcResult: fmt.Errorf("fail")}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--timeout", "5s"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("status should exit 0 even with unhealthy adapters: %v", err)
	}
}

func TestSourcesStatusBadTimeoutExitsUserError(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
	}{
		{"zero", "0s"},
		{"negative", "-1s"},
		{"unparseable", "not-a-duration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := adapters.NewRegistry(nil)
			var stderr bytes.Buffer
			cmd := newRootCmdWithSources(reg)
			cmd.SetErr(&stderr)

			cmd.SetArgs([]string{"sources", "status", "--timeout", tt.timeout})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error for timeout %q", tt.timeout)
			}
			// For "zero" and "negative", our RunE validation catches it and returns exitError.
			// For "unparseable", cobra's flag parser catches it before RunE.
			var exitErr exitError
			if asExitError(err, &exitErr) {
				if exitErr.code != ExitUserError {
					t.Errorf("expected ExitUserError (1), got code %d: %v", exitErr.code, err)
				}
			}
			// Otherwise it's a cobra flag-parse error (still a user error).
		})
	}
}

func TestSourcesShowUnregisteredExitsUserError(t *testing.T) {
	reg := adapters.NewRegistry(nil)

	var stderr bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"sources", "show", "nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown adapter")
	}
	var exitErr exitError
	if !asExitError(err, &exitErr) || exitErr.code != ExitUserError {
		t.Errorf("expected ExitUserError, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "unknown adapter") {
		t.Errorf("stderr should contain 'unknown adapter': %s", stderr.String())
	}
}

// asExitError checks if an error is (or wraps) an exitError.
func asExitError(err error, target *exitError) bool {
	for e := err; e != nil; e = unwrap(e) {
		if ee, ok := e.(exitError); ok {
			*target = ee
			return true
		}
	}
	return false
}

func unwrap(err error) error {
	if u, ok := err.(interface{ Unwrap() error }); ok {
		return u.Unwrap()
	}
	return nil
}

// --- REQ-CLI3-006: Edge cases ---

func TestSourcesListEmptyRegistry(t *testing.T) {
	reg := adapters.NewRegistry(nil)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "list"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources list with empty registry should succeed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Errorf("expected header row: %s", output)
	}
	// Only header, no data rows.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line (header), got %d: %s", len(lines), output)
	}
}

func TestSourcesStatusEmptyRegistry(t *testing.T) {
	reg := adapters.NewRegistry(nil)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--timeout", "3s"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources status with empty registry should succeed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Errorf("expected header row: %s", output)
	}
}

func TestSourcesListEmptyRegistryJSON(t *testing.T) {
	reg := adapters.NewRegistry(nil)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "list", "--format", "json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources list --format json with empty registry: %v", err)
	}

	var result struct {
		SchemaVersion string     `json:"schema_version"`
		Sources       []struct{} `json:"sources"`
	}
	if parseErr := json.Unmarshal(buf.Bytes(), &result); parseErr != nil {
		t.Fatalf("parse JSON: %v", parseErr)
	}
	if result.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want %q", result.SchemaVersion, "1")
	}
	if len(result.Sources) != 0 {
		t.Errorf("expected empty sources array, got %d entries", len(result.Sources))
	}
}

func TestSourcesNoSecretValueLeak(t *testing.T) {
	// Set a real env var and ensure its VALUE never appears in output.
	os.Setenv("TEST_SECRET_FOR_LEAK_CHECK", "hunter2-secret-value")
	defer os.Unsetenv("TEST_SECRET_FOR_LEAK_CHECK")

	f := &fakeAdapter{
		name: "secret-adapter",
		caps: makeFakeCaps("secret-adapter", withAuth("TEST_SECRET_FOR_LEAK_CHECK")),
	}
	reg, _ := buildSourcesTestRegistry(f)

	// Check list output.
	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "list"})
	_ = cmd.Execute()
	if strings.Contains(buf.String(), "hunter2-secret-value") {
		t.Error("list output contains secret value!")
	}

	// Check status output.
	buf.Reset()
	cmd = newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "status", "--timeout", "5s"})
	_ = cmd.Execute()
	if strings.Contains(buf.String(), "hunter2-secret-value") {
		t.Error("status output contains secret value!")
	}

	// Check show output.
	buf.Reset()
	cmd = newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "show", "secret-adapter"})
	_ = cmd.Execute()
	if strings.Contains(buf.String(), "hunter2-secret-value") {
		t.Error("show output contains secret value!")
	}
}

// --- NFR-CLI3-002: list is network-free ---

func TestSourcesListIssuesNoProbes(t *testing.T) {
	f := &fakeAdapter{name: "no-probe", caps: makeFakeCaps("no-probe"), hcResult: fmt.Errorf("should not be called")}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "list"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources list failed: %v", err)
	}

	if f.hcCallCount() != 0 {
		t.Errorf("sources list should not call Healthcheck, got %d calls", f.hcCallCount())
	}
}

// --- NFR-CLI3-004: No registry drift ---

func TestSourcesListMatchesRegistry(t *testing.T) {
	fakes := []*fakeAdapter{
		{name: "alpha", caps: makeFakeCaps("alpha")},
		{name: "bravo", caps: makeFakeCaps("bravo")},
		{name: "charlie", caps: makeFakeCaps("charlie")},
	}
	reg, _ := buildSourcesTestRegistry(fakes...)

	// Get expected names from registry directly.
	expected := reg.List()

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "list", "--format", "json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources list failed: %v", err)
	}

	var result struct {
		Sources []struct {
			Name string `json:"name"`
		} `json:"sources"`
	}
	if parseErr := json.Unmarshal(buf.Bytes(), &result); parseErr != nil {
		t.Fatalf("parse JSON: %v", parseErr)
	}

	var gotNames []string
	for _, s := range result.Sources {
		gotNames = append(gotNames, s.Name)
	}

	if len(gotNames) != len(expected) {
		t.Fatalf("name count: got %d, want %d", len(gotNames), len(expected))
	}
	for i, name := range expected {
		if i >= len(gotNames) || gotNames[i] != name {
			t.Errorf("name[%d]: got %q, want %q", i, gotNames[i], name)
		}
	}
}

// --- Show subcommand ---

func TestSourcesShowRegistered(t *testing.T) {
	f := &fakeAdapter{
		name: "test-source",
		caps: types.Capabilities{
			SourceID:       "test-source",
			DisplayName:    "Test Source",
			DocTypes:       []types.DocType{types.DocTypeArticle},
			SupportedLangs: []string{"en", "ko"},
			RequiresAuth:   false,
			AuthEnvVars:    nil,
		},
	}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "show", "test-source"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources show failed: %v", err)
	}

	output := buf.String()
	for _, want := range []string{"test-source", "Test Source", "news", "en, ko", "n"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q: %s", want, output)
		}
	}
}

func TestSourcesShowWithRateLimit(t *testing.T) {
	f := &fakeAdapter{
		name: "rated",
		caps: types.Capabilities{
			SourceID:        "rated",
			DisplayName:     "Rated Source",
			DocTypes:        []types.DocType{types.DocTypeRepo},
			RateLimitPerMin: 60,
		},
	}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "show", "rated"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources show failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "60/min") {
		t.Errorf("expected rate limit in output: %s", output)
	}
	if !strings.Contains(output, "code") {
		t.Errorf("expected category 'code': %s", output)
	}
}

func TestSourcesShowEmptyDocTypesAndLangs(t *testing.T) {
	f := &fakeAdapter{
		name: "minimal",
		caps: types.Capabilities{
			SourceID:    "minimal",
			DisplayName: "Minimal",
		},
	}
	reg, _ := buildSourcesTestRegistry(f)

	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "show", "minimal"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources show failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "(none)") {
		t.Errorf("expected '(none)' for empty doc types: %s", output)
	}
	if !strings.Contains(output, "language-agnostic") {
		t.Errorf("expected 'language-agnostic': %s", output)
	}
}

// --- Existing CLI-002 compatibility tests ---

func TestSourcesSubcommandExists(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	_ = cmd.Execute()
	helpTxt := buf.String()

	if !strings.Contains(helpTxt, "sources") {
		t.Errorf("help output missing 'sources' subcommand: %s", helpTxt)
	}
}

func TestSourcesHelpOutput(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	var buf bytes.Buffer
	cmd := newRootCmdWithSources(reg)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sources", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sources --help failed: %v", err)
	}
	helpTxt := buf.String()
	for _, sub := range []string{"list", "status", "show"} {
		if !strings.Contains(helpTxt, sub) {
			t.Errorf("sources --help missing subcommand %q: %s", sub, helpTxt)
		}
	}
}

// --- Helper: newRootCmdWithSources ---

// newRootCmdWithSources creates a root command with a sources subcommand
// backed by the given registry. Avoids calling pipeline.BuildProductionRegistry
// in tests.
func newRootCmdWithSources(reg *adapters.Registry) *cobra.Command {
	var buf bytes.Buffer
	cmd := &cobra.Command{
		Use:           "usearch",
		Short:         "Universal Search CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Only register sources subcommand with the test registry.
	cmd.AddCommand(newSourcesCmd(registryFactory(reg)))

	return cmd
}
