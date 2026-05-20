"""tokenizer-ko: Korean morphological tokenization sidecar.

Wraps mecab-ko (pymecab-ko) to provide morpheme-level segmentation
for the universal-search Korean index shard (SPEC-IDX-003).

FastAPI sidecar on port 8083 (compose-internal only).
"""

__version__ = "0.1.0"
