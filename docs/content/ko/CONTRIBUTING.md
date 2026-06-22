# KO Tier-1 Reviewer Log

This file records native Korean reviewer signoff for KO Tier-1 documentation pages
per SPEC-DOC-001 REQ-DOC-010 and SPEC-DOC-002 REQ-ADPDOC-017 / NFR-ADPDOC-006.

## Tier-1 Review Log (SPEC-DOC-002)

| Page               | File                                  | Reviewer               | Status                | Review date | Notes                                  |
| ------------------ | ------------------------------------- | ---------------------- | --------------------- | ----------- | -------------------------------------- |
| 어댑터 카탈로그    | `ko/reference/adapters/index.mdx`     | manager-docs (limbowl) | pending native review | 2026-05-31  | Awaiting native Korean speaker signoff |
| Naver 어댑터       | `ko/reference/adapters/naver.mdx`     | manager-docs (limbowl) | pending native review | 2026-05-31  | Awaiting native Korean speaker signoff |
| Korean News 어댑터 | `ko/reference/adapters/koreanews.mdx` | manager-docs (limbowl) | pending native review | 2026-05-31  | Awaiting native Korean speaker signoff |
| 오류 분류 체계     | `ko/reference/adapters/errors.mdx`    | manager-docs (limbowl) | pending native review | 2026-05-31  | Awaiting native Korean speaker signoff |

## How to Review

1. Read the Korean text for natural phrasing and accuracy
2. Verify technical terms are correctly translated (or kept in English where appropriate)
3. Check that Korean-specific content (Naver Console steps, mecab-ko notes) is accurate
4. Sign off by updating the Status to `approved` and adding your name to the Reviewer column
5. Submit a PR with your review notes

## Reviewer Guidelines

- Korean operator-facing content should use formal Korean (존댓말)
- Technical terms: keep English terms as-is when they are commonly used in Korean tech contexts
  - e.g. API, RSS, UTF-8, mecab-ko, Docker, Helm, EUC-KR
- Avoid over-translation of product names: Universal Search, Naver, Bluesky, etc.
