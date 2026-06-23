package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// DiscoveryResult holds the cached OIDC provider metadata.
type DiscoveryResult struct {
	Provider           *oidc.Provider
	EndSessionEndpoint string
}

// DiscoverProvider performs OIDC discovery against the given issuer URL.
// REQ-AUTH1-001: startup-time discovery with issuer validation.
// allowPrivateIssuer bypasses private IP block for dev/CI environments (D8).
func DiscoverProvider(ctx context.Context, issuerURL string, timeout time.Duration, allowPrivateIssuer bool) (*DiscoveryResult, error) {
	return DiscoverProviderWithClient(ctx, issuerURL, timeout, allowPrivateIssuer, nil)
}

// DiscoverProviderWithClient performs OIDC discovery with a custom HTTP client.
// If client is nil, the default http.Client is used.
func DiscoverProviderWithClient(ctx context.Context, issuerURL string, timeout time.Duration, allowPrivateIssuer bool, client *http.Client) (*DiscoveryResult, error) {
	if err := validateIssuerURL(issuerURL, allowPrivateIssuer); err != nil {
		return nil, fmt.Errorf("auth: issuer validation failed: %w", err)
	}

	discoverCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if client == nil {
		client = http.DefaultClient
	}
	// TLS certificate verification is disabled ONLY when allow_private_issuer is
	// enabled in OIDC config. This path is configuration-gated (NOT build-tag-gated)
	// and is runtime-reachable whenever allow_private_issuer: true is set, intended
	// for dev/CI OIDC against self-signed issuers. Disabled by default.
	// @MX:WARN: [AUTO] InsecureSkipVerify is config-gated by allowPrivateIssuer, off by default
	// @MX:REASON: runtime-reachable when allow_private_issuer is enabled; a startup WARN is logged so operators have evidence the insecure path is active
	if allowPrivateIssuer {
		slog.Warn("auth: TLS certificate verification disabled for OIDC discovery (allow_private_issuer enabled)", "issuer", issuerURL)
		if transport, ok := client.Transport.(*http.Transport); ok {
			if transport.TLSClientConfig == nil {
				transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} // #nosec G402 -- gated behind allow_private_issuer, disabled by default
			}
		} else if client.Transport == nil {
			client = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}, // #nosec G402 -- gated behind allow_private_issuer, disabled by default
				},
			}
		}
	}

	provider, err := oidc.NewProvider(
		oidc.ClientContext(discoverCtx, client),
		issuerURL,
	)
	if err != nil {
		return nil, fmt.Errorf("auth: OIDC discovery failed for %q: %w", issuerURL, err)
	}

	// Verify issuer matches exactly (REQ-AUTH1-001)
	if provider.Endpoint().AuthURL == "" {
		return nil, fmt.Errorf("auth: discovery returned empty authorization endpoint")
	}

	// Extract end_session_endpoint for RP-Initiated Logout (REQ-AUTH1-009)
	var claims struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&claims); err != nil {
		slog.Warn("auth: failed to parse provider claims for end_session_endpoint", "error", err)
	}

	return &DiscoveryResult{
		Provider:           provider,
		EndSessionEndpoint: claims.EndSessionEndpoint,
	}, nil
}

// validateIssuerURL performs SSRF protection checks on the issuer URL.
// REQ-AUTH1-011: HTTPS-only + private IP block.
// allowPrivate bypasses the private IP block only (HTTPS still enforced).
func validateIssuerURL(issuerURL string, allowPrivate bool) error {
	parsed, err := url.Parse(issuerURL)
	if err != nil {
		return fmt.Errorf("invalid issuer URL: %w", err)
	}

	// HTTPS enforcement (production path)
	if parsed.Scheme != "https" {
		return fmt.Errorf("auth: issuer must use https scheme, got %q", parsed.Scheme)
	}

	// Private IP check (unless explicitly allowed for dev/CI)
	if !allowPrivate {
		if err := checkPrivateIP(parsed.Hostname()); err != nil {
			return err
		}
	}

	return nil
}
