package log_test

// Coverage for the RegisterHandler "already tee'd" branch: a second call must
// append to the existing teeHandler's secondaries rather than nesting a new
// teeHandler, and records must fan out to both secondary targets.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	obslog "github.com/elymas/universal-search/internal/obs/log"
)

func TestRegisterHandlerSecondCallAppendsSecondary(t *testing.T) {
	var primary, sec1, sec2 bytes.Buffer
	logger := obslog.New(&primary, slog.LevelInfo)

	// First registration installs a teeHandler.
	logger = obslog.RegisterHandler(logger, slog.NewJSONHandler(&sec1, &slog.HandlerOptions{Level: slog.LevelInfo}))
	// Second registration must append to the same tee (the branch under test);
	// the returned logger is the same instance.
	logger2 := obslog.RegisterHandler(logger, slog.NewJSONHandler(&sec2, &slog.HandlerOptions{Level: slog.LevelInfo}))

	logger2.Info("fan out")

	if !strings.Contains(sec1.String(), "fan out") {
		t.Errorf("first secondary did not receive the record: %q", sec1.String())
	}
	if !strings.Contains(sec2.String(), "fan out") {
		t.Errorf("second secondary did not receive the record: %q", sec2.String())
	}
	if !strings.Contains(primary.String(), "fan out") {
		t.Errorf("primary did not receive the record: %q", primary.String())
	}
}
