# SPEC-SYN-002 Research — Citation Faithfulness Enforcement

Companion artifact for `.moai/specs/SPEC-SYN-002/spec.md`.
Version: 0.1.0 (draft)
Created: 2026-05-09
Author: limbowl (via manager-spec)

---

## 1. Goal

`.moai/project/roadmap.md` line 64 declares M4 deliverable
SPEC-SYN-002:

> Citation faithfulness | enforce `doc_id` trace on every synthesized
> claim, reject un-cited LLM output | expert-backend

SPEC-SYN-001 (implemented at commit `7fc338d`) gives us the
**structural** marker→doc mapping (NFR-SYN-002): every `[N]` in the
output text resolves to a real input doc. SPEC-SYN-002 raises the bar
to **behavioral faithfulness**: every *claim* (sentence) in the
synthesized paragraph SHALL carry at least one `doc_id` reference;
LLM output that contains un-cited prose SHALL be rejected at the
sidecar boundary, not surfaced to the CLI / API consumer.

This research artifact documents the existing synthesis pipeline,
identifies the exact enforcement chokepoint, surveys faithfulness
patterns in the open-source landscape, and enumerates risks for the
SPEC author.

---

## 2. Codebase Trace — Where Citations Are Currently Assembled

### 2.1 The synthesis call path

The Go-side and Python-side flow established by SPEC-SYN-001:

```
internal/synthesis/client.go:Synthesize        (Go entry point)
  └→ HTTP POST /synthesize                     (cross-process boundary)
        services/researcher/src/researcher/app.py:synthesize_endpoint
          ├→ REQ-SYN-004 input validation      (empty query / zero docs)
          └→ services/researcher/src/researcher/synthesis.py:synthesize
                ├→ build_prompt(query, lang, docs)              (line 30)
                ├→ gateway.complete(messages, model, lang)      (line 169)
                └→ _process_markers(text_raw, req.docs)         (line 192)
                      ├→ regex /\[(\d+)\]/g match
                      ├→ partition into found_markers + invalid_markers
                      ├→ strip out-of-range markers from text   (line 103)
                      └→ build Citation list for valid markers  (line 108)
```

### 2.2 The exact enforcement chokepoint

The single function that owns "LLM output → final response" conversion
is `services/researcher/src/researcher/synthesis.py:_process_markers`
(lines 66–118). Today it does **structural validation only**:

- Lines 82–87: enumerate every `[N]` regex match.
- Lines 89–94: log WARN when `N` is out of range.
- Lines 97–103: rewrite text with invalid markers removed.
- Lines 108–116: build `Citation` objects for valid markers (sorted,
  deduplicated, mapped to `docs[N-1].id`).

**Critical observation**: the function does NOT inspect *which
sentences carry markers*. A four-sentence paragraph with a single
`[1]` at the end will pass through unchanged. The output may look
like:

> "GPT-4 was released by OpenAI. Anthropic launched Claude later.
> Both companies use transformer architectures. Reddit users compared
> them in r/MachineLearning [1]."

Three of four sentences have zero `doc_id` evidence; only the last
sentence carries `[1]`. SPEC-SYN-001 considers this output valid
(the structural mapping holds). SPEC-SYN-002 must reject it.

### 2.3 `doc_id` representation in the system

`pkg/types/normalized_doc.go` (lines 40–58) defines the canonical
`NormalizedDoc.ID` field:

```go
type NormalizedDoc struct {
    ID          string  // unique within (SourceID, URL); adapter-assigned
    SourceID    string  // matches Adapter.Name()
    URL         string
    Title       string
    Body        string
    Snippet     string
    PublishedAt time.Time
    RetrievedAt time.Time
    Author      string
    Score       float64
    Lang        string
    DocType     DocType
    Citations   []string  // doc IDs referenced by this doc; SPEC-SYN-002 consumes for...
    Metadata    map[string]any
    Hash        string
}
```

Worth noting line 29 of that file already foreshadowed SPEC-SYN-002:

```
//   - Citations: doc IDs referenced by this doc; SPEC-SYN-002 consumes for ...
```

The Python mirror (`services/researcher/src/researcher/models.py`
lines 15–38) preserves `id: str` as the doc identifier. The Go-side
sidecar `Doc` struct (`internal/synthesis/types.go` lines 19–35) uses
`ID` with JSON tag `id`. The `Citation` Pydantic model already
exposes `doc_id` (line 61 of `models.py`):

```python
class Citation(BaseModel):
    marker: int
    doc_id: str   # ← already wired; populated by _process_markers line 110
    url: str
    title: str
```

Conclusion: the `doc_id` field is plumbed end-to-end. SPEC-SYN-002
adds *enforcement semantics*, not new schema.

### 2.4 The fanout / IR contract — upstream doc supply

For completeness, doc IDs originate upstream:

- `SPEC-IR-001` (implemented): `RoutingDecision.Lang` flows into
  synthesis as the `lang` hint (REQ-SYN-007).
- `SPEC-FAN-001` (implemented): fanout dedup uses
  `NormalizedDoc.Hash` (content-only quartet `SourceID | URL | Title |
  Body`) per `pkg/types/normalized_doc.go:80-97`.
- `SPEC-ADP-001..009` (adapters): each adapter sets `ID` per
  `pkg/types/normalized_doc.go:20`. Reddit uses post permalinks; HN
  uses item IDs; arXiv uses paper IDs; etc.

For SPEC-SYN-002: doc IDs are stable, deterministic, and
adapter-assigned by the time synthesis is invoked. We rely on this
invariant.

---

## 3. Open-Source Pattern Survey

### 3.1 gpt-researcher (assafelovic) — referenced by SPEC-SYN-001 §11.1

GPT Researcher's local-doc mode produces inline numeric citations of
the form `[1]`, `[2]` similar to ours. Faithfulness in the upstream
project is *not enforced at the assembly layer* — it is delegated to
a downstream "smart" prompt. The library's `prompts.py` instructs the
LLM to "cite each fact with `[N]`" but does not validate per-sentence
coverage. Same posture as our current SPEC-SYN-001 implementation.

Implication: we cannot lift a faithfulness gate from gpt-researcher.
We must build it.

### 3.2 Anthropic citations API (claude-citations)

Anthropic's `citations` block in the messages API
(`https://docs.anthropic.com/.../citations`, verified via WebFetch
2026-04-15) provides **per-claim source spans**. The API guarantees
that each cited statement in `citations.citations[]` is bounded by
`document_index`, `start_char_index`, and `end_char_index`. Output
without a corresponding citation block is the absence of a claim
attribution.

Pattern relevance: the *contract* — every claim has a structural
provenance pointer — matches what SPEC-SYN-002 wants. The
*mechanism* — span-level char offsets — is over-engineered for our
needs. We use sentence granularity, not character spans, because:

1. SPEC-SYN-001 already operates at sentence/paragraph level.
2. Char-offset enforcement requires the sidecar to teach the LLM a
   structured-output schema (tool use) — out of scope for v0
   per SPEC-SYN-001 §2.2 ("Tool-use / structured-output API for
   citation extraction").
3. Korean text (`Lang: ko`) breaks naive char-offset assumptions;
   sentence-level granularity is locale-stable.

Lift: the **per-claim provenance contract**, not the **char-span
mechanism**.

### 3.3 RAGAS faithfulness metric

RAGAS (`https://docs.ragas.io/en/stable/concepts/metrics/faithfulness.html`,
verified WebFetch 2026-03-22) defines faithfulness as:

> The number of statements in the generated answer that can be
> inferred from the given context, divided by the total number of
> statements in the generated answer.

Two-stage process:
1. **Statement extraction**: decompose the answer into atomic
   statements (LLM call).
2. **Statement verification**: for each statement, ask "is this
   inferable from the context?" (LLM call).

Score = supported_statements / total_statements.

Pattern relevance: RAGAS is the **measurement** discipline. SPEC-SYN-002
is the **enforcement** discipline. They differ in three ways:

| Dimension | RAGAS | SPEC-SYN-002 |
|-----------|-------|--------------|
| When does it run? | Offline / CI eval gate | Inline, per request |
| What does it produce? | Continuous score 0.0–1.0 | Boolean: accept / reject |
| LLM cost | 2 extra LLM calls per answer | 0 extra LLM calls (regex-based) |
| Latency budget | Seconds-to-minutes (eval batch) | Sub-50ms post-LLM (inline gate) |

SPEC-SYN-002 is structural-faithfulness only (every claim cites at
least one doc). Semantic-faithfulness (the cited doc actually
*supports* the claim) is RAGAS territory and belongs in
`SPEC-EVAL-001` (M4) per `roadmap.md` line 101 (DeepEval scorer at
≥0.85 on a 50-query golden set). We document this boundary
explicitly in the SPEC's Exclusions section to avoid scope drift.

### 3.4 LangChain CitationsExtractor pattern

LangChain's RAG pipelines use a `CitationsExtractor` that runs a
post-LLM regex sweep + cited-text block assembly, with optional retry
when citation density falls below a threshold. The retry policy maps
naturally to our event-driven REQ-SYN2-002 ("WHEN un-cited claim
detected, retry once").

Lift: the **single-retry-then-fail** policy. We bound it to one
retry to preserve NFR-SYN-001 p50 ≤ 8s.

---

## 4. The Faithfulness Algorithm — Sentence Granularity

Working definition (English + Korean compatible):

> A **claim** is one sentence in the synthesized paragraph. A
> sentence is **cited** if it contains at least one valid `[N]`
> marker, where N ∈ [1, len(docs)] and N maps to an input
> `NormalizedDoc.ID` via the existing SPEC-SYN-001 mechanism.

### 4.1 Sentence segmentation

Pseudocode (target home: `services/researcher/src/researcher/faithfulness.py`):

```
SENTENCE_END_RE = re.compile(r'[.!?。！？]\s+|[.!?。！？]$', re.MULTILINE)

def split_sentences(text: str) -> list[str]:
    parts = SENTENCE_END_RE.split(text)
    return [s.strip() for s in parts if s.strip()]
```

Korean punctuation (`。！？`) handled inline. Multi-byte safe (Python
3 strings are Unicode). Edge cases:
- "Dr." / "Mr." / "U.S." — false splits accepted (rare in synthesis
  output; LLM rarely abbreviates in research summaries). If empirics
  show false-positive rejections, switch to `pysbd` library in M4
  iteration. **Decision deferred to run phase.**
- Bullet lists (degraded mode output): each `[N] {title} — {url}`
  line is a "sentence" by this regex; each carries its own `[N]`. By
  construction, bullet-list output is 100% cited and trivially
  passes.

### 4.2 Per-sentence marker check

```
def find_uncited_sentences(text: str) -> list[tuple[int, str]]:
    """Returns list of (index, sentence) pairs that have no [N] marker."""
    sentences = split_sentences(text)
    uncited = []
    for i, s in enumerate(sentences):
        if not _MARKER_RE.search(s):  # reuse existing regex from synthesis.py:25
            uncited.append((i, s))
    return uncited
```

### 4.3 Reject-and-retry decision

```
def enforce_faithfulness(
    text: str,
    docs: list[NormalizedDocPayload],
    max_retries: int = 1,
) -> EnforcementOutcome:
    uncited = find_uncited_sentences(text)
    if not uncited:
        return EnforcementOutcome.ACCEPTED

    if retry_count == 0:
        # Compose stricter prompt: "Every sentence MUST end with [N]."
        return EnforcementOutcome.RETRY_REQUIRED

    return EnforcementOutcome.REJECTED  # → caller raises 422 or returns degraded
```

### 4.4 Sentence-stripping fallback (alternative to retry)

If the user accepts that a strict reject-and-retry path balloons p50
latency too much, an alternative is to *strip un-cited sentences*
silently and return only the cited subset. This degrades naturally
into the existing degraded-mode return-shape (text + citations).
**Decision deferred to run phase**: the SPEC will offer reject-retry
as the primary mechanism and stripping as an opt-in NFR knob.

---

## 5. Risks

### Risk 1: LLM output rejected too aggressively → many user-visible errors

Likelihood: **High**. With Claude Haiku 4.5 (`RESEARCHER_MODEL_DEFAULT`,
SPEC-SYN-001 NFR-SYN-001) generating Korean prose in ~50% of queries
(per the project's Korean-focus roadmap line 5), citation density
varies. Strict "every sentence has `[N]`" gate may reject
~10–25% of first-pass outputs.

Mitigation strategies (decided at SPEC time):
1. Single retry with stricter system prompt (REQ-SYN2-002b).
2. After retry failure, fall back to sentence-stripping (opt-in via
   `RESEARCHER_FAITHFULNESS_FALLBACK=strip|reject|degraded`).
3. Default mode = `strip` (best UX, no false errors).
4. Emit `usearch_synthesis_faithfulness_outcome{outcome=...}` so we
   can measure rejection rate empirically.

### Risk 2: Sentence segmentation false-positives produce false rejections

Likelihood: **Medium**. Examples like "The result was 3.14 which..."
get split mid-decimal; "Dr. Smith said..." gets split mid-honorific.
A spurious "uncited" sentence triggers a retry that may not improve
output.

Mitigation:
1. Property test (NFR-SYN2-001) over a corpus of synthetic LLM-style
   outputs.
2. Switch to `pysbd` (Pragmatic Sentence Boundary Disambiguator) if
   empirical rejection rate exceeds 5% on golden-set queries.
3. Document the simple regex as v0; flag pysbd upgrade as M4 iteration.

### Risk 3: Multi-claim sentences get a single `[N]` and are accepted

Likelihood: **Medium**. Example: "GPT-4 was released by OpenAI in
2023, while Anthropic released Claude 2 in 2023 [1]." The single
`[1]` covers both claims structurally but only one of them is
actually attributable to source `[1]`.

Mitigation:
- This is **semantic faithfulness** territory — out of scope for
  SPEC-SYN-002, deferred to SPEC-EVAL-001 (RAGAS / DeepEval).
- We document this as an explicit known-limitation in the spec's
  Exclusions section.
- Optionally we add an Optional EARS requirement (REQ-SYN2-005)
  recommending claim-splitting in the system prompt: "Each sentence
  should make exactly one claim, ending with the citation marker."
  This nudges the LLM toward cleaner output without enforcing it
  algorithmically.

### Risk 4: Hallucinated citations — `[N]` references a doc the model
        invented (`N` references a real index, but the model
        fabricated content not supported by `docs[N-1]`)

Likelihood: **Medium-high** for Haiku 4.5.

This is the classic hallucination problem and is **NOT** SPEC-SYN-002's
to solve. SPEC-SYN-002 enforces *structural* faithfulness:
"every claim has a `doc_id` pointer". Whether the *content* of
the claim is supported by the `doc_id` is RAGAS's job.

Mitigation: documented in Exclusions.

### Risk 5: Retry storm in Strict Mode

If reject-and-retry is enabled and the LLM is consistently producing
un-cited output (e.g., misconfigured model, low-quality routing), we
could trigger retries on every call. Even with single-retry max, that
doubles the LLM cost and pushes p50 from ~3s to ~6s.

Mitigation:
1. `max_retries = 1` is FROZEN in the SPEC; no escalation.
2. Counter `usearch_synthesis_faithfulness_retries_total` exposes
   retry rate; alert if >25% of calls retry.
3. After single retry failure, fall back to strip mode — no third
   LLM call.

### Risk 6: Korean sentence boundaries differ subtly

Likelihood: Low. Korean uses `.`, `?`, `!` like English in modern
prose, and adds `。`, `！`, `？` (full-width) only in mixed-mode text.
Our regex covers both. The harder case is informal Korean ending in
`~` or `요.` — Haiku in research-summarizer mode generates formal
Korean with proper terminal punctuation. Empirics will tell.

Mitigation: golden-set query expansion in M4 (out of scope here).

### Risk 7: Backward incompatibility with SPEC-SYN-001 callers

SPEC-SYN-001 promised: "every `[N]` marker has a corresponding
`citations[]` entry" (NFR-SYN-002). SPEC-SYN-002 must NOT break this.
The new behavior layers ON TOP: the structural mapping still holds
post-strip; only un-cited sentences are removed (and their absence
is logged + emitted as a counter). The Go client's `Result.Citations`
shape is unchanged.

Mitigation: REQ-SYN2-001 explicitly states "preserves SPEC-SYN-001
NFR-SYN-002 invariants"; SPEC-SYN-001 acceptance tests must continue
to pass in the run phase.

---

## 6. Summary of Top 3 Risks for Spec Author Attention

1. **False-positive rejection rate** (sentence segmentation +
   over-strict gate) — bound via fallback to strip mode and observable
   retry-rate counter.
2. **Multi-claim-single-`[N]` semantic gap** — explicitly excluded;
   deferred to SPEC-EVAL-001 (RAGAS).
3. **Hallucinated content under valid `[N]`** — explicitly excluded;
   deferred to SPEC-EVAL-001.

---

## 7. References

### Internal (file:line)

- `services/researcher/src/researcher/synthesis.py:66-118` —
  `_process_markers` (current chokepoint, structural validation only)
- `services/researcher/src/researcher/synthesis.py:25` — shared
  `_MARKER_RE` regex (reused by faithfulness.py)
- `services/researcher/src/researcher/models.py:55-63` — `Citation`
  Pydantic model (already has `doc_id` field, no schema change needed)
- `services/researcher/src/researcher/app.py:49-81` — endpoint that
  invokes `synthesize`; extension point for the faithfulness gate
- `services/researcher/src/researcher/synthesis.py:153-220` — top-level
  `synthesize()` function; insertion point post-LLM, pre-return
- `pkg/types/normalized_doc.go:29` — `Citations: doc IDs ... SPEC-SYN-002 consumes for ...`
- `pkg/types/normalized_doc.go:40-58` — `NormalizedDoc` schema
  (canonical `ID` field shared end-to-end)
- `internal/synthesis/types.go:38-58` — Go-side `Result` and
  `Citation` shapes (unchanged by this SPEC)
- `internal/synthesis/client.go:61-124` — Go-side `Synthesize` method
  (no behavioral change; only outcome enum extension if reject path adds new outcome)
- `.moai/specs/SPEC-SYN-001/spec.md` lines 137–179 — SPEC-SYN-001
  Out-of-Scope §2.2 explicitly defers citation faithfulness to SPEC-SYN-002
- `.moai/specs/SPEC-SYN-001/spec.md` line 202 — NFR-SYN-002 declares
  "structural mapping only; faithfulness is SPEC-SYN-002's job"
- `.moai/project/roadmap.md:64` — SPEC-SYN-002 row
- `.moai/project/roadmap.md:101` — SPEC-EVAL-001 row (RAGAS / DeepEval boundary)
- `.moai/project/roadmap.md:151` — M4 exit criterion ("citation
  faithfulness ≥0.85")

### External (verified URLs)

- gpt-researcher repo: `https://github.com/assafelovic/gpt-researcher`
  — local-doc citation pattern; verified 2026-04-28 (per SPEC-SYN-001
  §11 References)
- Anthropic citations API: `https://docs.anthropic.com/en/docs/build-with-claude/citations`
  — per-claim provenance pattern; verified WebFetch 2026-04-15
- RAGAS faithfulness:
  `https://docs.ragas.io/en/stable/concepts/metrics/faithfulness.html`
  — score formula and statement-extraction protocol; verified
  WebFetch 2026-03-22
- LangChain CitationsExtractor:
  `https://python.langchain.com/docs/concepts/retrieval/#citations`
  — single-retry-then-fail pattern; verified WebFetch 2026-04-30
- pysbd (Pragmatic SBD): `https://github.com/nipunsadvilkar/pySBD`
  — fallback for sentence segmentation if regex insufficient

---

*End of SPEC-SYN-002 research v0.1*
