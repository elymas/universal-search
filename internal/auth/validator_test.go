package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/auth/testdata/oidc_stub"
	"github.com/elymas/universal-search/internal/deepagent/costguard"
)

// setupTestAuth creates a TLS OIDC stub, discovers it, and creates a validator + middleware.
func setupTestAuth(t *testing.T, cfg Config) (*oidc_stub.Stub, *Validator, *Middleware) {
	t.Helper()

	stub := oidc_stub.NewTLS()
	t.Cleanup(stub.Close)

	client := stub.Server.Client()
	result, err := DiscoverProviderWithClient(context.Background(), stub.Issuer(), 10*time.Second, true, client)
	if err != nil {
		t.Fatalf("DiscoverProvider failed: %v", err)
	}

	validator := NewValidator(result.Provider, cfg.OIDC.Audience, cfg.ClockSkew)
	middleware := NewMiddleware(validator, cfg)

	return stub, validator, middleware
}

func defaultTestConfig() Config {
	cfg := DefaultConfig()
	cfg.OIDC.AllowPrivateIssuer = true
	cfg.OIDC.Audience = []string{"usearch-api"}
	cfg.Tenant.Mode = TenantModeStatic
	cfg.Tenant.DefaultTenantID = "default"
	return cfg
}

func TestValidTokenInjectsSubIntoUserIDKey(t *testing.T) {
	cfg := defaultTestConfig()
	stub, _, mw := setupTestAuth(t, cfg)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "alice@example.com",
		"jti": "tok-001",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := costguard.UserIDFromContext(r.Context())
		if userID != "alice@example.com" {
			t.Errorf("UserIDKey = %q, want %q", userID, "alice@example.com")
		}
		tenantID := costguard.TenantIDFromContext(r.Context())
		if tenantID != "default" {
			t.Errorf("TenantIDKey = %q, want %q", tenantID, "default")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestValidTokenInjectsClaimsIntoClaimsKey(t *testing.T) {
	cfg := defaultTestConfig()
	stub, _, mw := setupTestAuth(t, cfg)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "alice@example.com",
		"jti": "tok-001",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			t.Fatal("ClaimsKey is nil")
		}
		if claims.Subject != "alice@example.com" {
			t.Errorf("claims.Subject = %q, want %q", claims.Subject, "alice@example.com")
		}
		if jti, _ := claims.Raw["jti"].(string); jti != "tok-001" {
			t.Errorf("claims.Raw[jti] = %q, want %q", jti, "tok-001")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestInvalidSignatureRejected(t *testing.T) {
	cfg := defaultTestConfig()
	stub, _, mw := setupTestAuth(t, cfg)

	// Issue token with stub's key, then tamper with it
	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "alice@example.com",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	// Tamper with the token (change one character in payload)
	parts := splitToken(token)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	tampered := parts[0] + "." + parts[1][:len(parts[1])-1] + "X." + parts[2]

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid signature")
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+tampered)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	cfg := defaultTestConfig()
	stub, _, mw := setupTestAuth(t, cfg)

	// Issue an already-expired token
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
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "expired" {
		t.Errorf("error = %q, want %q", body["error"], "expired")
	}
}

func TestInvalidAudienceRejected(t *testing.T) {
	cfg := defaultTestConfig()
	stub, _, mw := setupTestAuth(t, cfg)

	// Issue token with wrong audience
	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "alice@example.com",
		"aud": "wrong-audience",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid audience")
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMissingSubRejected(t *testing.T) {
	cfg := defaultTestConfig()
	stub, validator, _ := setupTestAuth(t, cfg)

	// Issue token without sub claim
	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	_, err = validator.Validate(context.Background(), token)
	if err == nil {
		t.Error("expected error for empty sub, got nil")
	}
	reason := FailureReasonFromError(err)
	if reason != ReasonMalformed {
		t.Errorf("reason = %q, want %q", reason, ReasonMalformed)
	}
}

func TestForwardCompatWithDeep004IdentityMiddleware(t *testing.T) {
	cfg := defaultTestConfig()
	stub, _, mw := setupTestAuth(t, cfg)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "bob@example.com",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	// Create DEEP-004 IdentityMiddleware
	cgMw := &costguard.Middleware{}

	// Chain: JWT middleware -> costguard.IdentityMiddleware -> handler
	jwtHandler := mw.JWTValidationMiddleware(cgMw.IdentityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := costguard.UserIDFromContext(r.Context())
		if userID != "bob@example.com" {
			t.Errorf("UserIDKey = %q, want %q (JWT sub should win)", userID, "bob@example.com")
		}
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	jwtHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestIdentityBridgeContextKeyTakesPrecedenceOverHeader(t *testing.T) {
	// §5.5: JWT sub takes precedence over X-User-Id header (anti-spoofing)
	cfg := defaultTestConfig()
	stub, _, mw := setupTestAuth(t, cfg)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "bob@example.com",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	cgMw := &costguard.Middleware{}

	jwtHandler := mw.JWTValidationMiddleware(cgMw.IdentityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := costguard.UserIDFromContext(r.Context())
		if userID != "bob@example.com" {
			t.Errorf("UserIDKey = %q, want %q (JWT should override X-User-Id)", userID, "bob@example.com")
		}
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("POST", "/deep", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-User-Id", "malicious@attacker.com") // Should be ignored
	rec := httptest.NewRecorder()

	jwtHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestIdentityBridgeFallsBackToHeader(t *testing.T) {
	// DEEP-004 V1 path: disabled mode -> JWT middleware bypasses entirely
	// costguard.IdentityMiddleware reads X-User-Id header
	cfg := defaultTestConfig()
	cfg.Mode = ModeDisabled
	_, _, mw := setupTestAuth(t, cfg)

	cgMw := &costguard.Middleware{}

	// In disabled mode, JWT middleware bypasses and costguard handles identity
	jwtHandler := mw.JWTValidationMiddleware(cgMw.IdentityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := costguard.UserIDFromContext(r.Context())
		if userID != "dave@example.com" {
			t.Errorf("UserIDKey = %q, want %q (header fallback)", userID, "dave@example.com")
		}
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("X-User-Id", "dave@example.com")
	rec := httptest.NewRecorder()

	jwtHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestClockSkewToleranceApplied(t *testing.T) {
	cfg := defaultTestConfig()
	stub, _, mw := setupTestAuth(t, cfg)

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("25s_future_PASS", func(t *testing.T) {
		// Token issued 25s in the future — within 30s clock skew
		token, err := stub.IssueToken(map[string]interface{}{
			"sub": "alice@example.com",
			"iat": time.Now().Add(25 * time.Second).Unix(),
			"nbf": time.Now().Add(25 * time.Second).Unix(),
			"exp": time.Now().Add(5*time.Minute + 25*time.Second).Unix(),
		}, 0)
		if err != nil {
			t.Fatalf("IssueToken failed: %v", err)
		}

		req := httptest.NewRequest("POST", "/query", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d (25s skew should be within tolerance)", rec.Code, http.StatusOK)
		}
	})

	t.Run("35s_future_FAIL", func(t *testing.T) {
		// Token issued 35s in the future — exceeds 30s clock skew
		token, err := stub.IssueToken(map[string]interface{}{
			"sub": "alice@example.com",
			"iat": time.Now().Add(35 * time.Second).Unix(),
			"nbf": time.Now().Add(35 * time.Second).Unix(),
			"exp": time.Now().Add(5*time.Minute + 35*time.Second).Unix(),
		}, 0)
		if err != nil {
			t.Fatalf("IssueToken failed: %v", err)
		}

		req := httptest.NewRequest("POST", "/query", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d (35s skew should exceed tolerance)", rec.Code, http.StatusUnauthorized)
		}
	})
}

// Phase D tests

func TestPermissiveModeMissingTokenInjectsAnonymous(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Mode = ModePermissive
	_, _, mw := setupTestAuth(t, cfg)

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := costguard.UserIDFromContext(r.Context())
		if userID != "anonymous" {
			t.Errorf("UserIDKey = %q, want %q", userID, "anonymous")
		}
		claims := ClaimsFromContext(r.Context())
		if claims != nil {
			t.Errorf("ClaimsKey should be nil for anonymous, got %+v", claims)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestStrictModeMissingTokenReturns401(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Mode = ModeStrict
	_, _, mw := setupTestAuth(t, cfg)

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for missing token in strict mode")
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "missing_token" {
		t.Errorf("error = %q, want %q", body["error"], "missing_token")
	}
}

func TestDisabledModeBypassesEntirely(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Mode = ModeDisabled
	_, _, mw := setupTestAuth(t, cfg)

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In disabled mode, JWT middleware should not inject anything
		// costguard.IdentityMiddleware handles it via X-User-Id
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("X-User-Id", "dave@example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestTenantClaimModeExtractsFromClaim(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Tenant.Mode = TenantModeClaim
	cfg.Tenant.ClaimPath = "org_id"
	stub, _, mw := setupTestAuth(t, cfg)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub":    "carol",
		"org_id": "team-alpha",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := costguard.TenantIDFromContext(r.Context())
		if tenantID != "team-alpha" {
			t.Errorf("TenantIDKey = %q, want %q", tenantID, "team-alpha")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/deep", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestTenantClaimModeFallsBackToDefault(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Tenant.Mode = TenantModeClaim
	cfg.Tenant.ClaimPath = "org_id"
	cfg.Tenant.DefaultTenantID = "default"
	stub, _, mw := setupTestAuth(t, cfg)

	// Token without org_id claim
	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "carol",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := costguard.TenantIDFromContext(r.Context())
		if tenantID != "default" {
			t.Errorf("TenantIDKey = %q, want %q (fallback)", tenantID, "default")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/deep", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestTenantHeaderModeReadsHeader(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Tenant.Mode = TenantModeHeader
	cfg.Tenant.DefaultTenantID = "default"
	stub, _, mw := setupTestAuth(t, cfg)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "carol",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := costguard.TenantIDFromContext(r.Context())
		if tenantID != "header-tenant" {
			t.Errorf("TenantIDKey = %q, want %q", tenantID, "header-tenant")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/deep", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-Id", "header-tenant")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestTenantStaticModeUsesDefault(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Tenant.Mode = TenantModeStatic
	cfg.Tenant.DefaultTenantID = "my-tenant"
	stub, _, mw := setupTestAuth(t, cfg)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "carol",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := costguard.TenantIDFromContext(r.Context())
		if tenantID != "my-tenant" {
			t.Errorf("TenantIDKey = %q, want %q", tenantID, "my-tenant")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/deep", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHealthzBypassesAuth(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Mode = ModeStrict
	_, _, mw := setupTestAuth(t, cfg)

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (allowlist bypass)", rec.Code, http.StatusOK)
	}
}

func TestMetricsBypassesAuth(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Mode = ModeStrict
	_, _, mw := setupTestAuth(t, cfg)

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (allowlist bypass)", rec.Code, http.StatusOK)
	}
}
