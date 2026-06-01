// Package ratelimit provides per-tenant token-bucket rate limiting for
// SPEC-SEC-001 (REQ-SEC-014). V1 is alert-only by default: limit breaches
// emit a security event and metric without rejecting the request unless
// enforcement is explicitly enabled in config.
package ratelimit
