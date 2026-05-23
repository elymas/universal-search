package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/pkg/types"
)

// TestHandleAdapters verifies the full A1+A3+A4 chain: loopback middleware +
// registry snapshot + JSON handler.
func TestHandleAdapters(t *testing.T) {
	// Set up a registry with 9 adapters matching production count.
	reg := adapters.NewRegistry(nil)
	adapterNames := []string{
		"reddit", "hackernews", "arxiv", "github", "youtube",
		"bluesky", "searxng", "naver", "koreanews",
	}
	for _, n := range adapterNames {
		a := &testAdapter{
			name: n,
			caps: types.Capabilities{
				SourceID:    n,
				DisplayName: n,
				DocTypes:    []types.DocType{types.DocTypeOther},
			},
		}
		if err := reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
			t.Fatalf("Register(%q): %v", n, err)
		}
	}

	// Build the handler chain: loopback middleware -> adapters handler.
	handler := LoopbackOnly(NewAdaptersHandler(reg))

	t.Run("loopback IP returns 200 with JSON array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/adapters", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200", rec.Code)
		}

		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Errorf("Content-Type: got %q, want application/json", ct)
		}

		var views []adapters.AdapterAdminView
		if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if got, want := len(views), 9; got != want {
			t.Errorf("got %d adapters, want %d", got, want)
		}

		// Verify JSON structure: each entry has required fields.
		for _, v := range views {
			if v.ID == "" {
				t.Error("adapter view has empty ID")
			}
			if v.Status == "" {
				t.Errorf("adapter %q has empty Status", v.ID)
			}
		}
	})

	t.Run("external IP returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/adapters", nil)
		req.RemoteAddr = "192.168.1.42:12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("got status %d, want 403", rec.Code)
		}
	})

	t.Run("no secret values in response", func(t *testing.T) {
		// Register an adapter with a known secret.
		const secretVal = "sk-test-secret-xyz789"
		const envKey = "USEARCH_HANDLER_TEST_KEY"
		os.Setenv(envKey, secretVal)
		t.Cleanup(func() { os.Unsetenv(envKey) })

		authReg := adapters.NewRegistry(nil)
		a := &testAdapter{
			name: "secret-test",
			caps: types.Capabilities{
				SourceID:     "secret-test",
				DisplayName:  "Secret Test",
				RequiresAuth: true,
				AuthEnvVars:  []string{envKey},
			},
		}
		if err := authReg.Register(a); err != nil {
			t.Fatalf("Register: %v", err)
		}

		handler := LoopbackOnly(NewAdaptersHandler(authReg))
		req := httptest.NewRequest(http.MethodGet, "/api/admin/adapters", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		body := rec.Body.String()
		if strings.Contains(body, secretVal) {
			t.Errorf("response contains secret value: %s", body)
		}
	})

	t.Run("non-admin route unaffected by loopback middleware", func(t *testing.T) {
		// This verifies A2: admin middleware only applies to /api/admin/* routes.
		// We use a separate mux to demonstrate that non-admin routes work fine
		// from external IPs.
		mux := http.NewServeMux()
		mux.Handle("/api/admin/adapters", LoopbackOnly(NewAdaptersHandler(reg)))
		mux.Handle("/query/stream", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("stream"))
		}))

		// External IP should get 403 on admin endpoint.
		adminReq := httptest.NewRequest(http.MethodGet, "/api/admin/adapters", nil)
		adminReq.RemoteAddr = "10.0.0.1:9999"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, adminReq)
		if rec.Code != http.StatusForbidden {
			t.Errorf("admin endpoint: got %d, want 403", rec.Code)
		}

		// Same external IP should get 200 on non-admin endpoint.
		streamReq := httptest.NewRequest(http.MethodPost, "/query/stream", nil)
		streamReq.RemoteAddr = "10.0.0.1:9999"
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, streamReq)
		if rec.Code != http.StatusOK {
			t.Errorf("/query/stream: got %d, want 200", rec.Code)
		}
	})
}

// --- B1: POST /api/admin/adapters/{id}/resync ---

func TestHandleResync(t *testing.T) {
	t.Parallel()

	t.Run("valid adapter resync returns 200 with updated state", func(t *testing.T) {
		t.Parallel()
		reg := adapters.NewRegistry(nil)
		a := &testAdapter{name: "reddit", caps: types.Capabilities{SourceID: "reddit"}}
		if err := reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
			t.Fatalf("Register: %v", err)
		}

		handler := LoopbackOnly(NewResyncHandler(reg))
		req := httptest.NewRequest(http.MethodPost, "/api/admin/adapters/reddit/resync", nil)
		req.SetPathValue("id", "reddit")
		req.RemoteAddr = "127.0.0.1:12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Errorf("Content-Type: got %q, want application/json", ct)
		}

		var view adapters.AdapterAdminView
		if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if view.ID != "reddit" {
			t.Errorf("view.ID: got %q, want %q", view.ID, "reddit")
		}
		if view.Status != "connected" {
			t.Errorf("view.Status: got %q, want %q", view.Status, "connected")
		}
	})

	t.Run("unknown adapter ID returns 404 with structured error", func(t *testing.T) {
		t.Parallel()
		reg := adapters.NewRegistry(nil)

		handler := LoopbackOnly(NewResyncHandler(reg))
		req := httptest.NewRequest(http.MethodPost, "/api/admin/adapters/ghost/resync", nil)
		req.SetPathValue("id", "ghost")
		req.RemoteAddr = "127.0.0.1:12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("got status %d, want 404; body: %s", rec.Code, rec.Body.String())
		}

		var errResp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if errResp["error"] != "adapter_not_found" {
			t.Errorf("error: got %q, want %q", errResp["error"], "adapter_not_found")
		}
		if errResp["adapter_id"] != "ghost" {
			t.Errorf("adapter_id: got %q, want %q", errResp["adapter_id"], "ghost")
		}
	})

	t.Run("upstream failure returns 502 with sanitized error", func(t *testing.T) {
		t.Parallel()
		reg := adapters.NewRegistry(nil)
		a := &failingHealthAdapter{
			testAdapter: testAdapter{name: "drive", caps: types.Capabilities{SourceID: "drive"}},
			healthErr:   context.DeadlineExceeded,
		}
		if err := reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
			t.Fatalf("Register: %v", err)
		}

		handler := LoopbackOnly(NewResyncHandler(reg))
		req := httptest.NewRequest(http.MethodPost, "/api/admin/adapters/drive/resync", nil)
		req.SetPathValue("id", "drive")
		req.RemoteAddr = "127.0.0.1:12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("got status %d, want 502; body: %s", rec.Code, rec.Body.String())
		}

		body := rec.Body.String()

		// Verify structured error format.
		var errResp map[string]string
		if err := json.Unmarshal([]byte(body), &errResp); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if errResp["error"] != "upstream_adapter_error" {
			t.Errorf("error: got %q, want %q", errResp["error"], "upstream_adapter_error")
		}
		if errResp["adapter_id"] != "drive" {
			t.Errorf("adapter_id: got %q, want %q", errResp["adapter_id"], "drive")
		}
		if errResp["detail"] == "" {
			t.Error("detail: expected non-empty sanitized message")
		}

		// NO raw upstream stack traces in error responses.
		if strings.Contains(body, "goroutine") {
			t.Errorf("response contains goroutine stack trace: %s", body)
		}
		if strings.Contains(body, "/usr/local/go/") {
			t.Errorf("response contains internal path: %s", body)
		}
	})
}

// --- B2: POST /api/admin/adapters/{id}/toggle ---

func TestHandleToggle(t *testing.T) {
	t.Parallel()

	t.Run("enable then disable roundtrip", func(t *testing.T) {
		t.Parallel()
		reg := adapters.NewRegistry(nil)
		a := &testAdapter{name: "hackernews", caps: types.Capabilities{SourceID: "hackernews"}}
		if err := reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
			t.Fatalf("Register: %v", err)
		}

		handler := LoopbackOnly(NewToggleHandler(reg))

		// First toggle: enabled -> disabled.
		req := httptest.NewRequest(http.MethodPost, "/api/admin/adapters/hackernews/toggle", nil)
		req.SetPathValue("id", "hackernews")
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("toggle 1: got status %d, want 200; body: %s", rec.Code, rec.Body.String())
		}

		var view1 adapters.AdapterAdminView
		if err := json.Unmarshal(rec.Body.Bytes(), &view1); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if view1.Status != "disabled" {
			t.Errorf("after first toggle: got status %q, want %q", view1.Status, "disabled")
		}

		// Second toggle: disabled -> enabled.
		req2 := httptest.NewRequest(http.MethodPost, "/api/admin/adapters/hackernews/toggle", nil)
		req2.SetPathValue("id", "hackernews")
		req2.RemoteAddr = "127.0.0.1:12345"
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusOK {
			t.Fatalf("toggle 2: got status %d, want 200; body: %s", rec2.Code, rec2.Body.String())
		}

		var view2 adapters.AdapterAdminView
		if err := json.Unmarshal(rec2.Body.Bytes(), &view2); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if view2.Status != "connected" {
			t.Errorf("after second toggle: got status %q, want %q", view2.Status, "connected")
		}
	})

	t.Run("unknown adapter ID returns 404", func(t *testing.T) {
		t.Parallel()
		reg := adapters.NewRegistry(nil)

		handler := LoopbackOnly(NewToggleHandler(reg))
		req := httptest.NewRequest(http.MethodPost, "/api/admin/adapters/nonexistent/toggle", nil)
		req.SetPathValue("id", "nonexistent")
		req.RemoteAddr = "127.0.0.1:12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("got status %d, want 404; body: %s", rec.Code, rec.Body.String())
		}

		var errResp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if errResp["error"] != "adapter_not_found" {
			t.Errorf("error: got %q, want %q", errResp["error"], "adapter_not_found")
		}
	})

	t.Run("toggle state persists in response", func(t *testing.T) {
		t.Parallel()
		reg := adapters.NewRegistry(nil)
		a := &testAdapter{name: "arxiv", caps: types.Capabilities{SourceID: "arxiv"}}
		if err := reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
			t.Fatalf("Register: %v", err)
		}

		toggleHandler := LoopbackOnly(NewToggleHandler(reg))
		adaptersHandler := LoopbackOnly(NewAdaptersHandler(reg))

		// Toggle to disable.
		req := httptest.NewRequest(http.MethodPost, "/api/admin/adapters/arxiv/toggle", nil)
		req.SetPathValue("id", "arxiv")
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		toggleHandler.ServeHTTP(rec, req)

		// Check state via list endpoint.
		listReq := httptest.NewRequest(http.MethodGet, "/api/admin/adapters", nil)
		listReq.RemoteAddr = "127.0.0.1:12345"
		listRec := httptest.NewRecorder()
		adaptersHandler.ServeHTTP(listRec, listReq)

		var views []adapters.AdapterAdminView
		if err := json.Unmarshal(listRec.Body.Bytes(), &views); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		var arxivView *adapters.AdapterAdminView
		for i := range views {
			if views[i].ID == "arxiv" {
				arxivView = &views[i]
				break
			}
		}
		if arxivView == nil {
			t.Fatal("arxiv adapter not found in list response")
		}
		if arxivView.Status != "disabled" {
			t.Errorf("arxiv status after toggle: got %q, want %q", arxivView.Status, "disabled")
		}
	})
}

// --- Shared test helpers ---

// testAdapter is a minimal types.Adapter for handler tests.
type testAdapter struct {
	name string
	caps types.Capabilities
}

func (a *testAdapter) Name() string                     { return a.name }
func (a *testAdapter) Capabilities() types.Capabilities { return a.caps }
func (a *testAdapter) Healthcheck(_ context.Context) error  { return nil }
func (a *testAdapter) Search(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
	return nil, nil
}

// failingHealthAdapter wraps testAdapter to return an error on Healthcheck.
type failingHealthAdapter struct {
	testAdapter
	healthErr error
}

func (a *failingHealthAdapter) Healthcheck(_ context.Context) error { return a.healthErr }
