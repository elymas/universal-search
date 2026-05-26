package auth

import (
	"log/slog"
	"time"
)

// OIDCConfig holds OIDC-specific configuration.
type OIDCConfig struct {
	Issuer             string        `yaml:"issuer" json:"issuer"`
	Audience           []string      `yaml:"audience" json:"audience"`
	AllowPrivateIssuer bool          `yaml:"allow_private_issuer" json:"allow_private_issuer"`
	DiscoveryTimeout   time.Duration `yaml:"-" json:"-"`
}

// Config holds the complete authentication configuration.
type Config struct {
	Mode           AuthMode         `yaml:"mode" json:"mode"`
	OIDC           OIDCConfig       `yaml:"oidc" json:"oidc"`
	ClockSkew      time.Duration    `yaml:"clock_skew_seconds" json:"clock_skew_seconds"`
	Tenant         TenantConfig     `yaml:"tenant" json:"tenant"`
	Revocation     RevocationConfig `yaml:"revocation" json:"revocation"`
	Callback       CallbackConfig   `yaml:"callback" json:"callback"`
	AllowEndpoints []string         `yaml:"allow_endpoints" json:"allow_endpoints"`
}

// TenantConfig holds tenant resolution configuration.
type TenantConfig struct {
	Mode            TenantMode `yaml:"mode" json:"mode"`
	ClaimPath       string     `yaml:"claim_path" json:"claim_path"`
	DefaultTenantID string     `yaml:"default_tenant_id" json:"default_tenant_id"`
}

// RevocationConfig holds token revocation configuration.
type RevocationConfig struct {
	Enabled     bool                  `yaml:"enabled" json:"enabled"`
	FailureMode RevocationFailureMode `yaml:"failure_mode" json:"failure_mode"`
}

// CallbackConfig holds OAuth2 callback configuration.
type CallbackConfig struct {
	RateLimitPerMinute int `yaml:"rate_limit_per_minute" json:"rate_limit_per_minute"`
}

// DefaultConfig returns the V1 default configuration.
func DefaultConfig() Config {
	return Config{
		Mode: ModePermissive,
		OIDC: OIDCConfig{
			DiscoveryTimeout: 30 * time.Second,
		},
		ClockSkew: 30 * time.Second,
		Tenant: TenantConfig{
			Mode:            TenantModeStatic,
			DefaultTenantID: "default",
		},
		Revocation: RevocationConfig{
			Enabled:     false,
			FailureMode: RevocationFailOpen,
		},
		Callback: CallbackConfig{
			RateLimitPerMinute: 60,
		},
		AllowEndpoints: []string{"/healthz", "/metrics", "/v1/auth/callback", "/v1/auth/login", "/v1/auth/logout"},
	}
}

// EmitProductionWarning checks if production environment is running in non-strict mode.
// NFR-AUTH1-009: USEARCH_ENV=production + mode != strict → startup WARN log.
// Returns true if a warning was emitted.
func EmitProductionWarning(env string, mode AuthMode) bool {
	if env == "production" && mode != ModeStrict {
		slog.Warn("auth mode is not 'strict' in production environment; expect untrusted requests",
			"mode", string(mode),
			"environment", env,
		)
		return true
	}
	return false
}
