package rbac

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

//go:embed model.conf
var embeddedModel []byte

//go:embed policy_default.csv
var embeddedPolicy []byte

// Config holds RBAC configuration.
type Config struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	DefaultTeamID string `yaml:"default_team_id" json:"default_team_id"`
	PGDSN         string `yaml:"pg_dsn" json:"pg_dsn"`
	AuditToStderr bool   `yaml:"audit_to_stderr" json:"audit_to_stderr"`
}

// DefaultConfig returns V1 default RBAC configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		DefaultTeamID: "default",
		PGDSN:         "",
		AuditToStderr: true,
	}
}

// globalEnforcer holds the singleton enforcer instance.
var (
	globalEnforcer     *casbin.Enforcer
	globalEnforcerOnce sync.Once
	globalEnforcerErr  error
	globalMu           sync.RWMutex
)

// Enforcer wraps a Casbin enforcer with thread-safe access and
// bootstrap policy loading.
type Enforcer struct {
	inner *casbin.Enforcer
	mu    sync.RWMutex
}

// NewEnforcer creates a new Enforcer from the given PG adapter.
// REQ-AUTH2-001: Uses embedded model.conf for RBAC-with-domains model.
func NewEnforcer(pgAdapter *PGAdapter) (*Enforcer, error) {
	m, err := model.NewModelFromString(string(embeddedModel))
	if err != nil {
		return nil, fmt.Errorf("rbac: load model: %w", err)
	}

	e, err := casbin.NewEnforcer(m, pgAdapter.Adapter())
	if err != nil {
		return nil, fmt.Errorf("rbac: create enforcer: %w", err)
	}

	if err := e.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("rbac: load policy: %w", err)
	}

	// Bootstrap: if no policies in PG, load default policy.
	// DI-3: only bootstrap when PG is empty (operator edits preserved).
	policies, _ := e.GetPolicy()
	if len(policies) == 0 {
		if err := bootstrapDefaultPolicy(e); err != nil {
			return nil, fmt.Errorf("rbac: bootstrap default policy: %w", err)
		}
	}

	return &Enforcer{inner: e}, nil
}

// MustInit initializes the global enforcer or exits on failure.
// REQ-AUTH2-001: Init failure is fatal when RBAC is enabled.
func MustInit(cfg Config) {
	if !cfg.Enabled {
		slog.Info("rbac disabled, skipping enforcer init")
		return
	}

	globalEnforcerOnce.Do(func() {
		pgAdapter, err := NewPGAdapter(cfg.PGDSN)
		if err != nil {
			globalEnforcerErr = err
			slog.Error("rbac init failed", "error", err)
			panic(fmt.Sprintf("rbac: fatal init error: %v", err))
		}

		ef, err := NewEnforcer(pgAdapter)
		if err != nil {
			globalEnforcerErr = err
			slog.Error("rbac enforcer init failed", "error", err)
			panic(fmt.Sprintf("rbac: fatal enforcer error: %v", err))
		}

		globalMu.Lock()
		globalEnforcer = ef.inner
		globalMu.Unlock()

		policyCount, _ := ef.inner.GetPolicy()
		slog.Info("rbac enforcer initialized", "policy_count", len(policyCount))
	})
}

// GlobalEnforcer returns the singleton enforcer. Returns nil if RBAC is disabled
// or not yet initialized.
func GlobalEnforcer() *casbin.Enforcer {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalEnforcer
}

// Enforce evaluates the RBAC policy for the given (sub, dom, obj, act) tuple.
// REQ-AUTH2-002: Thread-safe via internal RWMutex.
// @MX:ANCHOR: [AUTO] Single policy evaluation entry point; callers: EnforceMiddleware, per-adapter checks, admin handlers
// @MX:REASON: deny-by-default invariant + policy_effect semantics are concentrated here. All RBAC decisions flow through this.
func (e *Enforcer) Enforce(sub string, dom string, obj string, act string) (bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.inner.Enforce(sub, dom, obj, act)
}

// LoadPolicy reloads policies from PG storage.
// REQ-AUTH2-009: Admin reload endpoint.
// Edge2: failure preserves existing in-memory enforcer.
// @MX:WARN: [AUTO] LoadPolicy acquires write lock, briefly blocking in-flight Enforce calls.
// @MX:REASON: With 100+ policy rows, lock duration may impact SLO. Monitor NFR-AUTH2-002.
func (e *Enforcer) LoadPolicy() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Recover from panic if adapter is nil (e.g., in-memory enforcer in tests).
	// Edge2: failure preserves existing in-memory enforcer.
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("rbac: load policy failed: %v", r)
			}
		}()
		err = e.inner.LoadPolicy()
	}()
	return err
}

// GetPolicyCount returns the number of policy rows currently loaded.
func (e *Enforcer) GetPolicyCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	policies, _ := e.inner.GetPolicy()
	return len(policies)
}

// AddRoleForUserInDomain adds a role assignment for a user in a domain.
// REQ-AUTH2-010: Member management.
func (e *Enforcer) AddRoleForUserInDomain(user string, role string, domain string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, err := e.inner.AddRoleForUserInDomain(user, role, domain)
	return err
}

// DeleteRoleForUserInDomain removes a role assignment for a user in a domain.
// REQ-AUTH2-010: Member management.
func (e *Enforcer) DeleteRoleForUserInDomain(user string, role string, domain string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, err := e.inner.DeleteRoleForUserInDomain(user, role, domain)
	return err
}

// SavePolicy persists current in-memory policies to PG storage.
func (e *Enforcer) SavePolicy() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("rbac: save policy failed: %v", r)
			}
		}()
		err = e.inner.SavePolicy()
	}()
	return err
}

// GetRolesForUserInDomain returns the roles a user has in a domain.
func (e *Enforcer) GetRolesForUserInDomain(user string, domain string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.inner.GetRolesForUserInDomain(user, domain)
}

// GetUsersForRoleInDomain returns users with a given role in a domain.
func (e *Enforcer) GetUsersForRoleInDomain(role string, domain string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.inner.GetUsersForRoleInDomain(role, domain)
}

// Inner returns the underlying casbin.Enforcer for testing.
func (e *Enforcer) Inner() *casbin.Enforcer {
	return e.inner
}

// bootstrapDefaultPolicy loads the embedded policy_default.csv into the enforcer.
func bootstrapDefaultPolicy(e *casbin.Enforcer) error {
	policies, err := parseEmbeddedPolicy()
	if err != nil {
		return err
	}
	if len(policies) == 0 {
		return nil
	}
	_, err = e.AddPolicies(policies)
	if err != nil {
		return fmt.Errorf("add default policies: %w", err)
	}
	return e.SavePolicy()
}

// parseEmbeddedPolicy parses the embedded CSV into policy fields.
// Strips the ptype prefix (e.g., "p") from each CSV row since AddPolicies
// expects only the values without the ptype.
func parseEmbeddedPolicy() ([][]string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(embeddedPolicy))
	var policies [][]string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) < 2 {
			continue
		}
		// Strip the ptype prefix (first field, e.g., "p").
		policies = append(policies, fields[1:])
	}
	return policies, scanner.Err()
}
