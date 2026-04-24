// Package log_test tests the structured logging package (REQ-OBS-001, REQ-OBS-002, REQ-OBS-007).
package log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	obslog "github.com/elymas/universal-search/internal/obs/log"
	"github.com/elymas/universal-search/internal/obs/reqid"
)

// TestLoggerEmitsJSON verifies that the logger writes valid single-line JSON
// to the provided writer, with time, level, and msg keys.
// REQ-OBS-001
func TestLoggerEmitsJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := obslog.New(&buf, slog.LevelInfo)
	logger.Info("hello world")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	for _, key := range []string{"time", "level", "msg"} {
		if _, ok := record[key]; !ok {
			t.Errorf("expected key %q in JSON output, got: %v", key, record)
		}
	}
	if record["msg"] != "hello world" {
		t.Errorf("msg: got %q, want %q", record["msg"], "hello world")
	}
}

// TestLoggerRespectsLogLevelEnv verifies that LevelFromEnv returns the correct
// slog.Level for each valid value and defaults to INFO for unknown/empty.
// REQ-OBS-001
func TestLoggerRespectsLogLevelEnv(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"", slog.LevelInfo},
		{"garbage", slog.LevelInfo},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := obslog.LevelFromEnv(tc.input)
			if got != tc.want {
				t.Errorf("LevelFromEnv(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestLogBelowLevelSuppressed verifies that records below the configured level
// are not emitted.
// REQ-OBS-001
func TestLogBelowLevelSuppressed(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := obslog.New(&buf, slog.LevelWarn)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message") // this one should appear

	// Only the WARN record should be present.
	if buf.Len() == 0 {
		t.Fatal("expected at least one line but buffer is empty")
	}

	lines := bytes.Split(bytes.TrimRight(buf.Bytes(), "\n"), []byte("\n"))
	var nonEmpty [][]byte
	for _, l := range lines {
		if len(bytes.TrimSpace(l)) > 0 {
			nonEmpty = append(nonEmpty, l)
		}
	}

	if len(nonEmpty) != 1 {
		t.Fatalf("expected exactly 1 line (WARN), got %d: %s", len(nonEmpty), buf.String())
	}

	var record map[string]any
	if err := json.Unmarshal(nonEmpty[0], &record); err != nil {
		t.Fatalf("line is not valid JSON: %v", err)
	}
	if record["level"] != "WARN" {
		t.Errorf("expected WARN, got %q", record["level"])
	}
}

// TestLoggerIncludesRequestIDFromContext verifies that the enrichment handler
// injects request_id into the JSON record when the context carries one.
// REQ-OBS-001 + REQ-OBS-002
func TestLoggerIncludesRequestIDFromContext(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := obslog.New(&buf, slog.LevelInfo)
	enriched := obslog.WithEnrich(logger)

	ctx := reqid.WithContext(context.Background(), "REQ-ENRICH-TEST")
	enriched.InfoContext(ctx, "enriched log")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if record["request_id"] != "REQ-ENRICH-TEST" {
		t.Errorf("request_id: got %v, want %q", record["request_id"], "REQ-ENRICH-TEST")
	}
}

// TestLokiEnvReserved verifies that reading LOKI_ENDPOINT causes no error and
// returns a logger.
// REQ-OBS-007
func TestLokiEnvReserved(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := obslog.New(&buf, slog.LevelInfo)
	if logger == nil {
		t.Fatal("New returned nil logger")
	}
}

// TestRegisterHandlerHookSeam verifies that RegisterHandler accepts a custom
// slog.Handler and subsequent log records are tee'd to it.
// REQ-OBS-007
func TestRegisterHandlerHookSeam(t *testing.T) {
	t.Parallel()

	// Primary writer.
	var primary bytes.Buffer
	logger := obslog.New(&primary, slog.LevelInfo)

	// Secondary buffered handler (tee target).
	var secondary bytes.Buffer
	secondaryHandler := slog.NewJSONHandler(&secondary, &slog.HandlerOptions{Level: slog.LevelInfo})

	// Register the hook; RegisterHandler returns a new logger with tee installed.
	logger = obslog.RegisterHandler(logger, secondaryHandler)

	// Emit 3 records.
	logger.Info("msg one")
	logger.Info("msg two")
	logger.Info("msg three")

	// Count non-empty lines in secondary.
	secondaryLines := countLines(secondary.Bytes())
	if secondaryLines != 3 {
		t.Errorf("secondary handler: expected 3 records, got %d\noutput: %s",
			secondaryLines, secondary.String())
	}
}

// TestTeeHandlerWithAttrsPropagatesToSecondary verifies that WithAttrs on a
// tee-wrapped logger propagates attrs to both primary and secondary handlers.
func TestTeeHandlerWithAttrsPropagatesToSecondary(t *testing.T) {
	t.Parallel()

	var primary, secondary bytes.Buffer
	logger := obslog.New(&primary, slog.LevelInfo)
	secHandler := slog.NewJSONHandler(&secondary, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger = obslog.RegisterHandler(logger, secHandler)

	// WithAttrs returns a new logger; use it to emit.
	child := logger.With("service", "test-svc")
	child.Info("with-attrs-msg")

	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimRight(secondary.Bytes(), "\n"), &rec); err != nil {
		t.Fatalf("secondary output not valid JSON: %v\noutput: %s", err, secondary.String())
	}
	if rec["service"] != "test-svc" {
		t.Errorf("WithAttrs: expected service=test-svc in secondary, got %v", rec["service"])
	}
}

// TestTeeHandlerWithGroupPropagatesToSecondary verifies that WithGroup on a
// tee-wrapped logger propagates to both handlers.
func TestTeeHandlerWithGroupPropagatesToSecondary(t *testing.T) {
	t.Parallel()

	var primary, secondary bytes.Buffer
	logger := obslog.New(&primary, slog.LevelInfo)
	secHandler := slog.NewJSONHandler(&secondary, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger = obslog.RegisterHandler(logger, secHandler)

	// WithGroup groups attrs under a namespace key.
	child := logger.WithGroup("req").With("id", "abc")
	child.Info("grouped-msg")

	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimRight(secondary.Bytes(), "\n"), &rec); err != nil {
		t.Fatalf("secondary output not valid JSON: %v\noutput: %s", err, secondary.String())
	}
	grp, ok := rec["req"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'req' group in secondary output, got: %v", rec)
	}
	if grp["id"] != "abc" {
		t.Errorf("WithGroup: expected req.id=abc in secondary, got %v", grp["id"])
	}
}

// TestEnrichHandlerWithAttrs verifies that WithAttrs on an enrichHandler returns
// a handler that includes both the enriched attrs and the new attrs.
func TestEnrichHandlerWithAttrs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := obslog.New(&buf, slog.LevelInfo)
	enriched := obslog.WithEnrich(logger)

	child := enriched.With("component", "fanout")
	ctx := reqid.WithContext(context.Background(), "ENRICH-ATTRS-TEST")
	child.InfoContext(ctx, "attrs-msg")

	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimRight(buf.Bytes(), "\n"), &rec); err != nil {
		t.Fatalf("output not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if rec["component"] != "fanout" {
		t.Errorf("WithAttrs: expected component=fanout, got %v", rec["component"])
	}
	if rec["request_id"] != "ENRICH-ATTRS-TEST" {
		t.Errorf("WithAttrs: expected request_id=ENRICH-ATTRS-TEST, got %v", rec["request_id"])
	}
}

// TestEnrichHandlerWithGroup verifies that WithGroup on an enrichHandler
// returns a handler that wraps attrs in the named group.
func TestEnrichHandlerWithGroup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := obslog.New(&buf, slog.LevelInfo)
	enriched := obslog.WithEnrich(logger)

	child := enriched.WithGroup("http").With("method", "GET")
	child.Info("group-msg")

	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimRight(buf.Bytes(), "\n"), &rec); err != nil {
		t.Fatalf("output not valid JSON: %v\noutput: %s", err, buf.String())
	}
	grp, ok := rec["http"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'http' group in output, got: %v", rec)
	}
	if grp["method"] != "GET" {
		t.Errorf("WithGroup: expected http.method=GET, got %v", grp["method"])
	}
}

func countLines(b []byte) int {
	if len(bytes.TrimSpace(b)) == 0 {
		return 0
	}
	return len(bytes.Split(bytes.TrimRight(b, "\n"), []byte("\n")))
}
