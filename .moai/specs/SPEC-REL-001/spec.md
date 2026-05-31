---
id: SPEC-REL-001
version: 1.0.0
status: implemented
created: 2026-05-22
updated: 2026-05-31
author: limbowl
priority: P0
issue_number: 0
title: V1 tag + release notes — semver 1.0.0 cut with single-source version pinning, Keep-a-Changelog consolidation of M1..M9, MIGRATION.md 0.x→1.0 breaking-change guide, signed git tag, multi-arch goreleaser binary distribution, SLSA L2 provenance + cosign attestations for binaries / images / chart, SBOM publication, pre-tag verification matrix, and post-release API-freeze policy
milestone: M9 — V1 release
owner: manager-git
methodology: ddd
coverage_target: 85
depends_on: [SPEC-DOC-001, SPEC-DOC-002, SPEC-DEPLOY-001, SPEC-SEC-001, SPEC-EVAL-001, SPEC-EVAL-002, SPEC-EVAL-003]
blocks: []
related: [SPEC-BOOT-001, SPEC-DEP-001, SPEC-CLI-001, SPEC-CLI-002, SPEC-MCP-001, SPEC-UI-001, SPEC-SKILL-001, SPEC-AUTH-001, SPEC-AUTH-002, SPEC-AUTH-003, SPEC-IDX-001, SPEC-IDX-002, SPEC-IDX-003, SPEC-IDX-004, SPEC-IDX-005, SPEC-CACHE-001, SPEC-IR-001, SPEC-FAN-001, SPEC-CORE-001, SPEC-OBS-001, SPEC-LLM-001, SPEC-SYN-001, SPEC-SYN-002, SPEC-SYN-003, SPEC-SYN-004, SPEC-DEEP-001, SPEC-DEEP-002, SPEC-DEEP-003, SPEC-DEEP-004, SPEC-ADP-001, SPEC-ADP-002, SPEC-ADP-003, SPEC-ADP-004, SPEC-ADP-005, SPEC-ADP-006, SPEC-ADP-007, SPEC-ADP-008, SPEC-ADP-009]
---

# SPEC-REL-001: V1 tag + release notes — `v1.0.0` cut, signed tags, SLSA L2 attested binaries / images / chart, migration guide

## HISTORY

- 2026-05-31 (implemented v1.0.0, limbowl via manager-docs — sync):
  DDD implementation complete + evaluator-active PASS after 1 fix
  cycle. Commits: `0849f31` (plan gate), `9d3d732` (impl), `e93dbe4`
  (goreleaser archive collision fix). Evaluator-active FAIL on first
  pass (goreleaser archive path collision — two archives mapped same
  output path); resolved in `e93dbe4` by disambiguating archive name
  patterns per binary. Final scores: Func 90 / Sec 88 / Craft 85 /
  Cons 88. Status approved → implemented.
  Carry-forward (post-merge operational):
  - Actual `v1.0.0` git tag + CHANGELOG `[1.0.0]` body consolidation
    per RELEASE.md ceremony (merge PRs #42–#48 → resolve conflicts →
    verify G5–G9 on main → GPG-sign + push tag → release.yml runs).
  - `<org>` → `elymas` propagation in 7 dep SPECs (DEPLOY-001 /
    SEC-001 / DOC-001 / DOC-002 / EVAL trio) before merge.
  - goreleaser not installed locally; 12-archive output verified in CI
    via static template analysis (no collision confirmed).
  - release.yml G5–G9 dep workflows (security.yml / eval-*.yml /
    chart-ci.yml / docs.yml) land post-merge; graceful gh-run lookups.
  - LOW: release.yml packages:write permission (no image push —
    least-privilege); SLSA outputs implicit dependency.
  - Deferred post-V1: gitsign, Homebrew/apt/Windows, release-please,
    LTS policy.

- 2026-05-31 (amend v0.2.0, limbowl via manager-spec — pre-reaudit
  reconciliation):
  plan-auditor blocker dispositions + stale-ref / contradiction
  cleanup. **No scope expansion** — same 18 REQ / 7 NFR / 5 new files.
  Changes:
  - **B2 (phantom image)**: REQ-REL-017 + gate G7 verified a
    `ghcr.io/elymas/universal-search:1.0.0` PRIMARY app image that is
    **never built**. SPEC-DEPLOY-001 produces exactly three Go images
    via `Dockerfile.usearch-{api,mcp,migrate}` (the `universal-search`
    name only ever names the Helm **chart** at `oci://ghcr.io/elymas/
    charts/universal-search`, not an app image). Fixed REQ-REL-017 +
    G7 to verify the **real** images — `usearch-api` + `usearch-mcp`
    as the primary app images (+ `usearch-migrate`) per DEPLOY-001's
    Dockerfile set. The gate no longer hard-fails on a nonexistent
    image.
  - **B1 (`<org>` → `elymas`)**: REL-001 already resolves the registry
    org to `elymas` throughout (0 `<org>` placeholders remain in this
    SPEC). Added an explicit PRE-MERGE OPERATIONAL note: the 7
    dependency SPECs (DEPLOY-001 / SEC-001 / DOC-001 / DOC-002 / EVAL-
    trio) still carry `ghcr.io/<org>/` placeholders in their deferred
    publish steps; resolving those to `elymas` is an operational step
    executed before merge. REL-001 documents the canonical value
    (`elymas`); it does NOT edit the other SPECs.
  - **B3 (stale test regex)**: spec.md + acceptance.md cited a strict
    `^usearch v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$`. The REAL
    `cmd/usearch/main_test.go:12` uses a looser prefix-anchored
    `^usearch v\d+\.\d+\.\d+` (no `$`, no prerelease group). Corrected
    REQ-REL-002 + S2 + AC-001/AC-002 to target the REAL regex as the
    current characterization-test state (no tightening — preservation
    only).
  - **B-doc (MIGRATION ordering)**: spec.md §D4 and acceptance.md
    AC-005 listed the 12 MIGRATION.md sections in DIFFERENT orders.
    Reconciled AC-005 to the canonical D4 ordering (§1 Overview … §12
    Rollback procedure) across both documents.
  - **Stale line refs**: corrected version-const line numbers —
    `cmd/usearch/main.go:14` (was :21), `cmd/usearch-api/main.go:20`
    (was :18), `cmd/usearch-mcp/main.go:19` (was :13).
  - **Tag-deferral framing**: confirmed REL-001 ships release
    **machinery + procedure** only; the actual `v1.0.0` git tag +
    CHANGELOG `[1.0.0]` body + MIGRATION per-section content are
    OPERATIONAL/post-merge (after all 7 dependency PRs merge to main).
    Acceptance does NOT require a live tag at implementation time.
  - Status remains `draft`.

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  M9 (V1 release)의 **네 번째이자 terminal SPEC**이며, **본 SPEC은
  blocks: [] — 본 SPEC을 차단할 다른 SPEC은 존재하지 않는다**.
  `.moai/project/roadmap.md:115` ("SPEC-REL-001 | V1 tag + release
  notes | semver 1.0.0, CHANGELOG, migration guide | manager-git")의
  full EARS 확장 + §5 M9 exit criteria 전체 ("`v1.0.0` tagged;
  Helm chart deployable; docs site live") 의 **최종 gating SPEC**.
  세 exit criteria 중 "Helm chart deployable"은 SPEC-DEPLOY-001에,
  "docs site live"는 SPEC-DOC-001 + DOC-002에 단독 위임되어 있으나,
  "`v1.0.0` tagged"는 본 SPEC만이 책임진다 — 그리고 본 SPEC PASS는
  다른 두 criterion이 모두 PASS한 후에만 가능하다 (REQ-REL-013 pre-
  tag verification matrix). 즉 본 SPEC은 **M9의 master gate**이자
  V1 출시 의식 (release ceremony) 의 codification이다.

  본 SPEC은 **새로운 release 시스템을 발명하지 않는다**. 본 SPEC은
  **이미 존재하는 6개 release 자산** (CHANGELOG.md의 KaC v1.1.0
  format + 항목 ~150 줄, conventional-commit history 약 60개,
  `.github/workflows/go.yml` + `deps-audit.yml` CI baseline, 3개
  binary entrypoint의 `Version`/`version` 상수, `cmd/usearch`의
  `--version`/`-v` 플래그 + `TestVersionFlag` regression test,
  go.mod toolchain pin 1.25.x) 을 (a) consolidate (3 → 1 single
  source of truth), (b) automate (수동 → goreleaser + release-
  workflow), (c) certify (unsigned → SLSA L2 + cosign + SBOM)의
  세 축으로 release-shaped 출시 의식으로 묶는다. 따라서 methodology
  는 DDD (ANALYZE existing release surface → PRESERVE conventional-
  commit + Keep-a-Changelog history with byte fidelity → IMPROVE
  with single-source version + signed-tag + multi-arch goreleaser +
  attestations).

  현재 코드베이스에 이미 배치되어 있어 본 SPEC이 의존하는 release
  자산 (research.md §1 inventory):

  - `cmd/usearch/main.go:14` — `const Version = "0.1.0-dev"` (exported,
    title-case). cobra root command (`newRootCmd`/`runCobra`) 가
    `usearch v%s` 형식으로 `--version`/`-v` 출력에 사용.
    `cmd/usearch/main_test.go:12` `semverPattern` =
    **`^usearch v\d+\.\d+\.\d+`** (prefix-anchored, NO `$`, NO
    prerelease group) 가 `TestVersionFlag` (`:16-`) +
    `TestVersionShortFlag` (`:34-`) 에서 매칭으로 보호. REQ-BOOT-012
    implementation.
  - `cmd/usearch-api/main.go:20` — `const version = "0.1.0-dev"`
    (lowercase, unexported). `:27` `ServiceVersion: version`로
    OBS-001 의 obs.Init() 에 전달.
  - `cmd/usearch-mcp/main.go:19` — `const version = "0.1.0-dev"`
    (lowercase, unexported). `:28` `fmt.Printf("usearch-mcp %s\n",
    version)` (`--version` flag) + obs.Init ServiceVersion 로 사용.
  - `CHANGELOG.md` (182 lines, Keep-a-Changelog v1.1.0 format,
    SemVer 2.0.0 footer reference): `[Unreleased]` 섹션 하나만
    존재 (zero released versions, zero git tags). M6 (2026-05-22
    AUTH rollout) ← M4 (2026-05-09 SYN hardening) ← M3 (2026-05-08
    IDX/CACHE + 2026-05-07 ADP suite) ← M2 ← M1 (2026-04-24 boot)
    chronological reverse. SPEC-당 detail level은 hash + 핵심
    artifact + coverage + new metric families + REQ coverage 노트.
    Footer link: `[Unreleased]: https://github.com/elymas/universal-
    search/commits/main` — 본 SPEC이 v1.0.0 link 추가 + Unreleased
    range 수정 필요.
  - `git log --oneline` head 60 commits: 100% conventional-commit
    format (`docs(spec):`, `feat(adapter):`, `feat(auth):`, `chore
    (obs):`, `refactor(adapter):`, `docs(sync):`). PR linkage
    (#NN) 도 일부 사용. release-please / git-cliff 가 직접 소비
    가능한 history.
  - `git tag --list | wc -l` = 0 — clean slate. v1.0.0가 첫 tag.
  - `.github/workflows/`: `compose-check.yml` (510B), `deps-audit.
    yml` (9.3KB, govulncheck + pip-audit + pnpm-audit + hadolint +
    license-scan + searxng-digest-check), `go.yml` (875B, vet +
    test + race), `pre-commit-autoupdate.yml`, `pre-commit.yml`,
    `python.yml`, `web.yml`. **No release.yml**. **No goreleaser**.
    **No build-images.yml** (SPEC-DEPLOY-001 가 신설할 예정).
  - `go.mod`: Go toolchain 1.25.x (BOOT-001 commit 70e4bdc post-fix
    per CHANGELOG line "SPEC-BOOT-001 toolchain alignment 1.23 →
    1.25").
  - `README.md` (87 lines): repo-level quickstart. version badge,
    install instructions는 없음 (v1.0.0 ship 시 신설 필요 — DOC-
    001 scope에서 다룰 수도 있으나 release artifact 의 download
    URL은 본 SPEC에서 ship).
  - 누락된 release 자산 (gap inventory): **VERSION 파일 없음**,
    **MIGRATION.md 없음**, **RELEASE.md (운영자용 release runbook)
    없음**, **`.goreleaser.yml` 없음**, **`.github/workflows/
    release.yml` 없음**, **`SECURITY.md` 없음** (SPEC-SEC-001에서
    신설 예정), **`internal/version/` 패키지 없음** (3개 분산
    상수가 단일화될 잠재 target location).

  본 SPEC이 신규로 도입하는 것:

  - `internal/version/version.go` (NEW): 3-binary 분산 상수를
    consolidate하는 단일 source of truth. `Version`, `Commit`,
    `BuildDate`, `GoVersion` 4개 변수 + `String()` formatter +
    `Short()` (semver only). `Version`은 `0.1.0-dev` default;
    release build 시 `go build -ldflags "-X github.com/elymas/
    universal-search/internal/version.Version=1.0.0 -X ...
    Commit=$GITHUB_SHA ..." ./cmd/usearch ./cmd/usearch-api ./cmd/
    usearch-mcp` 로 inject. 3개 binary `main.go`에서 import +
    consume.
  - `CHANGELOG.md` 의 `[1.0.0] - 2026-MM-DD` 신규 섹션 (release
    날짜는 tag 시점 결정 — 본 SPEC implementation 시 결정):
    `[Unreleased]` 의 모든 항목을 1.0.0 으로 promote + M1..M9
    chronological 정리 + Added/Changed/Deprecated/Removed/Fixed/
    Security 6-section grouping. 본 SPEC 자체는 **CHANGELOG 편집을
    실행하지 않는다** (HARD constraint per user brief) — REQ는 편집
    절차와 검증 기준만 명세. release 실행 시 본 SPEC을 reference
    하여 manager-git 이 수행.
  - `MIGRATION.md` (NEW): 0.x → 1.0 breaking-change 가이드. 본
    SPEC implementation 단계에서 작성. 잠재 breaking-change 목록
    은 §2 REQ-REL-004 + research §4에서 enumerate.
  - `RELEASE.md` (NEW): maintainer-facing release runbook. pre-
    tag verification matrix (REQ-REL-013) 의 manual checklist 형태
    + emergency rollback 절차 + post-release tasks.
  - `.goreleaser.yml` (NEW): goreleaser v2 config. linux/darwin ×
    amd64/arm64 × 3 binaries (usearch + usearch-api + usearch-mcp)
    = 12 archive. CGO_ENABLED=0 static. archive name 표준:
    `usearch_${VERSION}_${OS}_${ARCH}.tar.gz` (linux/darwin),
    `usearch_${VERSION}_windows_${ARCH}.zip` 는 V1 scope 밖 (research
    §5.3 rationale).
  - `.github/workflows/release.yml` (NEW): `on.push.tags: [v*.*.*]`
    trigger. (a) pre-tag verification matrix 실행 (REQ-REL-013),
    (b) goreleaser run, (c) SLSA provenance generation, (d) cosign
    sign-blob for binaries + cosign sign for images (SPEC-DEPLOY-001
    build-images.yml 결과물), (e) GitHub Release 생성 + CHANGELOG
    [v1.0.0] section 본문을 release notes body로 추출 + 모든
    artifact 첨부.
  - `SECURITY.md` 는 SPEC-SEC-001 REQ-SEC-011 V14 evidence로
    이미 신설 예정 — 본 SPEC은 단지 v1.0.0 cut 시점에 존재함을
    REQ-REL-013 verification matrix 에서 assert만 한다.
  - `README.md` 의 version badge + install snippet 업데이트 (existing
    file modification). v1.0.0 release URL 추가.

  Pinned decisions (D1..D9 9개 scope pillar + 보조 D10..D11):

  (D1) **Version source authority — single source via `internal/
       version/`**: 현재 3개 binary에 분산된 `Version`/`version`
       상수 (research §1.2)는 release ceremony 의 fragility 원천.
       1.0.0 cut 시점에 3개 파일을 동시 편집해야 하고, 어느 하나라도
       빠지면 `usearch --version` 과 `usearch-api`의 obs.Init() 가
       서로 다른 버전을 reports. **결정**: `internal/version/
       version.go` 단일 패키지로 consolidate. 3개 main.go가 import.
       빌드 시점에 `-ldflags "-X ...Version=$VERSION -X ...Commit=
       $SHA"` 로 inject. 운영자 `usearch --version` 출력이 한 곳에서
       관리. `cmd/usearch/main_test.go:TestVersionFlag` regex 호환성
       유지 (HARD — characterization test).
       - Anti-decision: `VERSION` 파일 (text-only) 방식은 (a) Go
         binary embed 절차 추가 (go:embed), (b) `usearch --version`
         이 file read 의존성 도입, (c) Docker image 내 file
         layout 영향 → 배제. ldflags-injected 단일 Go package가
         관용적.
       - Anti-decision: 3개 분산 상수 유지는 (a) drift risk, (b)
         release ceremony 복잡도, (c) CHANGELOG 항목 "3 files
         modified per release" 노이즈 → 배제.

  (D2) **CHANGELOG automation — hand-curated + KaC v1.1.0 format
       유지**: 본 프로젝트의 CHANGELOG.md (182 lines) 는 이미
       SPEC-당 dense detail (hash, coverage, metric families, REQ
       coverage)을 carry. release-please / git-cliff 의 자동
       생성은 conventional-commit subject만 capture — SPEC narrative
       (예: "REQ-IDX1-001..020 + NFR-IDX1-001..005 fully implemented
       with @MX:ANCHOR on `Upsert`/`Search`/`Fetch`") 는 손실. **결정**:
       v1.0.0 cut 은 hand-curated. `[Unreleased]` 의 모든 항목을
       `[1.0.0] - 2026-MM-DD` 로 promote + Added/Changed/Deprecated/
       Removed/Fixed/Security 6-section grouping. SPEC-당 narrative
       preservation. v1.0.1 patch 부터는 hybrid (자동 conventional
       commit harvest + manual SPEC narrative 보강) — 단 V1 scope
       밖 (post-V1 별도 SPEC 가능).
       - Anti-decision: release-please bot 자동화는 (a) SPEC
         narrative 손실, (b) `[Unreleased]` 의 기존 dense entries
         재구조화 부담, (c) commit subject 충실도 신뢰 어려움 →
         V1 ship 직후 hand-curated.

  (D3) **Release-notes generation — CHANGELOG section extraction
       to GitHub Release body**: `release.yml` workflow가 `[v1.0.0]
       - YYYY-MM-DD` 섹션부터 다음 `## [` 섹션 직전까지를 본문
       추출 → GitHub Release body 로 publish. 영어 + 한국어 요약
       (research §6.2 bilingual policy) — 영어는 CHANGELOG verbatim,
       한국어 요약 (200-400자) 는 본 SPEC implementation 단계에서
       MIGRATION.md 와 함께 작성. GitHub Release body에는 영문
       CHANGELOG 추출 + 상단에 한국어 요약 collapsible (`<details>`)
       block.
       - extraction tool: `awk '/^## \[1\.0\.0\]/,/^## \[/'` 또는
         `git-cliff` 의 section-extract mode. release.yml 에서
         shell script 로 구현. external dep 추가 없음.

  (D4) **MIGRATION.md scope + structure**: 0.x → 1.0 breaking-
       change document. **현재 0.x 사용자는 사실상 0명** (zero git
       tags, zero releases). 그러나 V1 의 의미는 "**향후** 의 1.x
       시리즈가 본 문서를 reference한다" 다. **결정**: MIGRATION.md
       structure:
       - §1 Overview — semver 1.0.0의 public API freeze 약속이
         무엇이고, 0.x-dev 시점에서 1.x 로 옮긴 사용자가 마주칠
         breaking change 의 카테고리.
       - §2 CLI breaking changes — `usearch query` flag 이름,
         exit code, output format (text/json) 의 변경 사항.
         **현재 known breaking change: 없음** (cmd/usearch는 M2 이후
         flag 안정).
       - §3 Config schema breaking changes — `.moai/config/sections/
         *.yaml` schema. **현재 known: deep.yaml 의 cost guard
         schema** (SPEC-DEEP-004 의 user_id key 도입), **auth.yaml
         의 OIDC vars** (SPEC-AUTH-001 추가). 0.x-dev 시점 사용자가
         있었다면 영향 가능.
       - §4 Env var renames — research §4.2의 grep 결과 확인.
         **현재 known: 없음** (env-var 이름은 M1 부터 stable).
       - §5 MCP protocol surface — SPEC-MCP-001 의 tool name +
         schema. **현재 known: SPEC-MCP-001 implementation 중**
         (draft status). V1 ship 시점 명확화.
       - §6 Adapter plugin contract — SPEC-CORE-001 의 `pkg/types/
         Adapter` interface signature. **현재 known: stable since
         M2** (CORE-001 implementation commit f728aa2).
       - §7 MoAI Skill manifest — SPEC-SKILL-001 의 skill metadata
         schema. **현재 known: SPEC-SKILL-001 draft**. V1 ship
         시점 명확화.
       - §8 REST/GraphQL endpoint — cmd/usearch-api 의 `POST /query`
         + `POST /query/stream` (SPEC-SYN-004 추가). **현재 known:
         endpoint path는 stable; response schema는 schemaVersion=1
         lock per cmd/usearch/output_json.go:19**.
       - §9 Database schema — `deploy/postgres/migrations/0001..
         0007.sql` 의 migration sequence. **현재 known: forward-only;
         down migration은 데이터 손실 위험 → 권장 안 함** (SPEC-
         DEPLOY-001 NFR-DEPLOY-004 와 동일 policy).
       - §10 Adapter status taxonomy reference — SPEC-DOC-002 의
         alpha/beta/stable badge 정의. V1 freeze 항목 vs free
         항목 명시.
       - §11 Upgrade procedure — `helm upgrade` (SPEC-DEPLOY-001),
         `go install` (binary), Skill marketplace 재설치 (SPEC-
         SKILL-001).
       - §12 Rollback procedure — `helm rollback`, container tag
         downgrade.
       각 섹션은 "**0.x 시점**: ...; **1.x 시점**: ...; **운영자
       action**: ..." 의 3-block 형식. 현재 known breaking change
       가 없는 섹션은 "v1.0.0 — no breaking changes in this category"
       로 explicit, 향후 1.x 에서 채워질 placeholder.

  (D5) **Git tag protocol — annotated + signed + verified**:
       - **annotated**: `git tag -a v1.0.0 -m "Release v1.0.0 ..."`.
         lightweight tag (단순 ref) 는 metadata 누락 → 배제.
       - **signed**: GPG 또는 sigstore (gitsign). **결정**: GPG
         signing — maintainer (limbowl@elymas) GPG key 사전 등록.
         sigstore gitsign 은 (a) keyless OIDC ergonomics 좋지만,
         (b) GitHub Actions ephemeral identity 와 tag-pushing
         human identity 간 분리 모호, (c) maintainer 환경 setup
         복잡도 → V1은 GPG, post-V1 gitsign 평가 (별도 SPEC 가능).
       - **verified**: GitHub repo의 "Require signed commits" + tag
         protection rules 활성. CI workflow `release.yml` 의 첫
         step이 `git verify-tag $GITHUB_REF_NAME` 으로 signed
         tag 강제. verification 실패 시 workflow 실패 → release
         차단.
       - **tag message format**: title `Release v1.0.0 — Universal
         Search` + 본문 = CHANGELOG `[1.0.0]` section first 30
         lines summary + `Signed-off-by: limbowl <…>` + 모든 M9
         dependency SPEC reference.

  (D6) **Pre-tag verification matrix — release.yml gate**: release
       workflow trigger 직후 first job. **모두 PASS** 시에만 다음
       step (goreleaser, image sign, GitHub Release publish) 진행.
       - **G1 Code health**: `go vet ./...` clean; `go test -race
         ./...` PASS; coverage ≥ 85% (per `.moai/config/sections/
         quality.yaml` baseline).
       - **G2 Lint**: `golangci-lint run --timeout=10m ./...` clean
         (if configured; else skip with warn). `pre-commit run
         --all-files` clean.
       - **G3 LSP gate (per CLAUDE.md §6 LSP Quality Gates sync
         phase)**: zero errors, ≤ 10 warnings, clean LSP.
       - **G4 Dependency audit**: `.github/workflows/deps-audit.yml`
         latest run on `main` PASS (govulncheck + pip-audit + pnpm-
         audit + hadolint + license-scan + searxng-digest-check 모두
         green).
       - **G5 Security workflow**: `.github/workflows/security.yml`
         (SPEC-SEC-001 신설 예정) latest run PASS (gitleaks +
         gosec + semgrep + Trivy 모두 green).
       - **G6 EVAL gate**: SPEC-EVAL-001 citation faithfulness ≥
         0.85; SPEC-EVAL-002 adapter dashboard live (last-data
         within 24h); SPEC-EVAL-003 Korean benchmark manual sign-
         off recorded in `.moai/reports/`.
       - **G7 Helm chart smoke**: SPEC-DEPLOY-001 `chart-ci.yml`
         latest run PASS (helm lint + helm template + kubeconform
         1.28..1.31 + kind smoke install).
       - **G8 Docs site build + link-check**: SPEC-DOC-001 `docs.yml`
         latest run PASS (Nextra build + link-check + Pagefind
         indexing).
       - **G9 Adapter drift detection**: SPEC-DOC-002 `adapter-
         reference-drift.yml` (또는 chart-ci.yml 의 parity test)
         PASS.
       - **G10 CI green sustained**: 직전 24h 동안 main branch
         `go.yml`/`deps-audit.yml`/`pre-commit.yml`/`web.yml`/
         `python.yml` 모두 green (간헐적 실패 후 fix 의 false-
         positive 방지).
       - **G11 Signed tag**: `git verify-tag $GITHUB_REF_NAME`
         exit 0.
       - **G12 Version consistency**: `usearch --version` 출력 의
         semver 가 `$GITHUB_REF_NAME` (without `v` prefix) 와
         정확히 일치. ldflags inject 실패 detection.
       각 gate 는 release.yml 의 `needs:` 그래프로 expressed; 어느
       하나라도 실패 시 entire workflow 실패 + GitHub Release 생성
       안 함.

  (D7) **Artifact distribution channels — 3-channel**:
       - **Channel A (Go binaries via GitHub Releases)**: `.goreleaser.
         yml` 가 build → archive → upload. linux/darwin × amd64/
         arm64 × 3 binaries (`usearch` + `usearch-api` + `usearch-
         mcp`) = 12 archives. archive naming: `usearch_${VERSION}
         _${OS}_${ARCH}.tar.gz`. SHA256SUMS 파일 첨부. cosign
         signed-blob `.sig` + `.crt` (keyless OIDC).
       - **Channel B (container images via ghcr.io)**: SPEC-DEPLOY-001
         REQ-DEPLOY-018 의 `build-images.yml` 가 build + sign +
         push 책임. DEPLOY-001 이 실제로 build 하는 image 는 정확히
         3개 — `usearch-api`, `usearch-mcp` (primary app images) +
         `usearch-migrate` (migration job) — `Dockerfile.usearch-
         {api,mcp,migrate}` 로부터. **`universal-search` 라는 app
         image 는 존재하지 않는다** (`universal-search` 는 Helm
         **chart** 이름 — Channel C 참조). 본 SPEC은 build-images.yml
         결과물의 cosign signature + SLSA provenance 가 v1.0.0 tag
         시점 ghcr.io 에 존재함을 verify (G7 의 일부) + release notes
         에서 `docker pull ghcr.io/elymas/usearch-api:1.0.0` +
         `docker pull ghcr.io/elymas/usearch-mcp:1.0.0` 명시.
       - **Channel C (Helm chart via OCI)**: SPEC-DEPLOY-001 REQ-
         DEPLOY-017 의 `chart-release.yml` 가 `oci://ghcr.io/elymas/
         charts/universal-search:1.0.0` 에 push. 본 SPEC은 결과물
         존재 + cosign verification PASS 를 verify (G7 + G11).
       - Out-of-scope: Homebrew tap, apt/yum 패키지, Snap, AUR —
         V1 scope 밖 (research §5.5 community-driven post-V1).

  (D8) **SBOM + SLSA + cosign — supply chain transparency**:
       - **SBOM**: `anchore/syft v1.x` 가 SPDX format JSON 생성.
         각 binary archive 마다 `.spdx.json` 첨부. 컨테이너 image
         SBOM은 SPEC-DEPLOY-001 build-images.yml 담당. helm chart
         SBOM은 chart-release.yml 담당. 본 SPEC scope: Go binary
         SBOM 신규.
       - **SLSA**: SPEC-SEC-001 REQ-SEC-016 가 SLSA Level 2 (provenance
         + signed releases) 정의 + `slsa-framework/slsa-github-
         generator/.github/workflows/generator_generic_slsa3.yml
         @v2.0.0` use. 본 SPEC release.yml 이 동일 generator를 Go
         binary 에도 적용. provenance attestation `*.intoto.jsonl`
         은 GitHub Release artifact + cosign attach attestation 로
         이중 첨부.
       - **cosign**: `sigstore/cosign-installer@v3.7.0` keyless
         (GitHub Actions OIDC identity). binary 는 `cosign sign-
         blob`, image 는 `cosign sign`, chart 는 `cosign sign`.
         verification 명령:
         ```
         cosign verify-blob \
           --certificate usearch_1.0.0_linux_amd64.tar.gz.crt \
           --signature usearch_1.0.0_linux_amd64.tar.gz.sig \
           --certificate-identity-regexp "https://github.com/elymas/
         universal-search/.github/workflows/release.yml@.*" \
           --certificate-oidc-issuer "https://token.actions.
         githubusercontent.com" \
           usearch_1.0.0_linux_amd64.tar.gz
         ```
         RELEASE.md + GitHub Release body 에 명시. SPEC-SEC-001 의
         runbook (`ops/security/runbook.md`) + DOC-001 (`operators/
         security/image-verification.mdx`) 가 user-facing 표면.

  (D9) **Post-tag API freeze policy — semver 1.x.y discipline**:
       - **Freeze scope (1.x.y 동안 stable contract)**:
         - CLI commands (`usearch query`, future `usearch deep`,
           subcommand 추가는 minor)
         - CLI flag 이름 + 의미 (`--source`, `--format`, `--timeout`,
           `--json` 등)
         - exit code 의미 (cmd/usearch/exitcode.go)
         - MCP protocol tool names + schemas (SPEC-MCP-001
           implementation 결과)
         - MoAI Skill manifest schema (SPEC-SKILL-001)
         - Adapter plugin interface (`pkg/types/Adapter`, `Capabilities`)
         - REST endpoint paths + response schemaVersion (cmd/
           usearch-api)
         - `.moai/config/sections/*.yaml` schema (additive keys OK,
           removal/rename = breaking)
         - env-var 이름 (additive OK, removal/rename = breaking)
         - K8s Helm chart values.schema.json (SPEC-DEPLOY-001;
           additive OK, removal = breaking)
       - **Free zone (1.x.y 동안 backward-incompat 변경 허용)**:
         - `internal/` Go 패키지 (Go convention)
         - 실험적 adapter (alpha/beta status per SPEC-DOC-002
           taxonomy)
         - AI prompt template 내용 (사용자 통제 밖, accuracy
           개선 목적 변경 frequent)
         - 내부 metric label values (cardinality allowlist 내
           addition OK)
         - Python sidecar internal API (서로 다른 서비스 간
           cross-call 없음 if internal)
       - **Deprecation cycle**: breaking change 도입 시 (a) 직전
         minor (1.X.0) 에서 deprecation warning emit + DEPRECATED.md
         entry, (b) 다음 minor (1.X+1.0) 에서 alternative ship 가능
         시 alternative 안내, (c) 다음 major (2.0.0) 에서 removal.
         minimum 1 minor cycle (≈ 3개월 가정, no hard timeline).
       - **2.0.0 trigger**: 다음의 1개 이상 발생 시 major bump
         정당화 — (a) Go modules path 변경 (github.com/elymas/
         universal-search → 다른 path), (b) database migration
         non-backward-compat schema 변경, (c) MCP/Skill protocol
         major version bump, (d) adapter plugin interface signature
         변경.
       - **Anti-pattern**: 0.x-dev 시절 의 임의 변경 자유 사고를
         post-1.0 에 들고 오기. 본 정책은 maintainer self-discipline
         + PR review checklist 로 enforce.

  (D10) **Post-release tasks — automated + manual**:
       - **Automated (release.yml post-publish steps)**:
         - GitHub Release publish 완료 시 slog/Slack/email notification
           (configured webhook 가 있을 때).
         - `roadmap.md` 의 M9 section 자동 PR 생성 → "M9 ✅
           shipped 2026-MM-DD" + M10 placeholder 추가. release.yml
           이 `gh pr create` 로 PR open.
         - `CHANGELOG.md` 의 footer link 자동 update — `[Unreleased]:
           …/compare/v1.0.0...HEAD` + `[1.0.0]: …/releases/tag/
           v1.0.0`. 단 CHANGELOG 본문 편집은 release ceremony
           manual (D2 hand-curated rationale).
       - **Manual (RELEASE.md maintainer checklist)**:
         - 보안 advisory channel setup — GitHub Security Advisories
           활성 (private vulnerability reporting), SPEC-SEC-001 의
           `SECURITY.md` 가 reporter contact email + GPG key 명시.
         - 운영자 announcement 작성 (영문 + 한국어 요약) — GitHub
           Discussions + 가능 시 community channel (post-V1 별도).
         - Post-mortem if any release-day incident — SPEC-AUTH-003
           audit log + observability dashboard 회고. 본 SPEC 자체는
           incident 시 사후 작성.
       - **Out-of-scope (V1 release ceremony)**: 블로그 글, 컨퍼런스
         submission, sponsor relations — 본 SPEC §4.2 exclusions.

  (D11) **Locale + timing — Korean ops team consideration**:
       - **Tag push window**: KST 영업시간 (09:00-18:00 KST,
         UTC+9 = 00:00-09:00 UTC) 내 manual `git push origin
         v1.0.0`. 한국 시간대의 maintainer (limbowl) 가 zero-day
         incident response 가능한 시점. 자동 schedule 안 함.
       - **CHANGELOG + MIGRATION + GitHub Release body**: 영문이
         authoritative; 한국어 summary collapsible. SPEC-DOC-001
         의 bilingual EN+KO 정책과 정렬.
       - **Tag message + commit message**: 영문 (per `.moai/config/
         sections/language.yaml` `git_commit_messages: en`).

  Companion artifacts:
  - `.moai/specs/SPEC-REL-001/research.md` — Phase 0.5 research
    (12 sections: existing release surface inventory, version
    constant deep-dive, CHANGELOG structure analysis, breaking-
    change closure, release-tooling survey [release-please /
    git-cliff / goreleaser / changesets], signing infrastructure
    options [GPG vs sigstore gitsign vs cosign], artifact channel
    matrix, SLSA / SBOM tool selection, pre-tag verification
    sequencing, OSS reference projects [crush / golangci-lint /
    helm], open risks).
  - `.moai/specs/SPEC-REL-001/plan.md` — DDD phased plan (Sprint
    Contract REQUIRED per harness: thorough).

  18 EARS REQs (12 × P0 + 5 × P1 + 1 × P2) + 7 NFRs + 5 new files
  (`internal/version/version.go`, `CHANGELOG.md` `[1.0.0]` section,
  `MIGRATION.md`, `RELEASE.md`, `.goreleaser.yml`) + 1 new CI
  workflow (`release.yml`) + 3 existing files modified (3 `main.go`
  for ldflags consumption, `README.md` for version badge). Methodology:
  **DDD** (existing release surface consolidation — byte-fidelity
  preservation of Keep-a-Changelog history + conventional-commit
  log + version test regex; IMPROVE with single-source version +
  signed-tag + multi-arch goreleaser + attestations). Coverage
  target 85% applies to `internal/version/` package + release.yml
  shell scripts. Harness: **thorough** (P0 + V1-defining release
  ceremony + cross-SPEC integration with 7 dependency SPECs —
  Sprint Contract MANDATORY per `.claude/rules/moai/design/
  constitution.md` §11). Owner: manager-git.

---

## 1. Overview

SPEC-REL-001은 M9 (V1 release) 의 네 번째이자 terminal SPEC이며,
roadmap §5 M9 exit criteria 전체 ("`v1.0.0` tagged; Helm chart
deployable; docs site live") 의 최종 gating SPEC이다. 본 SPEC은
**새로운 release 시스템을 발명하지 않으며**, 6개 기존 release 자산
(분산된 version constants, Keep-a-Changelog CHANGELOG.md, conventional-
commit history, 부분 CI baseline, `--version` flag + regression
test, Go toolchain pin) 을 (a) single-source consolidation, (b)
goreleaser-shaped automation, (c) SLSA/SBOM/cosign supply-chain
attestation 의 세 축으로 V1 release ceremony 로 묶는다.

### 1.1 What ships

| Layer | Artifact | Purpose |
|-------|----------|---------|
| Code | `internal/version/version.go` (NEW) | Single-source version package consumed by 3 binaries (cmd/usearch, cmd/usearch-api, cmd/usearch-mcp) via ldflags injection per D1 |
| Code | `cmd/usearch/main.go` (MODIFIED) | Replace `const Version = "0.1.0-dev"` literal with `version.Version` reference; preserve REQ-BOOT-012 `--version`/`-v` semantics + `TestVersionFlag` regex compatibility |
| Code | `cmd/usearch-api/main.go` (MODIFIED) | Replace `const version = "0.1.0-dev"` literal with `version.Version` reference; preserve `obs.Init(...ServiceVersion: ...)` integration |
| Code | `cmd/usearch-mcp/main.go` (MODIFIED) | Replace `const version = "0.1.0-dev"` literal with `version.Version` reference; preserve `obs.Init(...ServiceVersion: ...)` integration |
| Docs | `CHANGELOG.md` (MODIFIED at release time, not by this SPEC) | Promote `[Unreleased]` content to `[1.0.0] - 2026-MM-DD` section with Added/Changed/Deprecated/Removed/Fixed/Security 6-section grouping per D2 |
| Docs | `MIGRATION.md` (NEW) | 0.x → 1.0 breaking-change guide with 12 sections per D4 |
| Docs | `RELEASE.md` (NEW) | Maintainer-facing release runbook with pre-tag verification matrix manual checklist + emergency rollback procedure + post-release tasks per D10 |
| Docs | `README.md` (MODIFIED) | Add version badge + install snippet referencing v1.0.0 release URL |
| Build | `.goreleaser.yml` (NEW) | goreleaser v2 config producing linux/darwin × amd64/arm64 × 3 binaries = 12 archives with SHA256SUMS + SBOM per D7 |
| CI | `.github/workflows/release.yml` (NEW) | Tag-trigger workflow executing pre-tag verification matrix (G1..G12) → goreleaser run → SLSA provenance generation → cosign sign-blob → GitHub Release publish with CHANGELOG section extraction as body per D6, D7, D8 |
| Process | Annotated GPG-signed `v1.0.0` git tag | Single canonical release marker with tag message containing CHANGELOG `[1.0.0]` first 30 lines summary + maintainer Signed-off-by per D5 (OPERATIONAL — see below) |

[HARD framing — machinery vs ceremony]: REL-001 ships the release
**machinery + procedure** (the `internal/version/` package, `.go`
refactor, `.goreleaser.yml`, `release.yml` with the G1..G12 pre-tag
matrix, RELEASE.md runbook, MIGRATION.md / CHANGELOG skeletons +
format contract). The **actual `v1.0.0` git tag**, the **CHANGELOG
`[1.0.0]` section body**, and the **MIGRATION per-section breaking-
change content** are OPERATIONAL/post-merge artifacts produced
during the release ceremony **after all 7 dependency PRs merge to
main**. Acceptance at implementation time therefore does NOT require
a live `v1.0.0` tag, a published GitHub Release, or live cross-SPEC
gate PASS (those gates — G5..G9 — resolve post-merge via `gh run`
lookups against the merged dependency workflows). The implementation
acceptance gates (A1..A13, §6) verify the machinery in dry-run /
snapshot / lint modes only.

### 1.2 Motivation

V1 release ceremony 부재는 다음의 운영자-facing failure mode 를 초래
한다 (research §2 grounding):

- **Version drift**: 3개 binary 의 `--version` 출력이 서로 다른 값을
  보고하는 상황 (현재 0.1.0-dev 로 동일하지만, 1.0.0 cut 시 어느 하나
  edit miss 시점에 발생 가능). 운영자 incident triage 시 "어느 버전이
  설치되어 있는가" 질문에 답할 수 없음.
- **Unverifiable supply chain**: cosign signature + SLSA provenance +
  SBOM 없이 ghcr.io image 와 binary 를 pull 하면 supply-chain attack
  surface 노출. SPEC-SEC-001 REQ-SEC-016 의 SLSA L2 commitment 가
  binary 에 적용되지 않으면 incomplete.
- **Untraceable history**: git tag 없이 SHA reference 만 으로 운영자
  rollback 시도. "v1.0.0 의 SHA가 무엇인가" 질문에 답 없음. release
  history 가 commit log 와 동등해지면 changelog 의 의미 손실.
- **Breaking-change blindness**: MIGRATION.md 없으면 1.x → 2.x 시점에
  무엇이 freeze 약속이고 무엇이 free 인지 사용자 + 컨트리뷰터 양측
  모두 불명확. semver 의 약속이 메시지 없는 약속이 됨.
- **Release ceremony drift**: 매 release 마다 ad-hoc procedure 시
  무엇을 verify 했는지 audit 불가. pre-tag verification matrix (G1..
  G12) 가 명문화되지 않으면 어느 PR이 release-ready 인지 판단 기준
  부재.

본 SPEC 이 PASS 해야 하는 이유: roadmap §5 M9 exit criteria 의
"`v1.0.0` tagged" 단독 책임. 본 SPEC 미달성 시 V1 release ceremony
미수행 → 외부 사용자 입장에서 "Universal Search V1" 은 존재하지 않음.
SPEC-DOC-001 docs site 의 install instructions 가 reference 할
`https://github.com/elymas/universal-search/releases/tag/v1.0.0`
URL 이 404. SPEC-DEPLOY-001 chart 의 `appVersion: 1.0.0` claim 이
unbacked.

### 1.3 Forward-compatibility commitments

본 SPEC 은 다음 sibling/upstream SPEC 과의 contract 를 명시한다:

- **SPEC-DOC-001 (M9 sibling, drafted)** — User guide site. 본 SPEC
  release ceremony 가 publish 한 `v1.0.0` artifact URL 을 DOC-001
  `operators/installation.mdx` 가 reference. DOC-001 PASS 가 본 SPEC
  G8 (docs site build + link-check) 의 전제. release.yml 이 DOC-001
  의 link-check 가 v1.0.0 release URL 을 successfully resolve 하는지
  검증.
- **SPEC-DOC-002 (M9 sibling, drafted)** — Adapter reference. 본 SPEC
  release notes (CHANGELOG `[1.0.0]` section) 가 V1 ship adapter
  목록을 enumerate; DOC-002 의 status taxonomy (alpha/beta/stable)
  를 reference 하여 어느 adapter 가 V1 freeze scope (D9) 에 포함되고
  어느 것이 experimental 인지 명시. G9 (drift detection) 의 전제.
- **SPEC-DEPLOY-001 (M9 sibling, drafted)** — Helm chart. 본 SPEC
  은 chart 자체를 build 하지 않고, DEPLOY-001 의 `build-images.yml`
  + `chart-release.yml` 가 produce 한 image + chart artifact 의
  `1.0.0` tag 가 본 SPEC tag push 와 동시에 published 되는지 verify
  (G7). chart appVersion 과 본 SPEC version 의 sync 는 manual
  release ceremony 의 한 step (RELEASE.md).
- **SPEC-SEC-001 (M8 sibling, drafted)** — Security hardening. 본 SPEC
  의 SLSA L2 + cosign + SBOM 정책은 SEC-001 REQ-SEC-016 + D8 와 정렬.
  본 SPEC 은 SEC-001 의 인프라 (security.yml, cosign installer
  pinned version) 를 reuse; 별도 발명 안 함. G5 (security workflow
  PASS) 전제.
- **SPEC-EVAL-001/002/003 (M8 siblings, drafted)** — Evaluation
  benchmarks. 본 SPEC G6 gate 가 EVAL trio 의 PASS 를 요구. citation
  faithfulness ≥ 0.85, adapter dashboard live, Korean benchmark
  sign-off — V1 quality claim 의 evidence.
- **SPEC-BOOT-001 (implemented, M1)** — Repo scaffold. 본 SPEC 의
  Go toolchain version (1.25.x), go.mod path (`github.com/elymas/
  universal-search`), conventional-commit pattern 모두 BOOT-001 의
  자산. 본 SPEC implementation 단계에서 go.mod 의 module path 변경
  없음 (D9 anti-pattern).
- **SPEC-CLI-001 (implemented, M2)** — usearch query subcommand. 본
  SPEC G12 verification 이 `usearch --version` 출력의 semver 가
  `$GITHUB_REF_NAME` (without `v` prefix) 와 일치하는지 assert.
  REQ-BOOT-012 `--version`/`-v` semantics + `TestVersionFlag` regex
  preservation 은 D1 HARD constraint.
- **SPEC-CLI-002 (drafted, M7)** — usearch CLI v1. 본 SPEC freeze
  scope (D9) 에 CLI flag + subcommand + exit code 포함. CLI-002 의
  PASS 이후 본 SPEC release ceremony 진행.
- **SPEC-MCP-001 (drafted, M7)** — MCP server. 본 SPEC freeze scope
  (D9) 에 MCP tool names + schemas 포함. MCP-001 PASS 이후 본 SPEC
  release ceremony.
- **SPEC-SKILL-001 (drafted, M7)** — Claude Skill plugin. 본 SPEC
  freeze scope (D9) 에 Skill manifest schema 포함. SKILL-001 PASS
  이후 본 SPEC release ceremony.
- **SPEC-UI-001 (drafted, M7)** — Web UI v1. 본 SPEC freeze scope
  (D9) 에 `web/` Next.js app 의 URL path + REST endpoint 포함 (UI
  자체가 cmd/usearch-api 를 consume).
- **SPEC-AUTH-001/002/003 (implemented, M6)** — Auth foundation.
  본 SPEC freeze scope (D9) 에 OIDC env-var names, RBAC policy
  schema, audit log row schema 포함.
- **SPEC-IDX-001..005 (implemented, M3/M6)** — Hybrid index +
  multi-tenancy. 본 SPEC freeze scope (D9) 에 database migration
  sequence (0001..0007.sql) 포함; forward-only migration policy
  (D4 §9) 명시.
- **SPEC-CORE-001 (implemented, M2)** — Adapter contract. 본 SPEC
  freeze scope (D9) 에 `pkg/types/Adapter` interface signature 포함.
  CORE-001 변경은 향후 v2.0.0 trigger (D9).
- **SPEC-ADP-001..009 (implemented, M2/M3)** — 9개 adapter. 본 SPEC
  release notes 에서 enumeration; 각 adapter status (alpha/beta/
  stable per DOC-002) 가 V1 freeze scope 결정.

### 1.4 Pinned architectural decisions

HISTORY 의 D1..D11 11개 결정은 §2 requirements 를 bind 하는 constraint
이다. 재논의 대상이 아니며, annotation cycle 에서만 modification 가능.

### 1.5 Registry org resolution + pre-merge operational propagation

본 SPEC 은 registry org 를 **`elymas`** 로 canonical 하게 resolve
한다 — 본 SPEC 내 모든 `ghcr.io/...` reference 는 `ghcr.io/elymas/`
를 사용하며 unresolved `<org>` placeholder 는 **0개**다. pre-tag
matrix 의 registry / cosign verification 은 모두 `ghcr.io/elymas/`
를 target 한다.

[OPERATIONAL — pre-merge, not a REL-001 edit]: 7개 dependency SPEC
(SPEC-DEPLOY-001, SPEC-SEC-001, SPEC-DOC-001, SPEC-DOC-002, SPEC-
EVAL-001/002/003) 은 자신의 deferred publish step 에서 아직
`ghcr.io/<org>/` placeholder 를 carry 한다 (예: DEPLOY-001 의
`oci://ghcr.io/<org>/charts/...`, `ghcr.io/<org>/usearch-api`). 이
placeholder 들을 `elymas` 로 치환하는 것은 **release ceremony 직전
operational step** (각 dependency PR 의 merge 전 수행) 이다. 본
SPEC 은 canonical value (`elymas`) 를 **document 만** 하며, 다른
SPEC 파일을 편집하지 않는다 (scope discipline). G5/G7/G8/G9 gate 는
이 propagation 이 완료되어 `ghcr.io/elymas/` artifact 가 published
된 상태를 전제로 verify 한다.

---

## 2. EARS Requirements

EARS Pattern legend:
- Ubiquitous: "The system shall ..."
- Event-driven: "When <event>, the system shall ..."
- State-driven: "While <condition>, the system shall ..."
- Optional: "Where <feature available>, the system shall ..."
- Unwanted: "If <unwanted>, then the system shall ..."

### 2.1 P0 — Release-blocking (12 REQs)

**REQ-REL-001 [Ubiquitous, P0]** — The repository shall provide a
single-source version package at `internal/version/version.go`
exposing exported variables `Version` (string, default `"0.1.0-
dev"`), `Commit` (string, default `"unknown"`), `BuildDate`
(string, default `"unknown"`), `GoVersion` (string, default
`runtime.Version()`), plus helpers `String()` returning `"usearch
v<Version> (<Commit>, built <BuildDate>, <GoVersion>)"` and
`Short()` returning `Version`. The package shall be consumed by
`cmd/usearch/main.go`, `cmd/usearch-api/main.go`, and `cmd/usearch-
mcp/main.go` replacing the existing per-file `Version`/`version`
constants. Release builds shall override `Version`, `Commit`, and
`BuildDate` via `go build -ldflags "-X github.com/elymas/universal-
search/internal/version.Version=<semver> -X ...Commit=$GITHUB_SHA -
X ...BuildDate=<ISO-8601>"`. [Trace: HISTORY D1, research §3]

**REQ-REL-002 [Ubiquitous, P0]** — The existing
`cmd/usearch/main_test.go` regression tests `TestVersionFlag` and
`TestVersionShortFlag` shall continue to PASS after the REQ-REL-001
refactor. Specifically, `usearch --version` output shall match the
existing semver regex **`^usearch v\d+\.\d+\.\d+`** (the actual
`semverPattern` at `cmd/usearch/main_test.go:12` — prefix-anchored,
NO trailing `$`, NO prerelease capture group) for both the
unstripped development build (`0.1.0-dev`) and the ldflags-injected
release build (`1.0.0`). This is a HARD characterization constraint:
the regex is **preserved as-is**, not tightened — the refactor must
not change `main_test.go`. REQ-BOOT-012 `--version` / `-v` flag
semantics shall be preserved without modification. [Trace: HISTORY
D1 HARD constraint, research §3.4]

**REQ-REL-003 [Ubiquitous, P0]** — The repository shall ship
`CHANGELOG.md` in Keep-a-Changelog v1.1.0 format declaring a
`[1.0.0] - <YYYY-MM-DD>` section consolidating the entire `[Unreleased]`
content into six standard subsections (Added / Changed /
Deprecated / Removed / Fixed / Security). Every SPEC implemented
between M1 (SPEC-BOOT-001) and M9 (this SPEC) inclusive shall
appear at least once in the `[1.0.0]` section with its SPEC ID and
one-line summary. The CHANGELOG footer shall declare `[1.0.0]:
https://github.com/elymas/universal-search/releases/tag/v1.0.0` and
update `[Unreleased]: https://github.com/elymas/universal-search/
compare/v1.0.0...HEAD`. The actual CHANGELOG edit shall be executed
during the release ceremony (not by this SPEC draft per HARD
constraint); this SPEC defines the format and completeness contract
only. [Trace: HISTORY D2, research §4]

**REQ-REL-004 [Ubiquitous, P0]** — The repository shall ship
`MIGRATION.md` at repository root with twelve sections covering:
(§1) Overview of semver 1.0.0 API freeze promise, (§2) CLI
breaking changes, (§3) Config schema breaking changes, (§4) Env
var renames, (§5) MCP protocol surface, (§6) Adapter plugin
contract, (§7) MoAI Skill manifest, (§8) REST/GraphQL endpoint
schema, (§9) Database schema migration policy, (§10) Adapter
status taxonomy reference (cross-link to SPEC-DOC-002), (§11)
Upgrade procedure for Helm / binary / Skill, (§12) Rollback
procedure. Sections with no current known breaking change shall
explicitly state "v1.0.0 — no breaking changes in this category"
rather than omit the section. Each populated entry shall follow
the three-block format: "**0.x state**: …; **1.x state**: …;
**Operator action**: …". [Trace: HISTORY D4, research §4]

**REQ-REL-005 [Ubiquitous, P0]** — The repository shall ship
`RELEASE.md` at repository root as a maintainer-facing release
runbook containing: (a) the pre-tag verification matrix (G1..G12
per HISTORY D6) as a manual checklist mirroring the automated
release.yml gates, (b) the annotated-GPG-signed tag creation
procedure with `git tag -a v<X.Y.Z> -m <message>` and `git push
origin v<X.Y.Z>` example, (c) the emergency rollback procedure
covering tag deletion (`git push --delete origin v<X.Y.Z>`),
GitHub Release retraction, and container image / chart unpublish,
(d) the post-release task checklist covering roadmap.md update,
security advisory channel, and announcement drafting, and (e) the
locale + timing protocol per HISTORY D11 (KST business hours
tag-push window). [Trace: HISTORY D5, D6, D10, D11]

**REQ-REL-006 [Ubiquitous, P0]** — The repository shall ship
`.goreleaser.yml` (goreleaser v2 schema) configuring builds for
the matrix `goos ∈ {linux, darwin}` × `goarch ∈ {amd64, arm64}`
× `binary ∈ {usearch, usearch-api, usearch-mcp}` producing exactly
twelve archives. Each archive shall be named `usearch_${Version}_
${Os}_${Arch}.tar.gz`, contain the single binary plus `LICENSE`
and `README.md`, and be built with `CGO_ENABLED=0` and ldflags
overriding `internal/version.Version`, `Commit`, `BuildDate`. The
config shall produce `SHA256SUMS` checksum file. Windows targets
shall be explicitly excluded with rationale comment referencing
research §5.3. [Trace: HISTORY D7, research §5]

**REQ-REL-007 [Event-driven, P0]** — When a git tag matching the
pattern `v[0-9]+.[0-9]+.[0-9]+` (or its pre-release form `v[0-9]+.
[0-9]+.[0-9]+-[a-zA-Z0-9.-]+`) is pushed to the repository, the
`.github/workflows/release.yml` workflow shall trigger and execute
in order: (1) pre-tag verification matrix gates G1..G12 (REQ-REL-
013) as the `pre-tag-verify` job; (2) `goreleaser release --clean`
in a job that requires `pre-tag-verify`; (3) SLSA provenance
generation via `slsa-framework/slsa-github-generator/.github/
workflows/generator_generic_slsa3.yml@v2.0.0` reusable workflow;
(4) `cosign sign-blob` keyless on each goreleaser archive using
GitHub Actions OIDC identity; (5) GitHub Release creation with
title `v<X.Y.Z>` and body extracted from CHANGELOG.md `[X.Y.Z]`
section via `awk '/^## \[<X.Y.Z>\]/,/^## \[/' CHANGELOG.md | sed
'$d'`, with all goreleaser archives, SHA256SUMS, `*.intoto.jsonl`
provenance, `*.sig`/`*.crt` cosign artifacts, and SPDX SBOM
attached. [Trace: HISTORY D6, D7, D8, research §6]

**REQ-REL-008 [Ubiquitous, P0]** — The release ceremony shall use
an annotated GPG-signed git tag (`git tag -a -s v<X.Y.Z>`) with
tag message format consisting of: a title line `Release v<X.Y.Z>
— Universal Search`, an empty line, the first 30 lines of the
CHANGELOG `[X.Y.Z]` section as summary, an empty line, a
references block listing all M9 dependency SPEC IDs (SPEC-DOC-001,
SPEC-DOC-002, SPEC-DEPLOY-001, SPEC-SEC-001, SPEC-EVAL-001, SPEC-
EVAL-002, SPEC-EVAL-003), and a `Signed-off-by: <maintainer>
<<email>>` trailer. The repository shall enable GitHub branch /
tag protection rules requiring signed tags for the `v*.*.*`
pattern. [Trace: HISTORY D5]

**REQ-REL-009 [State-driven, P0]** — While the release.yml `pre-
tag-verify` job is executing, the workflow shall enforce all
twelve gates G1..G12 in parallel where dependency-free and in
sequence otherwise. Specifically: G1 (go vet + go test -race +
coverage ≥ 85%), G2 (golangci-lint + pre-commit run --all-files),
G3 (LSP zero errors + ≤ 10 warnings per CLAUDE.md sync-phase
thresholds), G4 (deps-audit.yml latest main run PASS), G5
(security.yml latest main run PASS per SPEC-SEC-001), G6 (EVAL
trio PASS: EVAL-001 faithfulness ≥ 0.85, EVAL-002 dashboard live
within 24h, EVAL-003 manual sign-off recorded), G7 (chart-ci.yml
latest main run PASS per SPEC-DEPLOY-001 + Helm chart appVersion
matches tag), G8 (docs.yml latest main run PASS per SPEC-DOC-
001), G9 (adapter drift detection PASS per SPEC-DOC-002), G10 (24h
sustained CI green on go.yml + deps-audit.yml + pre-commit.yml +
web.yml + python.yml), G11 (`git verify-tag $GITHUB_REF_NAME`
exit 0), G12 (`$(./usearch --version | awk '{print $2}' | sed
's/^v//')` equals `${GITHUB_REF_NAME#v}`). If any gate fails,
subsequent release.yml jobs (goreleaser, SLSA, cosign, GitHub
Release publish) shall NOT execute. [Trace: HISTORY D6, research
§7]

**REQ-REL-010 [Ubiquitous, P0]** — The release.yml workflow shall
produce SPDX-format SBOM (Software Bill of Materials) via
`anchore/syft v1.x` for every goreleaser archive, attached to the
GitHub Release as a separate `.spdx.json` file per archive plus
one aggregate `usearch_<Version>.spdx.json`. The SBOM shall include
all direct and transitive Go module dependencies from `go.mod` and
`go.sum`. Container image SBOM (SPEC-DEPLOY-001 build-images.yml
responsibility) and Helm chart SBOM (chart-release.yml
responsibility) shall be cross-referenced from the GitHub Release
body but not regenerated by this workflow. [Trace: HISTORY D8,
research §8]

**REQ-REL-011 [Ubiquitous, P0]** — The release.yml workflow shall
attach SLSA Level 2 provenance attestation for every goreleaser
archive via `slsa-framework/slsa-github-generator/.github/workflows/
generator_generic_slsa3.yml@v2.0.0` reusable workflow (the workflow
name says slsa3 but achieves Level 2 on GitHub-hosted runners per
SPEC-SEC-001 REQ-SEC-016 rationale). The provenance file `multiple.
intoto.jsonl` shall be attached to the GitHub Release. Verification
instructions shall appear in RELEASE.md with `slsa-verifier verify-
artifact --provenance-path multiple.intoto.jsonl --source-uri
github.com/elymas/universal-search --source-tag v<X.Y.Z>
usearch_*.tar.gz` example. [Trace: HISTORY D8, research §8.2]

**REQ-REL-012 [Ubiquitous, P0]** — The release.yml workflow shall
sign every goreleaser archive via `sigstore/cosign-installer@v3.
7.0` keyless `cosign sign-blob --yes <archive>` using GitHub
Actions OIDC identity. Each archive shall produce two companion
artifacts attached to the GitHub Release: `<archive>.sig`
(signature) and `<archive>.crt` (Fulcio-issued ephemeral
certificate). Verification instructions shall appear in RELEASE.md
with the full `cosign verify-blob --certificate <crt> --signature
<sig> --certificate-identity-regexp "https://github.com/elymas/
universal-search/.github/workflows/release.yml@.*" --certificate-
oidc-issuer "https://token.actions.githubusercontent.com"
<archive>` invocation per HISTORY D8. [Trace: HISTORY D8]

### 2.2 P1 — Production-readiness + downstream coordination (5 REQs)

**REQ-REL-013 [Ubiquitous, P1]** — The release.yml workflow shall
emit a structured JSON summary file `release-summary.json` attached
to the GitHub Release containing: tag (e.g. `v1.0.0`), commit SHA,
build timestamp (ISO-8601 UTC), gate-by-gate status (G1..G12 each
with `passed: bool`, `evidence_url: string`), artifact inventory
(twelve archive paths + SHA256 + cosign sig path + SLSA provenance
path + SBOM path), dependency SPEC PASS evidence (DOC-001 / DOC-002
/ DEPLOY-001 / SEC-001 / EVAL-001 / EVAL-002 / EVAL-003 each with
workflow run URL), and a `release_ceremony_runtime_seconds`
duration metric. The summary file shall be a machine-readable
audit trail consumed by post-V1 release retrospective tooling.
[Trace: HISTORY D6, D10]

**REQ-REL-014 [Event-driven, P1]** — When the release.yml workflow
completes successfully (GitHub Release published), the workflow
shall open a pull request against `main` modifying `.moai/project/
roadmap.md` to mark the M9 section as "✅ shipped <YYYY-MM-DD>"
with the v1.0.0 tag URL and adding a placeholder M10 section
header (no content). The PR shall be authored by a bot (e.g.
`github-actions[bot]`) and request review from the repository
maintainer. The PR shall NOT be auto-merged. [Trace: HISTORY D10]

**REQ-REL-015 [Optional, P1]** — Where the maintainer environment
includes the `gh` CLI and a configured webhook URL in repository
secrets (`RELEASE_NOTIFICATION_WEBHOOK`), the release.yml workflow
shall send a notification with payload `{tag, release_url,
artifacts_count, gate_summary}` after successful publish. If the
secret is absent, the workflow shall skip notification with a slog
INFO line. [Trace: HISTORY D10]

**REQ-REL-016 [Ubiquitous, P1]** — The repository shall declare in
`MIGRATION.md` §1 (Overview) the explicit semver 1.x freeze scope
per HISTORY D9 listing: CLI commands, CLI flag names + semantics,
exit codes, MCP protocol tool names + schemas, MoAI Skill manifest
schema, adapter plugin interface (`pkg/types/Adapter`,
`Capabilities`), REST endpoint paths + response schemaVersion,
`.moai/config/sections/*.yaml` schema, env-var names, K8s Helm
chart values.schema.json. The free zone shall also be explicit
listing: `internal/` Go packages, alpha/beta adapters, AI prompt
template content, internal metric label values, Python sidecar
internal API. Each freeze item shall name the canonical source
file or SPEC ID defining its current state at v1.0.0 cut. [Trace:
HISTORY D9]

**REQ-REL-017 [Ubiquitous, P1]** — The release.yml workflow shall
verify, as part of gate G7, that the container images **actually
built by SPEC-DEPLOY-001 REQ-DEPLOY-018** exist with valid cosign
signatures. DEPLOY-001 produces exactly three Go images from
`Dockerfile.usearch-{api,mcp,migrate}`:
`ghcr.io/elymas/usearch-api:1.0.0` and
`ghcr.io/elymas/usearch-mcp:1.0.0` (the primary application images)
plus `ghcr.io/elymas/usearch-migrate:1.0.0` (the migration job
image). There is **no `ghcr.io/elymas/universal-search` application
image** — `universal-search` is the Helm chart name only (see
below), so G7 MUST NOT verify a `universal-search` image (doing so
would hard-fail on a nonexistent artifact). G7 shall additionally
verify the Helm chart at `oci://ghcr.io/elymas/charts/universal-
search:1.0.0` (SPEC-DEPLOY-001 REQ-DEPLOY-017 output) exists with
`Chart.yaml` `appVersion: 1.0.0` matching the git tag. Of the three
images, `usearch-api` and `usearch-mcp` are the must-verify primary
app images; `usearch-migrate` is verified when present (it is a
DEPLOY-001 deliverable). Verification failure on a required image or
the chart shall fail G7 and abort the release. [Trace: HISTORY D7,
DEPLOY-001 REQ-DEPLOY-017/018 Dockerfile set]

### 2.3 P2 — Forward-compatibility (1 REQ)

**REQ-REL-018 [Optional, P2]** — Where the release is a pre-release
(tag matching `v[0-9]+.[0-9]+.[0-9]+-[a-zA-Z0-9.-]+` pattern, e.g.
`v1.0.0-rc1`), the release.yml workflow shall set the GitHub
Release `prerelease: true` flag, prepend the release notes body
with a warning banner "**This is a pre-release. Do not deploy to
production.**", and skip REQ-REL-014 (roadmap.md PR creation).
Full-release post-publish tasks shall execute only on stable tag
patterns. [Trace: HISTORY D11]

---

## 3. Non-Functional Requirements

**NFR-REL-001 [Release workflow runtime budget]** — The
`.github/workflows/release.yml` workflow shall complete end-to-end
(from tag push to GitHub Release published) within 30 minutes
wall-clock on `ubuntu-24.04` hosted runners for the median release
(no first-time goreleaser cache warming, all gates green on first
attempt). Pre-tag verification matrix (G1..G12) shall complete
within 15 minutes; goreleaser build + sign + publish shall
complete within 15 minutes. Hard ceiling: 60 minutes (workflow
timeout setting).

**NFR-REL-002 [Version-injection determinism]** — Two release
builds of the same git tag from the same SHA shall produce byte-
identical `usearch` binaries except for the `BuildDate` field
(ISO-8601 timestamp varies). The `Version` and `Commit` fields
shall be byte-identical. Verified via CI step `diff <(./usearch
--version | grep -v "built") <(<previous build> --version | grep
-v "built")` after re-run.

**NFR-REL-003 [Tag immutability]** — Once `v1.0.0` is pushed and
the GitHub Release is published, the tag SHALL NOT be force-
overwritten. Emergency tag deletion (RELEASE.md procedure)
requires explicit maintainer sign-off with audit log entry in
`ops/release-incidents.md`. Re-release after deletion uses a new
patch version (e.g. `v1.0.1`), never the same tag.

**NFR-REL-004 [Verification command reproducibility]** — All
verification commands documented in RELEASE.md (cosign verify-
blob, slsa-verifier verify-artifact, `git verify-tag`) shall be
exactly reproducible by a fresh operator clone on Linux, macOS,
and Windows (operator-side verification only; release-side build
excludes Windows). The commands shall not require any local secret
or token (keyless cosign + public Fulcio + public Rekor).

**NFR-REL-005 [CHANGELOG completeness]** — The `[1.0.0]` section
shall reference every SPEC implemented in M1..M9 at least once.
Verified via release.yml step `grep -c "SPEC-" CHANGELOG.md`
between the `## [1.0.0]` line and the next `## [` line, asserting
the count equals or exceeds the number of SPEC directories under
`.moai/specs/` whose `spec.md` frontmatter has `status: implemented`
(extracted via shell). Missing SPEC IDs fail G1.

**NFR-REL-006 [Pre-tag dry-run support]** — The release.yml
workflow shall support a `workflow_dispatch` manual trigger with
input `dry_run: true` executing all gates G1..G12 and goreleaser
build (but NOT cosign sign, NOT SLSA generation, NOT GitHub Release
publish, NOT tag verification). This enables release-readiness
audit prior to actual tag push. Dry-run shall not modify any
external state (no PR creation, no notification).

**NFR-REL-007 [Maintainer time burden]** — From tag push to
GitHub Release published, the maintainer manual action shall be
limited to: (1) `git tag -a -s v<X.Y.Z> -m <message>`, (2) `git
push origin v<X.Y.Z>`, (3) review the auto-generated roadmap.md
PR within 24h. Total maintainer hands-on time ≤ 10 minutes
assuming all gates green. Gate failures require additional triage
time documented in RELEASE.md emergency procedure.

---

## 4. Scope Boundary

### 4.1 In Scope

- `internal/version/version.go` single-source version package
- 3-binary `main.go` modification to consume version package via
  ldflags injection
- CHANGELOG.md `[1.0.0]` section format + completeness contract
  (actual edit deferred to release ceremony per HARD constraint)
- MIGRATION.md 12-section structure (full content authored at
  release ceremony)
- RELEASE.md maintainer-facing release runbook
- README.md version badge + install snippet (additive edit)
- `.goreleaser.yml` for 3 binaries × linux/darwin × amd64/arm64
- `.github/workflows/release.yml` with pre-tag verification matrix
  (G1..G12) + goreleaser + SLSA + cosign + SBOM + GitHub Release
  publish
- Annotated GPG-signed git tag protocol
- Post-release tasks: roadmap.md PR, CHANGELOG footer update
- Cross-SPEC verification of DEPLOY-001 + SEC-001 + DOC-001 +
  DOC-002 + EVAL-001/002/003 PASS at G6/G7/G8/G9

### 4.2 Exclusions (What NOT to Build) [HARD]

본 SPEC scope **밖**의 항목 — 명시적으로 다른 SPEC 또는 post-V1로
deferred:

- **CHANGELOG.md `[1.0.0]` section actual content edit** — 본 SPEC
  은 format + 6-section grouping + per-SPEC enumeration contract만
  명세. 실제 편집은 release ceremony 시 manager-git 이 수행. 본
  draft는 release process specification 이지 release execution 이
  아니다 (HARD per user brief).
- **Actual git tag creation + push** — 본 SPEC draft는 release
  process 정의. tag push 는 release ceremony 시 maintainer manual
  action.
- **MIGRATION.md actual breaking-change content** — 본 SPEC은 12-
  section structure 만 명세. 각 섹션 의 실제 0.x → 1.x diff
  content 는 release ceremony 시 작성 (현재 known breaking change
  목록 본 SPEC HISTORY D4 enumerate).
- **Marketing announcement copy, blog post drafts, conference
  submissions, sponsor relations** — V1 scope 밖. roadmap §6
  post-V1 backlog 후보.
- **Homebrew tap / apt / yum / Snap / AUR / Chocolatey packages**
  — V1은 GitHub Release archive + ghcr.io image + OCI chart 의
  3-channel 만 ship. OS-specific package manager 는 community-
  driven post-V1.
- **Windows binary** — V1 은 linux/darwin 만. Windows 사용자는 WSL
  + linux/amd64 binary 사용 권장 (research §5.3 rationale).
- **Long-term support (LTS) commitment** — V1은 1.x.y 시리즈
  semver discipline 만 commit. specific LTS window (예: "v1.x
  security patches until <date>") 는 V1 ship 후 사용자 기반 확보
  + maintainer resource 평가 후 별도 SPEC.
- **Security patch cadence guarantee** — V1은 best-effort. SPEC-
  SEC-001 NFR-SEC-002 의 MTTR target (CRITICAL ≤ 7일, HIGH ≤ 30
  일) 가 effective policy; 본 SPEC 은 그 policy를 reference 만.
- **Release-please / git-cliff full automation migration** — V1
  은 hand-curated CHANGELOG (D2 rationale). V1.0.1 patch 부터의
  hybrid 자동화는 post-V1 별도 SPEC.
- **Sigstore gitsign tag signing** — V1 은 GPG (D5 rationale).
  post-V1 별도 SPEC 가능.
- **Container image별 standalone release notes** — image-specific
  release notes 는 ghcr.io 의 image description 으로 충분; 별도
  artifact ship 안 함.
- **Helm chart standalone CHANGELOG** — SPEC-DEPLOY-001 NFR-DEPLOY-
  005 가 `charts/universal-search/CHANGELOG.md` 정의; 본 SPEC 은
  cross-link 만.
- **Backport policy 1.x to 0.x branches** — 0.x branches 존재
  안 함 (zero tags). 적용 불가.
- **Semver pre-release escalation (alpha → beta → rc → stable)
  ladder** — V1 은 직접 `v1.0.0` 으로 cut. 필요 시 pre-release
  `v1.0.0-rc1` 가능 (REQ-REL-018) 하나 권장 아님 (V1 entire
  M1..M9 가 effectively pre-release period).
- **Release notes translation to additional languages beyond EN +
  KO summary** — V1 은 EN authoritative + KO collapsible summary
  만 (HISTORY D3 + DOC-001 bilingual policy).
- **Tag protection rules code (GitHub branch protection API
  automation)** — 본 SPEC 은 protection rule 활성 의 manual
  procedure 만 RELEASE.md 에 documented. terraform / pulumi 자동화
  는 post-V1.
- **`go install github.com/elymas/universal-search/cmd/usearch@v1.0.
  0` 동작 검증** — Go convention 으로 자동 동작 가정; 별도 verification
  script 작성 안 함 (G12 가 release.yml 내 빌드된 binary 만 검증).
- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). M9
  release SPEC 패턴 (DOC-001 / DOC-002 / DEPLOY-001 / SEC-001 와
  동일).

### 4.3 Deferred to Post-V1

- LTS commitment + security patch cadence guarantee
- Homebrew / apt / yum / Snap / AUR / Chocolatey 패키지 분배
- Windows binary support
- Sigstore gitsign tag signing 전환
- Release-please / git-cliff full automation
- Tag protection rules automation (Terraform / Pulumi)
- Multi-language release notes (EN + KO 외 추가)
- Marketing 채널 + 블로그 + 컨퍼런스 announcement coordination
- SLSA Level 3 (isolated builder) — GitHub Actions hosted runner
  한계, SPEC-SEC-001 rationale 와 동일

---

## 5. Test Scenarios (Given-When-Then)

본 섹션은 evaluator-active 와의 Sprint Contract 협상 시 test
scenarios 의 ground truth. 12 시나리오 (S1..S12).

### S1 — `internal/version/` package consumes ldflags injection

**Given** the `internal/version/version.go` package with default
`Version = "0.1.0-dev"`.
**When** the build command `go build -ldflags "-X github.com/
elymas/universal-search/internal/version.Version=1.0.0 -X github.
com/elymas/universal-search/internal/version.Commit=abc123 -X
github.com/elymas/universal-search/internal/version.BuildDate=
2026-05-22T12:00:00Z" ./cmd/usearch` is executed.
**Then** the resulting binary's `--version` output contains
`usearch v1.0.0` matching the existing semver regex, and
`internal/version.Commit` returns `"abc123"`.
[REQ-REL-001, REQ-REL-002]

### S2 — Existing `TestVersionFlag` continues to PASS

**Given** the refactored `cmd/usearch/main.go` consuming
`internal/version.Version` instead of a local literal.
**When** `go test -run TestVersionFlag ./cmd/usearch/...` is
executed without ldflags injection (default `0.1.0-dev`).
**Then** the test passes, asserting `usearch --version` output
matches the actual `semverPattern` `^usearch v\d+\.\d+\.\d+`
(prefix-anchored, no trailing `$`, no prerelease group — per
`cmd/usearch/main_test.go:12`). [REQ-REL-002, HARD characterization]

### S3 — Three binaries report identical version

**Given** ldflags injection of `Version=1.0.0 Commit=abc123` for
all three binaries built in the same goreleaser run.
**When** `./usearch --version`, `./usearch-api --version-or-equivalent`,
`./usearch-mcp --version-or-equivalent` are invoked.
**Then** all three outputs report `1.0.0` and `abc123` consistently
(no drift). [REQ-REL-001]

### S4 — CHANGELOG `[1.0.0]` section completeness

**Given** the released CHANGELOG.md with `[1.0.0] - YYYY-MM-DD`
section.
**When** the verification step `grep -c "SPEC-" CHANGELOG.md`
extracts SPEC ID count between `## [1.0.0]` and next `## [` line.
**Then** the count is ≥ number of SPEC directories under `.moai/
specs/` with `status: implemented` in spec.md frontmatter.
[REQ-REL-003, NFR-REL-005]

### S5 — MIGRATION.md 12-section structure

**Given** the released `MIGRATION.md`.
**When** structural validation script extracts all `^## ` headers.
**Then** exactly 12 sections appear in the documented order (§1
Overview through §12 Rollback procedure) per REQ-REL-004.
[REQ-REL-004]

### S6 — Goreleaser produces 12 archives

**Given** `.goreleaser.yml` configured per REQ-REL-006 and a `v1.
0.0` tag.
**When** `goreleaser release --clean --skip=publish` is invoked
locally.
**Then** `dist/` directory contains exactly 12 archive files
matching the `usearch_1.0.0_{linux,darwin}_{amd64,arm64}.tar.gz`
naming pattern plus `SHA256SUMS` and SBOM files.
[REQ-REL-006]

### S7 — Pre-tag verification matrix gate failure aborts release

**Given** `release.yml` workflow triggered by tag push with G6
(EVAL trio) failing (e.g. EVAL-001 faithfulness reports 0.82,
below 0.85 threshold).
**When** workflow executes.
**Then** `pre-tag-verify` job fails at G6, subsequent jobs
(goreleaser, SLSA, cosign, GitHub Release) do not execute, no
GitHub Release is created. [REQ-REL-007, REQ-REL-009]

### S8 — Successful release publishes GitHub Release with
CHANGELOG extraction

**Given** all G1..G12 gates PASS and `goreleaser release` succeeds.
**When** the GitHub Release publish step executes.
**Then** a GitHub Release with title `v1.0.0` is created with
body containing the verbatim CHANGELOG `[1.0.0]` section content,
all 12 archives attached, `SHA256SUMS` attached, `*.intoto.jsonl`
provenance attached, `*.sig` + `*.crt` cosign artifacts attached,
SPDX SBOM files attached. [REQ-REL-007, REQ-REL-010, REQ-REL-011,
REQ-REL-012]

### S9 — Cosign verify-blob succeeds for released archive

**Given** the published GitHub Release with `usearch_1.0.0_linux_
amd64.tar.gz` + `.sig` + `.crt`.
**When** a fresh operator runs `cosign verify-blob --certificate
usearch_1.0.0_linux_amd64.tar.gz.crt --signature usearch_1.0.0_
linux_amd64.tar.gz.sig --certificate-identity-regexp "https://
github.com/elymas/universal-search/.github/workflows/release.yml
@.*" --certificate-oidc-issuer "https://token.actions.githubuserco
ntent.com" usearch_1.0.0_linux_amd64.tar.gz`.
**Then** verification succeeds, exit code 0. [REQ-REL-012, NFR-
REL-004]

### S10 — SLSA verifier confirms provenance

**Given** the published GitHub Release with `multiple.intoto.
jsonl` provenance.
**When** a fresh operator runs `slsa-verifier verify-artifact
--provenance-path multiple.intoto.jsonl --source-uri github.com/
elymas/universal-search --source-tag v1.0.0 usearch_1.0.0_linux_
amd64.tar.gz`.
**Then** verification succeeds, exit code 0. [REQ-REL-011, NFR-
REL-004]

### S11 — Cross-SPEC verification at G7

**Given** SPEC-DEPLOY-001 build-images.yml has published the real
app images `ghcr.io/elymas/usearch-api:1.0.0` +
`ghcr.io/elymas/usearch-mcp:1.0.0` (+ `usearch-migrate:1.0.0`) with
valid cosign signatures, and chart-release.yml has published
`oci://ghcr.io/elymas/charts/universal-search:1.0.0` with
`appVersion: 1.0.0`.
**When** release.yml G7 step executes verification.
**Then** verification succeeds; if a required app image
(`usearch-api` / `usearch-mcp`) is missing, or the chart appVersion
mismatches the tag, G7 fails. G7 does NOT verify any
`universal-search` app image (none is built). [REQ-REL-017]

### S12 — Dry-run mode does not publish

**Given** release.yml `workflow_dispatch` invoked with input
`dry_run: true`.
**When** workflow executes.
**Then** all G1..G12 gates run + goreleaser build runs + dist/
archives are generated as workflow artifacts, but cosign sign /
SLSA generation / GitHub Release publish / roadmap.md PR
creation / notification webhook are all skipped. [NFR-REL-006]

---

## 6. Acceptance Gates

본 SPEC 은 다음 acceptance gate 모두 PASS 시 release-ready:

| Gate | Verification | Threshold |
|------|--------------|-----------|
| **A1** Single-source version package compiles | `go build ./internal/version/...` | exit 0 |
| **A2** Existing version regression tests PASS | `go test -run TestVersion ./cmd/usearch/...` | exit 0 |
| **A3** All 3 binaries build with ldflags | manual `go build -ldflags "..." ./cmd/{usearch,usearch-api,usearch-mcp}` | 3 binaries, all `--version` consistent |
| **A4** Goreleaser config validates | `goreleaser check` | exit 0 |
| **A5** Goreleaser dry-run produces 12 archives | `goreleaser release --snapshot --clean` | `dist/` contains 12 archives + checksums + SBOM |
| **A6** Release workflow YAML valid | `actionlint .github/workflows/release.yml` | exit 0, no errors |
| **A7** Pre-tag verification matrix YAML valid | inspect `pre-tag-verify` job structure | all 12 gates declared |
| **A8** RELEASE.md has all 5 sections | manual review | A..E sections per REQ-REL-005 |
| **A9** MIGRATION.md has 12 sections | structural lint | §1..§12 headers exist |
| **A10** Cross-SPEC verification configured | release.yml G7/G8/G9 references DEPLOY-001/DOC-001/DOC-002 workflow run URLs | manual review |
| **A11** TRUST 5 — Tested | unit tests on `internal/version/` + release.yml shell script tests | ≥ 85% coverage |
| **A12** TRUST 5 — Secured | gitleaks + Trivy on workflow YAML | zero finding |
| **A13** TRUST 5 — Trackable | conventional commits + SPEC reference | every PR cites SPEC-REL-001 |

---

## 7. Risks + Mitigations

| ID | Risk | Likelihood | Impact | Mitigation |
|----|------|-----------|--------|-----------|
| R1 | Dependency SPEC delay cascade (DOC-001 / DOC-002 / DEPLOY-001 / SEC-001 / EVAL-trio 중 1개라도 PASS 못 함) | High | Critical | G6/G7/G8/G9 가 어느 dependency 라도 fail 시 release 차단; manager-spec + manager-strategy 가 weekly status report 로 dependency PASS forecast 추적. |
| R2 | GPG signing key 손실 또는 maintainer 환경 사고 | Low | Critical | RELEASE.md emergency 절차에서 backup key + recovery 절차 명시. tag protection rules 가 unsigned tag push 차단 — 사고 시 release 완전 차단 (fail-safe). |
| R3 | Sigstore Rekor transparency log downtime mid-release | Low | High | cosign sign-blob 가 Rekor unavailable 시 retry; release.yml `cosign-sign` step `retry: 3`. Rekor downtime > 1h 시 manual delay decision (RELEASE.md procedure). |
| R4 | Last-minute API surface discovery (V1 freeze scope 누락) | Medium | High | MIGRATION.md §1 freeze scope enumeration 이 sanity check; PR review checklist 에 freeze scope 변경 cross-link 강제. RELEASE.md 의 maintainer dry-run 단계 (NFR-REL-006) 가 freeze scope 점검 시점. |
| R5 | Supply-chain attack on release pipeline (compromised GitHub Actions runner) | Low | Critical | SLSA L2 가 builder identity 증명; SPEC-SEC-001 SECURITY.md 가 reporter contact. 사고 detection 시 GitHub Release retraction + tag rotation. |
| R6 | KST 영업시간 tag push window 외 emergency hotfix 필요 | Medium | Medium | RELEASE.md 가 emergency hotfix 절차 (UTC 시점 무관) + on-call documentation. 단 V1 cut 자체는 KST 영업시간 강제 (D11). |
| R7 | ldflags injection 실패 시 binary 가 `0.1.0-dev` 보고 | Medium | High | G12 verification 이 `usearch --version` 출력 vs tag 일치 강제. release.yml 의 goreleaser step 후 자동 grep 검증. |
| R8 | CHANGELOG `[1.0.0]` section 누락 SPEC ID | Medium | Medium | NFR-REL-005 의 자동 count 검증이 G1 의 일부. missing SPEC ID enumeration 실패 시 release 차단. |
| R9 | Helm chart appVersion ≠ git tag drift | Medium | High | G7 verification 이 chart `appVersion` 과 `$GITHUB_REF_NAME` 일치 확인. DEPLOY-001 chart-release.yml 이 자동 sync 책임. drift 시 release 차단. |
| R10 | Cosign / SLSA tool version drift mid-release | Low | Medium | release.yml 의 `cosign-installer@v3.7.0` + `slsa-github-generator@v2.0.0` pinned. quarterly maintainer audit. |
| R11 | Roadmap.md auto-PR 실패 (gh CLI 인증 만료, etc.) | Low | Low | REQ-REL-014 step 의 error 가 release publish 자체 차단 안 함 (post-publish task). RELEASE.md 의 manual fallback. |
| R12 | EVAL-003 Korean benchmark manual sign-off 의 in-band delay | Medium | High | EVAL-003 owner 가 V1 ship 시점 1주 전 final manual scoring 완료. 본 SPEC RELEASE.md timeline section 에 명시. |

---

## 8. Open Questions (for plan-auditor / annotation cycle)

본 SPEC draft 가 implementation 전 해소해야 할 open question:

- **OQ1** — `internal/version/` package 의 정확한 변수 명명 +
  exposure level. 현재 spec 은 `Version` (exported) 만 제시; `Commit`,
  `BuildDate`, `GoVersion` 도 exported 여야 하는지 (외부 패키지에서
  read 필요 여부) 미확정. **Mitigation**: run phase ANALYZE 에서
  현재 obs.Init 의 `ServiceVersion` field 외에 다른 consumer 가
  있는지 grep 으로 결정.

- **OQ2** — `cmd/usearch-api` 및 `cmd/usearch-mcp` 의 `--version`
  CLI 플래그 부재. 현재 두 binary 는 `obs.Init(ServiceVersion: ...)`
  로 internal 사용만 있고, 운영자가 `usearch-api --version` 입력 시
  의도된 behavior 미정. **Mitigation**: 본 SPEC 에서 두 binary 에도
  `--version` flag 추가 (REQ-REL-001 scope 확장) vs 추가하지 않음
  (현 상태 유지). plan-auditor 결정 필요.

- **OQ3** — Pre-release `v1.0.0-rc1` 사용 여부. 본 SPEC REQ-REL-018
  은 pre-release 지원 명시; 그러나 V1 cut 시 실제 사용 vs 직접
  `v1.0.0` cut 결정. **Mitigation**: RELEASE.md 의 timeline section
  에서 maintainer 의사결정.

- **OQ4** — CHANGELOG `[1.0.0]` section 의 release date 결정 시점.
  D2 의 hand-curated rationale 상 tag push 시점에 확정; 그러나
  release.yml 의 CHANGELOG extraction 이 정확한 section header 필요.
  **Mitigation**: release ceremony 직전 CHANGELOG PR 에서 date
  fix; release.yml 의 extraction regex 가 `## \[1\.0\.0\] -
  [0-9]{4}-[0-9]{2}-[0-9]{2}` pattern 으로 lenient match.

- **OQ5** — `slsa-github-generator` reusable workflow 의 정확한
  binary target 지정 방식. v2.0.0 의 input schema 는 goreleaser
  output directory 또는 explicit file list 지원; 본 SPEC 은
  `dist/` directory 전체 가정. **Mitigation**: run phase 의 첫
  task 에서 generator workflow inputs 확정 (research §8.2 follow-
  up).

- **OQ6** — `release-summary.json` (REQ-REL-013) 의 schema 가 본
  SPEC 외부에 consumer 없음. **Mitigation**: 본 SPEC 단독 정의;
  post-V1 별도 SPEC 에서 retrospective tooling 가 consume 시 schema
  evolution 협의.

- **OQ7** — Multiple release-time KST timezone 검증의 자동화 vs
  manual. D11 의 영업시간 protocol 은 maintainer self-discipline
  으로 충분 vs CI 가 시간대 검증 차단. **Mitigation**: RELEASE.md
  의 procedural 항목 만; 자동 차단 안 함 (over-engineering 우려).

- **OQ8** — README.md version badge 의 정확한 markdown 형식. shields.
  io GitHub release badge 가 표준이나, 본 프로젝트 의 다른 badge
  policy 미정. **Mitigation**: README.md PR review 시 결정;
  본 SPEC 은 "badge 추가" 만 명시.

이 항목들은 plan-auditor PASS 를 차단하지 않는다 — known unresolved
scope edges 로 rationale 과 함께 tagged.

---

## 9. References

External (research.md §12 cited):

- Keep a Changelog v1.1.0: https://keepachangelog.com/en/1.1.0/
- Semantic Versioning 2.0.0: https://semver.org/spec/v2.0.0.html
- goreleaser v2 docs: https://goreleaser.com/
- SLSA Framework: https://slsa.dev/spec/v1.0/
- slsa-github-generator: https://github.com/slsa-framework/slsa-github-generator
- Sigstore Cosign: https://docs.sigstore.dev/cosign/overview/
- Sigstore Rekor: https://docs.sigstore.dev/rekor/overview/
- Sigstore Fulcio: https://docs.sigstore.dev/fulcio/overview/
- Anchore Syft (SBOM): https://github.com/anchore/syft
- SPDX format: https://spdx.dev/specifications/
- Conventional Commits 1.0.0: https://www.conventionalcommits.org/en/v1.0.0/
- release-please (rejected for V1): https://github.com/googleapis/release-please
- git-cliff (rejected for V1): https://github.com/orhun/git-cliff
- changesets (rejected for V1): https://github.com/changesets/changesets
- GPG signing for git tags: https://docs.github.com/en/authentication/managing-commit-signature-verification

Internal (project files):

- `.moai/project/product.md` §1 (auditable self-hosted positioning),
  §8 (Apache-2.0 license)
- `.moai/project/roadmap.md` §M9 SPEC-REL-001 row + §5 M9 exit
  criteria
- `.moai/project/tech.md` (forbidden libraries, Go toolchain pin)
- `.claude/rules/moai/core/moai-constitution.md` (TRUST 5 Trackable
  pillar — conventional commits, SPEC reference)
- `.claude/rules/moai/design/constitution.md` §11 (Sprint Contract
  required for thorough harness)
- `.moai/specs/SPEC-DOC-001/spec.md` (M9 user guide — release link
  consumer)
- `.moai/specs/SPEC-DOC-002/spec.md` (M9 adapter reference — status
  taxonomy source for V1 freeze scope)
- `.moai/specs/SPEC-DEPLOY-001/spec.md` REQ-DEPLOY-017 + REQ-DEPLOY-
  018 (chart + image release coordination)
- `.moai/specs/SPEC-SEC-001/spec.md` REQ-SEC-016 (SLSA L2 + cosign
  policy source)
- `.moai/specs/SPEC-EVAL-001/spec.md` (faithfulness CI gate evidence)
- `.moai/specs/SPEC-EVAL-002/spec.md` (adapter dashboard live
  evidence)
- `.moai/specs/SPEC-EVAL-003/spec.md` (Korean benchmark manual
  sign-off evidence)
- `.moai/specs/SPEC-BOOT-001/spec.md` REQ-BOOT-012 (`--version` /
  `-v` flag semantics — HARD preservation constraint)
- `.moai/specs/SPEC-CLI-001/spec.md` (subcommand dispatcher
  pattern — freeze scope source)
- `.moai/specs/SPEC-CLI-002/spec.md` (CLI v1 — freeze scope)
- `.moai/specs/SPEC-MCP-001/spec.md` (MCP protocol — freeze scope)
- `.moai/specs/SPEC-SKILL-001/spec.md` (Skill manifest — freeze
  scope)
- `.moai/specs/SPEC-CORE-001/spec.md` (adapter contract interface
  — freeze scope)
- `cmd/usearch/main.go:14` (existing `const Version = "0.1.0-
  dev"` — single-source consolidation source)
- `cmd/usearch/main.go:16-20` (`main()` → cobra `newRootCmd` /
  `runCobra`; `--version` / `-v` handled inside cobra root)
- `cmd/usearch-api/main.go:20-30` (existing `const version` at :20 +
  `obs.Init` ServiceVersion consumer at :27)
- `cmd/usearch-mcp/main.go:19-30` (existing `const version` at :19 +
  `--version` flag print at :28 + `obs.Init` consumer)
- `cmd/usearch/main_test.go:12` (`semverPattern` =
  `^usearch v\d+\.\d+\.\d+`) + `:16-` `TestVersionFlag` + `:34-`
  `TestVersionShortFlag` — HARD preservation
- `CHANGELOG.md` (Keep-a-Changelog v1.1.0 — format source)
- `.github/workflows/go.yml` (existing CI baseline)
- `.github/workflows/deps-audit.yml` (existing dependency audit
  baseline — G4 source)
- `go.mod` (Go toolchain 1.25.x pin source)

---

*End of SPEC-REL-001 v0.1.0 (draft).*
