// Package adapters provides the runtime adapter Registry. The registry stores
// concrete Adapter implementations (Reddit, Hacker News, arXiv, ...) and
// wraps each with per-call observability (Prometheus + slog + OTel) before
// returning them via Get / List. All exports live in registry.go; the
// per-source Adapter implementations land in their own packages
// (internal/adapters/reddit, internal/adapters/hackernews, etc) under
// SPEC-ADP-* milestones.
//
// The reference noop adapter at internal/adapters/noop/ provides the
// compile-time interface assertion and a deterministic test fixture for
// downstream SPECs (FAN-001, IR-001, IDX-001).
//
// SPEC-CORE-001 fills this package. See .moai/specs/SPEC-CORE-001/spec.md.
package adapters
