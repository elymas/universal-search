package audit

// Config holds all audit subsystem configuration.
// Loaded from .moai/config/sections/audit.yaml and environment variables.
type Config struct {
	// Async controls whether EmitEvent enqueues to Asynq (true) or writes synchronously.
	// Default: true. REQ-AUTH3-002, NFR-AUTH3-002.
	Async bool `json:"async" yaml:"async"`

	// IndexWriteEnabled controls emission of index.write/index.delete events.
	// D4: V1 all-in with toggle. Default: true.
	IndexWriteEnabled bool `json:"index_write_enabled" yaml:"index_write_enabled"`

	// RetentionHotDays is the number of days to retain hot partitions.
	// REQ-AUTH3-007: default 90.
	RetentionHotDays int `json:"retention_hot_days" yaml:"retention_hot_days"`

	// RequireS3Archive requires archived_at IS NOT NULL before dropping a partition.
	// REQ-AUTH3-007: default true.
	RequireS3Archive bool `json:"require_s3_archive" yaml:"require_s3_archive"`

	// CostMirrorStrict controls whether cost_ledger trigger failure aborts the INSERT.
	// Default: true. When false, audit INSERT failure is logged but cost_ledger INSERT succeeds.
	CostMirrorStrict bool `json:"cost_mirror_strict" yaml:"cost_mirror_strict"`

	// HashChainEnabled enables the optional hash chain for tamper detection.
	// D6: default OFF. REQ-AUTH3-008.
	HashChainEnabled bool `json:"hash_chain_enabled" yaml:"hash_chain_enabled"`

	// MaskQueryText replaces payload.query.text with text_sha256.
	// REQ-AUTH3-006: default false (operator must enable).
	MaskQueryText bool `json:"mask_query_text" yaml:"mask_query_text"`

	// MaskIP nullifies the IP column for all events.
	// REQ-AUTH3-006: default false.
	MaskIP bool `json:"mask_ip" yaml:"mask_ip"`

	// S3Enabled enables the S3 export job.
	// REQ-AUTH3-005: default false.
	S3Enabled bool `json:"s3_enabled" yaml:"s3_enabled"`

	// S3Bucket is the target S3 bucket for exports.
	S3Bucket string `json:"s3_bucket" yaml:"s3_bucket"`

	// S3Endpoint is the S3-compatible endpoint (MinIO or AWS).
	S3Endpoint string `json:"s3_endpoint" yaml:"s3_endpoint"`

	// S3Region is the AWS region for S3.
	S3Region string `json:"s3_region" yaml:"s3_region"`

	// ExportOlderThanDays exports partitions older than this.
	// REQ-AUTH3-005: default 7.
	ExportOlderThanDays int `json:"export_older_than_days" yaml:"export_older_than_days"`

	// LiteLLMEndpoint is the URL for LiteLLM /spend/logs.
	LiteLLMEndpoint string `json:"litellm_endpoint" yaml:"litellm_endpoint"`

	// ReconcileIntervalMinutes is the polling interval for LiteLLM reconciliation.
	// D3: default 5 minutes.
	ReconcileIntervalMinutes int `json:"reconcile_interval_minutes" yaml:"reconcile_interval_minutes"`

	// ReplayRateLimitPerMin is the rate limit for replay requests.
	// REQ-AUTH3-004: default 1.
	ReplayRateLimitPerMin int `json:"replay_rate_limit_per_min" yaml:"replay_rate_limit_per_min"`

	// ReplayAllowedEventTypes restricts which event types can be replayed.
	// If empty, defaults to [query.submit, deep.start].
	ReplayAllowedEventTypes []string `json:"replay_allowed_event_types" yaml:"replay_allowed_event_types"`
}

// DefaultConfig returns production-safe defaults per SPEC-AUTH-003.
func DefaultConfig() Config {
	return Config{
		Async:                    true,
		IndexWriteEnabled:        true,
		RetentionHotDays:         90,
		RequireS3Archive:         true,
		CostMirrorStrict:         true,
		HashChainEnabled:         false,
		MaskQueryText:            false,
		MaskIP:                   false,
		S3Enabled:                false,
		S3Bucket:                 "",
		S3Endpoint:               "",
		S3Region:                 "",
		ExportOlderThanDays:      7,
		LiteLLMEndpoint:          "",
		ReconcileIntervalMinutes: 5,
		ReplayRateLimitPerMin:    1,
		ReplayAllowedEventTypes:  []string{string(EventQuerySubmit), string(EventDeepStart)},
	}
}
