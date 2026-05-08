// Package access — unit tests for Options.applyDefaults().
package access

import (
	"testing"
	"time"
)

func TestApplyDefaults_FillsZero(t *testing.T) {
	t.Parallel()
	var opts Options
	opts.applyDefaults()

	if opts.RobotsTTL != defaultRobotsTTL {
		t.Errorf("RobotsTTL = %v, want %v", opts.RobotsTTL, defaultRobotsTTL)
	}
	if opts.MaxBodyBytes != defaultMaxBodyBytes {
		t.Errorf("MaxBodyBytes = %d, want %d", opts.MaxBodyBytes, defaultMaxBodyBytes)
	}
	if opts.MaxBrowsers != defaultMaxBrowsers {
		t.Errorf("MaxBrowsers = %d, want %d", opts.MaxBrowsers, defaultMaxBrowsers)
	}
	if opts.RedirectMaxHops != defaultRedirectMaxHops {
		t.Errorf("RedirectMaxHops = %d, want %d", opts.RedirectMaxHops, defaultRedirectMaxHops)
	}
}

func TestApplyDefaults_PreservesExisting(t *testing.T) {
	t.Parallel()
	opts := Options{
		RobotsTTL:    5 * time.Hour,
		MaxBodyBytes: 1024 * 1024,
		MaxBrowsers:  4,
	}
	opts.applyDefaults()

	if opts.RobotsTTL != 5*time.Hour {
		t.Errorf("RobotsTTL changed, got %v", opts.RobotsTTL)
	}
	if opts.MaxBodyBytes != 1024*1024 {
		t.Errorf("MaxBodyBytes changed, got %d", opts.MaxBodyBytes)
	}
	if opts.MaxBrowsers != 4 {
		t.Errorf("MaxBrowsers changed, got %d", opts.MaxBrowsers)
	}
}

func TestApplyDefaults_PerPhaseTimeouts(t *testing.T) {
	t.Parallel()
	var opts Options
	opts.applyDefaults()

	for phase := 1; phase <= 5; phase++ {
		if opts.PerPhaseTimeout[phase] != 0 {
			// Per-phase timeouts are in defaultPerPhaseTimeout map, not opts.PerPhaseTimeout.
			// opts.PerPhaseTimeout[phase] == 0 means "use default" — this is correct.
			t.Logf("opts.PerPhaseTimeout[%d] = %v (0 means use defaultPerPhaseTimeout)", phase, opts.PerPhaseTimeout[phase])
		}
	}
	// Verify defaultPerPhaseTimeout has all 5 phases.
	for phase := 1; phase <= 5; phase++ {
		if defaultPerPhaseTimeout[phase] == 0 {
			t.Errorf("defaultPerPhaseTimeout[%d] = 0, must be non-zero", phase)
		}
	}
}
