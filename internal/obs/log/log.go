// Package log provides the structured logging foundation for Universal Search.
// It wraps log/slog with context-aware enrichment (request ID, trace/span IDs)
// and an optional tee mechanism for custom handler registration (Loki seam).
//
// REQ-OBS-001: JSON output, level from LOG_LEVEL env, single-line records.
// REQ-OBS-007: RegisterHandler hook seam for optional Loki forwarding.
package log

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/elymas/universal-search/internal/obs/reqid"
)

// LevelFromEnv maps a LOG_LEVEL string (DEBUG|INFO|WARN|ERROR) to slog.Level.
// Unknown or empty values default to slog.LevelInfo.
// @MX:ANCHOR: [AUTO] Level resolution; callers: obs.Init, New, tests
// @MX:REASON: fan_in >= 3; changing default affects all log emission decisions
func LevelFromEnv(raw string) slog.Level {
	switch raw {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New constructs a *slog.Logger whose handler emits JSON to w at the given
// minimum level. AddSource is disabled per REQ-OBS-001 acceptance criteria.
func New(w io.Writer, level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	})
	return slog.New(h)
}

// teeHandler is an slog.Handler that forwards each log record to both a primary
// handler and one or more registered secondary handlers.
type teeHandler struct {
	primary     slog.Handler
	mu          sync.RWMutex
	secondaries []slog.Handler
}

func (t *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return t.primary.Enabled(ctx, level)
}

func (t *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	if err := t.primary.Handle(ctx, r); err != nil {
		return err
	}
	t.mu.RLock()
	secondaries := t.secondaries
	t.mu.RUnlock()
	for _, h := range secondaries {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (t *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	t.mu.RLock()
	secs := make([]slog.Handler, len(t.secondaries))
	copy(secs, t.secondaries)
	t.mu.RUnlock()

	newSecs := make([]slog.Handler, len(secs))
	for i, s := range secs {
		newSecs[i] = s.WithAttrs(attrs)
	}
	return &teeHandler{
		primary:     t.primary.WithAttrs(attrs),
		secondaries: newSecs,
	}
}

func (t *teeHandler) WithGroup(name string) slog.Handler {
	t.mu.RLock()
	secs := make([]slog.Handler, len(t.secondaries))
	copy(secs, t.secondaries)
	t.mu.RUnlock()

	newSecs := make([]slog.Handler, len(secs))
	for i, s := range secs {
		newSecs[i] = s.WithGroup(name)
	}
	return &teeHandler{
		primary:     t.primary.WithGroup(name),
		secondaries: newSecs,
	}
}

// RegisterHandler registers a secondary slog.Handler on the given logger.
// Subsequent records emitted via the logger are tee'd to the secondary handler.
// This is the Loki seam described in REQ-OBS-007.
//
// If the logger's handler is not a *teeHandler (i.e., the logger was created
// with New and RegisterHandler has never been called), this function replaces
// the logger's handler with a teeHandler wrapping the original.
//
// Note: slog.Logger does not expose its handler mutably, so this function
// returns a new *slog.Logger with the tee handler installed.
// Callers should reassign: logger = obslog.RegisterHandler(logger, h).
func RegisterHandler(logger *slog.Logger, h slog.Handler) *slog.Logger {
	if tee, ok := logger.Handler().(*teeHandler); ok {
		tee.mu.Lock()
		tee.secondaries = append(tee.secondaries, h)
		tee.mu.Unlock()
		return logger
	}
	// Wrap the existing handler in a teeHandler.
	tee := &teeHandler{
		primary:     logger.Handler(),
		secondaries: []slog.Handler{h},
	}
	return slog.New(tee)
}

// enrichHandler wraps an inner slog.Handler and injects request_id (and
// optionally trace_id/span_id) from ctx into every record.
type enrichHandler struct {
	inner slog.Handler
}

// WithEnrich returns a new *slog.Logger whose handler injects request_id from
// context on every log call that provides a context (e.g. InfoContext).
func WithEnrich(logger *slog.Logger) *slog.Logger {
	return slog.New(&enrichHandler{inner: logger.Handler()})
}

func (e *enrichHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return e.inner.Enabled(ctx, level)
}

func (e *enrichHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := reqid.FromContext(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return e.inner.Handle(ctx, r)
}

func (e *enrichHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &enrichHandler{inner: e.inner.WithAttrs(attrs)}
}

func (e *enrichHandler) WithGroup(name string) slog.Handler {
	return &enrichHandler{inner: e.inner.WithGroup(name)}
}
