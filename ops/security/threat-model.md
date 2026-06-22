<!-- last-reviewed-at: 2026-05-29 -->
<!-- SPEC-SEC-001 NFR-SEC-005: re-sign on every minor release; CI warns if older than 90 days. -->

# usearch STRIDE Threat Model

STRIDE-based threat analysis for universal-search (usearch), per SPEC-SEC-001
(research §13). Each threat lists the affected component, mitigation, and
residual risk. Reviewed and re-signed on every minor version release.

- **Source SPEC**: SPEC-SEC-001
- **Last reviewed**: 2026-05-29 (V1 baseline)
- **Review cadence**: every minor release (V1.1, V1.2, ...) — NFR-SEC-005
- **Staleness**: CI warns (does not fail) if `last-reviewed-at` > 90 days old

---

## S — Spoofing (identity)

| ID  | Threat                                 | Component           | Mitigation                                                                                             | Residual |
| --- | -------------------------------------- | ------------------- | ------------------------------------------------------------------------------------------------------ | -------- |
| S1  | Forged JWT impersonates user           | API server          | AUTH-001 JWT signature + iss/aud validation                                                            | Low      |
| S2  | Spoofed OIDC discovery via DNS poison  | OIDC client         | AUTH-001 D8 HTTPS-only + host allowlist + private-IP block (now `internal/security/ssrf`, REQ-SEC-007) | Low      |
| S3  | MITM of LLM API call                   | LLM client          | LiteLLM proxy TLS; provider TLS 1.2+                                                                   | Low      |
| S4  | Adapter source spoofing via DNS hijack | Adapter HTTP client | TLS verification by default; Phase 4 TLS-aware GET                                                     | Low      |
| S5  | Container image substitution           | Release artifact    | REQ-SEC-016 SLSA L2 provenance + cosign verify                                                         | Low      |
| S6  | Git commit author spoofing             | CI pipeline         | Signed commits (post-V1; V1 documented residual)                                                       | Medium   |

## T — Tampering (integrity)

| ID  | Threat                                         | Component                   | Mitigation                                                                                                                                                           | Residual |
| --- | ---------------------------------------------- | --------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| T1  | Audit log modified to hide actions             | AUTH-003 audit DB           | AUTH-003 hash chain (`internal/audit`: `ComputeThisHash`/`VerifyChain`) + append-only triggers + daily `audit.chain_verify`; SEC-001 reuses this chain (REQ-SEC-017) | Low      |
| T2  | Helm values tampered to disable security gates | DEPLOY-001 Helm             | Chart values schema validation; required fields enforced                                                                                                             | Medium   |
| T3  | Dependency confusion attack                    | Build pipeline              | `go.sum` verification + `GOFLAGS=-mod=readonly`                                                                                                                      | Low      |
| T4  | Indexed document content manipulates synthesis | Adapter → Index → Synthesis | SYN-002 citation faithfulness + REQ-SEC-015 prompt sanitization                                                                                                      | Medium   |
| T5  | Image tampering between build and deploy       | Container registry          | cosign signature + verify on pull (DEPLOY-001)                                                                                                                       | Low      |
| T6  | LLM response tampering by intermediate proxy   | LLM client                  | LiteLLM HTTPS; not all providers sign responses                                                                                                                      | Medium   |

## R — Repudiation (logging/audit)

| ID  | Threat                                    | Component                  | Mitigation                                                       | Residual |
| --- | ----------------------------------------- | -------------------------- | ---------------------------------------------------------------- | -------- |
| R1  | User denies issuing a paid /deep query    | AUTH-003 + DEEP-004 ledger | Audit log includes query content hash; cost-ledger row immutable | Low      |
| R2  | Operator denies modifying security config | Config files               | Git commit history (signed commits post-V1)                      | Medium   |
| R3  | Admin denies revoking access              | AUTH-002 RBAC              | RBAC policy change audited via AUTH-003                          | Low      |
| R4  | Failed-auth attempt repudiated            | AUTH-001 + SEC-001 events  | `auth.failed` event recorded with source + timestamp             | Low      |

## I — Information Disclosure

| ID  | Threat                                   | Component               | Mitigation                                                                               | Residual |
| --- | ---------------------------------------- | ----------------------- | ---------------------------------------------------------------------------------------- | -------- |
| I1  | SSRF exposes cloud metadata (IAM creds)  | access Phase 3-5 + OIDC | REQ-SEC-007/008 SSRF guards + hostname blocklist                                         | Low      |
| I2  | Secret leaked in log output              | All packages            | REQ-SEC-018 no-secrets-in-logs CI grep (`scripts/check-no-secret-logs.sh`) + `redactKey` | Low      |
| I3  | Cross-tenant data leak via filter bypass | IDX-001/004             | AUTH-002 RBAC tenant filter at query construction                                        | Medium   |
| I4  | Error reveals internal stack trace       | API error handler       | Sanitized error responses; full trace only server-side                                   | Low      |
| I5  | LLM reveals system prompt                | Synthesis LLM           | Provider instruction-following; REQ-SEC-015 structural separation                        | Medium   |
| I6  | Audit log readable by non-admin          | AUTH-003 admin UI       | Admin role gate + AUTH-002 RBAC                                                          | Low      |
| I7  | Image base layer leaks build secrets     | Dockerfile              | Multi-stage builds; minimal runtime stage only                                           | Low      |
| I8  | git history exposes historical secrets   | Repository              | REQ-SEC-004 gitleaks + REQ-SEC-005 rotation runbook                                      | Medium   |

## D — Denial of Service

| ID  | Threat                                     | Component           | Mitigation                                          | Residual |
| --- | ------------------------------------------ | ------------------- | --------------------------------------------------- | -------- |
| D1  | One tenant exhausts query quota for others | API server          | REQ-SEC-014 per-tenant token bucket (V1 alert-only) | Medium   |
| D2  | /deep query spirals LLM cost               | DEEP-004 cost guard | Per-user daily cap (existing)                       | Low      |
| D3  | Slowloris on API server                    | API server          | stdlib `net/http` timeouts + middleware (BOOT-001)  | Medium   |
| D4  | Playwright child-process memory exhaustion | access Phase 5      | CACHE-001 browser pool + memory ceiling             | Low      |
| D5  | Adversarial SearXNG huge body              | ADP-007             | CACHE-001 MaxBodyBytes 10MB cap                     | Low      |
| D6  | Adversarial prompt induces long response   | Synthesis LLM       | LiteLLM max_tokens + budget tracking                | Medium   |

## E — Elevation of Privilege

| ID  | Threat                                            | Component                | Mitigation                                                                      | Residual |
| --- | ------------------------------------------------- | ------------------------ | ------------------------------------------------------------------------------- | -------- |
| E1  | Anonymous user gains team-scoped access           | AUTH-001 permissive mode | REQ-SEC-012 cookie flags + strict-mode recommendation                           | Medium   |
| E2  | Team member reads another team's docs             | AUTH-002 RBAC + IDX-004  | RBAC policy + tenant filter dual enforcement                                    | Medium   |
| E3  | Operator account compromise                       | K8s deploy               | K8s RBAC (DEPLOY-001) + audit monitoring                                        | Medium   |
| E4  | Indirect prompt injection invokes privileged tool | Synthesis LLM            | REQ-SEC-015 sanitization + MCP server read-only (no destructive tools)          | Low      |
| E5  | Container escape via Chromium 0-day               | access Phase 5           | K8s SecurityContext (drop ALL caps, readOnlyRootFilesystem); Playwright updates | Medium   |
| E6  | Compromised dependency executes code in CI        | CI pipeline              | REQ-SEC-001 govulncheck + Trivy + least-privilege GITHUB_TOKEN                  | Medium   |

---

## Residual risks (documented, accepted for V1)

These threats have no V1 mitigation — operator awareness + post-V1 roadmap.

- **R-DOC-1** — Playwright Phase 5 JS-based SSRF (Chromium network stack
  bypasses Go-level guards). Future: `--proxy-server` evaluation.
- **R-DOC-2** — Multi-document collusion prompt injection (split adversarial
  instructions across docs). Structural separation is partial; classifier model
  is post-V1.
- **R-DOC-3** — Zero-day in OIDC provider (Keycloak/Authentik). Operator
  subscribes to provider advisories; incident-response runbook applies.
- **R-DOC-4** — Supply-chain attack on `slsa-github-generator` itself. Pin to a
  specific version; review release notes before bumping.
- **R-DOC-5** — GitHub Actions hosted-runner compromise. Platform risk; SLSA L3
  self-hosted runner is post-V1.

---

## References

- OWASP ASVS v4.0.3: https://github.com/OWASP/ASVS/tree/v4.0.3
- OWASP SSRF Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html
- CWE-918 SSRF: https://cwe.mitre.org/data/definitions/918.html
- CWE-1427 LLM Prompt Injection: https://cwe.mitre.org/data/definitions/1427.html
- Greshake et al. 2023 (indirect prompt injection): https://arxiv.org/abs/2302.12173
- SLSA Framework v1.0: https://slsa.dev/spec/v1.0/
- Sigstore docs: https://docs.sigstore.dev/
