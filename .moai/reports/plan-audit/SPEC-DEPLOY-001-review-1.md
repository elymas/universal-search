# SPEC Review Report: SPEC-DEPLOY-001 (v0.2.0)

Iteration: 1/3
Verdict: **PASS-WITH-FINDINGS**
Overall Score: **0.88**

> Reasoning context ignored per M1 Context Isolation. The amendment's self-description
> (HISTORY block) was treated as a CLAIM to be disproven, not as evidence. All verdicts
> below are anchored to live code (`go.mod`, `deploy/docker-compose.yml`,
> `internal/index/pg/client.go`, `services/*/Dockerfile`, `deploy/postgres/migrations/`,
> `.env.example`) and the three SPEC docs.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency** — REQ-DEPLOY-001..024 sequential, no gaps, no
  duplicates, consistent 3-digit zero-pad. Verified end-to-end: spec.md §2.1 (001-016),
  §2.2 (017-022), §2.3 (023-024). plan.md references the full 001..024 set (grep
  confirmed 24 unique). NFR-DEPLOY-001..008 likewise contiguous.

- **[PASS] MP-2 EARS format compliance** — Every REQ carries an explicit EARS pattern tag
  and matches it. Spot-to-full verified: REQ-001 (Ubiquitous "Chart shall publish…"),
  REQ-006 (Event-driven "When `helm install`… the system shall…" spec.md:L640),
  REQ-010/012 (State-driven "While…" L677/L694), REQ-021 (Optional "Where
  `ingress.enabled: true`…" L778), REQ-023 (Optional, deferred). No informal "should"
  in normative REQ text. §5 Given/When/Then is correctly scoped as TEST SCENARIOS, not
  mislabeled EARS.

- **[PASS] MP-3 YAML frontmatter validity** — spec.md L1-18: id, version (0.2.0), status
  (draft), created/updated (ISO), priority (P0), plus depends_on/blocks arrays. Required
  fields present and typed. acceptance.md L1-10 frontmatter consistent (same id/version).
  Minor: `labels` is expressed via `milestone`/`priority`/`methodology` rather than a
  literal `labels:` key — non-blocking under this project's frontmatter convention.

- **[PASS] MP-4 Section-22 language neutrality** — N/A-to-FAIL risk checked. The SPEC
  hardcodes no single LSP/tool as "primary." Topology + image set are project-specific
  (Go api/mcp/migrate + 5 Python sidecars), not a 16-language tooling matrix. amd64-only
  is justified (team scale) and arm64 deferral is symmetric. PASS.

---

## Category Scores (rubric-anchored)

| Dimension | Score | Band | Evidence |
|-----------|-------|------|----------|
| Clarity | 0.90 | 0.75-1.0 | Unambiguous REQs; D1-D10 pin decisions. Minor: "10 SQL files" vs actual file-vs-migration count (see D2). |
| Completeness | 1.0 | 1.0 | HISTORY, Overview/WHY, Scope/WHAT, D-decisions/HOW, §2 REQs, §5 scenarios, §6 gates, §4.2 Exclusions (12 specific entries), §4.3 Deferred (well-itemized). acceptance.md has 14 ACs + coverage matrix. |
| Testability | 0.85 | 0.75 | ACs largely binary (`exit 0`, `wc -l ≥ 30`, `< 100MB`, byte-identical diff). One weasel: A10 "README + NOTES quality — manual review by manager-docs" (D4). |
| Traceability | 0.80 | 0.75 | Every REQ 001-024 + NFR 001-008 has ≥1 AC (matrix acceptance.md L356-389). One mismatch: AC-014's REQ-DEPLOY-015 mapping is mislabeled (D1). |

---

## Targeted Verification Results

| Check | Live-code result | Verdict |
|-------|------------------|---------|
| **B1** EnsureSchema is real runner | `internal/index/pg/client.go:90` — `EnsureSchema` `os.ReadDir`s `MigrationsDir`, execs every `*.sql` lexicographically, then `verifySchema` drift-check on `docs` columns. NO `schema_migrations` table. Confirmed. | CONFIRMED |
| **B1** spec uses `usearch migrate`→EnsureSchema Job, NOT golang-migrate | spec D4 (L279-311), REQ-003 (L613-622), REQ-006 (L640-649), §4.1 (L895), acceptance AC-003/AC-006. All say EnsureSchema, "NOT golang-migrate." | CONFIRMED |
| **B1** golang-migrate removed from spec/plan/acceptance | All 18 occurrences in spec/plan/acceptance are NEGATED ("NOT golang-migrate" / "배제 이유" / "OUT-OF-SCOPE"). Live golang-migrate refs survive ONLY in research.md (historical, L155/484/532/926) and tasks/progress (pre-amendment blocker log) — acceptable. | CONFIRMED-RESOLVED |
| **B3** Go base image | `go.mod` → `go 1.25.8`. spec REQ-002 (L609) + acceptance AC-002 (L60) → `golang:1.25.x-alpine`. Matches. | RECONCILED |
| **B3** Dockerfiles all-new | No root or `deploy/` Dockerfile exists; spec standardizes on `deploy/Dockerfile.usearch-{api,mcp,migrate}`. | RECONCILED |
| **B3** migration hook timing | spec REQ-006 `pre-install,pre-upgrade` (L641); acceptance corrected from post→pre (L18, L145). Sound: schema before app Deployments. | RECONCILED |
| **B3** sidecar Dockerfiles reused | `ls services/*/Dockerfile` → all 5 present (researcher, embedder, tokenizer-ko, storm, koreanews). spec L145-149 + acceptance AC-003 reference-only. | RECONCILED |
| **Topology** real 10 compose services | `deploy/docker-compose.yml` L32-252: qdrant, meilisearch, postgres, redis, searxng, litellm, researcher, embedder, prometheus, tokenizer-ko = **10**. `app:` (L19) is a `networks:` entry, NOT a service. storm/koreanews = `services/` dirs only, chart `enabled:false`. usearch-api/mcp newly containerized (host binaries, prometheus scrapes `host.docker.internal`). | CONFIRMED, no phantom |
| **Reductions** coherence | cosign/SBOM/SLSA deferred but BUILD job kept (REQ-018); arm64→V1.1 (NFR-007); tier-3 ESO→V1.1 (REQ-023, blocks install w/ message); ServiceMonitor annotation fallback, no hard CRD require (REQ-019 L763). No V1 AC gates a deferred item — A6/A7 marked DEFERRED, A9 package-verify, S12 deferred, NFR-008 explicitly "NOT a V1 gate." | CLEAN |
| **SEC decoupling** | Chart templates K8s Secret boundary only; does NOT import `internal/security/` (PR#42, absent on main). NFR-008 SEC integration test deferred. D5 + R6 articulate K8s-Secret decoupling. | SOUND |

---

## Defects Found

**D1. acceptance.md:L277,L287 — REQ-DEPLOY-015 mis-mapped — Severity: minor**
AC-014 lists REQ-DEPLOY-015 in its "Covers" header, and the body (L287) ties it to
"OTLP exporter env vars … (REQ-DEPLOY-015)." But spec.md REQ-DEPLOY-015 (L717-723) is the
**in-chart meilisearch/litellm/searxng Deployment + AGPL disclosure** requirement — it has
nothing to do with OTLP. OTLP is wired under D6 / REQ-DEPLOY-007 ConfigMap surface. So
REQ-015's actual behavior (in-chart custom Deployments for meili/litellm/searxng with
pinned tags + AGPL NOTES disclosure) has **no dedicated binary-testable AC**. The coverage
matrix shows a ✓ but it is a phantom trace. Fix: add an AC asserting the three in-chart
Deployments render with compose-pinned image tags and that NOTES/README disclose searxng
AGPL-3.0; correct the OTLP attribution to REQ-007.

**D2. spec.md:L501,L843,L896 + acceptance L142 — "10 SQL files" conflates files vs forward-migrations — Severity: minor**
`ls deploy/postgres/migrations/` returns **10 files**, but one is `0002_deep_runs.down.sql`
(a DOWN migration). EnsureSchema execs **every** `*.sql` lexicographically (client.go:L97-110,
no `.down` exclusion). So the Job will run `0002_deep_runs.down.sql` as part of forward
install — between `0002_cost_ledger.sql` and `0002_deep_runs.up.sql`. The spec asserts
idempotent forward-only application and "no down-migration is invoked" (acceptance EC-003
L332), which is **contradicted by the runner's actual behavior** on the real file set. This
is a latent correctness risk, not just a wording nit: whether the down SQL is `IF EXISTS`-guarded
determines whether install succeeds. R3 (PRESERVE grep audit) partially covers it but the
spec text overstates "forward-only." Fix: state explicitly that EnsureSchema execs the
`.down.sql` too, and require the PRESERVE phase to verify `0002_deep_runs.down.sql` is
idempotent/non-destructive on a fresh DB (or that the SQL-owning SPEC excludes it).

**D3. spec.md:L843 / NFR-DEPLOY-003 — 60-second migration budget unverifiable as written — Severity: minor**
"Migration Job … shall complete within 60 seconds for the 10-file migration sequence on an
empty database" — binary-testable only if a timing assertion exists in CI. No AC/scenario
measures it (AC-013 measures total install < 5 min but not the Job in isolation). Fix: add a
timing check or relax to a documented target.

**D4. spec.md:L1104 (gate A10) / acceptance A10 — weasel acceptance gate — Severity: minor**
"README + NOTES quality | manual review by manager-docs | DOC-001 cross-link integrity" —
"quality" is subjective. Acceptable as a human-review gate but flag: the only binary part is
"cross-link integrity," which should be the stated threshold.

**D5. spec.md:L87-90 (note 6) + OQ3 / acceptance L288,L348 — known-FAIL gate shipped as PASS-conditional — Severity: minor (coordination, not a chart blocker)**
`.env.example` confirmed to lack `OIDC_*`/`JWT_*`/`SESSION_SECRET` (live grep: zero matches).
REQ-DEPLOY-024 parity script (gate A8) will FAIL until those keys are added. The spec correctly
flags this as OQ3 coordination and folds the `.env.example` edit into the chart PR — but A8 is
nonetheless a V1 gate that is **currently red**. This is internally consistent (the fix is in
scope) but the run phase MUST land the `.env.example` edit in the same PR or A8 blocks. Not a
spec defect per se; tracked so it is not lost.

---

## Chain-of-Verification Pass

Second-look findings (re-read sections rather than skimming):

- **Re-read REQ-001..024 end-to-end** (not sampled): numbering clean; EARS tags all match.
  Caught D1 (REQ-015 phantom AC) only by reading REQ-015's actual text against AC-014's claim.
- **Re-checked topology against raw compose** (grep of service block L32-252 + networks
  block): disproved my own initial false-positive that `app:` was an 11th service — it is a
  `networks:` entry. Spec's "10" is CORRECT. No phantom.
- **Re-read migration file listing**: caught D2 — the `.down.sql` in the set vs the
  forward-only idempotency claim. First pass accepted "10 files" at face value; second pass
  cross-referenced the runner's exec-all-`*.sql` loop (client.go L97-110) and found the gap.
- **Exclusions specificity** (§4.2): 12 concrete entries (Terraform/IaC, multi-tenant SaaS,
  KEDA/VPA, ArgoCD/Flux manifests, Grafana JSON, Loki/Tempo/OTel deploy, cert-manager
  bundling, down-migration auto-rollback, macOS automation, mobile, federated multi-cluster).
  Genuinely specific — not vague. PASS.
- **Contradiction sweep across the 4 claimed-B3 fixes**: spec↔acceptance now agree on Go
  1.25, deploy/ Dockerfile path, pre-install hook, sidecar reuse. The 4 original
  contradictions are reconciled. One NEW minor inconsistency surfaced (D1, OTLP↔REQ-015
  mis-trace) but it is a traceability label error, not a value contradiction.

No critical or major defect found. All five findings are minor.

---

## Regression Check (Iteration 2+ only)
N/A — iteration 1.

---

## Recommendation

**Status transition: amend-then-approve** (lightweight). The amendment successfully resolved
B1 (golang-migrate fully removed, EnsureSchema runner adopted) and the 4 B3 contradictions
(reconciled to real Go 1.25.8 / deploy-path / pre-install / sidecar-reuse), and the 10-service
topology is verified against live compose with no phantom. Reductions are coherent and no V1
acceptance criterion gates on a deferred item. SEC decoupling via the K8s-Secret boundary is
sound.

The five findings are all **minor** and do not block draft→approved on their own, but two
(D1, D2) touch correctness/traceability and should be fixed in the same amendment before
implementation rather than discovered in the run phase:

**must_fix_before_implementation (ordered):**
1. **D2** — State that EnsureSchema execs `0002_deep_runs.down.sql` on forward install;
   require PRESERVE-phase verification that it is idempotent/non-destructive on a fresh DB,
   or have the SQL-owning SPEC exclude `.down.sql` from the migrations dir. Reconcile the
   "forward-only / no down-migration invoked" wording (acceptance EC-003 L332).
2. **D1** — Add a dedicated AC for REQ-DEPLOY-015 (in-chart meili/litellm/searxng Deployments
   with compose-pinned tags + searxng AGPL-3.0 disclosure); fix AC-014's OTLP attribution
   from REQ-015 to REQ-007. Correct the coverage matrix.
3. **D3** — Add a CI timing assertion for the 60s migration budget, or relax NFR-003 to a
   documented target.
4. **D5** — Ensure the `.env.example` OIDC/JWT/SESSION_SECRET edit lands in the chart PR so
   gate A8 turns green (already in scope per OQ3 — track explicitly in tasks.md).
5. **D4** — Restate gate A10 threshold as "DOC-001 cross-link integrity" (binary), keeping
   "quality" review as advisory.

If the team prefers, items 3-5 may be accepted as deferred annotations and only D1+D2 fixed
pre-implementation — in that case the verdict upgrades cleanly to PASS.
