# Migration Guide: 0.x → 1.0.0

This document guides users who may transition from 0.x pre-release development builds to the 1.x stable release series. While there are currently no active 0.x users (v1.0.0 is the first tagged release), this document establishes the baseline for all future 1.x minor and major version changes.

**Release Version**: v1.0.0
**Release Date**: [Determined at release ceremony]

## 1. Overview

Semantic versioning 1.0.0 marks the **API freeze boundary**. This section describes which components are frozen (backward-compatible changes only) and which remain free (may break between 1.x versions and certainly at 2.0.0).

### Freeze Scope (Stable in 1.x)

The following surfaces MUST maintain backward compatibility throughout 1.x:

- **CLI commands**: `usearch query`, future `usearch deep` subcommands (new subcommands = minor bump)
- **CLI flag names and semantics**: `--source`, `--format`, `--timeout`, `--json`, `--help`, `-v`, `--version`
- **Exit code meanings**: Per `cmd/usearch/exitcode.go`
- **MCP protocol tool names and schemas**: Defined in SPEC-MCP-001 (stable as of M7)
- **MoAI Skill manifest schema**: Defined in SPEC-SKILL-001 (stable as of M7)
- **Adapter plugin interface**: `pkg/types/Adapter`, `Capabilities`, method signatures (stable since SPEC-CORE-001, M2)
- **REST/gRPC endpoint paths**: `POST /query`, `POST /query/stream` (SPEC-SYN-004 stable as of M7)
- **REST response schemaVersion**: Locked at `schemaVersion=1` in JSON output per `cmd/usearch/output_json.go:19`
- **Configuration schema**: `.moai/config/sections/*.yaml` structure; additive keys OK, removal/rename = breaking
- **Environment variable names**: `LOG_LEVEL`, `OTLP_ENDPOINT`, etc.; additive OK, removal/rename = breaking
- **Kubernetes Helm chart**: `values.schema.json` constraints; additive OK, removal = breaking

### Free Zone (May Change in 1.x)

The following surfaces may change without a major version bump (within 1.x):

- **`internal/` Go packages**: Go convention — no stability promise
- **Experimental/alpha/beta adapters**: Status per SPEC-DOC-002 taxonomy; breaking changes permitted
- **AI prompt templates**: Content may change for accuracy/quality improvements; users do not control this
- **Internal metrics label values**: Cardinality allowlist within addition OK
- **Python sidecar internal API**: Non-user-facing, internal to the service(s) only

### Deprecation Cycle

When a breaking change is necessary after 1.0.0:

1. **First minor bump (e.g., 1.1.0)**: Emit deprecation warning + add entry to `DEPRECATED.md` with migration guidance
2. **Second minor bump (e.g., 1.2.0, assuming ≈3 months)**: Provide the alternative; original behavior still works with warning
3. **Next major bump (e.g., 2.0.0)**: Remove deprecated feature

**Minimum cycle**: 1 minor release (~3 months, no hard timeline enforced)

---

## 2. CLI Breaking Changes

v1.0.0 — no breaking changes in this category.

The `usearch query` subcommand has been stable since M2 (SPEC-CLI-001). Flag names (`--source`, `--format`, `--timeout`, `--json`) are unchanged.

**If upgrading**: No action required. All existing `usearch query "..."` invocations continue to work identically.

---

## 3. Config Schema Breaking Changes

The `.moai/config/sections/` YAML schema evolved during development:

### Known Changes (0.x-dev → 1.0.0)

- **deep.yaml**: SPEC-DEEP-004 introduced `cost_guard.user_id` key (M7). If you manually configured `deep.yaml` in 0.x-dev, you may need to add the `user_id` section.
  - **0.x state**: cost guard configuration had no user isolation
  - **1.x state**: user_id field is optional but recommended for multi-user deployments
  - **Action**: If you have a `deep.yaml` file, add `cost_guard:\n  user_id: <optional-identifier>` or leave empty

- **auth.yaml**: SPEC-AUTH-001 through AUTH-003 (M6) introduced OIDC provider configuration (`oidc_provider.client_id`, `oidc_provider.client_secret`, etc.)
  - **0.x state**: auth.yaml did not exist (auth was a stub)
  - **1.x state**: auth.yaml is auto-generated with placeholder values on first run
  - **Action**: On first v1.0.0 startup, auth.yaml will be created automatically. Fill in OIDC values if needed.

**If upgrading from 0.x-dev**: Run the binary once; any missing config sections are created with defaults.

---

## 4. Env Var Renames

v1.0.0 — no breaking changes in this category.

Environment variables (`LOG_LEVEL`, `OTLP_ENDPOINT`, `USEARCH_ADMIN_PORT`, `MCP_TRANSPORT`) have been stable since M2. No renames at v1.0.0.

**If upgrading**: No action required. All existing `LOG_LEVEL=...` and similar invocations work identically.

---

## 5. MCP Protocol Surface

SPEC-MCP-001 (M7, now stable) defines the MCP tool contracts: `search`, `deep_research`, `list_sources`, `get_citation`, with JSON request/response schemas.

v1.0.0 — no breaking changes in this category.

The tool names and request/response shapes are locked for 1.x.

**If upgrading MCP clients**: No action required. All MCP 0.x-dev clients work with v1.0.0.

---

## 6. Adapter Plugin Contract

SPEC-CORE-001 (M2) defines the `pkg/types/Adapter` interface and `Capabilities` struct:

```go
type Adapter interface {
  Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
  Capabilities() Capabilities
  Close() error
}
```

v1.0.0 — no breaking changes in this category.

The interface signature and method semantics are frozen for 1.x. New optional methods may be added in 2.0.0.

**If writing custom adapters**: You can safely implement this interface in v1.0.0 through v1.z.y (any patch/minor). No changes expected.

---

## 7. MoAI Skill Manifest

SPEC-SKILL-001 (M7, now stable) defines the Skill metadata schema (YAML frontmatter in skill definition files).

v1.0.0 — no breaking changes in this category.

The manifest structure is locked for 1.x.

**If creating Skill plugins**: The manifest format is stable. No migration needed.

---

## 8. REST/GraphQL Endpoint Schema

SPEC-SYN-004 (M7) defines the REST endpoint landscape:

- `POST /query` — query synthesis with JSON request/response
- `POST /query/stream` — SSE streaming endpoint (M7 additive)

Response structure includes `schemaVersion: 1` lock per `cmd/usearch/output_json.go:19`.

v1.0.0 — no breaking changes in this category.

The endpoint paths and response schemas are frozen for 1.x. New endpoints may be added (minor bump).

**If calling REST endpoints**: All v0.x-dev clients work with v1.0.0. The `schemaVersion: 1` in responses guarantees no breaking changes to the JSON structure.

---

## 9. Database Schema Migration Policy

The PostgreSQL migration sequence (`deploy/postgres/migrations/0001.sql` through `0007.sql`, per spec.md §D4 §9) is **forward-only**:

- All migrations are additive or non-breaking column changes
- Down migrations are not provided (data loss risk)
- In case of a breaking change, a **new migration** is created (e.g., `0008.sql`), never a revert

v1.0.0 — no breaking changes in this category.

The current 7-migration sequence is production-ready. No schema resets between 1.x versions.

**If operating the database**: Run `helm upgrade` or manual migration; all 0001..0007 are safe and idempotent. Never downgrade the schema.

---

## 10. Adapter Status Taxonomy

SPEC-DOC-002 (M9) defines adapter status levels:

- **stable**: Fully tested, API frozen, production-ready (v1.0.0 freeze scope)
- **beta**: Feature-complete, may have performance/edge-case issues (v1.x may add/remove)
- **alpha**: Experimental, early-stage, expect breaking changes (v1.x may change drastically)

v1.0.0 ships with adapters marked alpha/beta/stable per DOC-002. Stable adapters follow the freeze scope; alpha/beta adapters may break between 1.x versions.

**If integrating adapters**: Check the adapter status badge in the documentation. Only **stable** adapters carry the 1.x freeze promise.

---

## 11. Upgrade Procedure

### From 0.x-dev to 1.0.0

#### Option A: Binary (Direct install)

```bash
# Download v1.0.0 binary (check architecture)
curl -L https://github.com/elymas/universal-search/releases/download/v1.0.0/usearch_1.0.0_linux_amd64.tar.gz \
  | tar xz
cp usearch /usr/local/bin/usearch

# Verify version
usearch --version  # Should print "usearch v1.0.0"

# Verify cosign signature (optional, for security-conscious users)
# (see RELEASE.md for full verification command)
```

#### Option B: go install (For Go developers)

```bash
go install github.com/elymas/universal-search/cmd/usearch@v1.0.0
usearch --version
```

#### Option C: Helm Chart (For Kubernetes deployments)

```bash
helm repo add elymas-charts oci://ghcr.io/elymas/charts
helm upgrade --install universal-search elymas-charts/universal-search \
  --version 1.0.0 \
  --values values.yaml
```

#### Option D: Skill Marketplace Reinstall (For MoAI Skills)

If you installed the usearch Skill plugin in your MoAI project:

```bash
# Remove old skill
moai skill remove usearch

# Reinstall v1.0.0
moai skill add github.com/elymas/universal-search/cmd/usearch@v1.0.0
```

### Check Configuration

After upgrade, verify your config files are compatible:

```bash
# For binary/go install:
usearch --help  # Should show all expected flags

# For Helm deployments:
kubectl logs -l app=usearch  # Check startup logs for config errors

# For Skills:
moai skill verify usearch  # If available
```

If you have `.moai/config/sections/deep.yaml` or `auth.yaml` files, they may have changed. See §3 (Config Schema Breaking Changes) for guidance.

---

## 12. Rollback Procedure

### From 1.0.0 back to 0.x-dev (Emergency)

Since v1.0.0 is the first official release, there are no prior stable versions to roll back to. However, if you need to revert for any reason:

#### Binary Rollback

If you installed via `go install`:

```bash
# Downgrade to pre-release development build
go install github.com/elymas/universal-search/cmd/usearch@main

# Verify
usearch --version  # Should print "usearch v0.1.0-dev"
```

#### Helm Rollback

```bash
# See previous release(s)
helm history universal-search

# Rollback to previous helm release (if it exists)
helm rollback universal-search <REVISION>
```

**Note**: v1.0.0 is the first release. Rollback is only possible if you explicitly kept a prior version. In production, we recommend:

1. **Never rollback without a specific incident reason** (file an issue first)
2. **Wait for a patch release** (v1.0.1) if v1.0.0 has a known issue
3. **Use feature flags** in your integration to gracefully degrade if needed

#### Database Rollback

**Do NOT attempt to downgrade the database schema** (migrations are forward-only per §9). If a schema change causes issues:

1. File a GitHub issue with the database log
2. The team will release a patch (v1.0.1) with a corrective migration
3. Apply the corrective migration; do NOT revert

---

## Appendix: Quick Reference

| Change                               | v1.0.0 Status | Breaking?     | Migration Required  |
| ------------------------------------ | ------------- | ------------- | ------------------- |
| CLI commands/flags                   | Stable        | No            | No                  |
| Config schema (deep.yaml, auth.yaml) | Mostly stable | Additive only | No, but recommended |
| Environment variables                | Stable        | No            | No                  |
| MCP protocol                         | Stable        | No            | No                  |
| Adapter interface                    | Stable        | No            | No                  |
| REST endpoints                       | Stable        | No            | No                  |
| Database schema                      | Forward-only  | No backward   | No, run migrations  |
| Skill manifest                       | Stable        | No            | No                  |

**Summary**: v1.0.0 introduces minimal breaking changes because it is the first official release. All documented surfaces are frozen for 1.x. Code written for 0.x-dev will work with v1.0.0 without modification.

---

_Last updated: 2026-05-31_
