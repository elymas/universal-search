# SPEC-ADP-001a Independent Audit

**Auditor**: plan-auditor (adversarial, code-verified)
**Date**: 2026-06-04
**Scope**: spec.md, plan.md, acceptance.md, research.md
**Stance**: Default assumption "this SPEC has defects"; every cited file:line independently verified against the working tree.

> Reasoning context from the SPEC author was not supplied separately; the audit relied only on the four SPEC files plus direct codebase inspection.

---

## Verdict: APPROVE-WITH-FIXES

| Severity | Count |
|----------|-------|
| BLOCKER  | 0 |
| MAJOR    | 3 |
| MINOR    | 3 |
| INFO     | 2 |

The SPEC is technically sound, accurately cited, and scope-disciplined. The MAJOR items are refinements (split an overloaded REQ, tighten a concurrency bound, honestly size the test migration) — none redesign the approach or block the run phase. They should be addressed before `/moai run`.

---

## Dimension Summary

| Dimension | Result | Note |
|-----------|--------|------|
| 1. EARS compliance | PASS w/ defects | Patterns valid; REQ-006 overloaded (M-1), REQ-003 vague bound (M-3) |
| 2. Acceptance / traceability | PASS | No orphan REQs, no orphan criteria; one loose bound (M-3) |
| 3. Code-reference accuracy | PASS (strong) | Every cited path:line resolved and was accurate (I-2) |
| 4. OAuth technical soundness | PASS | client_credentials + oauth.reddit.com design is correct per Reddit's API |
| 5. Scope discipline | PASS (strong) | Tight to Reddit OAuth; exclusions specific with destinations |
| 6. Backward-compat risk | PASS w/ concern | Real and large, but disclosed; magnitude understated (M-2) |

---

## Findings

### MAJOR

**[M-1] REQ-ADP-001a-006 is overloaded and mis-patterned** — `spec.md:239`
The "Ubiquitous" REQ bundles at least four distinct requirements: (a) custom User-Agent on both requests, (b) `Accept: application/json`, (c) graceful registration skip when a credential is absent, (d) `New` returning `ErrMissingCredentials`, plus the CLI `Register` skip and the full `Capabilities()` shape. The registration-skip and `New`-error behavior is *event-driven* ("WHEN `New` is invoked with a missing credential..."), not ubiquitous. Cramming it under one Ubiquitous statement weakens EARS atomicity and makes per-behavior traceability ambiguous.
*Fix*: split REQ-006 into (i) Ubiquitous — UA + Accept on every request; (ii) Event-Driven — `New`/registration credential gate; (iii) Ubiquitous — `Capabilities()` auth shape. Re-map the existing acceptance bullets to the split IDs.

**[M-2] Backward-compat blast radius is materially understated** — `spec.md:493-494`, `plan.md:113`, acceptance.md DoD
The proposed `New` gate errors when credentials are absent and `!SkipAuthCheck && HTTPClient == nil`. I verified the existing Reddit test suite (59 `Test`/`Benchmark` funcs across 7 files) constructs the adapter via `New(Options{BaseURL: ts.URL})` or `New(Options{})` — i.e., **`BaseURL`-based stubs, not `HTTPClient`** — at ~25+ sites: `reddit_test.go:14,28,37,50,118`; `search_test.go:48,81,124,150,176,199,238,267,294,322,361,391,424,452,479,528,557,599,667,713`; `client_test.go:126,155,181,234,260,317`. Because these set `BaseURL` (not `HTTPClient`), the `HTTPClient == nil` escape hatch does **not** spare them — every one will start failing the credential check and must have `SkipAuthCheck: true` added. The SPEC's phrasing ("Tests that construct the adapter with a stub server adopt `SkipAuthCheck=true`") makes this sound incidental; it is a ~25-edit sweep across three test files plus `TestSearchConcurrentSafe`.
*Fix*: state the explicit count and enumerate the affected files in §8 / Milestone 7 so the run phase budgets for it and the drift guard does not flag it as unplanned scope.

**[M-3] Concurrency guarantee is vague in the REQ and too loose in acceptance** — `spec.md:236` (REQ-003), `spec.md:248` (NFR-001), `spec.md:290,346` (acceptance)
The REQ body says the token endpoint "SHALL be contacted at most a bounded small number of times (ideally once)". "A bounded small number" is not independently testable — the measurable bound (`≤ 5`) lives only in acceptance. Worse, `≤ 5` is too permissive for the mandated mechanism: `sync.Mutex` + double-checked expiry (or singleflight) guarantees **exactly 1** token POST under 50 concurrent first-time callers. An implementation that regressed to 2–5 POSTs (e.g., a check-then-act race) would still pass `≤ 5`, defeating the NFR's purpose.
*Fix*: make the REQ normative bound concrete ("at most 1 under the locked-refresh mechanism; document the exact bound") and tighten the acceptance assertion to `== 1` (or `≤ 2` only if a documented benign double-acquire window is intentionally accepted).

### MINOR

**[m-4] Ambiguous boolean grouping in the `New` gate description** — `spec.md:141` (§2.1 item a)
Written as `ClientID == "" || ClientSecret == "" AND !SkipAuthCheck AND HTTPClient == nil`. In Go precedence (`&&` binds tighter than `||`) this parses as `ClientID=="" || (ClientSecret=="" && !SkipAuthCheck && HTTPClient==nil)` — meaning an empty `ClientID` would error even when `SkipAuthCheck=true`, breaking the test seam. The *intended* logic (correctly restated in the REQ-006 acceptance at `spec.md:319-321`) is `(ClientID=="" || ClientSecret=="") && !SkipAuthCheck && HTTPClient==nil`, matching the GitHub reference (`github.go:78`: `!opts.SkipAuthCheck && opts.HTTPClient == nil && opts.Token == ""`).
*Fix*: add parentheses around the OR in item (a).

**[m-5] REQ-001 and REQ-002 are mildly compound** — `spec.md:234,235`
Each chains several `SHALL` clauses (acquire + parse + cache + proceed; issue + header + "SHALL NOT acquire" + param identity). Each clause is testable, so this is tolerable, but consider trimming the parent-parity restatements (query params, parsing) to a single "identical to parent SPEC" reference rather than re-listing them inside the normative REQ.

**[m-6] Default search host change requires `SkipAuthCheck` on non-stub tests too** — `spec.md:381`, `reddit.go:17`
Changing the default base URL from `www.reddit.com/search.json` to `oauth.reddit.com/search` means tests that rely on the default (e.g., `TestHealthcheck` at `reddit_test.go:118`, which sets only `HealthcheckTarget`) now also hit the credential gate. This is a subset of M-2 but worth calling out as a distinct case (no `BaseURL`, no stub) so it is not overlooked.

### INFO

**[I-1] Frontmatter has no `labels` field** — `spec.md:1-17`
The frontmatter uses `owner`, `methodology`, `coverage_target` instead of a generic `labels` array. This matches the sibling SPEC-ADP-001 convention in this repo, so it is not treated as a schema violation — noted only for completeness. `id`, `version`, `status`, `priority`, `created`/`updated` are all present and well-typed.

**[I-2] Code-reference accuracy is excellent — contrary to the repo's historical warning**
The memory note warns that this repo's drafts have cited nonexistent paths/IDs before. For this SPEC that risk did **not** materialize. Every spot-checked citation resolved and was accurate:
- `reddit.go:17,33-49,63-91,99-118` — base URL, 4-field Options, no-validation `New`, `RequiresAuth=false`/`RateLimitPerMin=10` Capabilities. ✓
- `client.go:24-29,71-75,102-124` — 4-host allowlist, `doRequest` UA+Accept, `categorizeStatus` with `4xx→Permanent` and **no** 401 carve-out (confirming the SPEC's premise). ✓
- `search.go:43-110,113-125`; `errors.go:16,33-63`. ✓
- `github.go:46-61,77-84,137-161` (`Token`/`SkipAuthCheck`, `New` gate, `RequiresAuth=true`/`USEARCH_GITHUB_TOKEN`); `github/errors.go:13-14` (`ErrMissingToken`). ✓
- `naver.go:118-132,196-197` (dual-secret). ✓
- `cmd/usearch/query.go:461-465` (Reddit unconditional) and `:476-487` (GitHub conditional) — **exact**. ✓
- `registry.go:147-166` (`RegisterWithOptions` AuthEnvVars validation). ✓
- `pkg/types/capabilities.go:38-61`; `pkg/types/errors.go` — `ErrPermanent`, `ErrSourceUnavailable`, `ErrRateLimited` and `SourceError{Adapter,Category,HTTPStatus,Cause,RetryAfter}` all exist, so the acceptance assertions (`errors.Is(err, types.ErrSourceUnavailable)`, `HTTPStatus`) are valid. ✓
- Parent `SPEC-ADP-001/spec.md:39-43,69-77,253-256,913-915` — OAuth deferral and rate discrepancy verbatim. ✓

---

## Technical Soundness of the OAuth Design (Dimension 4 detail)

- **`POST https://www.reddit.com/api/v1/access_token`, HTTP Basic (`client_id:client_secret`), body `grant_type=client_credentials`** — correct for Reddit's app-only / "userless" flow. ✓
- **Bearer against `https://oauth.reddit.com`** — correct; authenticated traffic must not go to `www.reddit.com`. ✓
- **`Authorization: bearer <token>` (lowercase "bearer")** — intentional and correct; Reddit's docs use lowercase. Not a defect.
- **401 → invalidate + refresh + retry-once → `CategoryUnavailable`/`ErrTokenRefreshExhausted`; 403 stays `CategoryPermanent`; token-POST 401/403 → `CategoryPermanent`/`ErrTokenAcquisitionFailed`** — sound, and the 401 carve-out correctly fixes the existing `categorizeStatus` defect (`client.go:112-114` sweeps 401 into `Permanent`).
- **`RateLimitPerMin` 10 → 60** — consistent with the authenticated ceiling cited in the parent SPEC.
- **Graceful-skip registration mirroring GitHub** — structurally faithful to `query.go:476-487`; the registry second-net at `registry.go:151-157` does validate `AuthEnvVars` on `Register`, so the design's "two-layer" claim is accurate.

No correctness defect found in the OAuth mechanics.

---

## Recommended Fix Set (priority-ordered)

1. (M-1) Split REQ-ADP-001a-006 into UA/Accept (Ubiquitous), credential-gate (Event-Driven), and Capabilities-shape (Ubiquitous); re-map acceptance bullets.
2. (M-3) Replace "a bounded small number" with a concrete bound in REQ-003/NFR-001 and tighten the concurrency assertion from `≤ 5` toward `== 1`.
3. (M-2 / m-6) Enumerate the ~25+ affected test constructions and the default-host non-stub cases in §8 / Milestone 7 so the migration is planned, not drift.
4. (m-4) Parenthesize the `New` credential condition in §2.1 item (a).
5. (m-5) Trim parent-parity restatements inside REQ-001/002 to a single reference.

No BLOCKER; the run phase may proceed once M-1, M-2, and M-3 are addressed.
