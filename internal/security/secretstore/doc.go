// Package secretstore provides a multi-backend secret resolver for
// SPEC-SEC-001 (REQ-SEC-016). Backends: env (default), k8s mounted files, and
// a vault stub. The directory is named "secretstore" rather than "secrets" to
// avoid collision with the repo-root ./secrets/** credential-protection deny
// rule; the config key remains secrets.backend.
package secretstore
