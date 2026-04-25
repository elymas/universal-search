// Package types is the public SDK boundary for Universal Search consumers.
//
// It declares the canonical types every search adapter produces and consumes:
//
//   - NormalizedDoc       — the unified search-result shape
//   - Adapter (interface) — the contract every search source implements
//   - Query / Filter      — the normalized search request
//   - Capabilities        — adapter-static metadata
//   - DocType             — canonical document categories
//   - SourceError, Category, ErrTransient/ErrPermanent/ErrRateLimited/
//     ErrSourceUnavailable, CategorizeError, OutcomeFromError
//     — adapter error taxonomy
//   - ValidationError     — typed error from NormalizedDoc.Validate
//
// This package is the SDK boundary (per .moai/project/structure.md §5).
// Breaking changes here require a major-version bump for any external Go
// consumer building their own Adapter implementation. The package
// intentionally depends only on the Go standard library.
//
// SPEC-CORE-001 fills this package surface. See .moai/specs/SPEC-CORE-001/spec.md.
package types
