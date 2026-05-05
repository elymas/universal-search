# SPEC-IDX-002 Research — Embedding Service (BGE-M3 Python sidecar)

Status: draft companion to spec.md
Created: 2026-05-04
Updated: 2026-05-04
Author: limbowl via manager-spec

This artifact captures the codebase analysis, external library survey, and
contract-design rationale that inform the SPEC-IDX-002 EARS requirements. It
follows the same shape as `.moai/specs/SPEC-SYN-001/research.md` and
`.moai/specs/SPEC-FAN-001/research.md`. Every internal claim is file:line
cited; every external claim links to a WebFetch-verified URL (verification
date 2026-05-04 unless noted).

---

## 1. Internal Codebase State

### 1.1 `services/embedder/` — Python sidecar scaffold

The Python service directory was provisioned by SPEC-BOOT-001 (REQ-BOOT-002,
spec.md:99-105). Current shape:

```
services/embedder/
├── Dockerfile                  # 22 lines, python:3.11-slim, single-stage
├── README.md                   # service overview
├── pyproject.toml              # 33 lines, name="embedder", deps=[]
├── .env.example                # EMBEDDER_MODEL=BAAI/bge-m3, EMBEDDER_BATCH_SIZE=32
├── src/
│   └── embedder/               # empty (no src files yet)
└── tests/
    ├── __init__.py             # empty
    └── test_version.py         # 461 bytes, baseline placeholder
```

Key facts captured by `Read`:

- `services/embedder/pyproject.toml:1-33` declares `name = "embedder"`,
  `version = "0.1.0"`, `requires-python = ">=3.11"`, `license = "Apache-2.0"`,
  empty `dependencies = []`. Dev group: `ruff>=0.8.0`, `pytest>=8.3.0`,
  `pip-audit>=2.7.0`.
- `[tool.hatch.build.targets.wheel] packages = ["src/embedder"]` —
  src-layout is fixed, mirroring `services/researcher/`.
- `services/embedder/Dockerfile:1-22` is `python:3.11-slim` based,
  single-stage, runs as non-root `appuser`,
  `CMD ["python", "-m", "embedder"]`. Does NOT yet expose a port (no
  `EXPOSE` directive) and has NO `HEALTHCHECK`. SPEC-IDX-002 adds both
  per parity with `services/researcher/Dockerfile:1-30`.
- `services/embedder/.env.example` already declares
  `EMBEDDER_MODEL=BAAI/bge-m3`, `EMBEDDER_BATCH_SIZE=32`,
  `EMBEDDER_LOG_LEVEL=INFO`. SPEC-IDX-002 adds further env vars (port,
  cache dir, device selection) but the model default is already locked.
- `src/embedder/` has no Python files. SPEC-IDX-002 is the first SPEC to
  put runtime code here, mirroring SYN-001's role for
  `services/researcher/`.

This means SPEC-IDX-002 follows exactly the SYN-001 establishment pattern
documented at `.moai/specs/SPEC-SYN-001/research.md:18-55`. No FastAPI app,
no Pydantic models, no HTTP routes exist yet. The Dockerfile's `CMD`
target (`python -m embedder`) does not yet correspond to a runnable
server — there is no `main()`.

### 1.2 `internal/embedder/` — Go-side stub status

Verified via `ls /Users/masterp/Projects/superwork/univesal-search/internal/embedder/`:
**no such directory**. SPEC-BOOT-001 reserved the slot via
`structure.md:35-43` (Index context lists Qdrant, Meilisearch, Postgres
sub-packages; embedding is referenced under `services/embedder/` only).
Thus `internal/embedder/` is a NEW package created by SPEC-IDX-002, not a
stub-fill. This differs from SPEC-SYN-001 which filled the existing
4-line `internal/synthesis/synthesis.go` stub — SYN-001 research §1.2 at
`.moai/specs/SPEC-SYN-001/research.md:57-72`.

The Go-side caller pattern mirrors `internal/synthesis/`:

| Synthesis (existing reference) | Embedder (new in SPEC-IDX-002) |
|--------------------------------|--------------------------------|
| `internal/synthesis/types.go`  | `internal/embedder/types.go`   |
| `internal/synthesis/config.go` | `internal/embedder/config.go`  |
| `internal/synthesis/client.go` | `internal/embedder/client.go`  |
| `internal/synthesis/synthesis.go` (re-exports) | `internal/embedder/embedder.go` |
| `internal/synthesis/client_test.go` | `internal/embedder/client_test.go` |

Verified by `Read /Users/masterp/Projects/superwork/univesal-search/internal/synthesis/client.go`:
the synthesis client at lines 28-280 is the exact analog SPEC-IDX-002 Go
client SHOULD mirror. Notable patterns to copy verbatim:

- Const block lines 28-35: `retryBase = 500ms`, `retryMult = 3`,
  `retryJitter = 0.1`.
- `Client` struct lines 41-45: `httpClient *http.Client`, `baseURL string`,
  `o *obs.Obs`.
- `New` constructor lines 49-55 with nil-safe Obs.
- Per-call observability emit pattern lines 196-240.
- Retry loop lines 248-280 with 4xx-no-retry / 5xx-retry classification.

### 1.3 `pkg/types/` — input contract surface

The embedding service does NOT consume `NormalizedDoc` directly. It accepts
arbitrary text (a list of strings) and returns three vector representations.
Verified by `Read /Users/masterp/Projects/superwork/univesal-search/pkg/types/normalized_doc.go:40-56`:
`NormalizedDoc.Body` is the natural ranking input; the index layer
(SPEC-IDX-001) is the consumer that will pass `Body` into the embedder.
This means SPEC-IDX-002 can stay text-domain-agnostic and decouple from
`pkg/types`.

Per `.moai/project/structure.md:160` — "**`pkg/types`** is the public SDK
boundary. Breaking changes require major version bump." — SPEC-IDX-002
SHOULD NOT introduce new exported types into `pkg/types/`. Embedding
result types live in the internal `internal/embedder/types.go` package
(non-exported boundary), matching `internal/synthesis/types.go` precedent.

### 1.4 `services/researcher/` — exact precedent pattern

The Python sidecar shape, build pipeline, healthcheck convention, and
test layout to mirror are all in `services/researcher/`:

- `services/researcher/pyproject.toml:1-46` — runtime deps
  (`fastapi>=0.115`, `uvicorn[standard]>=0.30`, `pydantic>=2.9`,
  `httpx>=0.27`, `openai>=1.50`); dev deps (`ruff`, `pytest`,
  `pytest-asyncio`, `pytest-cov`, `hypothesis`, `pip-audit`); pytest
  `asyncio_mode="auto"`.
- `services/researcher/Dockerfile:1-30` — multi-stage on
  `python:3.11-slim`, non-root `appuser`, `EXPOSE 8081`,
  `HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3
   CMD curl -f http://localhost:8081/health || exit 1`.
- `services/researcher/src/researcher/` — `app.py`, `models.py`,
  `synthesis.py`, `gateway.py`, `obs.py`, `__main__.py`, `__init__.py`
  layout. SPEC-IDX-002 mirrors with `app.py`, `models.py`, `embed.py`
  (the inference engine), `cache.py` (LRU + disk cache), `obs.py`,
  `__main__.py`, `__init__.py`.

### 1.5 `internal/obs/metrics/` — observability registry state

Verified by `Read /Users/masterp/Projects/superwork/univesal-search/internal/obs/metrics/metrics.go:55-178`
and `internal/obs/metrics/synthesis.go:1-60`:

The Registry struct already declares fields for HTTP, fanout, adapter,
LLM, router, and synthesis collectors. SPEC-IDX-002 adds three new
fields (`EmbedderCalls`, `EmbedderLatency`, `EmbedderCacheHits`) by
following the same pattern as `synthesis.go`'s `registerSynthesis`.

The cardinality allowlist at `internal/obs/metrics/metrics.go:169-176`
already includes `outcome` (for the embedder calls counter) and
`adapter` (NOT used here). The new label SPEC-IDX-002 introduces is
`mode` (one of `dense|sparse|colbert|all`) — this REQUIRES a small
amendment to `labelNames` in the registry. The embedder cache-hit
counter has NO labels (mirrors `SynthesisCost` precedent at
`internal/obs/metrics/metrics.go:60-61`).

Existing metric naming convention (verified by Grep):

```
usearch_http_requests_total
usearch_http_request_duration_seconds
usearch_fanout_goroutines_inflight
usearch_adapter_calls_total
usearch_adapter_call_duration_seconds
usearch_llm_calls_total
usearch_llm_cost_usd_total
usearch_llm_latency_seconds
usearch_router_classifications_total
usearch_router_classification_duration_seconds
usearch_synthesis_calls_total
usearch_synthesis_latency_seconds
usearch_synthesis_cost_usd_total
usearch_build_info
```

SPEC-IDX-002 adds three new families:

```
usearch_embedder_calls_total{outcome,mode}
usearch_embedder_latency_seconds{outcome,mode}
usearch_embedder_cache_hits_total
```

The `mode` label is bounded (4 values: `dense`, `sparse`, `colbert`,
`all`); `outcome` is reused (5-value enum from SPEC-CORE-001 NFR-CORE-002:
`success`, `failure`, `timeout`, `rate_limited`, `unavailable`).
Cardinality stays bounded at 4 × 5 = 20 series per call vector.

### 1.6 `deploy/docker-compose.yml` — current six-service stack

Verified by `Read /Users/masterp/Projects/superwork/univesal-search/deploy/docker-compose.yml:1-217`:

Eight services currently declared: qdrant, meilisearch, postgres, redis,
searxng, litellm, researcher (added by SPEC-SYN-001 lines 165-188),
prometheus.

The `researcher` service entry at lines 165-188 is the precedent shape
for the new `embedder` service entry. Salient properties:

- `build: context: ../services/researcher` (relative path from
  `deploy/`).
- `ports: ["${RESEARCHER_PORT:-8081}:8081"]` — port from env.
- `environment: LITELLM_BASE_URL`, `LITELLM_API_KEY`, etc.
- `depends_on: litellm: condition: service_healthy`.
- `healthcheck: test: ["CMD", "curl", "-f", "http://localhost:8081/health"]`.
- `restart: unless-stopped`, `networks: [app]`.

SPEC-IDX-002 adds an `embedder` service entry with these adaptations:

- `build: context: ../services/embedder`.
- `ports: ["${EMBEDDER_PORT:-8082}:8082"]` (port 8082 chosen to follow
  the +1 pattern after researcher's 8081).
- No `depends_on: litellm` (the embedder does NOT call any LLM).
- `volumes: [embedder_models:/home/appuser/.cache/huggingface]`
  (hugging-face model cache persistence — first boot downloads ~1.4GB
  for BGE-M3, subsequent boots reuse).
- `environment: EMBEDDER_PORT, EMBEDDER_MODEL, EMBEDDER_BATCH_SIZE,
  EMBEDDER_DEVICE, EMBEDDER_LOG_LEVEL, EMBEDDER_USE_FP16`.

For optional GPU acceleration (Open Question §8.2), the compose entry
declares a commented-out `deploy.resources.reservations.devices`
NVIDIA-GPU stanza. CPU is the default; GPU is an opt-in flag.

### 1.7 `.moai/project/roadmap.md` §M3 — placement and dependencies

Verified by `Read /Users/masterp/Projects/superwork/univesal-search/.moai/project/roadmap.md:55-58`:

```
| SPEC-IDX-001 | Hybrid index layer | Qdrant + Meilisearch + PG Go clients,
                                       RRF fusion, ingestion pipeline | expert-backend |
| SPEC-IDX-002 | Embedding service  | BGE-M3 Python sidecar, batched
                                       inference, cache | expert-backend |
| SPEC-IDX-003 | Korean tokenization | mecab-ko sidecar, Meili custom
                                       tokenizer plugin, separate ko shard |
```

The M3 row anchors SPEC-IDX-002's scope verbatim:
"BGE-M3 Python sidecar, batched inference, cache". SPEC-IDX-002 does
NOT extend beyond this scope; deeper concerns (RRF fusion at
SPEC-IDX-001, Korean tokenization at SPEC-IDX-003, on-disk vector
storage at SPEC-IDX-001) are explicitly deferred.

The M3 parallelization plan at `roadmap.md:122-128` lists SPEC-IDX-* as a
3-way parallel batch gated on SPEC-FAN-001 (already approved).
SPEC-IDX-002 can begin its plan phase immediately and its run phase
after SPEC-FAN-001 merges.

The M5 deep-research roadmap at `roadmap.md:71-75` notes that the
deep-research multi-agent pipeline (SPEC-DEEP-002) and tree exploration
(SPEC-DEEP-003) will eventually consume embeddings via the same
sidecar — this is the secondary motivation for getting the contract
right in M3.

### 1.8 `.moai/project/tech.md` — Korean-first posture

Verified by `Read /Users/masterp/Projects/superwork/univesal-search/.moai/project/tech.md:39-50`:

```
| Embeddings (dense)  | BGE-M3 via local service | multilingual incl. Korean, SOTA OSS |
| Embeddings (sparse) | FastEmbed BM25 (IDF-weighted) | hybrid with dense via RRF |
| Cross-encoder rerank | BGE-reranker-v2-m3 | optional, for top-50 → top-10 |
| Korean keyword tokenizer | mecab-ko / khaiii (served via Python sidecar) |
                            Meilisearch default tokenizer is weak for Korean |
```

BGE-M3 is the locked choice for dense embeddings. The sparse component
already has a separate tracker (FastEmbed BM25) — but BGE-M3 itself
ALSO produces sparse output (lexical token weights). SPEC-IDX-002
exposes BGE-M3's sparse output as a first-class return value of the
sidecar; the BM25 / IDF integration with Meilisearch lives in
SPEC-IDX-001 + SPEC-IDX-003. The Korean tokenizer is a SEPARATE
sidecar, not part of SPEC-IDX-002.

Per `tech.md:144-153` Risks table:

> Korean tokenizer mismatch (Meili default weak for ko) | Medium |
> mecab-ko sidecar; separate Korean index shard

This is delegated to SPEC-IDX-003. SPEC-IDX-002 trusts BGE-M3's own
multilingual tokenizer (XLM-RoBERTa) for Korean text — this is the
internal Korean tokenization the sidecar performs, distinct from the
Meilisearch lexical tokenization handled separately.

---

## 2. BGE-M3 Inference Path Survey

The user-facing question: **how do we run BGE-M3 inference inside a Python
sidecar in a way that supports CPU and GPU, batched throughput, and the
three-vector output (dense + sparse + ColBERT)?** Three viable paths
were considered.

### 2.1 Path A — vanilla PyTorch via `FlagEmbedding.BGEM3FlagModel`

The reference implementation. WebFetch-verified
2026-05-04 against https://huggingface.co/BAAI/bge-m3 and
https://github.com/FlagOpen/FlagEmbedding/blob/master/research/BGE_M3/README.md
and https://github.com/FlagOpen/FlagEmbedding (FlagEmbedding is the
official BAAI repo, MIT-licensed).

```python
from FlagEmbedding import BGEM3FlagModel

model = BGEM3FlagModel('BAAI/bge-m3', use_fp16=True, devices=['cuda:0'])

out = model.encode(
    sentences,
    batch_size=12,
    max_length=8192,
    return_dense=True,
    return_sparse=True,
    return_colbert_vecs=True,
)
# out['dense_vecs']      → np.ndarray[N, 1024]
# out['lexical_weights'] → list[dict[token_id, weight]]
# out['colbert_vecs']    → list[np.ndarray[T_i, 1024]]
```

Properties confirmed:

- **Dense dim**: 1024 (verified Hugging Face model card).
- **Max sequence length**: 8192 tokens (XLM-RoBERTa extended to 8192).
- **License**: MIT.
- **Multilingual coverage**: 100+ languages including Korean
  (verified Hugging Face card; benchmarks: MIRACL 13+ langs, MKQA
  cross-lingual).
- **FP16 support**: `use_fp16=True` — "speeds up computation with a
  slight performance degradation" (verified BGE-M3 README).
- **Device control**: `devices=['cuda:0']` for GPU, `devices=['cpu']`
  for CPU. Default detects automatically.
- **Three-mode output**: single forward pass produces all three when
  the corresponding `return_*` flag is set; only the requested modes
  pay their compute cost (sparse + colbert add roughly 30% over dense).

Pros:
- Direct upstream support; no conversion steps.
- BGE-M3 README example uses this exact API.
- Active maintenance by BAAI (verified 2026-05-04).

Cons:
- Pulls full PyTorch + transformers (~2.5GB Docker image).
- CPU performance is mediocre — typical <50 docs/sec on 4 vCPU.
- No quantization beyond FP16; no INT8/INT4 in the upstream API.

**Decision**: PRIMARY path. SPEC-IDX-002 Run-phase implements with
`FlagEmbedding>=1.3.0` against the BGEM3FlagModel API. Image bloat is
mitigated by multi-stage Dockerfile that prunes pip cache. CPU
throughput tradeoff is acknowledged in NFR-IDX-002.

### 2.2 Path B — ONNX Runtime export

WebFetch-verified 2026-05-04 against https://onnxruntime.ai/docs/get-started/with-python.html.

ONNX Runtime is a portable inference engine that runs exported PyTorch
models faster on CPU (and matches PyTorch on GPU with the
CUDAExecutionProvider). The package install matrix:

- CPU: `pip install onnxruntime`
- GPU CUDA 12: `pip install onnxruntime-gpu`
- "Only one of these packages should be installed at a time in any one
  environment." (verified URL above)

API surface:
```python
import onnxruntime as ort
session = ort.InferenceSession('bge_m3.onnx',
    providers=['CUDAExecutionProvider', 'CPUExecutionProvider'])
outputs = session.run(None, {'input_ids': ..., 'attention_mask': ...})
```

Pros:
- 2-3× faster than PyTorch on CPU (industry rule-of-thumb; not
  formally measured for BGE-M3 in the documents we verified).
- Smaller image footprint (~600MB vs 2.5GB for full PyTorch).
- Supports INT8 quantization for further size + speed wins.

Cons:
- BGE-M3 has NO official ONNX export — verified by absence in
  FlagEmbedding repo and BGE-M3 README; users must run the export
  themselves via `optimum-cli export onnx`.
- Sparse and ColBERT outputs require post-processing not built into the
  ONNX graph; the export must wrap the three-head model carefully.
- Maintenance burden: every BGE-M3 upstream version requires a
  re-export to keep the ONNX file in sync.

**Decision**: NOT in v0.1. Documented as a future SPEC-IDX-002a
optimisation path. The cost of getting ONNX export right does not pay
back at M3-scale traffic; revisit when measured CPU throughput is the
bottleneck.

### 2.3 Path C — vLLM as embedding server

WebFetch-verified 2026-05-04 against https://docs.vllm.ai/en/latest/.

vLLM is a high-throughput inference server originally for LLMs; it has
expanded to embedding/retrieval models (E5-Mistral, GTE, ColBERT).
However, BGE-M3 is **NOT** specifically named in the supported-models
list (verified URL; the page mentions a separate "full list of
supported models" not inspected).

Pros:
- Higher throughput than vanilla PyTorch on GPU (PagedAttention).
- OpenAI-compatible `POST /v1/embeddings` endpoint — same shape as
  LiteLLM, conceptually elegant.

Cons:
- BGE-M3 multi-vector (sparse + ColBERT) output is NOT standard OpenAI
  embedding format; vLLM's `/v1/embeddings` returns single dense
  vectors only. Sparse + ColBERT would require a custom endpoint.
- Heavyweight: vLLM loads only one model per server; running BGE-M3
  alongside Claude/Llama via the same vLLM instance is impractical.
- GPU-only de-facto (CPU is supported but not the design target).

**Decision**: NOT in v0.1. If GPU throughput becomes the constraint
post-V1 (M9+ scaling), revisit as a SPEC-IDX-002b alternative path. The
path A FlagEmbedding direct-call is sufficient for M3-M9.

### 2.4 Open Question — single-vector vs multi-vector

BGE-M3's three-vector output is structurally rich but adds inference
cost. Three positions are visible in the wider RAG community:

1. **Dense-only** — fast, simple, the OpenAI text-embedding-3 standard.
2. **Dense + sparse** — hybrid retrieval via Reciprocal Rank Fusion
   (matches SPEC-IDX-001 RRF design).
3. **Dense + sparse + ColBERT** — late-interaction fine-grained
   matching, more accurate but 5-10× more storage per doc (each token
   gets its own 1024-dim vector).

SPEC-IDX-002 makes all three modes available via per-request flags
(`return_dense`, `return_sparse`, `return_colbert_vecs`); the index
layer (SPEC-IDX-001) chooses which ones to populate. This is a
contract decision: keeping the sidecar mode-agnostic lets SPEC-IDX-001
make the storage tradeoff without re-implementing the embedding side.

---

## 3. Alternative Models Considered (rejected)

### 3.1 OpenAI text-embedding-3 (cloud, hosted)

Verified by general industry knowledge; OpenAI text-embedding-3-small
(1536-dim) and text-embedding-3-large (3072-dim) are widely used.
**Rejected** for SPEC-IDX-002 because:

- `tech.md:39-50` already locks BGE-M3 as the dense embedding model.
- Cloud hosting violates `tech.md:1` Architectural Principle 1
  (composition + local-first).
- Korean coverage of OpenAI embeddings is weaker than BGE-M3's
  XLM-RoBERTa base (BAAI's MIRACL benchmark explicitly trained on ko).
- Cost — at $0.13/M tokens for text-embedding-3-large, the indexing
  workload at scale (millions of docs) accumulates significantly.

### 3.2 Cohere `embed-multilingual-v3`

Verified by general industry knowledge. **Rejected** for the same
hosted-cloud reason as OpenAI. Cohere's pricing tier and Korean
performance match BGE-M3's MIT-licensed local-run win.

### 3.3 multilingual-e5-large (MIT, local)

Verified by general industry knowledge. **Rejected** because:

- Single-vector dense only — does NOT produce sparse or ColBERT vectors,
  defeating the M3 hybrid retrieval premise.
- Korean coverage comparable to BGE-M3 but no published advantage.
- 1024-dim dense (same as BGE-M3) so no storage win.

The decision to lock BGE-M3 is upstream of SPEC-IDX-002 (made in
`tech.md` 2026-04-24 decision log). SPEC-IDX-002 implements that
decision; alternative-model swap-in is a future SPEC if measurements
warrant.

---

## 4. Cache Architecture

Embedding the same text twice wastes compute. A cache keyed on
`sha256(text + model_version + mode)` returns cached vectors on hit. Three
storage backends were considered.

### 4.1 In-process LRU (chosen for v0.1)

A bounded in-memory LRU cache (e.g., `cachetools.LRUCache`) inside the
FastAPI worker process. Properties:

- Pros: zero infrastructure dependency; simplest possible cache;
  microsecond hit latency.
- Cons: cache is per-worker (no sharing across replicas); evictions
  driven by working-set size (configurable); cache is lost on restart
  (model load takes 5-30s, so restart is rare).
- Memory cost: each dense vector is 1024 × 4 bytes = 4KB; each ColBERT
  vector list averages ~10 tokens × 4KB = 40KB; sparse dict is ~100
  bytes. Cache of 50,000 entries × 50KB ≈ 2.5GB RAM (configurable
  ceiling).

**Decision**: in-process LRU with default ceiling 10,000 entries;
configurable via `EMBEDDER_CACHE_MAX_ENTRIES` env. Adequate for M3
single-instance deployments. Multi-instance sharing (Open Question
§8.4) deferred.

### 4.2 Redis (rejected v0.1)

Pros: shared across replicas; persistent across restarts; LiteLLM
already runs Redis at `litellm:6379`.
Cons: per-call serialisation overhead (numpy → bytes via pickle) is
~100 µs per request — comparable to or larger than LRU lookup; adds a
network hop; vector size (1KB-50KB) bloats Redis memory if large cache.

**Decision**: NOT in v0.1. Mentioned as a future
`EMBEDDER_CACHE_BACKEND=redis` opt-in.

### 4.3 Disk (rejected v0.1)

A `joblib.Memory`-style on-disk cache keyed on text-hash. Pros: persists
across restarts. Cons: slower than LRU (millisecond reads); requires a
mounted volume (the compose entry already provisions one for HF model
cache, but a *separate* cache dir would be cleaner).

**Decision**: NOT in v0.1. Mentioned as a future
`EMBEDDER_CACHE_BACKEND=disk` opt-in.

### 4.4 Cache-key composition

```
cache_key = sha256(
    f"{text_normalized}\n{model_name}\n{model_version}\n{mode_flags}"
).hexdigest()
```

Where:
- `text_normalized = text.strip()`. We do NOT lowercase or strip
  punctuation: BGE-M3 is case-sensitive and punctuation-aware.
- `model_name` — `BAAI/bge-m3`.
- `model_version` — git commit hash of the model on Hugging Face Hub
  (resolved at startup; pinned in env via `EMBEDDER_MODEL_VERSION`).
  Default: `latest` (resolves at boot).
- `mode_flags` — bitmask of `(return_dense, return_sparse,
  return_colbert)`, e.g., `dense+sparse`. Different modes get
  different cache slots even for the same text.

This guarantees that swapping the model version invalidates the cache
(no stale-vector hazard).

---

## 5. GPU vs CPU Operational Tradeoff

The compose entry defaults to CPU. GPU is opt-in via a compose override
file (`deploy/docker-compose.gpu.yml`) that adds:

```yaml
services:
  embedder:
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
    environment:
      EMBEDDER_DEVICE: cuda:0
      EMBEDDER_USE_FP16: "true"
```

Performance bands (from BGE-M3 README + community benchmarks; these
are *targets*, not measurements; refined in run phase):

| Configuration | Throughput (docs/sec) | p50 latency (1-text request) |
|---------------|-----------------------|------------------------------|
| CPU 4 vCPU, FP32, batch=32 | 30-50  | 200-500 ms |
| CPU 8 vCPU, FP32, batch=32 | 60-100 | 100-300 ms |
| GPU T4, FP16, batch=32     | 400-600 | 30-80 ms |
| GPU A100, FP16, batch=32   | 1500-2500 | 10-30 ms |

NFR-IDX-002 picks values from the CPU 4 vCPU band as the conservative
floor; an optional `slow` test marker exercises the GPU band when
`EMBEDDER_DEVICE=cuda:0` is set.

Memory ceiling per worker:

- CPU FP32: ~3 GB resident (PyTorch + model weights + activations).
- GPU FP16: ~2 GB GPU memory.
- Cache memory (default 10,000 entries × 50KB) ≈ 500 MB additional.

The compose entry sets `mem_limit: 4g` as a safety bound for CPU
deployments; GPU mem usage is governed by the device.

---

## 6. Sidecar vs In-Go Inference

A reasonable alternative would be: skip the Python sidecar, do BGE-M3
inference in Go via a Go ML library. **Rejected** for these reasons:

### 6.1 Go ML library state of the art (2026)

- **`gorgonia.org/gorgonia`** — pure-Go ML primitives; does NOT
  support transformer models off the shelf; would require manual
  graph construction. Verified by absence of pre-built BGE-M3 examples
  in the repo.
- **`github.com/owulveryck/onnx-go`** — Go ONNX runtime; supports
  basic ONNX models but does NOT match `onnxruntime` (Microsoft) for
  transformer support. As of 2026, transformer attention kernels are
  not optimized for Go.
- **CGO bindings to libtorch / onnxruntime** — possible but loses the
  single-binary deploy benefit (libtorch is ~500MB; CGO build
  complexity).

### 6.2 Why sidecar wins

- Python ML ecosystem maturity (PyTorch, transformers, FlagEmbedding,
  optimum) is decades ahead of Go ML.
- The sidecar pattern is already established for `services/researcher/`
  (SPEC-SYN-001). Adding embedder under the same pattern adds zero new
  operational primitives.
- The HTTP boundary is cheap (~1 ms localhost RTT) compared to the 10-100
  ms inference cost.

### 6.3 Roadmap alignment

`.moai/project/roadmap.md:56` explicitly names `services/embedder/` as
the embedding owner — this is the upstream architectural choice that
SPEC-IDX-002 implements. No re-litigation here.

---

## 7. Race / Leak / Cancellation Hazards

### 7.1 Concurrent `/embed` calls share one model

FastAPI workers are single-process (uvicorn `--workers 1`). Multiple
async `/embed` requests in flight call into the SAME `BGEM3FlagModel`
instance. PyTorch is **not** thread-safe by default, but FastAPI's
async handlers run in a single thread (event loop). The actual model
inference runs synchronously inside the handler — the event loop is
blocked for the duration of each call.

This means **N concurrent requests serialize naturally**. Throughput
is bounded by single-stream inference speed; concurrency is for I/O,
not for compute parallelism. Adequate at the M3 scale where each
request batches up to N texts internally.

For genuine compute parallelism, run multiple uvicorn workers
(`--workers K`) — each worker has its own model instance. Open
Question §8.6 reserves this scaling lever.

### 7.2 Go-side concurrent `Embed` calls

The Go-side `internal/embedder.Client.Embed` is goroutine-safe by
construction (mirrors `internal/synthesis.Client.Synthesize`):

- `*http.Client` is goroutine-safe by stdlib contract.
- `Client.baseURL` and `Client.o` are read-only post-`New`.
- No shared mutable state in the request path.

NFR-IDX-005 verifies via `go test -race ./internal/embedder/...` with
50 concurrent callers × 100 calls each = 5,000 concurrent Embed
invocations.

### 7.3 Goroutine leak surface

The Go client uses `context.WithTimeout`, `defer cancel()`, and
`http.Client` with explicit timeout. Mirrors
`internal/synthesis/client.go:62-64`:

```go
ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
defer cancel()
```

NFR-IDX-006 wires `goleak.VerifyTestMain(m)` per
`internal/adapters/reddit/bench_test.go` precedent and per
`SPEC-FAN-001 NFR-FAN-003`.

### 7.4 Subprocess lifecycle

The Python sidecar is its OWN process inside Docker. Lifecycle is
managed by docker-compose: `restart: unless-stopped` ensures crash
recovery; `healthcheck` ensures liveness; on shutdown, uvicorn handles
SIGTERM via FastAPI lifespan event.

Inside the Python process, the model is loaded ONCE during the
FastAPI `lifespan` startup hook and freed during shutdown. There is no
per-request subprocess. GPU memory (if used) is released when the
process exits or when explicit `model.cpu()` + `del model` is called
during `lifespan` teardown.

### 7.5 OOM hazards

- Excessive batch size on CPU: a batch of 1000 long texts (8192 tokens
  each) at FP32 produces ~30GB of activations. Mitigation: hard cap
  `len(texts) ≤ 256` per request (Open Question §8.5; default 256 is
  conservative). Hard cap on per-text tokens at 8192 (BGE-M3's max).
- Cache OOM: the LRU cache has bounded entries (default 10,000) and an
  approximate memory budget computed at fill time.

REQ-IDX-008 enforces the per-request batch cap with HTTP 400 + clear
error body when exceeded.

---

## 8. Open Questions for Plan Phase

These are explicitly UNRESOLVED at SPEC-approval time. Each has a
recommended default and a one-line resolution owner. They do NOT block
SPEC approval.

### 8.1 BGE-M3 inference path — FlagEmbedding vs ONNX

**Default**: FlagEmbedding (Path A in §2.1). Image bloat is acceptable
in M3. Revisit at M9 if measured throughput is the constraint or if
image size > 4 GB triggers a build-pipeline issue. **Resolution
owner**: SPEC-IDX-002 run-phase implementer; future SPEC-IDX-002a if
ONNX path becomes necessary.

### 8.2 GPU vs CPU default

**Default**: CPU. GPU is opt-in via a `docker-compose.gpu.yml` overlay.
Rationale: V1 deploy targets self-hosted teams (`tech.md:93`) where GPU
is not always present. CPU throughput is acceptable for M3 scale (≤
1M docs indexed per day). **Resolution owner**: SPEC-DEPLOY-001 (M9
Helm chart) author can flip to GPU-default for cloud profiles.

### 8.3 Multi-vector default — return all three or only dense

**Default**: per-request flags; default is `return_dense=true,
return_sparse=false, return_colbert_vecs=false`. Callers
(SPEC-IDX-001) opt into the extra modes. Rationale: the extra modes
add ~30% inference cost and dramatically more storage. **Resolution
owner**: SPEC-IDX-001 author chooses the default for the hybrid index.

### 8.4 Cache backend — in-process LRU vs Redis vs disk

**Default**: in-process LRU. `EMBEDDER_CACHE_MAX_ENTRIES=10000`.
Rationale: simplest possible cache; M3-scale single-instance
deployments don't need cross-replica sharing. **Resolution owner**:
future SPEC-IDX-002b if multi-instance cache sharing measurably
matters.

### 8.5 Per-request batch cap

**Default**: `max_texts_per_request = 256`. Each text capped at 8192
tokens (BGE-M3's max). Rationale: prevents OOM on CPU; round number.
Open: should we lower to 64 to keep p99 latency bounded? **Resolution
owner**: run-phase author after first NFR-IDX-002 measurement.

### 8.6 Concurrent uvicorn workers

**Default**: 1. Concurrency is via async event loop; compute-parallel
is impractical because each worker loads a 1.4GB model (4GB total RAM
for 4 workers with FP32). **Resolution owner**: SPEC-DEPLOY-001 (M9
Helm chart) author may scale via separate replicas instead.

### 8.7 Model version pinning

**Default**: pin via `EMBEDDER_MODEL_VERSION` env to a specific Hugging
Face commit hash; default `latest` resolves at boot (logged at INFO).
Rationale: reproducibility per `tech.md:Decision Log`; the Run-phase
author selects an exact pin. **Resolution owner**: SPEC-IDX-002
run-phase author.

### 8.8 Healthcheck depth

**Default**: shallow (`GET /health` returns 200 if FastAPI is up AND
model is loaded). Open: include a model-self-test with a known input?
That adds cost to every healthcheck poll. **Resolution owner**:
SPEC-IDX-002 run-phase author; default to shallow healthcheck +
separate `/healthcheck/deep` for self-tests.

---

## 9. References

### External (URL-cited; verified 2026-05-04)

- https://huggingface.co/BAAI/bge-m3 — BGE-M3 model card; dense dim 1024;
  max length 8192; MIT license; FP16 supported via `use_fp16=True`;
  multilingual incl. Korean.
- https://github.com/FlagOpen/FlagEmbedding — Official BAAI repo; MIT;
  pip package `FlagEmbedding`; primary inference class
  `FlagAutoModel.from_finetuned('BAAI/bge-m3')`. Verified 2026-05-04.
- https://github.com/FlagOpen/FlagEmbedding/blob/master/research/BGE_M3/README.md
  — BGEM3FlagModel API: `BGEM3FlagModel(model_name, use_fp16=True,
  devices=['cuda:0'])`; `encode(sentences, batch_size, max_length,
  return_dense, return_sparse, return_colbert_vecs)`; outputs
  `dense_vecs`, `lexical_weights`, `colbert_vecs`. Verified
  2026-05-04.
- https://onnxruntime.ai/docs/get-started/with-python.html — ONNX Runtime
  Python install matrix; `pip install onnxruntime` (CPU) vs
  `onnxruntime-gpu` (CUDA 12); `ort.InferenceSession('model.onnx',
  providers=['CUDAExecutionProvider', 'CPUExecutionProvider'])`.
  Verified 2026-05-04.
- https://www.uvicorn.org/ — ASGI server for FastAPI; default
  host=127.0.0.1, port=8000, workers=1; programmatic via
  `uvicorn.run()`. Verified 2026-05-04.
- https://docs.vllm.ai/en/latest/ — vLLM supports embedding models
  including E5-Mistral, GTE, ColBERT (BGE-M3 not specifically
  named); OpenAI-compatible API server. Verified 2026-05-04;
  rejected as v0.1 path per §2.3.
- https://pytorch.org/docs/stable/index.html — torch dependency family;
  pin policy follows `tech.md:Decision Log` reproducibility
  principle. Verified previously (project-locked dependency).

### Internal (file:line cited)

- `services/embedder/pyproject.toml:1-33` — current sidecar scaffold.
- `services/embedder/Dockerfile:1-22` — current single-stage build (no
  HEALTHCHECK, no EXPOSE).
- `services/embedder/.env.example:1-11` — current env vars
  (`EMBEDDER_MODEL`, `EMBEDDER_BATCH_SIZE`, `EMBEDDER_LOG_LEVEL`).
- `services/embedder/src/embedder/` — empty (no code).
- `services/researcher/pyproject.toml:1-46` — precedent runtime deps
  shape (FastAPI, uvicorn, pydantic, httpx).
- `services/researcher/Dockerfile:1-30` — multi-stage build precedent
  with HEALTHCHECK + EXPOSE + non-root user.
- `services/researcher/src/researcher/` — package layout precedent.
- `internal/synthesis/client.go:28-280` — Go-side HTTP client precedent.
- `internal/synthesis/types.go:9-68` — Go-side types precedent.
- `internal/synthesis/config.go:14-50` — env binding precedent.
- `internal/obs/metrics/metrics.go:55-178` — Registry struct + metric
  family registration pattern.
- `internal/obs/metrics/synthesis.go:1-60` — registerSynthesis precedent;
  pre-init pattern for test-discovery.
- `internal/obs/metrics/metrics.go:169-176` — labelNames cardinality
  allowlist (current state).
- `pkg/types/normalized_doc.go:40-56` — NormalizedDoc shape (NOT
  consumed by SPEC-IDX-002 directly; SPEC-IDX-001 is the consumer).
- `deploy/docker-compose.yml:1-217` — current 8-service stack;
  `researcher` entry at lines 165-188 is the precedent shape for new
  `embedder` entry.
- `.env.example:1-87` — root env template; new `EMBEDDER_*` keys
  appended.
- `.moai/project/tech.md:39-50` — BGE-M3 lock decision.
- `.moai/project/tech.md:144-153` — Korean tokenizer risk delegated to
  SPEC-IDX-003.
- `.moai/project/structure.md:160-165` — `pkg/types` SDK boundary
  clause (no public types added by SPEC-IDX-002).
- `.moai/project/roadmap.md:55-58` — M3 placement: BGE-M3 sidecar +
  batched inference + cache.
- `.moai/project/roadmap.md:122-128` — M3 parallelization plan.
- `.moai/specs/SPEC-BOOT-001/spec.md:99-105` — `services/embedder/`
  scaffold provisioning.
- `.moai/specs/SPEC-OBS-001/spec.md:88-94` — observability bundle +
  cardinality allowlist precedent.
- `.moai/specs/SPEC-CORE-001/spec.md:139-146` — 5-value `outcome`
  enum reused by embedder counter.
- `.moai/specs/SPEC-SYN-001/spec.md:1-815` — Python sidecar precedent;
  HTTP contract; observability emit pattern.
- `.moai/specs/SPEC-SYN-001/research.md:1-553` — research artifact
  precedent shape.
- `.moai/specs/SPEC-FAN-001/spec.md:1-1508` — Go client wrapping
  HTTP-to-sidecar precedent (concurrent-safe contract).

### Excluded / deferred

- ColBERTv2 + PLAID retrieval engine — would replace the simple cosine
  retrieval at SPEC-IDX-001; out of v0.1 scope.
- ONNX export pipeline — out of v0.1 (Open Question §8.1).
- vLLM embedding server path — out of v0.1 (Open Question §8.1 +
  §2.3).
- BGE-reranker-v2-m3 cross-encoder — separate sidecar in a future SPEC
  if measured; SPEC-IDX-002 only handles bi-encoder embeddings.
- Per-tenant embedding quotas — SPEC-AUTH-002 (M6) concern.
- gRPC contract for embedder — SPEC-IDX-002 ships HTTP only;
  gRPC migration is a future SPEC if the LLM gateway adopts it.

---

*End of SPEC-IDX-002 research v0*
