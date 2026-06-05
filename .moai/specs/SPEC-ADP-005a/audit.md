# SPEC Audit Report: SPEC-ADP-005a — YouTube Extraction Sidecar (Build + Deploy)

**Auditor**: plan-auditor (independent, adversarial)
**Iteration**: 1/3
**Date**: 2026-06-04
**Verdict**: **APPROVE-WITH-FIXES**
**Overall Score**: 0.88

> M1 Context Isolation: No author reasoning context was supplied beyond the
> SPEC documents themselves; all claims were independently re-verified against
> the working tree at `/Users/masterp/Projects/superwork/universal-search/`.
> Every `file:line` citation in the SPEC was resolved against actual source.

---

## Headline: Wire Contract is BYTE-ACCURATE (the critical dimension PASSES)

The SPEC's central claim — that the `/search` + `/health` JSON schema in
§6.4/§6.5 was extracted byte-for-byte from the implemented Go adapter — is
**verified true**. I checked every documented field against the Go structs:

| SPEC claim | Go source | Verdict |
|---|---|---|
| Request body: `query/max_results/cursor_offset/transcript_lang/include_transcripts/since(*int64,omitempty)` | `search.go:37-44` | EXACT match |
| Request invariants (query non-empty, max∈[1,100], cursor≥0, sum≤100, lang floor "en", include_transcripts=true, since omitempty) | `search.go:65,88-103,106,109-118,126` | EXACT match |
| Response envelope `{items, has_more, error}` | `parse.go:23-27` | EXACT match |
| `view_count` is `*int64` (null for livestream) | `parse.go:51` | EXACT match |
| `error` is `*string` omitempty; non-null → silent skip | `parse.go:62,116-118` | EXACT match |
| `available_transcript_langs` `[]string`, nil→`[]` defensively | `parse.go:57,181-184` | EXACT match |
| `upload_date` Go layout `"2006-01-02"` (reformat from YYYYMMDD) | `parse.go:153` | EXACT match |
| `has_more` drives next_cursor when `offset+len<100` | `parse.go:127` | EXACT match |
| `/health` requires HTTP 200 + `status=="ok"` | `youtube.go:127-144` | EXACT match |
| Error categories `{unavailable, permanent, transient, rate_limited}`; unknown→unavailable | `parse.go:244-256` | EXACT match |
| 429→rate-limited + Retry-After (30s default); 5xx→unavailable; 4xx→permanent | `search.go:200-229`, `errors.go:38,44` | EXACT match |
| `503` body may use `reason` instead of `message` | `parse.go:33-34,97-100` | EXACT match |
| User-Agent / Accept headers | `client.go:40-41`, `youtube.go:27` | EXACT match |

**No wire-contract discrepancy was found.** The Python sidecar, if it
implements exactly what §6.4/§6.5 documents, will be parsed correctly by the
existing Go adapter. This is the single most important result of this audit.

---

## Must-Pass Results

- **[PASS] MP-1 REQ number consistency**: `REQ-ADP5a-001` … `REQ-ADP5a-008`
  are sequential, no gaps, no duplicates, consistent 3-digit zero-padding
  (spec.md:180-187). NFRs `NFR-ADP5a-001`…`005` likewise (spec.md:194-199).
- **[PASS] MP-2 EARS compliance**: All 8 REQs match a valid EARS pattern —
  REQ-001 Ubiquitous (`The repository SHALL`, L180), REQ-002/003/008
  Event-Driven (`WHEN … SHALL`, L181/182/187), REQ-004 Ubiquitous
  (`For each video item, the sidecar SHALL`, L183), REQ-005 State-Driven
  (`WHILE include_transcripts is true … SHALL`, L184), REQ-006 Unwanted
  (`IF … THEN the sidecar SHALL`, L185), REQ-007 Optional
  (`WHERE YT_COOKIES_PATH is set … SHALL`, L186).
- **[PASS / N/A·house-style] MP-3 YAML frontmatter**: `id`, `title`,
  `version` (0.1.0), `status` (draft), `priority` (P1), `created`,
  `updated`, `owner`, `methodology`, `depends_on` all present and correctly
  typed (spec.md:2-16). Deviations from the generic rubric: field is
  `created`/`updated` not `created_at`, and there is no `labels` array. The
  project uses a house amendment-SPEC schema (`milestone`/`owner`/
  `methodology`/`depends_on`), which is internally consistent and matches
  sibling SPECs. Flagged INFO, not a hard fail.
- **[PASS] MP-4 Language neutrality**: N/A — this SPEC is single-domain
  (Python YouTube sidecar + one Go constant). No multi-language tooling
  surface; no 16-language enumeration required.

---

## Category Scores (rubric-anchored)

| Dimension | Score | Band | Evidence |
|-----------|-------|------|----------|
| Clarity | 0.90 | 0.75–1.0 | Single unambiguous interpretation per REQ; precise field-level contract (§6.4). Minor: §6.1 "8 fixtures" is factually wrong (see D1). |
| Completeness | 0.95 | 1.0 | HISTORY (L21), Purpose/WHY (L79), Scope/WHAT (L111), Technical Approach/HOW (L229), EARS REQs (L176), Acceptance (acceptance.md), Exclusions (§2.2/§2.3/§7) all present and specific. |
| Testability | 0.92 | 0.75–1.0 | Every AC binary-testable (acceptance.md AC-001..026, AC-N01..05); coverage matrix §4 complete. No weasel words in normative text (SHOULD used appropriately for pre-cap). |
| Traceability | 1.00 | 1.0 | Every REQ→AC mapped (acceptance.md §4); no orphan ACs; TDD plan §8 maps all 21 tests to REQ/NFR IDs. |

---

## Defects Found

### D1. spec.md:247-249, plan.md:133-141 — "8 Go testdata fixtures" is FALSE; only 6 exist — Severity: **MAJOR**

spec.md:248 ("mirror the 8 Go testdata fixtures") and plan.md §5 step 1
("Copy the 8 Go testdata fixtures (`internal/adapters/youtube/testdata/*.json`)")
are factually wrong. The directory contains **6** fixtures, not 8:

```
search_response.json (25 items)   search_response_empty.json
search_response_korean.json        search_response_malformed.json
search_response_no_transcript.json search_response_pagination.json
```

The SPEC §6.1 enumerates: *"happy 25, empty, pagination, korean,
no-transcript, **429, 503-challenge**, malformed/partial-error"* (8 names).
The **429 and 503-challenge fixtures do not exist on disk**. Additionally:

- `search_response_malformed.json` is a **truncated/broken JSON envelope**
  (tests the JSON-parse-failure path at `parse.go:86-92`), NOT a per-item
  `"error"` fixture. There is **no partial-error fixture** anywhere — none of
  the 6 fixtures contains an item with a non-null `"error"` field
  (verified: `grep "\"error\"" testdata/*.json` → no matches).
- Therefore the contract-fidelity strategy in plan.md §5 ("Copy the 8 Go
  testdata fixtures … the sidecar's `models.py` serialisation MUST reproduce
  them") is **partially unexecutable as written**: the 429, 503-challenge,
  and partial-error golden fixtures must be **authored fresh** by the sidecar
  implementer, not copied. The 6 existing fixtures only cover the happy /
  empty / korean / no-transcript / pagination / malformed-JSON cases.

**Why MAJOR not BLOCKER**: the wire contract itself is correct, and the
implementer can author the missing fixtures. But the plan as written will
mislead the run-phase agent into expecting 8 copyable fixtures, and the
"reuse Go testdata" mitigation for the top risk (spec.md:535) is weaker than
stated for exactly the error-path cases (429/503/partial) that most need
contract validation.

### D2. spec.md:117, research.md:243-256 — tokenizer-ko has no `README.md` or `.env.example` to mirror — Severity: **MINOR**

spec.md §2.1(a) and research §3.1 state the new directory mirrors *both*
`services/tokenizer-ko/` and `services/embedder/`, and the research §3.1
layout diagram shows every reference sidecar with `README.md` + `.env.example`.
Verified on disk: `services/tokenizer-ko/` contains only
`Dockerfile / pyproject.toml / src / tests` — **no `README.md`, no
`.env.example`**. Only `services/embedder/` provides those two files.
The SPEC's required `README.md` (§2.1f, NFR-001) and
`services/youtube-extract/.env.example` are therefore valid deliverables, but
the *only* in-repo precedent for them is the embedder, not tokenizer-ko.
Tighten the reference attribution to avoid the implementer copying a
non-existent tokenizer-ko README.

### D3. spec.md:445-448, acceptance.md AC-026, plan.md M6 — `TestCapabilitiesShape` does NOT pin "8082"; no test update needed — Severity: **INFO**

The SPEC hedges correctly ("Verify … does not pin the '8082' literal; if it
does, update"). I resolved the uncertainty: `youtube_test.go:54-98`
`TestCapabilitiesShape` asserts only these Notes substrings —
`"yt-dlp Python sidecar"`, `"public no-auth"`, `"transcript snippet
truncated"`, `"Korean-locale auto-detection"`, `"max_results + cursor offset
cap 100"`. **None contains "8082".** Changing `capabilitiesNotes`
(`youtube.go:34`, actual text `"…at port 8082 (default);…"`) to `8084` does
not touch any asserted substring. The Go test stays green with the constant
change alone — no `youtube_test.go` edit is required. Recommend deleting the
conditional from §6.7/M6 and stating definitively "no test change needed".

### D4. research.md:83, spec.md:575 — searxng port-line citation drift — Severity: **INFO**

research §1.3 cites searxng at `docker-compose.yml:119` and the SPEC §12
references `:177` for ports. The `searxng:` service key is actually at
`docker-compose.yml:116`; the port mapping line is approximately :119.
Researcher/embedder/tokenizer-ko port cites (`:173`/`:204`/`:320`) are
accurate. Minor imprecision; does not affect correctness.

### D5. spec.md:2-16 — frontmatter omits `labels`; uses `created` not `created_at` — Severity: **INFO**

Relative to the generic SPEC frontmatter rubric, `labels` (array) is absent
and the date field is `created`/`updated` rather than `created_at`. This is
the project's house amendment-SPEC schema and is internally consistent; noted
for completeness only.

---

## Independent Verification Ledger (claims checked against codebase)

| Claim (SPEC) | Result |
|---|---|
| Wire contract §6.4/§6.5 byte-accurate vs Go source | **TRUE** (see Headline table) |
| `youtube.go:20` `defaultBaseURL = "http://localhost:8082"` | TRUE |
| `query.go:488` gates registration on `YOUTUBE_BASE_URL` | TRUE (`query.go:488`) |
| Port 8084 is free in `deploy/docker-compose.yml` | TRUE — `grep 8084` → not present; `storm` not in compose |
| 8084 used by `storm` only in Helm | TRUE — `charts/universal-search/values.yaml:481,483,498,506` |
| Compose port occupancy 8080/8081/8082/8083 | TRUE — searxng(116), researcher(173), embedder(204), tokenizer-ko(320) |
| `embedder/__main__.py` binds `EMBEDDER_PORT` default 8082 (`workers=1`) | TRUE (line 14-16) |
| embedder `/health` returns 200 `{status:ok,…}` / 503 `{status:loading,…}` | TRUE (`embedder/app.py:99-117`) |
| tokenizer-ko healthcheck `interval:15s timeout:5s retries:5 start_period:30s` | TRUE (`docker-compose.yml:329-332`) |
| tokenizer-ko is multi-stage Dockerfile precedent | Plausible (Dockerfile present; not line-audited) |
| `.env.example:91,95` embedder `BASE_URL`/`PORT` precedent | TRUE |
| Root `.env.example` has no YouTube vars today | TRUE |
| 8 Go testdata fixtures exist to copy | **FALSE — only 6; no 429/503/partial-error** (D1) |
| tokenizer-ko has README.md + .env.example to mirror | **FALSE** (D2) |
| `TestCapabilitiesShape` may pin "8082" | **FALSE — it does not** (D3) |
| happy fixture has 25 items | TRUE (`grep -c '"id"'` → 25) |
| pagination fixture `has_more: true` | TRUE (line 67) |

---

## Dimension-by-Dimension Findings

1. **EARS compliance** — PASS. All 8 REQs valid patterns; all testable. (MP-2)
2. **Acceptance criteria** — PASS. AC-001..026 + AC-N01..05 binary-testable;
   coverage matrix maps every REQ/NFR. EC-001..006 edge cases reasonable
   (EC-005 cursor-at-cap sum=100 aligns with `search.go:97` `> paginationCap`).
3. **Wire contract accuracy** — PASS (byte-accurate). Single most important
   dimension; no discrepancy. One adjacent flaw: the *fixture-based
   validation* of the error paths is overstated (D1).
4. **Port decision** — PASS. 8084 verified free; Go constant change scope
   (`youtube.go:20,2,34`) accurate and minimal; `YOUTUBE_BASE_URL` gate
   correctly retained.
5. **Sidecar design vs reference** — PASS. Multi-stage Dockerfile + non-root
   + healthcheck + compose block (tokenizer-ko), FastAPI lifespan + /health
   200/503 (embedder) faithfully mirrored. Caveat: README/.env.example
   precedent is embedder-only (D2).
6. **Reuse of ADP-005 D1-D8** — PASS. D2/D3/D4/D6/D7/D8 referenced
   consistently; the concrete values (cap 100, RequiresAuth=false, 30s
   Retry-After, sleep 1.0/2/5) all corroborate the implemented Go source.
   (ADP-005 parent doc not independently re-read; corroboration via source is
   strong.)
7. **GPL isolation** — PASS. yt-dlp invoked as subprocess
   (`asyncio.create_subprocess_exec`), pip-installed in its own process
   space, never linked; Apache-2.0 boundary preserved (NFR-ADP5a-002,
   AC-N02). This is the accepted GPLv3 process-isolation pattern (aggregation,
   not a derivative work).

---

## Chain-of-Verification Pass (M6)

Second-look findings:
- Re-read all 8 REQ entries end-to-end (not skimmed): REQ-004's "Ubiquitous"
  label is consistent between spec.md §3 and acceptance.md §4. OK.
- Re-checked REQ sequencing end-to-end: 001-008 contiguous. OK.
- Verified traceability for **every** REQ via coverage matrix, not a sample.
  OK.
- Exclusions specificity (§2.2/§2.3/§7): each exclusion names a destination
  SPEC (FAN-001, CACHE-001, IDX-001, EVAL-002, SYN-001, DEPLOY-001/M9) or a
  concrete rationale. Specific, not vague. OK.
- Contradiction scan: §6.4 (`view_count null→0→Score 0.5`) vs EC-003
  (`view_count 0 → emit 0, Score 0.5`) — consistent with
  `parse.go:145-148`. No contradiction.
- **NEW finding this pass**: `search_response_malformed.json` is a broken
  JSON envelope, NOT a per-item-error fixture — this exposed the deeper form
  of D1 (no partial-error fixture exists at all; the malformed/partial-error
  conflation in §6.1 is a category error across two distinct `parse.go` code
  paths, :86 vs :116).

---

## Recommendation (APPROVE-WITH-FIXES)

The SPEC is approved for implementation **after** the following fixes. The
wire contract — the make-or-break element — is correct, so the sidecar can be
built safely against §6.4/§6.5.

Required fixes (run-phase or a quick spec revision):

1. **Fix D1 (MAJOR)**: Correct spec.md:247-249 and plan.md:133-141. State
   that **6** Go fixtures exist and are reusable
   (`search_response{,_empty,_korean,_malformed,_no_transcript,_pagination}.json`),
   and that the **429, 503-challenge, and partial-error (per-item `"error"`)
   golden fixtures must be authored fresh** by the sidecar implementer (they
   do not exist in `internal/adapters/youtube/testdata/`). Clarify that
   `search_response_malformed.json` exercises the malformed-JSON path
   (`parse.go:86`), which is distinct from the per-item-error skip path
   (`parse.go:116`). Adjust the §10 top-risk mitigation accordingly.

2. **Fix D2 (MINOR)**: In spec.md §2.1(a) and research §3.1, attribute the
   `README.md` and `.env.example` precedent to `services/embedder/` only —
   `services/tokenizer-ko/` has neither. Keep tokenizer-ko as the
   Dockerfile/compose-block precedent.

3. **Fix D3 (INFO)**: Replace the conditional in spec.md §6.7 / acceptance.md
   AC-026 / plan.md M6 with the verified fact: `TestCapabilitiesShape`
   (`youtube_test.go:54-98`) does not assert "8082"; **no test edit is
   required** for the port change. The Go change reduces to `youtube.go:20`
   constant + `:2` doc + `:34` Notes.

4. **Fix D4/D5 (INFO, optional)**: Correct the searxng port-line citation
   (service key at `docker-compose.yml:116`); optionally add `labels` to
   frontmatter for rubric alignment if the house schema permits.

None of these block the build; D1 is the only one that materially affects the
run-phase plan and must be corrected to avoid a misled implementer.

---

*End of audit — iteration 1/3. plan-auditor.*
