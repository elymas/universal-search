#!/usr/bin/env python3
"""Generate 50-query golden set for SPEC-EVAL-001 (35 EN + 15 KO)."""

import json

OUT = "/Users/masterp/Projects/superwork/universal-search/internal/eval/golden/queries.jsonl"

EN_QUERIES = [
    # factual (13)
    (
        "What is quantum computing and how does it differ from classical computing?",
        "factual",
        ["doc-001", "doc-041", "doc-081"],
    ),
    (
        "How do transformer attention mechanisms work?",
        "factual",
        ["doc-007", "doc-047", "doc-087"],
    ),
    (
        "What are the main challenges in federated learning?",
        "factual",
        ["doc-009", "doc-049"],
    ),
    (
        "Explain the concept of knowledge distillation in machine learning",
        "factual",
        ["doc-017", "doc-057"],
    ),
    (
        "What is retrieval-augmented generation and why is it important?",
        "factual",
        ["doc-006", "doc-046", "doc-086"],
    ),
    (
        "How does contrastive learning improve embedding quality?",
        "factual",
        ["doc-011", "doc-051"],
    ),
    (
        "What are the key differences between SQL and NoSQL databases?",
        "factual",
        ["doc-021", "doc-061"],
    ),
    (
        "Describe the main approaches to neural architecture search",
        "factual",
        ["doc-020", "doc-060", "doc-100"],
    ),
    (
        "What is feature engineering and how does it affect model performance?",
        "factual",
        ["doc-031", "doc-071"],
    ),
    (
        "How do graph neural networks work for molecular design?",
        "factual",
        ["doc-010", "doc-050", "doc-090"],
    ),
    (
        "What are the fundamental principles of site reliability engineering?",
        "factual",
        ["doc-040", "doc-080", "doc-120"],
    ),
    (
        "Explain the zero-shot and few-shot learning paradigm",
        "factual",
        ["doc-019", "doc-059"],
    ),
    (
        "What is the role of activation functions in neural networks?",
        "factual",
        ["doc-004", "doc-044"],
    ),
    # comparison (9)
    (
        "Compare GPT and BERT architectures for NLP tasks",
        "comparison",
        ["doc-002", "doc-042", "doc-082"],
    ),
    (
        "How does reinforcement learning from human feedback compare to supervised fine-tuning?",
        "comparison",
        ["doc-003", "doc-043"],
    ),
    (
        "Compare vector database indexing strategies: HNSW vs IVF",
        "comparison",
        ["doc-005", "doc-045", "doc-085"],
    ),
    (
        "What are the trade-offs between model pruning and quantization?",
        "comparison",
        ["doc-008", "doc-048"],
    ),
    (
        "Compare microservices vs monolithic architectures for ML deployment",
        "comparison",
        ["doc-036", "doc-076", "doc-116"],
    ),
    (
        "How do diffusion models compare to GANs for image generation?",
        "comparison",
        ["doc-012", "doc-052"],
    ),
    (
        "Compare columnar storage formats Parquet and ORC for analytics",
        "comparison",
        ["doc-034", "doc-074"],
    ),
    (
        "What are the differences between Kubernetes and Docker Swarm orchestration?",
        "comparison",
        ["doc-038", "doc-078", "doc-118"],
    ),
    (
        "How does mixture of experts compare to dense transformer models?",
        "comparison",
        ["doc-004", "doc-044", "doc-084"],
    ),
    # synthesis (8)
    (
        "Synthesize the latest advances in large language model architectures",
        "synthesis",
        ["doc-002", "doc-007", "doc-022", "doc-062"],
    ),
    (
        "Provide a comprehensive overview of distributed training techniques",
        "synthesis",
        ["doc-028", "doc-068", "doc-108", "doc-148"],
    ),
    (
        "Summarize the state of multi-modal foundation models",
        "synthesis",
        ["doc-013", "doc-053", "doc-093"],
    ),
    (
        "What are the current best practices in prompt engineering?",
        "synthesis",
        ["doc-015", "doc-055", "doc-095"],
    ),
    (
        "Synthesize research on causal inference in machine learning",
        "synthesis",
        ["doc-021", "doc-061", "doc-101"],
    ),
    (
        "Provide an overview of autonomous vehicle perception systems",
        "synthesis",
        ["doc-025", "doc-065", "doc-105"],
    ),
    (
        "Summarize streaming data processing frameworks and their trade-offs",
        "synthesis",
        ["doc-033", "doc-073", "doc-113"],
    ),
    (
        "What is the current state of semi-supervised learning?",
        "synthesis",
        ["doc-018", "doc-058", "doc-098"],
    ),
    # edge (5)
    ("", "edge", ["doc-001"]),
    ("What is the meaning of life?", "edge", ["doc-001", "doc-002"]),
    ("asdfghjkl qwertyuiop zxcvbnm", "edge", ["doc-003"]),
    ("a", "edge", ["doc-001"]),
    (
        "Compare every programming language ever created",
        "edge",
        ["doc-037", "doc-077", "doc-117", "doc-157", "doc-197"],
    ),
]

KO_QUERIES = [
    # korean (15)
    (
        "한국어 자연어 처리 모델의 최신 발전 동향을 설명해주세요",
        "korean",
        ["doc-005", "doc-030", "doc-055"],
    ),
    (
        "대규모 언어 모델의 한국어 성능을 어떻게 평가하나요?",
        "korean",
        ["doc-010", "doc-035", "doc-060"],
    ),
    (
        "한국어 토크나이저의 종류와 장단점을 비교해주세요",
        "korean",
        ["doc-015", "doc-040"],
    ),
    (
        "한국어 임베딩 모델 벤치마크 결과를 요약해주세요",
        "korean",
        ["doc-020", "doc-045", "doc-070"],
    ),
    (
        "한국어 질의응답 시스템 구축 방법을 설명해주세요",
        "korean",
        ["doc-025", "doc-050"],
    ),
    (
        "한국어 텍스트 분류를 위한 최신 기법은 무엇인가요?",
        "korean",
        ["doc-030", "doc-055", "doc-080"],
    ),
    (
        "한국어 감성 분석 연구의 최신 동향을 정리해주세요",
        "korean",
        ["doc-035", "doc-060"],
    ),
    (
        "한국어 기계 번역 품질을 향상시키는 방법을 설명해주세요",
        "korean",
        ["doc-040", "doc-065", "doc-090"],
    ),
    (
        "한국어 음성 인식 기술의 발전 과정을 요약해주세요",
        "korean",
        ["doc-045", "doc-070"],
    ),
    (
        "한국어 정보 검색 시스템의 설계 원칙을 설명해주세요",
        "korean",
        ["doc-050", "doc-075", "doc-100"],
    ),
    ("한국 뉴스 데이터 마이닝 기법을 비교해주세요", "korean", ["doc-055", "doc-080"]),
    (
        "한국어 지식 그래프 구축 방법론을 설명해주세요",
        "korean",
        ["doc-060", "doc-085", "doc-110"],
    ),
    ("한국어 문서 요약 모델의 성능을 평가해주세요", "korean", ["doc-065", "doc-090"]),
    (
        "한국어 대화 시스템 개발 현황을 요약해주세요",
        "korean",
        ["doc-070", "doc-095", "doc-120"],
    ),
    ("한국 핀테크 AI 도입 사례를 분석해주세요", "korean", ["doc-080", "doc-105"]),
]


def main():
    records = []

    for i, (query, category, sources) in enumerate(EN_QUERIES, 1):
        records.append(
            {
                "id": f"EVAL-001-Q{i:03d}",
                "query": query,
                "locale": "en",
                "category": category,
                "expected_sources": sources,
            }
        )

    for i, (query, category, sources) in enumerate(KO_QUERIES, 36):
        records.append(
            {
                "id": f"EVAL-001-Q{i:03d}",
                "query": query,
                "locale": "ko",
                "category": category,
                "expected_sources": sources,
            }
        )

    assert len(records) == 50, f"Expected 50 queries, got {len(records)}"
    en_count = sum(1 for r in records if r["locale"] == "en")
    ko_count = sum(1 for r in records if r["locale"] == "ko")
    assert en_count == 35, f"Expected 35 EN, got {en_count}"
    assert ko_count == 15, f"Expected 15 KO, got {ko_count}"

    with open(OUT, "w") as f:
        for rec in records:
            f.write(json.dumps(rec, ensure_ascii=False) + "\n")

    print(f"Wrote {len(records)} queries ({en_count} EN + {ko_count} KO) to {OUT}")


if __name__ == "__main__":
    main()
