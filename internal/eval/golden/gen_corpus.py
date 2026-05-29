#!/usr/bin/env python3
"""Generate 200+ NormalizedDoc fixture files for SPEC-EVAL-001 golden corpus."""
import json
import os
import hashlib
import sys
from datetime import datetime, timezone, timedelta

OUT = "/Users/masterp/Projects/superwork/universal-search/internal/eval/golden/corpus"

SOURCES = [
    "reddit", "hackernews", "arxiv", "github", "youtube",
    "bluesky", "x", "searxng", "naver", "daum",
    "korea_news", "rss", "polymarket",
]

DOCTYPES = ["article", "post", "paper", "video", "repo", "issue", "social", "other"]
LANGS = ["en", "ko"]

# Deterministic topic pools for generating realistic-looking docs.
EN_TOPICS = [
    "quantum computing supremacy milestones",
    "large language model architecture improvements",
    "reinforcement learning from human feedback",
    "mixture of experts training techniques",
    "vector database indexing strategies",
    "retrieval-augmented generation pipelines",
    "transformer attention mechanism variants",
    "neural network pruning and quantization",
    "federated learning privacy guarantees",
    "graph neural networks for molecule design",
    "contrastive learning for embeddings",
    "diffusion models for image generation",
    "multi-modal foundation models",
    "constitutional AI and alignment research",
    "prompt engineering best practices",
    "chain-of-thought reasoning capabilities",
    "knowledge distillation methods",
    "semi-supervised learning techniques",
    "zero-shot and few-shot learning",
    "neural architecture search automation",
    "causal inference in machine learning",
    "natural language processing tokenization",
    "speech recognition end-to-end models",
    "robotics simulation-to-real transfer",
    "autonomous vehicle perception systems",
    "edge computing for model inference",
    "GPU cluster scheduling optimization",
    "distributed training communication efficiency",
    "model serving latency optimization",
    "feature store architecture patterns",
    "data pipeline orchestration frameworks",
    "stream processing with Apache Kafka",
    "columnar storage formats for analytics",
    "time series anomaly detection methods",
    "A/B testing statistical frameworks",
    "microservices observability practices",
    "container orchestration security policies",
    "infrastructure as code best practices",
    "continuous delivery pipeline design",
    "site reliability engineering principles",
]

KO_TOPICS = [
    "한국어 자연어 처리 모델 발전",
    "대규모 언어 모델 한국어 성능 평가",
    "한국어 토크나이저 비교 분석",
    "한국어 임베딩 모델 벤치마크",
    "한국어 질의응답 시스템 구축",
    "한국어 텍스트 분류 기법",
    "한국어 감성 분석 연구 동향",
    "한국어 기계 번역 품질 향상",
    "한국어 음성 인식 기술 발전",
    "한국어 정보 검색 시스템 설계",
    "한국 뉴스 데이터 마이닝 기법",
    "한국어 지식 그래프 구축 방법론",
    "한국어 문서 요약 모델 평가",
    "한국어 대화 시스템 개발 현황",
    "한국어 OCR 기술 적용 사례",
    "한국 SaaS 스타트업 기술 스택 분석",
    "한국 핀테크 AI 도입 사례 연구",
    "한국 헬스케어 AI 규제 동향",
    "한국 자율주행 기술 테스트베드 현황",
    "한국 스마트시티 데이터 플랫폼 구축",
]

def make_hash(source_id, url, title, body):
    h = hashlib.sha256()
    sep = "\x00"
    h.update(source_id.encode())
    h.update(sep.encode())
    h.update(url.encode())
    h.update(sep.encode())
    h.update(title.encode())
    h.update(sep.encode())
    h.update(body.encode())
    return h.hexdigest()[:16]

def gen_doc(idx):
    source = SOURCES[idx % len(SOURCES)]
    doctype = DOCTYPES[idx % len(DOCTYPES)]
    is_korean = idx % 5 == 0  # ~20% Korean docs
    lang = "ko" if is_korean else "en"
    topics = KO_TOPICS if is_korean else EN_TOPICS
    topic = topics[idx % len(topics)]

    doc_id = f"doc-{idx:03d}"
    url = f"https://example.com/{source}/{idx:04d}"
    title = f"{topic} - study {idx}"
    body = (
        f"This document discusses {topic}. "
        f"Research has shown significant progress in this area. "
        f"Multiple teams have contributed to the understanding of {topic}. "
        f"Key findings include improved accuracy, reduced latency, and better scalability. "
        f"The study {idx} was conducted with rigorous evaluation methodology. "
        f"Results demonstrate clear improvements over prior baselines. "
        f"Further research is needed to fully explore the implications. "
        f"The authors recommend additional experiments to validate these findings."
    )
    snippet = f"Summary of research on {topic}."
    published = datetime(2025, 1, 1, tzinfo=timezone.utc) + timedelta(days=idx)
    retrieved = datetime(2026, 5, 1, tzinfo=timezone.utc) + timedelta(hours=idx)

    doc = {
        "id": doc_id,
        "source_id": source,
        "url": url,
        "title": title,
        "body": body,
        "snippet": snippet,
        "published_at": published.isoformat(),
        "retrieved_at": retrieved.isoformat(),
        "author": f"author-{idx % 50}",
        "score": round(0.5 + (idx % 50) * 0.01, 4),
        "lang": lang,
        "doc_type": doctype,
        "hash": make_hash(source, url, title, body),
    }
    return doc_id, doc

def main():
    os.makedirs(OUT, exist_ok=True)
    count = 210  # 200 minimum + 10 buffer
    ids = []
    for i in range(1, count + 1):
        doc_id, doc = gen_doc(i)
        ids.append(doc_id)
        fname = f"doc-{i:03d}.json"
        with open(os.path.join(OUT, fname), "w") as f:
            json.dump(doc, f, indent=2)
    print(f"Generated {count} docs in {OUT}")
    print(f"IDs: {ids[:5]}...{ids[-5:]}")

if __name__ == "__main__":
    main()
