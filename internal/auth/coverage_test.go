package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/auth/testdata/oidc_stub"
	"github.com/elymas/universal-search/internal/deepagent/costguard"
)

// Coverage tests for less-covered paths

func TestExtractBearerTokenEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"empty header", "", ""},
		{"basic auth", "Basic dXNlcjpwYXNz", ""},
		{"bearer no space", "Bearer", ""},
		{"bearer empty value", "Bearer ", ""},
		{"bearer with value", "Bearer token123", "token123"},
		{"case insensitive", "bearer token123", "token123"},
		{"BEARER uppercase", "BEARER token123", "token123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			got := extractBearerToken(req)
			if got != tt.want {
				t.Errorf("extractBearerToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmitAuthEvent(t *testing.T) {
	// Test emitAuthEvent with nil logger — should not panic
	emitAuthEvent(nil, "success", "hash123", "https://issuer", "usearch-api", 100*time.Millisecond)
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"169.254.1.1", true},
		{"fc00::1", true},
		{"::1", true},
		{"8.8.8.8", false},
		{"1.2.3.4", false},
		{"not-an-ip", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			if got := isPrivateIP(tt.ip); got != tt.want {
				t.Errorf("isPrivateIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestCheckPrivateIPHostnameResolution(t *testing.T) {
	// Test with a hostname that resolves
	err := checkPrivateIP("localhost")
	// localhost resolves to 127.0.0.1 or ::1 — should be rejected
	if err == nil {
		t.Error("expected localhost to be rejected as private IP")
	}

	// Test with empty hostname — should pass
	err = checkPrivateIP("")
	if err != nil {
		t.Errorf("expected empty hostname to pass, got: %v", err)
	}
}

func TestValidateNbfWithDifferentTypes(t *testing.T) {
	v := &Validator{clockSkew: 30 * time.Second}

	tests := []struct {
		name   string
		claims map[string]interface{}
		want   bool // true = expect no error
	}{
		{
			"no nbf",
			map[string]interface{}{"sub": "test"},
			true,
		},
		{
			"nbf float64 past",
			map[string]interface{}{"nbf": float64(time.Now().Add(-1 * time.Hour).Unix())},
			true,
		},
		{
			"nbf float64 future beyond skew",
			map[string]interface{}{"nbf": float64(time.Now().Add(60 * time.Second).Unix())},
			false,
		},
		{
			"nbf float64 within skew",
			map[string]interface{}{"nbf": float64(time.Now().Add(10 * time.Second).Unix())},
			true,
		},
		{
			"nbf wrong type",
			map[string]interface{}{"nbf": "not-a-number"},
			true, // wrong type is silently accepted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.validateNbf(tt.claims)
			if (err == nil) != tt.want {
				t.Errorf("validateNbf() error = %v, want no error: %v", err, tt.want)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		errMsg string
		want   FailureReason
	}{
		{"token is expired", ReasonExpired},
		{"audience mismatch", ReasonInvalidAudience},
		{"issuer mismatch", ReasonInvalidIssuer},
		{"signing method mismatch", ReasonInvalidSignature},
		{"key is invalid", ReasonInvalidSignature},
		{"nbf is in the future", ReasonInvalidNbf},
		{"some unknown error", ReasonMalformed},
	}

	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			err := classifyError(errors.New(tt.errMsg))
			reason := FailureReasonFromError(err)
			if reason != tt.want {
				t.Errorf("classifyError(%q) = %q, want %q", tt.errMsg, reason, tt.want)
			}
		})
	}
}

func TestExtractAudienceEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		claims map[string]interface{}
		want   []string
	}{
		{"missing aud", map[string]interface{}{}, nil},
		{"string aud", map[string]interface{}{"aud": "usearch-api"}, []string{"usearch-api"}},
		{"array aud", map[string]interface{}{"aud": []interface{}{"usearch-api", "other"}}, []string{"usearch-api", "other"}},
		{"wrong type aud", map[string]interface{}{"aud": 12345}, nil},
		{"mixed array aud", map[string]interface{}{"aud": []interface{}{"usearch-api", 123}}, []string{"usearch-api"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAudience(tt.claims)
			if len(got) != len(tt.want) {
				t.Errorf("extractAudience() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractAudience()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFailureReasonFromErrorNil(t *testing.T) {
	reason := FailureReasonFromError(nil)
	if reason != "" {
		t.Errorf("FailureReasonFromError(nil) = %q, want empty string", reason)
	}
}

func TestValidateEmptyToken(t *testing.T) {
	cfg := defaultTestConfig()
	stub, validator, _ := setupTestAuth(t, cfg)

	_ = stub // just need the validator

	_, err := validator.Validate(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty token")
	}
	reason := FailureReasonFromError(err)
	if reason != ReasonMalformed {
		t.Errorf("reason = %q, want %q", reason, ReasonMalformed)
	}
}

func TestValidateMalformedToken(t *testing.T) {
	cfg := defaultTestConfig()
	stub, validator, _ := setupTestAuth(t, cfg)

	_ = stub

	_, err := validator.Validate(context.Background(), "not-a-jwt")
	if err == nil {
		t.Error("expected error for malformed token")
	}
}

func TestExpiredTokenNoAnonymousFallback(t *testing.T) {
	// §5.4: expired token should NOT fall back to anonymous
	cfg := defaultTestConfig()
	cfg.Mode = ModePermissive // permissive mode — but expired = 401
	stub, _, mw := setupTestAuth(t, cfg)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "alice@example.com",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	}, 0)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for expired token")
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (expired token should 401 even in permissive mode)", rec.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "expired" {
		t.Errorf("error = %q, want %q", body["error"], "expired")
	}
}

func TestDiscoveryProviderWithEndSessionEndpoint(t *testing.T) {
	stub := oidc_stub.NewTLS()
	t.Cleanup(stub.Close)

	// Override discovery to include end_session_endpoint
	originalHandler := stub.Server.Config.Handler
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]interface{}{
			"issuer":                                stub.Issuer(),
			"jwks_uri":                              stub.Issuer() + "/jwks",
			"end_session_endpoint":                  stub.Issuer() + "/logout",
			"authorization_endpoint":                stub.Issuer() + "/auth",
			"token_endpoint":                        stub.Issuer() + "/token",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		json.NewEncoder(w).Encode(doc)
	})
	mux.Handle("/jwks", originalHandler)
	stub.Server.Config.Handler = mux

	client := stub.Server.Client()
	result, err := DiscoverProviderWithClient(context.Background(), stub.Issuer(), 10*time.Second, true, client)
	if err != nil {
		t.Fatalf("DiscoverProvider failed: %v", err)
	}

	if result.EndSessionEndpoint != stub.Issuer()+"/logout" {
		t.Errorf("EndSessionEndpoint = %q, want %q", result.EndSessionEndpoint, stub.Issuer()+"/logout")
	}
}

func TestAllowlistEndpoints(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Mode = ModeStrict
	_, _, mw := setupTestAuth(t, cfg)

	endpoints := []string{"/healthz", "/metrics", "/v1/auth/callback", "/v1/auth/login"}
	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", endpoint, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d for allowlisted endpoint %s", rec.Code, http.StatusOK, endpoint)
			}
		})
	}
}

func TestResolveTenantDefaultMode(t *testing.T) {
	// Test with unknown mode falls back to default
	tenantCfg := TenantConfig{
		Mode:            "unknown",
		DefaultTenantID: "fallback",
	}
	result := resolveTenant(tenantCfg, nil, nil)
	if result != "fallback" {
		t.Errorf("resolveTenant with unknown mode = %q, want %q", result, "fallback")
	}
}

func TestCostguardIdentityMiddlewarePreservesExistingContext(t *testing.T) {
	// Verify that when UserIDKey is already set in context,
	// IdentityMiddleware preserves it (forward-compat with JWT middleware)
	cgMw := &costguard.Middleware{}

	handler := cgMw.IdentityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := costguard.UserIDFromContext(r.Context())
		if userID != "jwt-user" {
			t.Errorf("UserIDKey = %q, want %q (JWT should override header)", userID, "jwt-user")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("X-User-Id", "header-user")

	// Simulate JWT middleware having already set the user ID
	ctx := context.WithValue(req.Context(), costguard.UserIDKey, "jwt-user")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
