// Package oidc_stub provides an in-process OIDC provider for unit tests.
// It serves discovery, JWKS, and token-signing endpoints without external dependencies.
// SPEC-AUTH-001 D7: No testcontainers, no real Keycloak.
package oidc_stub

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Stub is an in-process OIDC provider for testing.
type Stub struct {
	Server     *httptest.Server
	KeyID      string
	privateKey *rsa.PrivateKey
	issuer     string
}

// New creates and starts an in-process OIDC stub server (HTTP).
// The caller must call Stub.Close() when done.
func New() *Stub {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("oidc_stub: failed to generate RSA key: %v", err))
	}

	s := &Stub{
		KeyID:      "stub-key-1",
		privateKey: key,
	}

	mux := http.NewServeMux()
	s.Server = httptest.NewServer(mux)
	s.issuer = s.Server.URL

	mux.HandleFunc("/.well-known/openid-configuration", s.handleDiscovery)
	mux.HandleFunc("/jwks", s.handleJWKS)

	return s
}

// NewTLS creates and starts an in-process OIDC stub server (HTTPS).
// Use this for tests that require HTTPS issuer validation.
func NewTLS() *Stub {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("oidc_stub: failed to generate RSA key: %v", err))
	}

	s := &Stub{
		KeyID:      "stub-key-1",
		privateKey: key,
	}

	mux := http.NewServeMux()
	s.Server = httptest.NewTLSServer(mux)
	s.issuer = s.Server.URL

	mux.HandleFunc("/.well-known/openid-configuration", s.handleDiscovery)
	mux.HandleFunc("/jwks", s.handleJWKS)

	return s
}

// Close shuts down the stub server.
func (s *Stub) Close() {
	s.Server.Close()
}

// Issuer returns the issuer URL for this stub.
func (s *Stub) Issuer() string {
	return s.issuer
}

// IssueToken creates and signs a JWT with the given claims and TTL.
// Standard claims (iss, aud, iat, exp, nbf) are set automatically unless
// explicitly provided in the claims map.
func (s *Stub) IssueToken(claims map[string]interface{}, ttl time.Duration) (string, error) {
	now := time.Now()
	mapClaims := make(map[string]interface{})

	// Copy user-provided claims
	for k, v := range claims {
		mapClaims[k] = v
	}

	// Set standard claims with defaults
	if _, ok := mapClaims["iss"]; !ok {
		mapClaims["iss"] = s.issuer
	}
	if _, ok := mapClaims["iat"]; !ok {
		mapClaims["iat"] = now.Unix()
	}
	if _, ok := mapClaims["exp"]; !ok {
		mapClaims["exp"] = now.Add(ttl).Unix()
	}
	if _, ok := mapClaims["nbf"]; !ok {
		mapClaims["nbf"] = now.Unix()
	}
	if _, ok := mapClaims["sub"]; !ok {
		mapClaims["sub"] = "test-user"
	}
	if _, ok := mapClaims["aud"]; !ok {
		mapClaims["aud"] = "usearch-api"
	}
	if _, ok := mapClaims["jti"]; !ok {
		mapClaims["jti"] = fmt.Sprintf("tok-%d", now.UnixNano())
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(mapClaims))
	token.Header["kid"] = s.KeyID

	return token.SignedString(s.privateKey)
}

func (s *Stub) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	doc := map[string]interface{}{
		"issuer":                                s.issuer,
		"jwks_uri":                              s.issuer + "/jwks",
		"authorization_endpoint":                s.issuer + "/auth",
		"token_endpoint":                        s.issuer + "/token",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

func (s *Stub) handleJWKS(w http.ResponseWriter, r *http.Request) {
	pubKey := &s.privateKey.PublicKey

	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"kid": s.KeyID,
				"use": "sig",
				"alg": "RS256",
				"n":   encodeBigInt(pubKey.N),
				"e":   encodeInt(pubKey.E),
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwks)
}

// encodeBase64URL encodes bytes using URL-safe base64 without padding.
func encodeBase64URL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// encodeBigInt encodes a big.Int as URL-safe base64 without leading zeros.
func encodeBigInt(n *big.Int) string {
	return encodeBase64URL(n.Bytes())
}

// encodeInt encodes an int as URL-safe base64 big-endian without leading zeros.
func encodeInt(n int) string {
	b := make([]byte, 4)
	b[0] = byte(n >> 24)
	b[1] = byte(n >> 16)
	b[2] = byte(n >> 8)
	b[3] = byte(n)
	// Trim leading zeros
	i := 0
	for i < len(b)-1 && b[i] == 0 {
		i++
	}
	return encodeBase64URL(b[i:])
}
