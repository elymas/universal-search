package naver

import (
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestInterfaceConformance verifies that *Adapter implements types.Adapter at
// compile time. The runtime check here catches any stale compile-time assertion
// introduced by the var _ pattern.
func TestInterfaceConformance(t *testing.T) {
	t.Parallel()
	var _ types.Adapter = (*Adapter)(nil)
}

// TestNew_MissingCredentials verifies that New returns an error when neither
// Options nor environment variables provide credentials. REQ-ADP8-001.
func TestNew_MissingCredentials(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv.
	// Unset env vars; rely on opts being empty.
	t.Setenv("NAVER_CLIENT_ID", "")
	t.Setenv("NAVER_CLIENT_SECRET", "")

	_, err := New(Options{})
	if err == nil {
		t.Fatal("New() error = nil, want non-nil when credentials missing")
	}
	if !strings.Contains(err.Error(), "NAVER_CLIENT_ID") {
		t.Errorf("New() error = %q, want error mentioning NAVER_CLIENT_ID", err.Error())
	}
}

// TestNew_MissingSecret verifies that New returns an error when only the secret
// is missing. REQ-ADP8-001.
func TestNew_MissingSecret(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv.
	t.Setenv("NAVER_CLIENT_ID", "")
	t.Setenv("NAVER_CLIENT_SECRET", "")

	_, err := New(Options{ClientID: "test-id"})
	if err == nil {
		t.Fatal("New() error = nil, want non-nil when secret missing")
	}
	if !strings.Contains(err.Error(), "NAVER_CLIENT_SECRET") {
		t.Errorf("New() error = %q, want error mentioning NAVER_CLIENT_SECRET", err.Error())
	}
}

// TestNew_WithOptions verifies successful construction when credentials are
// provided via Options. REQ-ADP8-001.
func TestNew_WithOptions(t *testing.T) {
	t.Parallel()
	a, err := New(Options{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if a == nil {
		t.Fatal("New() returned nil adapter")
	}
}

// TestName verifies Name returns "naver". REQ-ADP8-001.
func TestName(t *testing.T) {
	t.Parallel()
	a, err := New(Options{ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := a.Name(); got != "naver" {
		t.Errorf("Name() = %q, want %q", got, "naver")
	}
}

// TestCapabilities verifies the Capabilities descriptor fields. REQ-ADP8-001.
func TestCapabilities(t *testing.T) {
	t.Parallel()
	a, err := New(Options{ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	caps := a.Capabilities()

	if caps.SourceID != "naver" {
		t.Errorf("Capabilities().SourceID = %q, want %q", caps.SourceID, "naver")
	}
	if caps.DisplayName != "Naver" {
		t.Errorf("Capabilities().DisplayName = %q, want %q", caps.DisplayName, "Naver")
	}
	if !caps.RequiresAuth {
		t.Error("Capabilities().RequiresAuth = false, want true")
	}
	if len(caps.AuthEnvVars) != 2 {
		t.Errorf("Capabilities().AuthEnvVars = %v, want 2 entries", caps.AuthEnvVars)
	}
	// Check both env vars declared.
	authVarSet := make(map[string]bool)
	for _, v := range caps.AuthEnvVars {
		authVarSet[v] = true
	}
	if !authVarSet["NAVER_CLIENT_ID"] {
		t.Error("AuthEnvVars missing NAVER_CLIENT_ID")
	}
	if !authVarSet["NAVER_CLIENT_SECRET"] {
		t.Error("AuthEnvVars missing NAVER_CLIENT_SECRET")
	}

	// SupportedLangs should be ["ko"].
	if len(caps.SupportedLangs) != 1 || caps.SupportedLangs[0] != "ko" {
		t.Errorf("Capabilities().SupportedLangs = %v, want [ko]", caps.SupportedLangs)
	}

	// DefaultMaxResults should be 25.
	if caps.DefaultMaxResults != 25 {
		t.Errorf("Capabilities().DefaultMaxResults = %d, want 25", caps.DefaultMaxResults)
	}
}

// TestHealthcheck_Success verifies Healthcheck succeeds when the target is
// reachable. Injects a local listener via Options.HealthcheckTarget.
func TestHealthcheck_Success(t *testing.T) {
	t.Parallel()
	// Start a TCP listener on a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer ln.Close()

	a, err := New(Options{
		ClientID:          "id",
		ClientSecret:      "secret",
		HealthcheckTarget: ln.Addr().String(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.Healthcheck(ctx); err != nil {
		t.Errorf("Healthcheck() error = %v, want nil", err)
	}
}

// TestHealthcheck_Failure verifies Healthcheck returns an error when the target
// is unreachable. Uses a port known to be closed (loopback, unused).
func TestHealthcheck_Failure(t *testing.T) {
	t.Parallel()
	a, err := New(Options{
		ClientID:          "id",
		ClientSecret:      "secret",
		HealthcheckTarget: "127.0.0.1:1", // port 1 is privileged, always refused
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := a.Healthcheck(ctx); err == nil {
		t.Error("Healthcheck() error = nil, want non-nil for unreachable target")
	}
}

// TestSearch_EmptyQuery verifies Search rejects empty queries. REQ-ADP8-008.
func TestSearch_EmptyQuery(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil) // never called
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{Text: ""})
	if err == nil {
		t.Fatal("Search() error = nil, want ErrInvalidQuery")
	}
	var se *types.SourceError
	if !isSourceError(err, &se) {
		t.Fatalf("Search() error type = %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Search() category = %v, want CategoryPermanent", se.Category)
	}
}

// TestSearch_WhitespaceOnlyQuery verifies Search rejects whitespace-only queries.
func TestSearch_WhitespaceOnlyQuery(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{Text: "   \t\n"})
	if err == nil {
		t.Fatal("Search() error = nil, want ErrInvalidQuery")
	}
}

// newTestAdapter is a helper constructing an Adapter with the given base URL
// for all verticals, and dummy credentials.
func newTestAdapter(t *testing.T, baseURL string) *Adapter {
	t.Helper()
	a, err := New(Options{
		ClientID:       "test-id",
		ClientSecret:   "test-secret",
		BaseURLBlog:    baseURL,
		BaseURLNews:    baseURL,
		BaseURLWeb:     baseURL,
		BaseURLShop:    baseURL,
		BaseURLDataLab: baseURL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return a
}

// isSourceError is a helper that casts err to *types.SourceError.
func isSourceError(err error, target **types.SourceError) bool {
	if err == nil {
		return false
	}
	se, ok := err.(*types.SourceError)
	if ok && target != nil {
		*target = se
	}
	return ok
}
