// Package costguard provides quota enforcement, cost tracking, and Haiku
// pre-screening for the /deep pipeline (SPEC-DEEP-004).
package costguard

// Outcome enumerates the possible outcomes of a cost-guarded LLM call.
// Stored in cost_ledger.outcome (REQ-DEEP4-006).
type Outcome string

const (
	OutcomeSuccess    Outcome = "success"
	OutcomeError      Outcome = "error"
	OutcomeCapped     Outcome = "capped"
	OutcomeDegraded   Outcome = "degraded"
	OutcomeScreenOnly Outcome = "screen_only"
)

// CapDimension identifies which cap dimension triggered the limit.
type CapDimension string

const (
	DimensionCalls CapDimension = "calls"
	DimensionUSD   CapDimension = "usd"
	DimensionNone  CapDimension = "none"
)

// LedgerEntry represents a single row in the cost_ledger table.
// REQ-DEEP4-006: every Go-side llm.Client call writes one row.
type LedgerEntry struct {
	ID               int64   `json:"id"`
	UserID           string  `json:"user_id"`
	TenantID         string  `json:"tenant_id"`
	RequestID        string  `json:"request_id"`
	DeepRunID        *string `json:"deep_run_id,omitempty"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	USDCost          float64 `json:"usd_cost"`
	CacheHit         bool    `json:"cache_hit"`
	IntentCategory   *string `json:"intent_category,omitempty"`
	Outcome          Outcome `json:"outcome"`
}

// CapResult is returned by the atomic cap-check evaluation.
type CapResult struct {
	Allowed  bool         `json:"allowed"`
	Exceeded CapDimension `json:"exceeded,omitempty"`
	RemainingCalls int    `json:"remaining_calls"`
	RemainingUSD  float64 `json:"remaining_usd"`
	ResetAt       string  `json:"reset_at,omitempty"`
}

// ScreenResult is the parsed output of the Haiku pre-screen.
type ScreenResult struct {
	Score         int    `json:"score"`
	Rationale     string `json:"rationale"`
	SuggestedMode string `json:"suggested_mode"`
}

// TenantConfig holds per-tenant cap limits.
type TenantConfig struct {
	MaxCallsPerDay int     `json:"max_calls_per_day"`
	MaxUSDPerDay   float64 `json:"max_usd_per_day"`
}

// UserConfig holds per-user cap limits (enabled in V1.1 with AUTH-001).
type UserConfig struct {
	Enabled        bool    `json:"enabled"`
	MaxCallsPerDay int     `json:"max_calls_per_day"`
	MaxUSDPerDay   float64 `json:"max_usd_per_day"`
}

// Config holds all costguard configuration (deep.yaml costguard section).
type Config struct {
	Enabled          bool          `json:"enabled"`
	DefaultTenantID  string        `json:"default_tenant_id"`
	RedisFailureMode string        `json:"redis_failure_mode"` // "fail-closed" | "fail-open"
	Tenant           TenantConfig  `json:"tenant"`
	User             UserConfig    `json:"user"`
	AllowedTenants   []string      `json:"allowed_tenants"`
}

// DefaultConfig returns production-safe defaults per SPEC-DEEP-004 §6.1.
func DefaultConfig() Config {
	return Config{
		Enabled:          true,
		DefaultTenantID:  "default",
		RedisFailureMode: "fail-closed",
		Tenant: TenantConfig{
			MaxCallsPerDay: 20,
			MaxUSDPerDay:   5.00,
		},
		User: UserConfig{
			Enabled:        false,
			MaxCallsPerDay: 10,
			MaxUSDPerDay:   2.00,
		},
		AllowedTenants: []string{"default"},
	}
}
