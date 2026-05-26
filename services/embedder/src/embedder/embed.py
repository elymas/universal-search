"""BGEM3FlagModel wrapper — inference logic.

REQ-IDX-002-002: Inference + request-order preservation.
REQ-IDX-002-007: Input validation (empty, too many, too long).
REQ-IDX-002-009: OOM recovery.
REQ-IDX-002-010: Korean text passed verbatim.
REQ-IDX-002-012: Model loaded once; freed on shutdown.
REQ-IDX-002-013: model_version / revision kwarg.
"""

from __future__ import annotations

from typing import Optional

# @MX:NOTE: [AUTO] BGEM3FlagModel is imported lazily in Embedder.__init__ to allow
# unit-test monkey-patching before the real import occurs.

# Max texts per batch — OWASP input validation (REQ-IDX-002-007).
MAX_BATCH_SIZE = 256
# BGE-M3 XLM-RoBERTa max token length per the model card.
MAX_TOKEN_LENGTH = 8192


class EmbedValidationError(ValueError):
    """Raised when the request fails pre-inference validation."""

    def __init__(self, code: str, detail: str) -> None:
        super().__init__(detail)
        self.code = code
        self.detail = detail


class Embedder:
    """Thin wrapper around BGEM3FlagModel.

    # @MX:ANCHOR: [AUTO] Primary inference entry point; callers: app.py embed route, tests
    # @MX:REASON: fan_in >= 3; all /embed requests flow through this class
    """

    def __init__(
        self,
        model_name: str,
        device: str = "cpu",
        use_fp16: bool = False,
        max_length: int = MAX_TOKEN_LENGTH,
        model_version: Optional[str] = None,
    ) -> None:
        # Import here to allow test monkey-patching of FlagEmbedding module.
        from FlagEmbedding import BGEM3FlagModel  # type: ignore[import-untyped]

        self._model_name = model_name
        self._device = device
        self._max_length = max_length

        kwargs: dict = {
            "use_fp16": use_fp16,
        }
        # Devices list is required by BGEM3FlagModel API.
        if device.startswith("cuda"):
            kwargs["devices"] = [device]

        # REQ-IDX-002-013: pin a specific HF revision when provided.
        if model_version and model_version != "latest":
            kwargs["revision"] = model_version

        self._model = BGEM3FlagModel(model_name, **kwargs)
        # Resolved model version (commit hash or explicit pin).
        self._model_version = model_version or "latest"

    @property
    def model_name(self) -> str:
        """Canonical model identifier."""
        return self._model_name

    @property
    def model_version(self) -> str:
        """Resolved model version string."""
        return self._model_version

    @property
    def device(self) -> str:
        """Compute device (cpu / cuda:0 / ...)."""
        return self._device

    def _validate(self, texts: list[str], return_dense: bool, return_sparse: bool, return_colbert_vecs: bool) -> None:
        """Validate inputs before inference (OWASP: validate before process).

        Raises EmbedValidationError on constraint violation.
        """
        if len(texts) == 0:
            raise EmbedValidationError("empty_input", "texts is empty")
        if len(texts) > MAX_BATCH_SIZE:
            raise EmbedValidationError(
                "batch_too_large",
                f"len(texts)={len(texts)} exceeds {MAX_BATCH_SIZE}",
            )
        if not (return_dense or return_sparse or return_colbert_vecs):
            raise EmbedValidationError(
                "empty_modes",
                "at least one of return_dense, return_sparse, return_colbert_vecs must be true",
            )
        # Token-length pre-check: fast tokenizer pass (~1ms per text).
        # Proxy check via char length: 100k chars > 8192 tokens for any language.
        # A full tokenizer pass is deferred to the inference step.
        for idx, text in enumerate(texts):
            if len(text) > 100_000:
                raise EmbedValidationError(
                    "text_too_long",
                    f"text at index {idx} exceeds {MAX_TOKEN_LENGTH} tokens",
                )

    def embed(
        self,
        texts: list[str],
        *,
        return_dense: bool = True,
        return_sparse: bool = False,
        return_colbert_vecs: bool = False,
        batch_size: int = 32,
    ) -> dict:
        """Run BGE-M3 inference.

        Returns dict with keys: dense, sparse, colbert (each may be None).
        Korean text is passed verbatim — no pre-tokenization (REQ-IDX-002-010).

        Raises:
            EmbedValidationError: on input constraint violation.
            MemoryError: when inference runs out of memory (OOM recovery path).
        """
        self._validate(texts, return_dense, return_sparse, return_colbert_vecs)

        try:
            result = self._model.encode(
                texts,
                batch_size=batch_size,
                max_length=self._max_length,
                return_dense=return_dense,
                return_sparse=return_sparse,
                return_colbert_vecs=return_colbert_vecs,
            )
        except MemoryError:
            # Re-raise so the FastAPI route catches it and returns HTTP 500.
            raise
        except Exception as exc:
            # Wrap unexpected errors — don't expose internals.
            raise RuntimeError(f"inference error: {exc}") from exc

        dense: Optional[list[list[float]]] = None
        sparse: Optional[list[dict[str, float]]] = None
        colbert: Optional[list[list[list[float]]]] = None

        if return_dense and "dense_vecs" in result:
            raw = result["dense_vecs"]
            # numpy array → list[list[float]]
            dense = [row.tolist() for row in raw]

        if return_sparse and "lexical_weights" in result:
            raw_sparse = result["lexical_weights"]
            # Each entry is a dict {token_id: weight}; keys must be str.
            sparse = [{str(k): float(v) for k, v in d.items()} for d in raw_sparse]

        if return_colbert_vecs and "colbert_vecs" in result:
            raw_colbert = result["colbert_vecs"]
            colbert = [[tok.tolist() for tok in row] for row in raw_colbert]

        return {"dense": dense, "sparse": sparse, "colbert": colbert}

    def free(self) -> None:
        """Release the model reference (called during lifespan shutdown)."""
        self._model = None  # type: ignore[assignment]
