# Scoring rubric — 5-point anchors (Korean examples)

SPEC-EVAL-003 · v0.2.0

Every 1–5 score in `scoring-sheet-template.csv` MUST be anchored to this
rubric. 5 = best. Scores assigned without rubric reference are invalid.

---

## ranking_score — overall result relevance

| Score | Anchor                                                                | 예시 (query → 관찰)                                                           |
| ----- | --------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| 5     | Top-3 are exactly what a Korean user wants; ideal source + freshness. | "오늘 코스피 종가 시황" → 1~3위가 네이버 뉴스 당일 시황 기사                  |
| 4     | Top-3 mostly right; minor ordering or one weaker hit.                 | "제주도 3박4일 여행 코스 추천" → 1~2위 네이버 블로그 후기, 3위는 약한 일반 글 |
| 3     | Relevant results exist but not ranked first; user must scroll.        | "무선 이어폰 가성비 비교" → 네이버 쇼핑 결과가 4~5위에 위치                   |
| 2     | Mostly off-target; correct source buried or missing.                  | "강남 맛집 데이트 코스 후기" → 상위가 광고/무관 글, 블로그 후기 거의 없음     |
| 1     | Top-3 irrelevant or wrong language/source entirely.                   | "설날 차례상 차리는 순서" → 영어 위키 결과만 상위                             |

## source_relevance — are the right sources present?

| Score | Anchor                                                           |
| ----- | ---------------------------------------------------------------- |
| 5     | Expected sources (per category table) dominate the top results.  |
| 4     | Expected sources present, mixed with one or two off-source hits. |
| 3     | Expected sources present but minority of top-10.                 |
| 2     | Expected sources barely present (1 hit deep in the list).        |
| 1     | Expected sources absent entirely.                                |

For academic-tech, "expected source" is arXiv/GitHub, NOT Naver — a Naver hit
here LOWERS source_relevance.

## code_switching_handling — 한영 혼용 처리 (code-mixed rows only, required)

| Score | Anchor                                                               | 예시                                                                                      |
| ----- | -------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| 5     | Korean segments mecab-ko tokenized; English/code identifiers intact. | "PyTorch DataLoader 멀티프로세싱 에러" → `DataLoader` 보존, "멀티프로세싱/에러" 정상 분절 |
| 4     | Minor over/under-segmentation but meaning preserved.                 | "gRPC streaming 한국어 튜토리얼" → `gRPC` 보존, "튜토리얼" 약간 어색                      |
| 3     | One segment mishandled (a code term split or a Korean word merged).  | `OAuth2`가 `OAuth`+`2`로 분리                                                             |
| 2     | Multiple segments mishandled; results degraded.                      | 영문 식별자가 글자 단위로 쪼개짐                                                          |
| 1     | Code identifiers destroyed; Korean character-split.                  | "Kubernetes" → "K/u/b/e/..."                                                              |

## tokenization_quality — mecab-ko 분절 적정성

| Score | Anchor                                                    |
| ----- | --------------------------------------------------------- |
| 5     | All Korean terms segmented as a fluent reader would.      |
| 4     | One questionable boundary, no meaning loss.               |
| 3     | A compound noun over/under-split, minor relevance impact. |
| 2     | Several wrong boundaries hurting retrieval.               |
| 1     | Korean segmented into meaningless pieces.                 |

---

## Boolean / float fields (not 1–5)

- `top3_naver_hit`: `true` iff a `naver`-source result appears in the top 3.
- `mrr_top10`: 1/(rank of first naver hit within top-10), else `0.0`.
  Rank-1 → 1.0, rank-2 → 0.5, rank-3 → 0.333, …, none → 0.0.

---

## Reminders

- Anchor every score here; never improvise a scale.
- For `expected_naver_relevant:false` queries (academic-tech, most code-mixed),
  a missing Naver result is CORRECT — do not penalize ranking_score for it.
