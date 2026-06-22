---
id: SPEC-SEC-004
version: 0.1.0
status: draft
created: 2026-06-23
updated: 2026-06-23
author: limbowl
priority: P3
issue_number: 0
title: Semgrep Python logging secret-disclosure triage + check-doc-credentials Naver scan gap closure
milestone: post-V1 polish — security CI debt
owner: expert-security
methodology: ddd
coverage_target: 85
depends_on: [SPEC-SEC-001]
blocks: []
related: [SPEC-DOC-002]
---

# SPEC-SEC-004: Semgrep Python logging secret-disclosure triage + Naver doc-scan gap closure

## HISTORY

- 2026-06-23 (initial draft v0.1.0, limbowl via manager-spec):
  P3 security CI-debt remediation SPEC. This SPEC does NOT introduce a new
  security system. It resolves two independent, verified findings from an
  ad-hoc broad semgrep scan + a code review of the documentation credential
  scanner. DDD methodology (ANALYZE existing surface → PRESERVE working
  controls → IMPROVE with suppression + a wired pattern). Two root causes,
  both confirmed against the live codebase:

  (1) **Semgrep `logger-credential-leak` false positives (4 confirmed).**
  The `python.lang.security.audit.logging.logger-credential-leak` rule keys
  on `logger.<level>()` sinks whose argument name/format matches a credential
  wordlist that includes the generic token `error`. The four Python sidecar
  logging calls pass request_id (UUIDs), exception class names, and static
  validation-error strings — operational telemetry, never secrets. Every hit
  is a false positive. Crucially, these findings come from an **ad-hoc broad
  semgrep run** (`p/python` or `--config auto`), NOT the repository's merge
  gate, which loads only `p/golang` + `p/owasp-top-ten` + `p/jwt`
  (`.github/workflows/security.yml:165-167`). So the findings are currently
  non-blocking; the fix is suppress-with-justification, not refactor.

  (2) **`check-doc-credentials.sh` Naver scan is a no-op.**
  `NAVER_SECRET_PATTERN` is defined (`scripts/check-doc-credentials.sh:41`)
  but (a) never passed to `check_pattern`, and (b) its PCRE negative-lookahead
  `(?!...)` is invalid under the `grep -E` (ERE) that `check_pattern` uses
  (`:58-72`). Only github-pat/aws-key/hex-40/jwt are wired (`:69-72`). Naver
  credential detection in docs is silently absent. The gap is already
  self-documented in-code via `TODO(SPEC-DOC-002)` at `:36-40`, so this is
  deliberate-but-unresolved debt, not a regression.

  4 EARS REQs + 2 NFRs. Methodology: DDD. Coverage target 85 (script + any
  Go/Python test touched). Harness: standard (P3, no new attack surface).
  Owner: expert-security.

---

## 1. Overview

SPEC-SEC-004 is a P3 security CI-debt remediation. It closes out two
verified findings without inventing new security machinery: it documents and
suppresses four false-positive semgrep logging findings, clarifies which
semgrep rulesets are merge-blocking, and wires the dormant Naver
documentation-credential pattern so the doc scanner actually evaluates it.

### 1.1 Findings classification

| Finding | Locus | semgrep/scanner issue | Status | Disposition |
|---------|-------|-----------------------|--------|-------------|
| F1 | `services/tokenizer-ko/src/tokenizer_ko/obs.py:95-100` | `logger.warning(... error=%s, ..., error)` — `error` values are static strings ("empty text", "text too large") | **false-positive** | suppress-with-justification (`# nosem`) |
| F2 | `services/tokenizer-ko/src/tokenizer_ko/app.py:150-155` | `logger.error(... %s, ..., exc, exc_info=True)` — logs request_id + exception text | **false-positive** | suppress-with-justification (`# nosem`) |
| F3 | `services/researcher/src/researcher/app.py:105` | `logger.error({"message": "Unhandled exception", "error": str(exc)})` — generic handler, exception string only | **false-positive** | suppress-with-justification (`# nosem`) |
| F4 | `services/embedder/src/embedder/app.py:163,179` | `logger.error("embed.oom" (`:163`)/"embed.error" (`:179`), extra={request_id, exception_class})` — request_id + class name only | **false-positive** | suppress-with-justification (`# nosem`) |
| F5 | `services/youtube-extract/src/youtube_extract/app.py:114` | `logger.exception("search.unexpected_error")` — static string only, no `error` arg | **needs-decision** | include in `# nosem` sweep ONLY if a captured scan names it (likely does NOT trigger the rule) |
| F6 | `.github/workflows/security.yml:165-167` | merge gate runs `p/golang p/owasp-top-ten p/jwt` only; `python.lang.security.audit.logging.*` lives in `p/python` and is NOT scanned by the gate | **confirmed** | clarify gate scope (REQ-SEC4-020) |
| F7 | `scripts/check-doc-credentials.sh:41 (def) vs :58-72 (check_pattern)` | `NAVER_SECRET_PATTERN` defined-but-unwired; PCRE lookahead invalid under `grep -E` → Naver scan is a no-op | **confirmed** | wire pattern + switch to PCRE matcher (REQ-SEC4-030, REQ-SEC4-040) |

[HARD] Every false-positive (F1-F4) and the needs-decision item (F5) is
explicitly flagged above. No `logger.<level>()` call in F1-F4 logs a secret;
the disposition is **suppress, NOT refactor** — refactoring calls that log no
secrets adds risk for zero benefit.

### 1.2 Motivation

The four logging findings are noise that obscures real secret-disclosure
findings in future broad scans. The Naver doc-scan no-op means a 32-char
Naver client secret pasted into an adapter reference page would ship
undetected, defeating the purpose of `check-doc-credentials.sh`. Both are
small, bounded fixes that pay down accumulated CI debt.

### 1.3 Scope note — relationship to SPEC-SEC-001

SPEC-SEC-001 owns the consolidated security CI surface (`security.yml`,
gosec/semgrep/gitleaks/Trivy). This SPEC operates within that surface: it
edits the same `security.yml` semgrep step (gate-scope clarification) and the
Python sidecar logging calls. It does NOT add a new workflow, package, or
gate. `depends_on: SPEC-SEC-001` because `security.yml` and the semgrep
ruleset selection are SEC-001 artifacts.

---

## 2. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-SEC4-010** | Conditional (IF-THEN) | IF a Python service `logger.<level>()` call records only request identifiers (UUIDs), exception class names, or static validation-error strings — and records no credential, token, or secret value — THEN the semgrep `logger-credential-leak` finding on that call SHALL be classified a false positive and suppressed with an inline `# nosem: python.lang.security.audit.logging.logger-credential-leak` comment carrying a one-line justification. The suppression SHALL be applied to exactly the four confirmed sinks: `services/tokenizer-ko/src/tokenizer_ko/obs.py:95`, `services/tokenizer-ko/src/tokenizer_ko/app.py:150`, `services/researcher/src/researcher/app.py:105`, and `services/embedder/src/embedder/app.py` (both `:163` and `:179`). The candidate at `services/youtube-extract/src/youtube_extract/app.py:114` SHALL be suppressed ONLY if a captured semgrep scan (SARIF/JSON) names it; otherwise it SHALL be left untouched. The four logging calls SHALL NOT be refactored. | P3 | A captured semgrep `p/python` scan over `services/**/*.py` reports zero `logger-credential-leak` findings after the `# nosem` comments are added; the four call expressions are byte-identical to pre-change except for the appended comment. |
| **REQ-SEC4-020** | Ubiquitous | The security CI gate SHALL define exactly which semgrep rulesets are merge-blocking. The merge-blocking set SHALL be documented in `.github/workflows/security.yml` (at or adjacent to the semgrep step, currently `:165-167`) as a comment enumerating the loaded `--config` rulesets, AND findings from any ruleset outside that documented set SHALL NOT block merges. WHERE Python SAST is desired as a merge-blocking control, `--config p/python` SHALL be added to the documented gate set so the REQ-SEC4-010 suppressions become load-bearing; WHERE it is not desired, the comment SHALL state that `p/python` findings are advisory-only and obtained via ad-hoc scans. | P3 | `security.yml` contains a comment naming the exact merge-blocking semgrep ruleset set; the documented set matches the actual `--config`/`SEMGREP_RULES` values; the decision (p/python in-gate vs advisory-only) is explicit. |
| **REQ-SEC4-030** | Ubiquitous | The `scripts/check-doc-credentials.sh` script SHALL evaluate every credential pattern it defines, and no pattern SHALL remain defined-but-unwired. Specifically, `NAVER_SECRET_PATTERN` SHALL be passed to `check_pattern` (a `check_pattern "naver-secret" "${NAVER_SECRET_PATTERN}"` invocation SHALL be added after the existing wired patterns at `:72`). The `TODO(SPEC-DOC-002)` comment at `:36-40` acknowledging the gap SHALL be removed or updated to reflect resolution. | P3 | A grep/static check confirms every `*_PATTERN` variable defined in the script appears in a `check_pattern` call; `NAVER_SECRET_PATTERN` is wired; the stale TODO is gone or updated. |
| **REQ-SEC4-040** | Conditional (IF-THEN) | IF a credential pattern requires PCRE constructs such as a negative lookahead (`(?!...)`), THEN `check_pattern` SHALL execute that pattern with a PCRE-capable matcher (`grep -P` / `grep -nP`), and SHALL NOT pass it to an ERE-only matcher (`grep -E` / `grep -nE`). The `check_pattern` grep invocations (`scripts/check-doc-credentials.sh:62,64`) SHALL be switched from `-E`/`-nE` to `-P`/`-nP`. NOTE: `grep -P` (PCRE) is a GNU-grep extension — the `docs.yml` CI job runs on ubuntu (GNU grep) and is fine, but local and pre-commit invocations on macOS/BSD grep lack `-P` and will error; the script SHALL guard against this (e.g. detect a `-P`-capable grep / prefer `ggrep` when present) or carry a scope-note that the Naver PCRE check is CI-only. WHEN an adapter reference page under `docs/content/{en,ko}/reference/adapters/*.mdx` contains a 32-character alphanumeric value resembling a Naver client secret and not matching an approved placeholder, the script SHALL exit with a non-zero status. Before the Naver pattern is enabled in CI, a one-time false-positive review SHALL be run against the current `docs/content/**/reference/adapters/*.mdx`, and any confirmed false positive SHALL be added as a placeholder exclusion. | P3 | `check_pattern` uses `grep -P`/`grep -nP`; a fixture `.mdx` containing a bare 32-char alnum Naver-like secret causes a non-zero exit; a fixture containing only the approved placeholder passes; the one-time FP review against existing adapter docs is recorded (zero unexcused matches at enable time). |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-SEC4-001** | Behavior preservation (logging telemetry) | The four suppressed `logger.<level>()` calls SHALL emit byte-identical log output before and after the change (only an inline `# nosem` comment is added). No log message, format string, argument, or `extra=` payload SHALL change. Operational telemetry (request_id, exception class names, static error strings) SHALL remain intact. |
| **NFR-SEC4-002** | Doc-scan false-positive containment | Enabling the Naver pattern in `check-doc-credentials.sh` SHALL NOT regress the documentation drift gate. The one-time FP review (REQ-SEC4-040) SHALL bring unexcused matches against current `docs/content/**/reference/adapters/*.mdx` to zero before the pattern is enabled in CI. WHERE the bare 32-char alnum match floods with unacceptable false positives that placeholder exclusions cannot contain, the documented alternative SHALL be to delete `NAVER_SECRET_PATTERN` and its TODO rather than leave dead code. |

---

## 4. Exclusions (What NOT to Build)

[HARD] The following are explicitly out of scope. Each has a known
destination or rationale.

- **Refactoring the four logging calls (F1-F4).** → Not done. The calls log
  no secrets; refactoring adds regression risk for zero security benefit. The
  disposition is suppress-with-justification only.

- **Suppressing `youtube-extract/app.py:114` (F5) unconditionally.** → Only
  if a captured semgrep scan names it. It uses a static string with no
  `error` argument and probably does not trigger the rule; suppressing it
  blindly would add an unjustified `# nosem`.

- **Adding `gosec`/`gitleaks`/`Trivy` rules or a new CI workflow.** → Out of
  scope. SPEC-SEC-001 owns the security CI surface; this SPEC only edits the
  existing semgrep step and Python logging lines.

- **Rewriting `check_pattern` beyond the `-E`→`-P` switch and the Naver
  wiring.** → The github-pat/aws-key/hex-40/jwt patterns are working; they
  are left unchanged (they are valid under PCRE as well, so the matcher
  switch does not break them).

- **Replacing semgrep with a different SAST engine.** → Out of scope; the
  finding is a triage of existing semgrep output, not a tool reselection.

- **Broadening the merge gate to all of `p/python`.** → REQ-SEC4-020 records
  the decision (in-gate vs advisory-only) but does not mandate broadening;
  broadening is a deliberate choice the SPEC surfaces, not a default.

- **Capturing/committing the authoritative semgrep SARIF.** → The captured
  scan is an input used to lock the precise F1-F5 set during run phase; this
  SPEC does not require checking a SARIF artifact into the repo.

- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). → Polish-SPEC
  pattern, consistent with SPEC-SEC-001.

---

## 5. Acceptance Criteria

Headline acceptance (CI-debt SPEC): **the security CI semgrep job and the
`check-doc-credentials.sh` doc-scan check pass on `main`**, with the four
false-positive logging findings suppressed and the Naver pattern wired and
FP-reviewed. Per-REQ acceptance summaries are inline in §2; full
Given-When-Then scenarios live in `.moai/specs/SPEC-SEC-004/acceptance.md`.

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | Logging FP suppression: capture a `semgrep --config p/python` scan over `services/**/*.py`; add `# nosem: python.lang.security.audit.logging.logger-credential-leak` to the four confirmed sinks (obs.py:95, tokenizer app.py:150, researcher app.py:105, embedder app.py:163 & :179); re-scan reports zero `logger-credential-leak` findings; the four call expressions are unchanged except for the comment. | REQ-SEC4-010, NFR-SEC4-001 |
| §5.2 | F5 conditional: inspect captured scan; IF `youtube-extract/app.py:114` is named, add the same `# nosem`; ELSE leave it untouched and record the decision. | REQ-SEC4-010 |
| §5.3 | Gate-scope clarity: `security.yml` semgrep step carries a comment enumerating the merge-blocking rulesets; the comment matches the actual `--config`/`SEMGREP_RULES` values; the in-gate-vs-advisory decision for `p/python` is explicit. | REQ-SEC4-020 |
| §5.4 | Pattern wiring: every `*_PATTERN` variable defined in `check-doc-credentials.sh` appears in a `check_pattern` call; `NAVER_SECRET_PATTERN` is wired after `:72`; the `TODO(SPEC-DOC-002)` at `:36-40` is removed or updated. | REQ-SEC4-030 |
| §5.5 | PCRE matcher + Naver detection: `check_pattern` uses `grep -P`/`grep -nP`; a fixture `.mdx` with a bare 32-char alnum Naver-like secret exits non-zero; a fixture with only the approved placeholder passes; the existing wired patterns (github-pat/aws-key/hex-40/jwt) still pass. | REQ-SEC4-040 |
| §5.6 | One-time FP review: run the wired Naver pattern against current `docs/content/{en,ko}/reference/adapters/*.mdx`; record unexcused matches; add placeholder exclusions until zero; if the match floods uncontainably, exercise the documented delete-pattern alternative. | REQ-SEC4-040, NFR-SEC4-002 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-SEC-001 (implemented, M8)** — owns `.github/workflows/security.yml`
  and the semgrep ruleset selection (`p/golang` + `p/owasp-top-ten` +
  `p/jwt`). REQ-SEC4-020 edits the same semgrep step. This SPEC operates
  within the SEC-001 security CI surface and adds no new workflow.

### 6.2 Related (related)

- **SPEC-DOC-002** — owns `scripts/check-doc-credentials.sh` and the in-code
  `TODO(SPEC-DOC-002)` (`:36-40`) marking the unwired Naver pattern that
  REQ-SEC4-030 resolves. REQ-SEC4-040 also operates on this script.

### 6.3 Downstream blocked SPECs (blocks)

- (none — this is leaf CI-debt remediation.)

### 6.4 Blockers / open items (do NOT block plan-audit PASS)

- **B1 — precise F1-F5 set not yet locked from a captured scan.** The exact
  four cited loci are inferred from rule semantics (the wordlist token
  `error` on `logger` sinks), not from a semgrep SARIF/JSON committed in the
  repo. `youtube-extract/app.py:114` (F5) is a plausible 5th candidate but
  uses a static string only. **An authoritative semgrep scan output SHALL be
  captured in run phase to lock the precise set BEFORE mass-applying
  `# nosem`** (REQ-SEC4-010 conditions the F5 suppression on exactly this).

- **B2 — Naver wire-with-`grep -P` needs a one-time FP review.** A bare
  32-char alnum match may flood against current
  `docs/content/**/reference/adapters/*.mdx` (the in-code
  `TODO(SPEC-DOC-002)` explicitly warns of this). Enabling without exclusion
  tuning could break the docs drift gate. REQ-SEC4-040 + NFR-SEC4-002 require
  the review and provide a documented delete-pattern fallback.

### 6.5 External dependencies (run-phase)

- (none — uses existing semgrep + `grep -P` already available in CI.)

---

## 7. Files to Create / Modify

### 7.1 Modified (estimated; final list owned by run phase)

| Path | Change |
|------|--------|
| `services/tokenizer-ko/src/tokenizer_ko/obs.py` (`:95`) | append `# nosem: python.lang.security.audit.logging.logger-credential-leak` with justification (logs static error strings) — REQ-SEC4-010 |
| `services/tokenizer-ko/src/tokenizer_ko/app.py` (`:150`) | append `# nosem` with justification (logs request_id + exception text) — REQ-SEC4-010 |
| `services/researcher/src/researcher/app.py` (`:105`) | append `# nosem` with justification (generic handler, exception string only) — REQ-SEC4-010 |
| `services/embedder/src/embedder/app.py` (`:163`, `:179`) | append `# nosem` with justification (request_id + exception class name only) — REQ-SEC4-010 |
| `services/youtube-extract/src/youtube_extract/app.py` (`:114`) | CONDITIONAL — `# nosem` ONLY if captured scan names it — REQ-SEC4-010 |
| `.github/workflows/security.yml` (`:165-167`) | add comment enumerating merge-blocking semgrep rulesets; record p/python in-gate-vs-advisory decision — REQ-SEC4-020 |
| `scripts/check-doc-credentials.sh` (`:41`, `:62`, `:64`, `:72`, `:36-40`) | switch `check_pattern` grep `-E`/`-nE` → `-P`/`-nP`; add `check_pattern "naver-secret" "${NAVER_SECRET_PATTERN}"`; remove/update `TODO(SPEC-DOC-002)` — REQ-SEC4-030, REQ-SEC4-040 |

### 7.2 Created (estimated)

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | test fixture under `scripts/` or service test dir | `.mdx` fixtures: one bare 32-char Naver-like secret (must fail), one approved-placeholder-only (must pass) for REQ-SEC4-040 |

### 7.3 Existing — Unchanged

- The four logging calls' message/format/args/`extra=` payloads — only a
  `# nosem` comment is appended (NFR-SEC4-001).
- `check_pattern`'s github-pat/aws-key/hex-40/jwt wired patterns — left
  intact; valid under PCRE so the `-E`→`-P` switch does not break them.
- All other `.github/workflows/*.yml`, all Go packages — out of scope.

---

## 8. Open Questions

1. **F5 disposition** — does `youtube-extract/app.py:114` actually trigger
   `logger-credential-leak`? Resolved by the captured scan in run phase
   (B1). REQ-SEC4-010 already conditions the suppression on the scan output.

2. **p/python in-gate vs advisory** — should Python SAST be merge-blocking?
   REQ-SEC4-020 records the decision either way; the recommended default is
   advisory-only (keep the gate fast, suppressions documented) unless the
   team wants Python SAST enforced — decided in run phase / annotation.

3. **Naver FP rate** — if the bare 32-char match floods uncontainably against
   current adapter docs, the documented fallback (NFR-SEC4-002) is to delete
   the pattern + TODO rather than leave dead code. Decided by the one-time FP
   review (B2).

These do not block plan-audit PASS — they are known unresolved scope edges
tagged with rationale and a resolution path.

---

## 9. References

Internal (project files):

- `services/tokenizer-ko/src/tokenizer_ko/obs.py` (`:95-100`) — F1 sink
- `services/tokenizer-ko/src/tokenizer_ko/app.py` (`:120,128,150-155`) — F2 sink + static error string sources
- `services/researcher/src/researcher/app.py` (`:105`) — F3 sink
- `services/embedder/src/embedder/app.py` (`:163,179`) — F4 sinks
- `services/youtube-extract/src/youtube_extract/app.py` (`:114`) — F5 candidate
- `.github/workflows/security.yml` (`:165-167`) — F6 merge-gate semgrep step
- `scripts/check-doc-credentials.sh` (`:36-40,41,58-72`) — F7 unwired Naver pattern + ERE-only matcher
- `docs/content/{en,ko}/reference/adapters/*.mdx` — REQ-SEC4-040 scan target
- `.moai/specs/SPEC-SEC-001/spec.md` — security CI surface owner (semgrep ruleset selection)

External:

- semgrep `python.lang.security.audit.logging.logger-credential-leak`: https://semgrep.dev/r/python.lang.security.audit.logging.logger-credential-leak
- semgrep `# nosem` inline suppression: https://semgrep.dev/docs/ignoring-files-folders-code
- CWE-532 Insertion of Sensitive Information into Log File: https://cwe.mitre.org/data/definitions/532.html
- GNU grep `-P` (PCRE) vs `-E` (ERE): https://www.gnu.org/software/grep/manual/grep.html

---

*End of SPEC-SEC-004 v0.1.0 (draft).*
