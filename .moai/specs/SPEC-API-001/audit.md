# SPEC-API-001 Independent Audit

Auditor: plan-auditor (adversarial). Iteration: 1.
Method: M1 context isolation — audited spec.md/plan.md/acceptance.md/research.md against the actual `main` working tree only. Reasoning context from the author ignored per M1.

**Verdict: APPROVE-WITH-FIXES**

Severity counts: BLOCKER 0 · MAJOR 3 · MINOR 3 · INFO 2

---

## Claim Verification (every cited reference resolved)

| SPEC claim | Status | Evidence |
|------------|--------|----------|
| `cmd/usearch-api/main.go` says "see SPEC-IR-001", never `ListenAndServe`, nil synth, empty registry, `os.Exit(0)` | **CONFIRMED** | main.go L2 `// Full implementation lands in SPEC-IR-001.`, L41 `NewSynthesisHandler(nil, …)`, L45 `adapters.NewRegistry(nil)`, L48 `_ = mux … owned by SPEC-IR-001`, L50 `usearch-api: not implemented (see SPEC-IR-001)`, L51 `os.Exit(0)`. No `ListenAndServe` anywhere. |
| SPEC-IR-001 is the Intent Router, library-only (not the API server) | **CONFIRMED** | SPEC-IR-001 spec.md L3 `title: Intent Router v0`, L26 "library-only exposure (no HTTP endpoint)". It blocks FAN/CLI/SYN/ADP, not API. |
| Build helpers live in `package main` at `cmd/usearch/query.go` and cannot be imported by `cmd/usearch-api/` | **CONFIRMED** | query.go L10 `package main`; `buildProductionRegistry` L458, `buildRouter` L518, `buildProductionSynth` L533. Go forbids importing one `main` from another — extraction to `internal/` is necessary and correct (D2/M0). |
| Synthesis-seam mismatch: CLI-local `synthClientIface` returns `synthResult` vs `handlers.SynthesisClient` returns `synthesis.Result` | **CONFIRMED** | query.go L65-66 `synthClientIface.Synthesize(ctx, query, lang, docs) (synthResult, error)`; handlers/synthesis.go L22-24 `SynthesisClient.Synthesize(ctx interface{}, req synthesis.Request) (synthesis.Result, error)`. Real client `internal/synthesis/client.go:61` returns `(Result, error)`. The `productionSynthAdapter` (query.go L545-569) already bridges `*synthesis.Client` → `synthResult`. Plan's recommendation to expose the concrete `*synthesis.Client` from the shared package is sound and behavior-preserving. |
| Frontend route prefix `/api/...`; stub registered bare `/query/stream` | **CONFIRMED** | api.ts L37/43/47/55 use `/api/query`, `/api/query/stream`, `/api/sources`, `/api/history`. main.go L41 registers bare `/query/stream`. Mismatch real; D3 reconciliation correct. |
| SSE event-name mismatch `done` (server) vs `complete` (frontend) | **CONFIRMED** | streamsynth.go L151 `WriteEvent("done", …)`; sse-client.ts L48 listens for `complete`, reads `parsed.elapsed_ms`; L52 `error` reads `parsed.message`. Frontend has NO `done` listener (L85-88) — unmapped server never fires completion. Mapping in REQ-API-009 is required and correct. |
| SearchResult / Citation shapes | **CONFIRMED** | api.ts L6-20: `SearchResult{answer,citations[],query,sources_used[],elapsed_ms}`, `Citation{index,title,url,snippet,source}` — matches REQ-API-006 verbatim. |
| Deferred items exist: `deep.go` (nil deps), admin pkg, nil AuditQuerier | **CONFIRMED** | `cmd/usearch-api/handlers/deep.go` exists; `internal/api/admin/` has adapters/health/audit/loopback; main.go L81-83 `NewAuditHandler(nil)` with TODO. `AuditQuerier`/`QueryEntries` defined (handler_audit.go L25, mocked in tests). Out-of-Scope list accurate. |
| docker-compose lacks `usearch-api`; Dockerfile `--healthcheck` unimplemented | **CONFIRMED** | docker-compose.yml has searxng (L116, host port `${SEARXNG_PORT:-8080}`) and researcher (L173) but no usearch-api. Dockerfile.usearch-api L34 `EXPOSE 8080 9090`, L37 `CMD ["/usearch-api","--healthcheck"]`; flag not handled in main.go. REQ-API-004/016 correct. |
| depends_on [SPEC-IR-001, SPEC-FAN-001, SPEC-SYN-004, SPEC-CORE-001] all exist | **CONFIRMED** | All four dirs present. Titles: IR-001 Intent Router, FAN-001 Multi-source Fanout, SYN-004 Streaming response (SSE), CORE-001 Adapter Interface and NormalizedDoc Contract. All relevant to the search-path pipeline. |

The research.md is unusually accurate — every file:line citation I sampled resolved correctly. This is a high-integrity SPEC.

---

## Findings (severity-tagged)

### MAJOR

**M-1 — REQ-API-009 invents a `citation` SSE event that no source produces.**
streamsynth emits only `sentence` / `done` / `error` (streamsynth.go L128, L151, and the error path). Citations are **embedded inside the sentence payload** (`SentencePayload.Citations []CitationRef`, streamsynth.go L58). There is NO standalone per-citation event (`grep WriteEvent("citation"` returns nothing). Yet the frontend `sse-client.ts` L45-47 registers an `onCitation` listener for a discrete `event: citation`, and REQ-API-009 claims "per-citation data → `citation` (`{index,title,url,snippet,source}`)". The server cannot map a non-existent source event, and `CitationRef` is `{marker,doc_id,url,title}` (streamsynth.go L46-51), not `{index,title,url,snippet,source}`. Acceptance Scenario 4 silently omits any `event: citation` assertion, hiding the gap.
Fix: decide explicitly — either (a) the server synthesizes discrete `citation` events from `SentencePayload.Citations` and maps `CitationRef{marker,doc_id,url,title}` → frontend `{index,title,url,snippet,source}` (note `snippet`/`source` have no source — specify their origin), or (b) drop the `citation` event and document that the frontend reads citations from sentence payloads. Add a Scenario 4 assertion for whichever is chosen.

**M-2 — REQ-API-011 `/api/sources` shape is not derivable from `Capabilities()`.**
REQ-API-011 / research §7 specify building `{name, category, enabled, latency_ms?}` from `registry.List()` + per-adapter `Capabilities()`. But `pkg/types.Capabilities` (capabilities.go L38-62) has `SourceID, DisplayName, DocTypes, SupportedLangs, SupportsSince, RequiresAuth, AuthEnvVars, RateLimitPerMin, DefaultMaxResults, Notes` — **no `category` field and no `enabled` field**. `category` and `enabled` have no defined source.
Fix: specify the derivation — e.g. `category` from `DocTypes[0]` or a new mapping, `enabled` from registry registration/toggle state (admin SnapshotForAdmin carries enabled, but that path is out of scope and NFR-API-005 forbids the secret-bearing snapshot). Resolve before run.

**M-3 — REQ-API-013 and REQ-API-014 have no acceptance scenario (traceability gap).**
Every other REQ maps to a Scenario, but acceptance.md contains no scenario asserting (a) per-request deadline propagation + request-ID attachment (REQ-API-013) or (b) observability continuity / OTel spans per request (REQ-API-014, NFR-API-003). The "client disconnect" edge case partially touches context cancellation but asserts neither request-ID nor span creation.
Fix: add a scenario (or assertions within an existing httptest) verifying a request-ID header/field is present and that the handler runs under a deadline-bound context; note span emission even if assertable only indirectly.

### MINOR

**m-4 — NFRs are prose, not EARS.** NFR-API-001..005 use "MUST …" prose (spec.md L199-209), not the five EARS patterns. The task framed all 22 (17 REQ + 5 NFR) as EARS; the 17 functional REQs are valid EARS, but the 5 NFRs are not EARS-patterned. Conventional for NFRs, but flag for consistency — if EARS coverage is asserted for NFRs, reword (e.g. Ubiquitous "The system shall …").

**m-5 — Two compound requirements.** REQ-API-007 (restrict adapter set + respond 400) and REQ-API-013 (propagate context + attach request ID) each bundle two `shall` clauses. EARS hygiene prefers one response per pattern. Mild; splitting would improve testability.

**m-6 — Buffered `/api/query` handler reuse is ambiguous.** `handlers.SynthesisHandler.ServeHTTP` (synthesis.go L55-72) decodes a JSON **POST body with `docs` pre-supplied** and does NOT run Classify/Dispatch. The frontend calls `GET /api/query` with only `q`/`sources` (api.ts L31-38) — the server must dispatch to obtain docs. Plan M2 says "run Classify→Dispatch→Synthesize via the shared pipeline" (a new handler), which is correct, but the Traceability table (spec.md L227) lists `handlers.SynthesisHandler` as a reused component for the streaming row, and plan R3 only flags the GET-vs-POST issue for streaming. The buffered path needs the same dispatch-first treatment. Clarify that `handlers.SynthesisHandler` is NOT reused verbatim for buffered `/api/query`.

### INFO

**i-7 — Port collision (already noted in research §8).** searxng binds host `${SEARXNG_PORT:-8080}` and usearch-api wants 8080 (REQ-API-016). compose must map distinct host ports. Research flags it; ensure M5 honors it.

**i-8 — Dependencies sound.** All four `depends_on` SPECs exist and are directly relevant (router classify, fanout dispatch, SSE streaming, adapter/doc contract). No missing or spurious dependency.

---

## Chain-of-Verification Pass

Re-read end-to-end (not spot-checked):
- **EARS pattern, all 17 functional REQs**: every one matches a valid EARS pattern (Event-Driven/Ubiquitous/State-Driven/Unwanted). No informal "should/may" in normative REQ text. PASS.
- **REQ numbering**: REQ-API-001…017 sequential, zero-padded, no gaps/dupes; NFR-API-001…005 sequential. PASS (MP-1).
- **Traceability, every REQ**: REQ-API-013 and REQ-API-014 found uncovered on the second read — captured as M-3 (would have been missed on a sampled check).
- **Out of Scope specificity**: 7 concrete entries (admin endpoints, AuditQuerier, /deep, history persistence, gRPC, auth, MCP) — all verified against real code. PASS.
- **Contradictions**: none found. The "reuse handlers.SynthesisHandler" traceability entry vs the "build a new dispatch-first handler" plan text is a tension, not a contradiction — captured as m-6.
- **YAML frontmatter**: id/version/status/created/updated/author/owner/methodology/priority/depends_on all present and correctly typed. PASS (MP-3). (Field is `created`, not `created_at` — matches this repo's house style across SPEC-IR-001 etc., so consistent, not a defect.)
- New defects found on second pass: M-3 (uncovered REQs). No prior-pass verdict reversed.

---

## Recommendation

APPROVE-WITH-FIXES. The SPEC's factual foundation is solid: all four headline claims (stale SPEC-IR-001 reference, frontend route/event mismatch, cmd-package import barrier, synthesis seam) are verified accurate against the live tree, and the `internal/searchpipe/` extraction is a sound, behavior-preserving refactor gated by the existing CLI suite (M0). Resolve before/early in run:

1. **M-1**: Define the `citation` SSE event semantics precisely (synthesize from sentence payloads, with explicit field mapping including `snippet`/`source` origin) or drop it; add the matching Scenario 4 assertion.
2. **M-2**: Specify how `/api/sources` derives `category` and `enabled` (these fields are absent from `Capabilities`); keep it off the admin secret-bearing snapshot (NFR-API-005).
3. **M-3**: Add acceptance scenarios/assertions for REQ-API-013 (deadline + request-ID) and REQ-API-014 (obs continuity).
4. **m-6**: State that buffered `/api/query` uses a new dispatch-first handler, not `handlers.SynthesisHandler` verbatim; align the Traceability table.
5. **m-4/m-5**: Optional EARS hygiene (reword NFRs, split compound REQs).

🗿 MoAI <email@mo.ai.kr>
