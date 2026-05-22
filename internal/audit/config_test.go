package audit

import (
	"testing"
)

// TestDefaultConfig verifies default configuration values.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Async != true {
		t.Error("DefaultConfig().Async = false, want true")
	}
	if cfg.IndexWriteEnabled != true {
		t.Error("DefaultConfig().IndexWriteEnabled = false, want true")
	}
	if cfg.RetentionHotDays != 90 {
		t.Errorf("DefaultConfig().RetentionHotDays = %d, want 90", cfg.RetentionHotDays)
	}
	if cfg.RequireS3Archive != true {
		t.Error("DefaultConfig().RequireS3Archive = false, want true")
	}
	if cfg.HashChainEnabled != false {
		t.Error("DefaultConfig().HashChainEnabled = true, want false")
	}
	if cfg.MaskQueryText != false {
		t.Error("DefaultConfig().MaskQueryText = true, want false")
	}
	if cfg.MaskIP != false {
		t.Error("DefaultConfig().MaskIP = true, want false")
	}
	if cfg.S3Enabled != false {
		t.Error("DefaultConfig().S3Enabled = true, want false")
	}
}

// TestPIIConfig_maskingFlags verifies PII masking configuration.
func TestPIIConfig_maskingFlags(t *testing.T) {
	cfg := Config{
		MaskQueryText: true,
		MaskIP:        true,
	}

	if !cfg.MaskQueryText {
		t.Error("MaskQueryText should be true")
	}
	if !cfg.MaskIP {
		t.Error("MaskIP should be true")
	}
}

// TestCostMirrorStrict verifies cost mirror strictness toggle.
func TestCostMirrorStrict(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.CostMirrorStrict {
		t.Error("DefaultConfig().CostMirrorStrict = false, want true")
	}

	cfg.CostMirrorStrict = false
	if cfg.CostMirrorStrict {
		t.Error("CostMirrorStrict should be false after setting")
	}
}

// TestReplayConfig_rateLimit verifies replay rate limit defaults.
func TestReplayConfig_rateLimit(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ReplayRateLimitPerMin != 1 {
		t.Errorf("DefaultConfig().ReplayRateLimitPerMin = %d, want 1", cfg.ReplayRateLimitPerMin)
	}
}

// TestS3Config_defaultDisabled verifies S3 is disabled by default.
func TestS3Config_defaultDisabled(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.S3Enabled {
		t.Error("S3Enabled should be false by default")
	}
	if cfg.S3Bucket != "" {
		t.Errorf("S3Bucket = %q, want empty", cfg.S3Bucket)
	}
	if cfg.ExportOlderThanDays != 7 {
		t.Errorf("ExportOlderThanDays = %d, want 7", cfg.ExportOlderThanDays)
	}
}

// TestHashChainConfig_default verifies hash chain default off.
func TestHashChainConfig_default(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.HashChainEnabled {
		t.Error("HashChainEnabled should be false by default")
	}
}
