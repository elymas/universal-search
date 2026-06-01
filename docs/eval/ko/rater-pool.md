# Rater pool — anonymous recruitment record

SPEC-EVAL-003 · v0.2.0 · NFR-EVAL-005, NFR-EVAL-004

Anonymous rater IDs + fluency self-attestation date only. No PII (no names,
emails, phones, or account identifiers) — D8.

A scoring round requires at least 3 raters (REQ-EVAL-004, EC-001).

## Pool

| Rater ID | Fluency self-attestation date | Status |
|----------|-------------------------------|--------|
| _R1_ | _TBD — recorded at recruitment_ | _pending operational round_ |
| _R2_ | _TBD — recorded at recruitment_ | _pending operational round_ |
| _R3_ | _TBD — recorded at recruitment_ | _pending operational round_ |

> This pool is a **template**. The real entries are filled at the offline
> 3-rater round (task T09, operational). No raters have been recruited in this
> code-only session; the V1 baseline (`v1.0.0.json`) is deferred to that round.

## Fluency attestation statement (each rater self-affirms)

> "I am a native or near-native Korean speaker familiar with Korean web culture
> and can judge Naver vs Daum vs international source relevance for Korean
> queries." (NFR-EVAL-005)

## Learning-effect policy (spec §8 OQ-2)

The same rater may participate for up to 4 rounds; from the 5th round a rater
swap is recommended to control memory effects on κ. (Open question — confirmed
operationally.)
