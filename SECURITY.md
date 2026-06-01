# Security Policy

universal-search (usearch) takes security seriously. This document describes how
to report vulnerabilities and what to expect. (SPEC-SEC-001 REQ-SEC-011 V14.)

## Reporting a vulnerability

Please report security vulnerabilities **privately**. Do NOT open a public
GitHub issue for security reports.

- **Email**: security@usearch.dev
- Include: affected component/version, reproduction steps, impact assessment,
  and any proof-of-concept. Encrypt sensitive details if possible.

Please do not disclose the issue publicly until we have released a fix and
coordinated a disclosure timeline with you.

## Response SLA

We aim to acknowledge and triage reports on the following targets:

| Severity (CVSS) | Acknowledgement | Target fix |
|-----------------|-----------------|------------|
| CRITICAL (>= 9.0) | 24 hours | <= 7 days |
| HIGH (7.0-8.9) | 72 hours | <= 30 days |
| MEDIUM (4.0-6.9) | 7 days | best effort |
| LOW (< 4.0) | best effort | best effort |

## Scope

In scope: the usearch Go services, the Python sidecars (researcher, storm,
embedder, tokenizer-ko, koreanews), the web UI, CI/release pipelines, and
container images.

Out of scope: third-party OIDC providers (Keycloak/Authentik — report to the
vendor), and self-hosted operator misconfiguration (see `ops/security/`).

## Rewards

V1 does **not** run a paid bug-bounty program. We offer public acknowledgement
(with your consent) for valid, responsibly-disclosed reports. A bounty program
may be introduced post-V1 as the user base grows.

## Supported versions

Security fixes target the latest released minor version. Pre-1.0 releases are
not separately back-patched.

## Further reading

- Threat model: `ops/security/threat-model.md`
- Incident response runbook: `ops/security/runbook.md`
- OWASP ASVS L1 compliance: `ops/security/owasp-asvs-checklist.md`
