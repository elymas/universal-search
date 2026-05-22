package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/auth/testdata/oidc_stub"
)

func TestOIDCStubServesDiscovery(t *testing.T) {
	stub := oidc_stub.New()
	defer stub.Close()

	resp, err := http.Get(stub.Issuer() + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatalf("discovery request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var doc map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatalf("failed to decode discovery document: %v", err)
	}

	// Verify RFC-required fields
	if iss, _ := doc["issuer"].(string); iss != stub.Issuer() {
		t.Errorf("issuer = %q, want %q", iss, stub.Issuer())
	}
	if jwksURI, _ := doc["jwks_uri"].(string); jwksURI == "" {
		t.Error("jwks_uri is empty")
	}
}

func TestOIDCStubServesJWKS(t *testing.T) {
	stub := oidc_stub.New()
	defer stub.Close()

	resp, err := http.Get(stub.Issuer() + "/jwks")
	if err != nil {
		t.Fatalf("JWKS request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var jwks map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		t.Fatalf("failed to decode JWKS: %v", err)
	}

	keys, ok := jwks["keys"].([]interface{})
	if !ok || len(keys) == 0 {
		t.Fatal("JWKS keys array is empty or missing")
	}

	key, _ := keys[0].(map[string]interface{})
	if kty, _ := key["kty"].(string); kty != "RSA" {
		t.Errorf("kty = %q, want RSA", kty)
	}
	if kid, _ := key["kid"].(string); kid != stub.KeyID {
		t.Errorf("kid = %q, want %q", kid, stub.KeyID)
	}
	if n, _ := key["n"].(string); n == "" {
		t.Error("RSA modulus 'n' is empty")
	}
	if e, _ := key["e"].(string); e == "" {
		t.Error("RSA exponent 'e' is empty")
	}
}

func TestOIDCStubIssueTokenHelper(t *testing.T) {
	stub := oidc_stub.New()
	defer stub.Close()

	tokenStr, err := stub.IssueToken(map[string]interface{}{
		"sub": "alice@example.com",
		"jti": "tok-001",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	if tokenStr == "" {
		t.Fatal("token string is empty")
	}

	// Token should have 3 parts (header.payload.signature)
	parts := splitToken(tokenStr)
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}
}

func TestDiscoveryFetchesProviderMetadata(t *testing.T) {
	stub := oidc_stub.NewTLS()
	defer stub.Close()

	ctx := context.Background()
	client := stub.Server.Client()
	result, err := DiscoverProviderWithClient(ctx, stub.Issuer(), 10*time.Second, true, client)
	if err != nil {
		t.Fatalf("DiscoverProvider failed: %v", err)
	}

	if result.Provider == nil {
		t.Fatal("Provider is nil")
	}
}

func TestDiscoveryFailsOnIssuerMismatch(t *testing.T) {
	stub := oidc_stub.NewTLS()
	defer stub.Close()

	// Serve a discovery doc with a different issuer
	badServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			doc := map[string]interface{}{
				"issuer":   "https://different-issuer.example.com",
				"jwks_uri": stub.Issuer() + "/jwks",
			}
			json.NewEncoder(w).Encode(doc)
			return
		}
		if r.URL.Path == "/jwks" {
			client := stub.Server.Client()
			resp, err := client.Get(stub.Issuer() + "/jwks")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()
			resp.Write(w)
			return
		}
	}))
	defer badServer.Close()

	ctx := context.Background()
	client := badServer.Client()
	_, err := DiscoverProviderWithClient(ctx, badServer.URL, 10*time.Second, true, client)
	// go-oidc validates issuer mismatch internally
	if err == nil {
		t.Error("expected error for issuer mismatch, got nil")
	}
}

func TestDiscoveryFatalExitOnFailure(t *testing.T) {
	ctx := context.Background()
	_, err := DiscoverProvider(ctx, "https://nonexistent.invalid/issuer", 2*time.Second, false)
	if err == nil {
		t.Error("expected error for unreachable issuer, got nil")
	}
}

func TestHttpSchemeIssuerRejected(t *testing.T) {
	err := validateIssuerURL("http://example.com/realms/team", false)
	if err == nil {
		t.Error("expected http:// scheme to be rejected, got nil error")
	}
}

func TestNonAllowlistedHostRejected(t *testing.T) {
	err := validateIssuerURL("ftp://example.com/realms/team", false)
	if err == nil {
		t.Error("expected non-https scheme to be rejected")
	}
}

func TestPrivateIPIssuerRejected(t *testing.T) {
	testCases := []struct {
		name string
		host string
	}{
		{"RFC1918", "192.168.1.100"},
		{"loopback", "127.0.0.1"},
		{"link-local", "169.254.1.1"},
		{"IPv6 ULA", "fc00::1"},
		{"IPv6 loopback", "::1"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkPrivateIP(tc.host)
			if err == nil {
				t.Errorf("expected private IP %q to be rejected", tc.host)
			}
		})
	}
}

func TestPrivateIPIssuerAllowedWhenDevFlagSet(t *testing.T) {
	err := validateIssuerURL("https://127.0.0.1:9001", true)
	if err != nil {
		t.Errorf("expected private IP to be allowed with dev flag, got: %v", err)
	}
}

func TestStartupValidationFatalExit(t *testing.T) {
	testCases := []struct {
		name   string
		issuer string
	}{
		{"http scheme", "http://example.com"},
		{"invalid URL", "://missing-scheme"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateIssuerURL(tc.issuer, false)
			if err == nil {
				t.Errorf("expected validation failure for %q", tc.issuer)
			}
		})
	}
}

// splitToken splits a JWT string by '.' separator.
func splitToken(token string) []string {
	var parts []string
	start := 0
	for i, c := range token {
		if c == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}
