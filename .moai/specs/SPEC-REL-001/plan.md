# SPEC-REL-001 Plan — phased implementation

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: **DDD** (ANALYZE-PRESERVE-IMPROVE per `.claude/rules/
moai/workflow/workflow-modes.md`). DDD-mode justification: 본 SPEC
은 **기존 release 자산을 consolidate + extract + automate** 하는
작업이 본질이며 (3개 분산 `Version`/`version` 상수 → `internal/
version/`, hand-curated CHANGELOG → KaC + Section grouping 보강,
ad-hoc release procedure → release.yml + RELEASE.md runbook), 신규
release 시스템 발명이 아니다. ANALYZE 단계에서 현 release surface
inventory 정확히 capture; PRESERVE 단계에서 characterization test
로 behavior 동일성 보장 (특히 `TestVersionFlag` 의 실제 regex
`^usearch v\d+\.\d+\.\d+` [cmd/usearch/main_test.go:12] + KaC v1.1.0
format); IMPROVE 단계에서 single-source version + goreleaser +
SLSA + cosign + SBOM 통합. 신규 코드 (release.yml shell scripts,
`internal/version/` 패키지) 는 TDD 하위 cycle 로 실행.

Coverage target: 85% (per spec.md frontmatter)
Harness: **thorough** (per `.moai/config/sections/harness.yaml` —
**P0 + V1-defining release ceremony + cross-SPEC integration with
7 dependencies 는 thorough 강제**; Sprint Contract MANDATORY per
`.claude/rules/moai/design/constitution.md` §11 "Sprint Contracts
are required when harness level is `thorough`")

본 plan 은 SPEC-REL-001 구현을 priority-ordered phases 로 sequence
한다. `.claude/rules/moai/core/agent-common-protocol.md` 시간 예측
금지 — phase 는 priority + ordering 만 사용.

---

## 1. Implementation principle

본 SPEC 의 plan philosophy 5축:

1. **Risk-burndown ordering** — 가장 위험한 gap (version drift —
   1.0.0 cut 시 3개 binary 가 서로 다른 version 보고 가능) 부터
   해결. `internal/version/` consolidation 이 첫 phase.
2. **DDD characterization-first** — `cmd/usearch/main_test.go` 의
   `TestVersionFlag` 가 byte-fidelity HARD preservation 대상. 실제
   semver regex `^usearch v\d+\.\d+\.\d+` (prefix-anchored, NO `$`,
   NO prerelease group — `main_test.go:12`) 가 refactor 전후
   unchanged passing (regex 는 preserve only — 강화 안 함).
   `--version` / `-v` semantics 유지.
3. **Dependency-gate awareness** — 본 SPEC PASS 는 7개 dependency
   SPEC (DOC-001, DOC-002, DEPLOY-001, SEC-001, EVAL-001/002/003)
   PASS 조건부. plan 의 Phase 5 (release.yml authoring) 는
   dependency PASS 시점 확인 후 시작. dependency 지연 시 본 SPEC
   stall (R1).
4. **Non-rollbackable awareness** — V1 tag 는 한 번 push 후 mutation
   금지 (NFR-REL-003). 매 phase exit gate 에 plan-auditor 또는
   manual sign-off 요구 (특히 release.yml authoring + RELEASE.md
   procedure 완성도). dry-run mode (NFR-REL-006) 가 actual tag
   push 전 모든 risk 흡수.
5. **CHANGELOG / MIGRATION editorial deferral** — 본 SPEC plan 은
   **CHANGELOG `[1.0.0]` section 실제 본문** + **MIGRATION.md
   per-section content** 을 작성하지 않는다 (HARD per user brief).
   plan 의 implementation phase 는 format contract + structural
   skeleton 만 ship. release ceremony 시점에 manager-git 이
   editorial work 수행.

---

## 2. Sprint Contract (REQUIRED per thorough harness)

본 SPEC 은 SPEC-DEPLOY-001 / SPEC-SEC-001 / SPEC-DOC-001 / SPEC-
DOC-002 와 동일하게 thorough harness 적용. 따라서 evaluator-active
와의 Sprint Contract 가 MANDATORY 다. 본 plan 의 Phase 0 에서
contract 협상 후 Phase 1 implementation 시작.

### Sprint Contract scope (proposed; evaluator-active 가 final
acceptance)

- **Acceptance checklist** (10 items):
  1. `internal/version/version.go` package ships with 4 exported
     variables + `String()` + `Short()` helpers; `go build` clean.
  2. 3 `main.go` files import `internal/version` and consume
     `Version` (per REQ-REL-001). 기존 local `const` literal 제거.
  3. `TestVersionFlag` + `TestVersionShortFlag` 100% PASS after
     refactor (HARD preservation per REQ-REL-002).
  4. `.goreleaser.yml` validates via `goreleaser check`; dry-run
     `goreleaser release --snapshot --clean` produces exactly 12
     archives + checksums + SBOM (REQ-REL-006).
  5. `.github/workflows/release.yml` lints via `actionlint`;
     pre-tag-verify job declares all 12 gates G1..G12 as `needs:`
     graph (REQ-REL-007, REQ-REL-009).
  6. `RELEASE.md` ships with 5 sections (a..e per REQ-REL-005);
     verification commands (cosign, slsa-verifier, git verify-tag)
     are copy-paste reproducible.
  7. `MIGRATION.md` 12-section structural skeleton (§1..§12
     headers exist; per-section "v1.0.0 — no breaking changes in
     this category" placeholder for empty sections per REQ-REL-004).
  8. `release-summary.json` schema documented + sample fixture
     generated in dry-run mode (REQ-REL-013).
  9. Cross-SPEC verification placeholders for G6 (EVAL),
     G7 (DEPLOY-001), G8 (DOC-001), G9 (DOC-002) reference the
     correct workflow run URLs in release.yml.
  10. `README.md` version badge added; SPDX SBOM aggregation
      produces machine-readable `usearch_<Version>.spdx.json`.

- **Priority dimension**: Completeness > Functionality > Originality
  > Design Quality. release ceremony 의 본질은 모든 step 의 완전성
  보장 (missing step = release fail).

- **Test scenarios** (Sprint 1 — 8 scenarios from S1..S12, others
  deferred to Sprint 2):
  - S1 (ldflags injection)
  - S2 (TestVersionFlag preservation — HARD)
  - S3 (3-binary version consistency)
  - S4 (CHANGELOG completeness count)
  - S5 (MIGRATION.md 12-section structure)
  - S6 (goreleaser 12 archives)
  - S7 (gate failure aborts release — synthetic G6 fail)
  - S12 (dry-run mode does not publish)

- **Pass conditions per criterion**:
  - 단위 테스트 (TestVersionFlag 등): 100% PASS (HARD)
  - `internal/version/` coverage: ≥ 85%
  - `release.yml` shell script coverage: ≥ 80% (lenient — bash
    integration tests via shellcheck + bats)
  - 운영자 dry-run reproducibility: 3개 fresh clone × 3 OS (linux/
    darwin/WSL) 동일 결과
  - cosign verify-blob: exit 0 on all 12 archives

- **Sprint 2 deferred items** (post Sprint 1 PASS):
  - S8 (full GitHub Release publish, requires actual tag push
    in dry-run staging repo)
  - S9 (cosign verify by fresh operator)
  - S10 (SLSA verifier by fresh operator)
  - S11 (cross-SPEC verification with real DEPLOY-001 + SEC-001
    workflow outputs)

---

## 3. Phased plan

본 SPEC 은 **6 phase** 로 구성. 각 phase 의 exit gate 는 다음
phase 시작 조건.

---

### Phase 0 — ANALYZE: existing release surface inventory + Sprint
Contract negotiation

**Objective**: 본 SPEC 의 모든 modification target file 의 현
state 를 정확히 capture; Sprint Contract 협상 완료.

**Activities**:

- `cmd/usearch/main.go:14` 의 `const Version` declaration, cobra
  root command (`newRootCmd`/`runCobra`) 의 `--version`/`-v` 처리
  경로 확인
- `cmd/usearch-api/main.go:20` + `cmd/usearch-mcp/main.go:19` 의
  `const version` + `obs.Init` ServiceVersion consumer 확인 (mcp 는
  `:28` 에 `--version` flag print 도 존재)
- `cmd/usearch/main_test.go:12` 의 `semverPattern`
  (`^usearch v\d+\.\d+\.\d+`) byte-fidelity capture (refactor 후
  동일 regex 유지 확인용 — main_test.go 미편집)
- `CHANGELOG.md` 의 `[Unreleased]` section content 의 모든 SPEC
  ID 목록 추출 (`grep -oE 'SPEC-[A-Z]+-[0-9]+' CHANGELOG.md | sort
  -u`); M1..M9 implemented SPEC 와 cross-validate
- `.github/workflows/` 의 모든 기존 workflow YAML 의 trigger +
  job dependencies 확인 (release.yml 가 reference 할 baseline
  workflow run URL 패턴 결정)
- `go.mod` 의 toolchain version + module path 확인 (release ldflags
  의 `-X` flag target package path 결정)
- `.moai/specs/SPEC-{BOOT-001, CLI-001, CLI-002, MCP-001, SKILL-
  001, CORE-001}/spec.md` 의 V1 freeze scope 관련 항목 추출
- evaluator-active 와 Sprint Contract 협상 (§2 proposed scope
  기반) → final acceptance checklist 확정

**Exit gate**:
- Pre-existing version-related file 의 line-level reference 목록
  완성 (research.md §3 inventory 와 일치)
- Sprint Contract artifact `.moai/sprints/SPEC-REL-001/sprint-01.
  md` 작성 완료, evaluator-active sign-off
- 모든 dependency SPEC 의 PASS forecast 점검 (DOC-001 / DOC-002 /
  DEPLOY-001 / SEC-001 / EVAL-trio 각 owner status confirmation)

**Risk**: dependency SPEC 의 지연 가 본 Phase 진행을 차단 — Phase
1 (independent: internal/version) 만 우선 진행, Phase 5 (release.
yml authoring) 는 dependency PASS 후 시작.

---

### Phase 1 — IMPROVE: `internal/version/` package + 3-binary
refactor

**Objective**: 3개 분산 version constant 를 single-source 패키지로
consolidate; existing test PASS 유지.

**Activities**:

- TDD RED: `internal/version/version_test.go` 작성
  - `TestVersionDefault`: `Version` 기본값 `"0.1.0-dev"` 확인
  - `TestVersionMatchesSemverRegex`: `Version` 이 semver regex
    호환 (`TestVersionFlag` 와 동일 regex)
  - `TestStringFormat`: `String()` 출력 형식 검증
  - `TestShortReturnsVersionOnly`: `Short()` == `Version`
  - `TestLdflagsInjection`: ldflags 로 변수 override 가능 확인
    (integration test — `go build -ldflags ...` 후 binary 실행)
- TDD GREEN: `internal/version/version.go` 작성
  ```go
  package version
  var (
      Version   = "0.1.0-dev"
      Commit    = "unknown"
      BuildDate = "unknown"
  )
  // GoVersion uses runtime.Version() at call site (not stored).
  func String() string { /* "usearch v<Version> (<Commit>, built
  <BuildDate>, <GoVersion>)" */ }
  func Short() string { return Version }
  ```
- TDD REFACTOR: package documentation + `@MX:ANCHOR` on exported
  variables (fan_in >= 3 across 3 binaries + tests)
- `cmd/usearch/main.go` (+ cobra root command file) modification:
  - `cmd/usearch/main.go:14` 의 `const Version = "0.1.0-dev"` 삭제
  - `import "github.com/elymas/universal-search/internal/version"`
    추가
  - cobra root command 의 `--version`/`-v` 출력이 참조하는 `Version`
    심볼을 `version.Version` 로 교체 (`usearch v%s` 출력 형식 유지 —
    `main_test.go:12` semverPattern 호환)
  - obs ServiceVersion 전달 지점도 `version.Version` 로 교체
- `cmd/usearch-api/main.go` modification:
  - `const version = "0.1.0-dev"` 삭제
  - `import vver "github.com/elymas/universal-search/internal/version"`
    (alias 으로 local `version` 변수와의 shadow 회피; 또는 import
    경로명 자체 unaliased 유지하고 변수명 변경)
  - `ServiceVersion: version` → `ServiceVersion: vver.Version`
- `cmd/usearch-mcp/main.go` modification: 동일 패턴
- regression test execution:
  - `go test -run TestVersion ./cmd/usearch/...` PASS 확인
  - `go test -race ./...` 전체 PASS 확인 (특히 obs init 회로 무영향)

**Exit gate**:
- A1 (`go build ./internal/version/...` exit 0) PASS
- A2 (TestVersionFlag + TestVersionShortFlag PASS) PASS
- A3 (ldflags 로 3개 binary `--version` 일관 reporting) PASS —
  로컬 ldflags 빌드 후 manual `./usearch --version` confirm
- coverage: `internal/version/` ≥ 90% (small package; high
  coverage 가능)
- `go vet ./...` clean
- LSP zero errors (per CLAUDE.md run-phase)

**Risk**: import alias 충돌 — `cmd/usearch-api/main.go` 의 `obs.
Config.ServiceVersion` field 와 package-level local `version`
변수가 shadow 발생 시 컴파일 에러 (research §3.5).

---

### Phase 2 — IMPROVE: `.goreleaser.yml` configuration + dry-run
validation

**Objective**: 12-archive matrix build + SBOM + SHA256SUMS 자동화.

**Activities**:

- `.goreleaser.yml` 작성 (goreleaser v2 schema):
  - `version: 2`
  - `before.hooks`: `go mod tidy`, `go mod download`
  - `builds`: 3 binaries × 2 OS × 2 arch matrix
    ```yaml
    builds:
      - id: usearch
        main: ./cmd/usearch
        binary: usearch
        env: ["CGO_ENABLED=0"]
        goos: [linux, darwin]
        goarch: [amd64, arm64]
        ldflags:
          - -s -w
          - -X github.com/elymas/universal-search/internal/version.Version={{.Version}}
          - -X github.com/elymas/universal-search/internal/version.Commit={{.FullCommit}}
          - -X github.com/elymas/universal-search/internal/version.BuildDate={{.Date}}
      - id: usearch-api
        main: ./cmd/usearch-api
        binary: usearch-api
        # ... (same matrix + ldflags)
      - id: usearch-mcp
        # ...
    ```
  - `archives`: name template `usearch_{{.Version}}_{{.Os}}_{{.Arch}}`,
    extra files `LICENSE`, `README.md`
  - `checksum`: SHA256SUMS
  - `sboms`: syft 통합 (`anchore/syft v1.x`) — per-archive +
    aggregate
  - `release.disable: true` — actual GitHub Release publish 는
    release.yml workflow 책임 (goreleaser 의 default behavior
    회피)
  - `snapshot.name_template`: `{{ incpatch .Version }}-snapshot`
- `goreleaser check` 통과 확인
- `goreleaser release --snapshot --clean --skip=publish` 로컬
  실행 → `dist/` 디렉토리 검증:
  - 12 `.tar.gz` archives present
  - 1 `SHA256SUMS` file
  - 12 `.spdx.json` SBOM files (per-archive) + 1 aggregate
  - 각 archive 의 `usearch` binary `--version` 출력 confirm

**Exit gate**:
- A4 (`goreleaser check` exit 0) PASS
- A5 (snapshot 모드 12 archives) PASS
- Windows target 명시적 제외 + rationale comment in YAML

**Risk**: goreleaser v2 schema 가 v1 과 micro-incompatible —
research §5.1 의 v2 docs reference 정확성 검증 필요. 만약 v2
schema 가 SBOM 통합 미지원 시 V1.0.1 로 deferred + V1 은 별도
syft step 수동 추가.

---

### Phase 3 — IMPROVE: `MIGRATION.md` + `RELEASE.md` structural
skeleton authoring

**Objective**: 12-section MIGRATION.md + 5-section RELEASE.md
의 structural skeleton ship (per-section content placeholder).

**Activities**:

- `MIGRATION.md` 작성:
  - §1 Overview — semver 1.0.0 freeze scope (REQ-REL-016 의
    freeze + free zone enumeration verbatim)
  - §2..§9: 각 카테고리별 "v1.0.0 — no breaking changes in this
    category" placeholder OR known breaking change 의 3-block
    format (현재 known: 거의 없음 — 0.x-dev → 1.0.0 사실상 zero
    user, but framework 명시적 ship)
  - §10 Adapter status taxonomy reference — SPEC-DOC-002 alpha/
    beta/stable cross-link
  - §11 Upgrade procedure — `helm upgrade`, `go install`, Skill
    재설치 (각 채널의 1-liner)
  - §12 Rollback procedure — `helm rollback`, container tag
    downgrade, tag deletion 절차 (RELEASE.md cross-link)
- `RELEASE.md` 작성:
  - §A Pre-tag verification matrix manual checklist — G1..G12
    각각 의 manual command + expected output
  - §B Annotated GPG-signed tag creation procedure —
    `git tag -a -s v<X.Y.Z> -m <message>` example, GPG key
    setup pre-requisite documentation
  - §C Emergency rollback procedure —
    `git push --delete origin v<X.Y.Z>` + GitHub Release retraction
    UI step + ghcr.io image / chart unpublish (gh CLI commands)
  - §D Post-release task checklist — roadmap.md update PR review,
    SECURITY.md 활성 verification, announcement drafting
    (template link to docs/release-announcement.md template)
  - §E Locale + timing protocol — KST 영업시간 tag-push window
    (D11), maintainer on-call 정보, time zone considerations
- 운영자 dry-run reproducibility 검증: §A 의 manual commands 가
  로컬 환경에서 100% reproducible (env-var dependent 명시 +
  `gh auth status` 등 prerequisite documentation)

**Exit gate**:
- A8 (RELEASE.md 5 sections) PASS — structural lint via grep
- A9 (MIGRATION.md 12 sections) PASS — structural lint
- Manual review by manager-spec on freeze scope completeness

**Risk**: V1 freeze scope (D9) enumeration 누락 시 1.x cycle 내
breakage 발생 가능. Phase 3 의 MIGRATION.md §1 review 는 ALL
relevant SPEC owner (CLI-002, MCP-001, SKILL-001, CORE-001, ...)
sign-off 가능하다면 권장.

---

### Phase 4 — IMPROVE: CHANGELOG `[1.0.0]` section format contract +
README badge

**Objective**: CHANGELOG editing format + completeness contract
명시; README install snippet 추가 (release artifact URL pre-link).

**Activities**:

- CHANGELOG.md modification (본 SPEC 의 plan phase 는 format
  contract 만 ship — 실제 `[1.0.0]` section 본문은 release
  ceremony 시점에 manager-git 가 promote):
  - `[Unreleased]` section 의 모든 항목은 release ceremony 시점에
    `[1.0.0] - YYYY-MM-DD` 로 이동
  - 6-section grouping (Added / Changed / Deprecated / Removed
    / Fixed / Security) 강제 — 현재 single Added block 인 항목
    들을 분류
  - Footer link 형식: `[1.0.0]: https://github.com/elymas/
    universal-search/releases/tag/v1.0.0` (release.yml 의 footer
    update step 이 release publish 후 PR 생성)
  - 본 Phase 4 의 CHANGELOG modification 은 **format contract
    enforcement only** — `.moai/specs/SPEC-REL-001/CHANGELOG-
    FORMAT-CONTRACT.md` (auxiliary file, 또는 본 plan.md §6 본문)
    에 promotion procedure 문서화
- README.md modification:
  - Header 직후 version badge 추가:
    ```markdown
    [![Release](https://img.shields.io/github/v/release/elymas/
    universal-search)](https://github.com/elymas/universal-search/
    releases/latest)
    ```
  - Install section 신설 또는 보강 — v1.0.0 release URL 직접
    reference:
    ```bash
    curl -L https://github.com/elymas/universal-search/releases/
    download/v1.0.0/usearch_1.0.0_linux_amd64.tar.gz | tar xz
    ```
    + cosign verify-blob 명령 reference (RELEASE.md cross-link)
- `gen-changelog-format-check.sh` 신규 script: CHANGELOG.md 가
  `[1.0.0]` section 을 포함하지 않는 한 silent (pre-tag verify
  단계 에서만 hard-check); 포함 시 NFR-REL-005 의 SPEC ID count
  검증

**Exit gate**:
- CHANGELOG format contract documented (in plan.md §6 또는 RELEASE.md §A)
- README badge present, links resolvable (release URL 은 본 SPEC
  미실행 상태에서 404 expected — release.yml run 후 해소)
- `gen-changelog-format-check.sh` script tested with synthetic
  `[1.0.0]` fixture

**Risk**: CHANGELOG 의 `[Unreleased]` 항목 중 Added 외 항목 분류
오류 (예: searxng image pinning 이 Changed 인지 Security 인지) —
plan-auditor 또는 manual review 시 결정.

---

### Phase 5 — IMPROVE: `.github/workflows/release.yml` authoring +
dry-run gate sequencing

**Objective**: tag-trigger release pipeline 의 12-gate verification +
goreleaser + SLSA + cosign + GitHub Release publish 자동화.

**Activities**:

- `.github/workflows/release.yml` 작성 (top-level structure):
  ```yaml
  name: release
  on:
    push:
      tags: ["v[0-9]+.[0-9]+.[0-9]+", "v[0-9]+.[0-9]+.[0-9]+-*"]
    workflow_dispatch:
      inputs:
        dry_run: { type: boolean, default: false }
  permissions:
    contents: write    # release publish
    id-token: write    # cosign keyless + SLSA OIDC
    packages: write    # ghcr.io interaction (verification only)
    actions: read      # cross-workflow status query
  jobs:
    pre-tag-verify: ...      # G1..G12
    goreleaser-build: { needs: [pre-tag-verify] }
    slsa-provenance: { needs: [goreleaser-build], uses: slsa-framework/... }
    cosign-sign: { needs: [goreleaser-build] }
    publish-release: { needs: [slsa-provenance, cosign-sign] }
    post-release-tasks: { needs: [publish-release], if: "${{ !inputs.dry_run }}" }
  ```
- pre-tag-verify job 의 G1..G12 step 작성:
  - G1: `go vet ./...` + `go test -race -coverprofile=cover.out
    ./...` + `go tool cover -func=cover.out | tail -1` 추출 후 ≥
    85% assertion
  - G2: `golangci-lint run --timeout=10m ./...` (optional —
    `if: hashFiles('.golangci.yml') != ''`) + `pre-commit run
    --all-files`
  - G3: LSP gate (SPEC-LSP-CORE-002 의 `moai lsp` CLI 활용 가정;
    또는 `gopls check` 의 standalone command — research §7.3)
  - G4: `gh run list --workflow=deps-audit.yml --branch=main
    --limit=1 --json conclusion --jq '.[0].conclusion'` == `success`
  - G5: `gh run list --workflow=security.yml ...` (SEC-001
    ship 시) == `success`
  - G6: EVAL trio status — `gh run list --workflow=eval-faithfulness.
    yml ...` (EVAL-001) + `gh issue list --label=eval-002-dashboard-
    live ...` (EVAL-002 manual sign-off) + `.moai/reports/eval-003-
    korean-benchmark-*.md` 존재 (EVAL-003)
  - G7: `gh run list --workflow=chart-ci.yml ...` (DEPLOY-001) +
    `cosign verify` on the REAL app images
    `ghcr.io/elymas/usearch-api:${TAG#v}` +
    `ghcr.io/elymas/usearch-mcp:${TAG#v}` (NOT a `universal-search`
    app image — none is built; `universal-search` is the chart name)
    + `helm pull oci://ghcr.io/elymas/charts/universal-search:
    ${TAG#v}` + `helm show chart` 의 `appVersion` 비교
  - G8: `gh run list --workflow=docs.yml ...` (DOC-001) == `success`
  - G9: `gh run list --workflow=adapter-reference-drift.yml ...`
    (DOC-002) == `success` (또는 chart-ci.yml 의 parity test 의
    일부)
  - G10: 24h 윈도우 CI green — `gh run list --workflow=go.yml
    --branch=main --created-before=1day --limit=10 --json
    conclusion --jq 'all(.conclusion == "success")'` (intermittent
    fail tolerance: optional `count of success / total ≥ 0.95`)
  - G11: `git verify-tag ${GITHUB_REF_NAME}` (GPG signature
    검증 — GitHub Actions runner 는 default 로 GPG key 없음;
    `gpg --import <maintainer-public-key>` 사전 step 필요;
    public key 는 secrets 또는 repo `.gnupg/` 에 commit;
    research §7.5)
  - G12: goreleaser-build 후 step (또는 별도 step): goreleaser
    가 produced 한 archive 1개 추출 후 `./usearch --version |
    awk '{print $2}' | sed 's/^v//'` 와 `${GITHUB_REF_NAME#v}`
    equality assert
- goreleaser-build job: `goreleaser/goreleaser-action@v6` 사용,
  `version: latest`, `args: release --clean ${{ inputs.dry_run
  && '--skip=publish,sign' || '' }}`
- slsa-provenance job: `slsa-framework/slsa-github-generator/
  .github/workflows/generator_generic_slsa3.yml@v2.0.0` reusable
  workflow (per SEC-001 D8)
- cosign-sign job: `sigstore/cosign-installer@v3.7.0` + per-archive
  `cosign sign-blob --yes <archive>` loop; output `.sig` + `.crt`
- publish-release job: `softprops/action-gh-release@v2` 또는
  `gh release create` + body extraction:
  ```bash
  awk '/^## \[1\.0\.0\]/,/^## \[/' CHANGELOG.md | sed '$d' >
  release-notes.md
  gh release create v1.0.0 --title "v1.0.0" --notes-file
  release-notes.md dist/*
  ```
- post-release-tasks job: roadmap.md PR creation via `gh pr create`
  + CHANGELOG footer update PR + notification webhook (REQ-REL-
  014, REQ-REL-015)
- `actionlint .github/workflows/release.yml` 검증
- workflow_dispatch dry-run 으로 end-to-end test (signed-tag 없이
  → G11 fail expected → 정상 abort)

**Exit gate**:
- A6 (actionlint exit 0) PASS
- A7 (G1..G12 all declared in pre-tag-verify job) PASS
- A10 (cross-SPEC verification placeholders 모두 명시) PASS
- dry-run mode test: `gh workflow run release.yml -f dry_run=true`
  → archives generated 후 publish skip 확인

**Risk**: G11 GPG verification 의 GitHub Actions runner 환경 setup
이 maintainer public key import 의존 — secrets 또는 repo committed
public key 양쪽 모두 procedural complexity. RELEASE.md §B 에서
정확한 절차 명시 필요.

---

### Phase 6 — VERIFY: end-to-end dry-run + Sprint 1 acceptance +
plan-auditor sign-off

**Objective**: Sprint Contract 의 8 scenarios PASS 확인; release-
ready 상태 declaration.

**Activities**:

- Sprint 1 scenarios execution:
  - S1 (ldflags injection): `go build -ldflags "..." ./cmd/usearch`
    수행 + `--version` 출력 검증
  - S2 (TestVersionFlag preservation): `go test ./cmd/usearch/...`
  - S3 (3-binary consistency): 동일 ldflags 의 3 binary 빌드 +
    수동 비교
  - S4 (CHANGELOG completeness count): synthetic `[1.0.0]` fixture
    + grep count
  - S5 (MIGRATION.md 12-section structure): structural lint
  - S6 (goreleaser 12 archives): snapshot mode
  - S7 (gate failure aborts release): synthetic G6 fail injection
    (e.g. fake EVAL-001 workflow run with `conclusion: failure`) +
    workflow_dispatch dry_run
  - S12 (dry-run mode does not publish): workflow_dispatch dry_run
    end-to-end
- plan-auditor 독립 review:
  - SPEC-REL-001 spec.md / plan.md / research.md 의 EARS REQ
    추출 vs implementation coverage cross-check
  - HARD constraint preservation 확인 (CHANGELOG 본문 미편집,
    git tag 미생성, MIGRATION.md per-section content 는 release
    ceremony 위탁)
  - dependency SPEC PASS status 최종 점검 (DOC-001 / DOC-002 /
    DEPLOY-001 / SEC-001 / EVAL-trio)
- 운영자 walkthrough (RELEASE.md 의 procedure 가 actual
  maintainer environment 에서 reproducible 한지 검증)
- Sprint 2 deferred items 의 deferral rationale 명시

**Exit gate**:
- All 8 Sprint 1 scenarios PASS
- A11 (TRUST 5 Tested — 85% coverage) PASS
- A12 (TRUST 5 Secured — gitleaks + Trivy on workflow YAML) PASS
- A13 (TRUST 5 Trackable — PR cites SPEC-REL-001) PASS
- plan-auditor sign-off recorded in `.moai/reports/plan-auditor/
  SPEC-REL-001.md`
- release ceremony 진행 가능 declaration → manager-git 에게 baton
  pass

**Risk**: Sprint 2 의 S8..S11 (actual GitHub Release publish, cosign
verify, SLSA verify, cross-SPEC verification) 은 본 plan 의 6 phase
이후에 별도 staging repo 환경에서 검증. staging repo setup 의
overhead 가 release ceremony delay 원인 — RELEASE.md 의 dry-run
mode 가 staging 대체.

---

## 4. File-by-file change list

본 SPEC implementation 시 modify 또는 create 되는 모든 file 의
exhaustive list. Phase reference 와 함께.

### Created (NEW)

| File | Phase | Purpose |
|------|-------|---------|
| `internal/version/version.go` | 1 | Single-source version package (REQ-REL-001) |
| `internal/version/version_test.go` | 1 | TestVersionDefault + ldflags injection regression |
| `MIGRATION.md` | 3 | 12-section 0.x → 1.0 breaking-change guide (REQ-REL-004, REQ-REL-016) |
| `RELEASE.md` | 3 | 5-section maintainer release runbook (REQ-REL-005) |
| `.goreleaser.yml` | 2 | goreleaser v2 config for 3 binaries × 2 OS × 2 arch matrix (REQ-REL-006) |
| `.github/workflows/release.yml` | 5 | Tag-trigger release pipeline with G1..G12 + goreleaser + SLSA + cosign + publish (REQ-REL-007, REQ-REL-009, REQ-REL-010, REQ-REL-011, REQ-REL-012) |
| `scripts/gen-changelog-format-check.sh` | 4 | NFR-REL-005 enforcement script |
| `.moai/sprints/SPEC-REL-001/sprint-01.md` | 0 | Sprint Contract artifact |

### Modified

| File | Phase | Change |
|------|-------|--------|
| `cmd/usearch/main.go` | 1 | Replace `const Version = "0.1.0-dev"` with `version.Version` reference; preserve `--version` switch case + REQ-BOOT-012 semantics |
| `cmd/usearch-api/main.go` | 1 | Replace `const version = "0.1.0-dev"` with `vver.Version` reference (import aliased); preserve `obs.Init` integration |
| `cmd/usearch-mcp/main.go` | 1 | Replace `const version = "0.1.0-dev"` with `vver.Version` reference; preserve `obs.Init` integration |
| `README.md` | 4 | Add version badge (shields.io GitHub release) + Install section snippet linking to v1.0.0 release URL |
| `CHANGELOG.md` | 4 (format), release ceremony (content) | Format contract enforcement only; actual `[1.0.0]` section editing during release ceremony (HARD deferred) |
| `.moai/project/roadmap.md` | 6 (release.yml auto-PR) | M9 section marked "✅ shipped" by post-release-tasks job (REQ-REL-014) |

### Existing — Unchanged (HARD preservation)

| File | Rationale |
|------|-----------|
| `cmd/usearch/main_test.go` | `TestVersionFlag` + `TestVersionShortFlag` byte-fidelity preservation (REQ-REL-002 HARD) |
| `.github/workflows/go.yml` | Existing CI baseline; release.yml references via G1 |
| `.github/workflows/deps-audit.yml` | Existing dependency audit baseline; release.yml references via G4 |
| `.github/workflows/pre-commit.yml` | Existing baseline; release.yml references via G2 |
| `go.mod` / `go.sum` | Go toolchain pin (1.25.x) preserved; only `golang.org/x/mod` (or similar) added if needed for ldflags helpers — likely no new direct dependency |
| `pkg/types/Adapter` interface | V1 freeze scope (REQ-REL-016) — unchanged signature |
| `internal/access/`, `internal/auth/`, `internal/index/`, `internal/synthesis/`, `internal/adapters/`, `internal/cache/` | Domain logic preserved; 본 SPEC 은 cross-cutting release ceremony, domain 변경 없음 |
| `deploy/postgres/migrations/*.sql` | Migration forward-only policy (MIGRATION.md §9) — unchanged |

### Out-of-scope but cross-referenced

| File | Owner SPEC | Cross-reference |
|------|-----------|-----------------|
| `SECURITY.md` (repo root) | SPEC-SEC-001 REQ-SEC-011 V14 | 본 SPEC G5 verification 의 evidence |
| `charts/universal-search/` | SPEC-DEPLOY-001 | 본 SPEC G7 verification target |
| `docs/content/{en,ko}/` (Nextra site) | SPEC-DOC-001 | 본 SPEC G8 verification target |
| `docs/content/{en,ko}/reference/adapters/` | SPEC-DOC-002 | 본 SPEC G9 verification target |
| `.github/workflows/build-images.yml` | SPEC-DEPLOY-001 REQ-DEPLOY-018 | 본 SPEC G7 verification 의 image cosign sig source |
| `.github/workflows/chart-ci.yml` + `chart-release.yml` | SPEC-DEPLOY-001 REQ-DEPLOY-017 + REQ-DEPLOY-020 | 본 SPEC G7 verification 의 chart appVersion source |
| `.github/workflows/security.yml` | SPEC-SEC-001 | 본 SPEC G5 verification |
| `.github/workflows/eval-faithfulness.yml` | SPEC-EVAL-001 | 본 SPEC G6 verification |
| `.github/workflows/docs.yml` | SPEC-DOC-001 | 본 SPEC G8 verification |

---

## 5. Cross-SPEC verification matrix

본 SPEC 의 G6/G7/G8/G9 gate 가 dependency SPEC 의 PASS evidence 를
verify 하는 정확한 mechanism:

| Gate | Dependency | Evidence Mechanism | Failure Mode |
|------|-----------|--------------------|--------------| 
| G6 | SPEC-EVAL-001 | `gh run list --workflow=eval-faithfulness.yml --branch=main --limit=1 --json conclusion` == `success`; faithfulness score artifact ≥ 0.85 | EVAL-001 미PASS 시 본 SPEC release 차단 |
| G6 | SPEC-EVAL-002 | Adapter reliability dashboard last data timestamp within 24h; `gh api repos/elymas/universal-search/dashboard-status` 가 응답 (EVAL-002 정의의 dashboard API) | EVAL-002 미PASS 시 차단 |
| G6 | SPEC-EVAL-003 | `.moai/reports/eval-003-korean-benchmark-*.md` 파일 존재 + maintainer sign-off line ("Manual sign-off by limbowl on YYYY-MM-DD") 매칭 | EVAL-003 sign-off 누락 시 차단 |
| G7 | SPEC-DEPLOY-001 | `cosign verify ghcr.io/elymas/usearch-api:1.0.0` + `cosign verify ghcr.io/elymas/usearch-mcp:1.0.0` (real app images; `usearch-migrate` when present) exit 0 — NO `universal-search` app image (none built); `helm pull oci://ghcr.io/elymas/charts/universal-search:1.0.0 && helm show chart` 의 `appVersion` == git tag (without `v`) | DEPLOY-001 image/chart 미publish 또는 appVersion drift 시 차단 |
| G8 | SPEC-DOC-001 | `gh run list --workflow=docs.yml --branch=main --limit=1 --json conclusion` == `success`; link-check 결과 v1.0.0 URL resolvable (post-publish self-reference 는 release 후 검증) | DOC-001 docs site build 실패 시 차단 |
| G9 | SPEC-DOC-002 | `gh run list --workflow=adapter-reference-drift.yml --branch=main --limit=1 --json conclusion` == `success`; OR chart-ci.yml 의 parity test step PASS (DOC-002 implementation 방식에 따라) | DOC-002 drift detection fail 시 차단 |

각 gate failure 는 release.yml 의 `if: failure()` step 으로 detail
report 출력 + GitHub Actions summary 에 actionable message 등록.

---

## 6. CHANGELOG promotion procedure (deferred to release ceremony)

본 plan 은 CHANGELOG `[1.0.0]` section actual content 작성을
포함하지 않는다 (HARD per user brief). 그러나 promotion procedure
는 본 plan 에서 결정 — release ceremony 시점에 manager-git 이
실행:

1. **PR 생성**: branch `release/v1.0.0-changelog`, base `main`
2. **Section header 변경**: `## [Unreleased]` → `## [1.0.0] -
   YYYY-MM-DD` (tag push 예정 날짜)
3. **새 빈 `## [Unreleased]` section** 추가 (header + 빈
   subsection)
4. **6-section grouping**: 기존 single Added block 의 항목들을
   재분류:
   - **Added**: 신규 기능 (SPEC-CORE-001, SPEC-IR-001, SPEC-FAN-001,
     SPEC-ADP-001..009, SPEC-IDX-001..005, SPEC-CACHE-001, SPEC-
     SYN-001..004, SPEC-DEEP-001..004, SPEC-CLI-001/002, SPEC-MCP-
     001, SPEC-UI-001, SPEC-SKILL-001, SPEC-AUTH-001..003, SPEC-
     OBS-001, SPEC-LLM-001, SPEC-BOOT-001 — 모든 implemented SPEC)
   - **Changed**: SearXNG image pinning (existing CHANGELOG 의
     단일 항목), NOTICE update (existing)
   - **Deprecated**: V1.0.0 는 zero (0.x-dev 가 official release
     아님)
   - **Removed**: V1.0.0 는 zero
   - **Fixed**: 명시적 bug-fix commit (M1 ~ M9 history 에서 grep
     "fix" subject prefix) — 현재 거의 없음 (대부분 feat)
   - **Security**: SPEC-SEC-001 + SPEC-DEP-001 의 security
     hardening 항목 (SearXNG version pinning per REQ-DEP-005 →
     이미 CHANGELOG 의 Changed 에 있으나 Security 로 재분류 가능)
5. **Footer link update**:
   ```
   [Unreleased]: https://github.com/elymas/universal-search/compare/v1.0.0...HEAD
   [1.0.0]: https://github.com/elymas/universal-search/releases/tag/v1.0.0
   ```
6. **NFR-REL-005 verification**: PR 의 CI 가 `gen-changelog-format-
   check.sh` 실행 → SPEC ID count vs implemented SPEC directory
   count 비교 → fail 시 PR review 차단
7. **PR merge** 후 → maintainer 가 `git tag -a -s v1.0.0` 작성 +
   `git push origin v1.0.0`

이 6-step 은 RELEASE.md §B (tag creation procedure) 의 prerequisite
checklist 항목 1-6.

---

## 7. Open questions for plan-auditor

spec.md §8 의 8 open questions 중 plan phase 에서 해소 필요한 항목:

- **OQ1** (internal/version 변수 exposure): Phase 1 시작 전 `grep
  -rn "obs.Config" cmd/` + `grep -rn "ServiceVersion\|Commit\|
  BuildDate" cmd/ internal/` 으로 외부 consumer 목록 확인 → 모든
  4 변수 exported 확정 또는 `Commit` / `BuildDate` 만 internal
  결정.

- **OQ2** (usearch-api / usearch-mcp `--version` flag 추가): Phase
  1 시점 결정. 본 plan 의 default 가정 — flag 추가하지 **않음**
  (binary 가 daemon 형태로 운영; `--version` 은 ad-hoc query 가
  드물고, ldflags-injected version 은 obs.Init log 로 emit). 단
  운영자 편의 측면에서 추가 가능 — plan-auditor 결정.

- **OQ3** (pre-release `v1.0.0-rc1` 사용): release ceremony 시
  결정. 본 plan 은 양쪽 모두 지원 (REQ-REL-018). 권장: 직접 `v1.
  0.0` cut (M1..M9 전체가 effective pre-release period; 추가 rc
  cycle 은 release momentum 손상).

- **OQ4** (CHANGELOG `[1.0.0]` date 결정 시점): release ceremony PR
  생성 시점. release.yml extraction regex 의 lenient match (`## \
  [1\.0\.0\] - [0-9]{4}-[0-9]{2}-[0-9]{2}`) 가 흡수.

- **OQ5** (slsa-github-generator input schema): Phase 5 시작 직전
  최신 v2.0.0 docs review 후 확정. fallback: 직접 `slsa-verifier`
  binary 호출 + provenance generation 수동 wiring (research §8.2
  alternative).

- **OQ6** (release-summary.json schema consumer): 본 SPEC 단독 정의
  최종. post-V1 retrospective tooling 별도 SPEC 에서 evolution
  협상.

- **OQ7** (KST timezone 자동 검증 vs manual): manual 결정. CI 자동
  차단 안 함 (over-engineering; maintainer self-discipline 충분).

- **OQ8** (README badge format): Phase 4 시점 결정. default 가정 —
  shields.io GitHub release badge (`![Release](https://img.shields.
  io/github/v/release/elymas/universal-search)`); 다른 형식 (예:
  custom svg) 은 post-V1 polish.

---

## 8. Success criteria summary

본 SPEC plan 이 successful PASS 의 정의:

- **Code level**:
  - `internal/version/` package ships; 3 main.go refactored;
    `TestVersionFlag` regression PASS (Phase 1 exit)
- **Build level**:
  - `.goreleaser.yml` produces 12 archives + checksums + SBOM
    in snapshot mode (Phase 2 exit)
- **Process level**:
  - `RELEASE.md` reproducible by fresh maintainer environment
    (Phase 3 exit)
  - `MIGRATION.md` 12-section structural skeleton complete
    (Phase 3 exit)
- **CI level**:
  - `release.yml` lints clean; dry-run mode end-to-end successful;
    G1..G12 all declared with correct evidence mechanisms (Phase
    5 exit)
- **Sprint level**:
  - 8 Sprint 1 scenarios PASS; plan-auditor sign-off; cross-SPEC
    dependency PASS forecast green (Phase 6 exit)
- **Release ceremony readiness**:
  - All HARD constraints preserved (CHANGELOG 본문 미편집, git
    tag 미생성, MIGRATION.md per-section content 위탁)
  - manager-git baton pass executable (manager-git 가 본 SPEC
    artifact 기반으로 release ceremony 진행 가능)

본 plan PASS 후 release ceremony (manager-git owned) 는 별도
session 에서 진행: (1) CHANGELOG promotion PR, (2) MIGRATION.md
per-section content fill, (3) GPG-signed tag creation, (4) tag
push → release.yml 자동 trigger, (5) post-release tasks review.

---

*End of SPEC-REL-001 plan v0.1.0 (draft).*
