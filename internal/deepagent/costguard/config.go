package costguard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// DecisionEvent represents a costguard decision event for audit logging.
// SPEC-AUTH-003 compatible schema (REQ-DEEP4-010 stderr JSON line).
type DecisionEvent struct {
	Timestamp  string    `json:"timestamp"`
	EventType  string    `json:"event_type"`
	RequestID  string    `json:"request_id"`
	TenantID   string    `json:"tenant_id"`
	UserID     string    `json:"user_id"`
	Decision   string    `json:"decision"`    // allow | deny | degrade
	Dimension  string    `json:"dimension"`   // calls | usd | none
	Remaining  Remaining `json:"remaining"`
	ScreenScore *int      `json:"screen_score,omitempty"`
	CacheHit   *bool     `json:"cache_hit,omitempty"`
}

// Remaining holds the remaining cap values.
type Remaining struct {
	Calls int     `json:"calls"`
	USD   float64 `json:"usd"`
}

// DecisionLogger writes JSON decision events to stderr.
// REQ-DEEP4-010: decision event log as stderr JSON line.
type DecisionLogger struct {
	w  io.Writer
	mu sync.Mutex
}

// NewDecisionLogger creates a DecisionLogger that writes to w.
// Defaults to os.Stderr if w is nil.
func NewDecisionLogger(w io.Writer) *DecisionLogger {
	if w == nil {
		w = os.Stderr
	}
	return &DecisionLogger{w: w}
}

// Log writes a decision event as a JSON line.
func (l *DecisionLogger) Log(event DecisionEvent) error {
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if event.EventType == "" {
		event.EventType = "cap.evaluation"
	}

	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal decision event: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err = fmt.Fprintf(l.w, "%s\n", b)
	return err
}

// ConfigWatcher watches for SIGHUP to trigger config hot-reload.
// NFR-DEEP4-008: deep.yaml hot-reload via SIGHUP.
type ConfigWatcher struct {
	cfg     *Config
	path    string
	mu      sync.RWMutex
	reload  chan struct{}
	onLoad  func(path string) (Config, error)
}

// NewConfigWatcher creates a new config watcher.
func NewConfigWatcher(cfg *Config, path string, onLoad func(path string) (Config, error)) *ConfigWatcher {
	return &ConfigWatcher{
		cfg:    cfg,
		path:   path,
		reload: make(chan struct{}, 1),
		onLoad: onLoad,
	}
}

// Start begins watching for SIGHUP signals.
// Blocks until ctx is cancelled.
func (w *ConfigWatcher) Start(ctx context.Context) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sigCh:
			w.reloadConfig()
		case <-w.reload:
			w.reloadConfig()
		}
	}
}

// TriggerReload manually triggers a reload (for testing).
func (w *ConfigWatcher) TriggerReload() {
	select {
	case w.reload <- struct{}{}:
	default:
	}
}

// GetConfig returns the current config (thread-safe).
func (w *ConfigWatcher) GetConfig() Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return *w.cfg
}

func (w *ConfigWatcher) reloadConfig() {
	if w.onLoad == nil {
		return
	}

	newCfg, err := w.onLoad(w.path)
	if err != nil {
		return
	}

	w.mu.Lock()
	*w.cfg = newCfg
	w.mu.Unlock()
}
