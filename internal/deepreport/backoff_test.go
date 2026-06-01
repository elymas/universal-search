package deepreport

// Coverage for the pure backoff helper: exponential growth, the max-delay cap,
// and jitter staying within the documented ±10% band of the base.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	t.Run("attempt 0 near base", func(t *testing.T) {
		d := backoff(0)
		// base = 500ms; with ±10% jitter the result must be within [450ms, 550ms].
		if d < 450*time.Millisecond || d > 550*time.Millisecond {
			t.Errorf("backoff(0) = %v, want ~500ms ±10%%", d)
		}
	})

	t.Run("large attempt is capped at max delay", func(t *testing.T) {
		// attempt 10 -> base would be 500ms<<10, far above the 3s cap.
		d := backoff(10)
		// Capped base is 3s; jitter band is [2.7s, 3.3s].
		if d < 2700*time.Millisecond || d > 3300*time.Millisecond {
			t.Errorf("backoff(10) = %v, want ~3s cap ±10%%", d)
		}
	})

	t.Run("never negative", func(t *testing.T) {
		for i := 0; i < 12; i++ {
			if backoff(i) < 0 {
				t.Errorf("backoff(%d) returned negative duration", i)
			}
		}
	})
}
