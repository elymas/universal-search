# SPEC-REL-001 Research — V1 release ceremony 사전 분석

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: Pre-EARS research per `.claude/rules/moai/workflow/
spec-workflow.md` Plan Phase Sub-phase 1 (Research)

본 research 는 SPEC-REL-001 (M9 V1.0.0 release ceremony) 작성 전
deep-dive 분석이다. 본 SPEC 은 **신규 release 시스템을 발명하지
않으며**, 기존 6개 release 자산을 consolidate + automate + certify
하는 DDD-style SPEC 이다. 따라서 research 는 (a) 현재 release surface
의 inventory, (b) gap analysis vs V1 release ceremony 요구사항, (c)
외부 release tooling 평가, (d) signing infrastructure 옵션, (e)
artifact distribution channel 분석, (f) cross-SPEC dependency closure
를 광범위하게 다룬다.

목차:

1. 현재 release surface inventory — 6개 자산 line-level capture
2. V1 release ceremony 의 의미 — semantic V1 vs nominal V1
3. Version constant 분석 — 3-binary divergence + ldflags 패턴
4. Breaking-change closure — 0.x-dev → 1.0 의 categorical sweep
5. Artifact distribution channel matrix — GitHub Release / OCI /
   community
6. CHANGELOG automation tooling 평가 — release-please / git-cliff
   / changesets / hand-curated
7. Pre-tag verification matrix sequencing — G1..G12 dependency 분석
8. Signing infrastructure 평가 — GPG / sigstore / cosign / SLSA
9. SBOM tooling 평가 — syft / cyclonedx-gomod / spdx-go
10. Cross-SPEC dependency closure — 7-SPEC PASS coordination
11. Comparable OSS release ceremonies — crush / golangci-lint /
    helm / Hugo
12. Open risks + follow-ups

---

## 1. 현재 release surface inventory

`HEAD` (commit 726fa3d 시점) 의 release-relevant file 의 정확한
line-level inventory.

### 1.1 Version constants (3 files)

| File | Line | Symbol | Visibility | Value | Consumer call sites |
|------|------|--------|-----------|-------|---------------------|
| `cmd/usearch/main.go` | 21 | `Version` | **Exported** (title-case) | `"0.1.0-dev"` | `main.go:50` (`fmt.Printf("usearch v%s\n", Version)`), `main.go:104` (`obs.Config{ServiceVersion: Version}`), `main_test.go:14-39` (regex matching) |
| `cmd/usearch-api/main.go` | 18 | `version` | unexported (lowercase) | `"0.1.0-dev"` | `main.go:24` (`obs.Config{ServiceVersion: version}`), `main.go:35` (slog Info `"usearch-api starting"`) |
| `cmd/usearch-mcp/main.go` | 13 | `version` | unexported | `"0.1.0-dev"` | `main.go:19` (`obs.Config{ServiceVersion: version}`), `main.go:30` (slog Info `"usearch-mcp starting"`) |

3개 file 의 drift 가능성 — 1.0.0 cut 시 어느 하나라도 edit miss 시
`usearch-api --version-or-equivalent` (현재 daemon 이라 `--version`
flag 없음) 의 obs.Init log 가 `"0.1.0-dev"` 보고. user-facing
discoverability 가 일관되지 않음.

### 1.2 `--version` flag implementation (1 file)

`cmd/usearch/main.go` 가 유일한 `--version` flag 구현:

```go
// main.go:30-37 (dispatch entry)
if len(args) == 0 {
    fmt.Fprintln(os.Stderr, "usearch: no command given. Use --version
    for version info or 'query' to search.")
    os.Exit(2)
}
code := dispatch(args)
os.Exit(code)

// main.go:48-51 (dispatch switch)
switch args[0] {
case "--version", "-v":
    fmt.Printf("usearch v%s\n", Version)
    return ExitSuccess
```

REQ-BOOT-012 implementation (per `main.go:2` comment "It supports
--version / -v flags per REQ-BOOT-012"). 본 SPEC HARD preservation
대상.

### 1.3 Version regression test (1 file)

`cmd/usearch/main_test.go:14-39` 의 2 tests:

```go
// :14-29 TestVersionFlag
semverPattern := regexp.MustCompile(`^usearch v[0-9]+\.[0-9]+\.
[0-9]+(-[a-zA-Z0-9.-]+)?$`)
cmd := exec.Command("go", "run", ".", "--version")
output, err := cmd.Output()
if err != nil {
    t.Fatalf("--version exited non-zero: %v", err)
}
trimmed := strings.TrimSpace(string(output))
if !semverPattern.MatchString(trimmed) {
    t.Errorf("--version output %q does not match pattern %q",
    output, semverPattern.String())
}

// :33- TestVersionShortFlag (alias for -v)
```

**HARD preservation 항목**: 본 SPEC refactor 가 `internal/version/`
패키지로 consolidation 한 후에도 두 test 100% PASS 유지. semver
regex pattern 자체는 일치하는 형식이라 `0.1.0-dev` (현재) 및 `1.0.0`
(ldflags injected) 양쪽 모두 매칭.

### 1.4 CHANGELOG.md inventory

**File size**: 182 lines (`wc -l CHANGELOG.md`).

**Format**: Keep-a-Changelog v1.1.0 (header 참조: "Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/)").
versioning: SemVer 2.0.0.

**Section structure**:
- Lines 1-5: Header + format reference
- Line 7: `## [Unreleased]`
- Line 9: `### Added`
- Lines 11..177: M6 → M4 → M3 → M2 → M1 → individual SPEC entries
  (chronological reverse, dense per-SPEC narrative)
- Line 179-180: `### Changed` (2 항목 — SearXNG image pinning,
  NOTICE update)
- Line 182: `[Unreleased]: https://github.com/elymas/universal-
  search/commits/main`

**SPEC ID 의 enumeration**:
- `grep -oE 'SPEC-[A-Z]+-[0-9]+' CHANGELOG.md | sort -u | wc -l` ≈
  29 (개략값; M1 BOOT-001 / DEP-001 / OBS-001 / LLM-001 / CORE-001
  / IR-001 / ADP-001..009 / IDX-001..005 / CACHE-001 / FAN-001 /
  SYN-001..004 / DEEP-001..004 / CLI-001 / AUTH-001..003 등 30개
  미만)
- M9 (REL-001 본인) + M9 draft SPECs (DOC-001 / DOC-002 / DEPLOY-
  001 / SEC-001) + M8 EVAL trio 는 implemented 아님 → CHANGELOG
  미진입.

**Detail level per SPEC entry**: dense (commit hash, package path,
metric families, REQ coverage, coverage %, @MX:ANCHOR notation,
implementation date). release-please / git-cliff 의 자동 생성은
이 narrative 보존 불가 → D2 hand-curated 정당화.

**`[Unreleased]` → `[1.0.0]` promotion 시 6-section grouping
필요**:
- 현재 single `### Added` block 에 모든 항목 — promotion 시 분류
  필요
- `### Changed` 의 SearXNG pinning + NOTICE → 그대로 유지 가능
- Deprecated / Removed / Fixed / Security 는 현재 0 항목 — V1.0.0
  cut 시점 신규 작성 가능

### 1.5 CI workflow inventory

`.github/workflows/`:

| File | Size | Trigger | Purpose | release.yml relevance |
|------|------|---------|---------|----------------------|
| `compose-check.yml` | 510B | pull_request | docker-compose smoke | G10 (sustained CI green) input |
| `deps-audit.yml` | 9.3KB | pull_request + Monday schedule | govulncheck + pip-audit + pnpm-audit + hadolint + license-scan + searxng-digest-check | G4 verification target |
| `go.yml` | 875B | push to main + pull_request | go vet + go test -race | G1 evidence source + G10 input |
| `pre-commit-autoupdate.yml` | 1.7KB | schedule | pre-commit hook autoupdate | not direct |
| `pre-commit.yml` | 1.2KB | pull_request | pre-commit run --all-files | G2 verification target |
| `python.yml` | 975B | pull_request (path filter: services/) | Python sidecar tests | G10 input |
| `web.yml` | 967B | pull_request (path filter: web/) | Next.js UI build | G10 input |

**누락 (본 SPEC 또는 cross-SPEC 신설 예정)**:
- `release.yml` — 본 SPEC 신설
- `security.yml` — SPEC-SEC-001 신설
- `build-images.yml` — SPEC-DEPLOY-001 신설
- `chart-ci.yml` + `chart-release.yml` — SPEC-DEPLOY-001 신설
- `docs.yml` — SPEC-DOC-001 신설
- `adapter-reference-drift.yml` — SPEC-DOC-002 신설 (또는 chart-
  ci.yml 의 일부)
- `eval-faithfulness.yml` — SPEC-EVAL-001 신설

### 1.6 Other relevant files

- `go.mod`: Go 1.25.x (BOOT-001 commit 70e4bdc 이후 toolchain
  alignment). module path `github.com/elymas/universal-search`.
- `README.md`: 87 lines. version badge **없음**. install
  instructions 거의 없음 (Docker quickstart + `go run` 만). v1.0.0
  release URL pre-link 추가 필요 (Phase 4).
- `LICENSE`: Apache-2.0 (product.md §8 + standard repo header).
  goreleaser archive 에 포함.
- `NOTICE`: dependency-related notices (per product.md). CHANGELOG
  의 "NOTICE updated" 항목 reference.
- `git tag --list`: **zero tags**. v1.0.0 가 첫 tag.

### 1.7 Gap inventory (missing for V1 release ceremony)

본 SPEC 신설 대상:
- [MISSING] `internal/version/version.go` — 3-binary consolidation
- [MISSING] `MIGRATION.md` — 0.x → 1.0 breaking-change guide
- [MISSING] `RELEASE.md` — maintainer-facing runbook
- [MISSING] `.goreleaser.yml` — release build automation
- [MISSING] `.github/workflows/release.yml` — release pipeline
- [MISSING] `CHANGELOG.md` `[1.0.0]` section (release ceremony 시
  promote)
- [MISSING] README.md version badge + install snippet

Cross-SPEC neighborhood missing:
- [MISSING per SEC-001] `SECURITY.md` (REQ-SEC-011 V14)
- [MISSING per SEC-001] `.github/workflows/security.yml`
- [MISSING per DEPLOY-001] `.goreleaser.yml` 의 image variant +
  `build-images.yml` + `chart-ci.yml`
- [MISSING per DOC-001] Nextra `docs/` 디렉토리 + `docs.yml`

---

## 2. V1 release ceremony 의 의미

### 2.1 Semantic V1 (본 SPEC 의 약속)

V1.0.0 tag 가 user-facing 으로 무엇을 약속하는가:

- **Public API stability across 1.x.y series**:
  - CLI 표면 (`usearch query ...`, future `usearch deep ...`,
    flag 이름 + 의미, exit code)
  - MCP protocol (SPEC-MCP-001 의 tool name + schema)
  - MoAI Skill manifest (SPEC-SKILL-001)
  - Adapter plugin interface (`pkg/types/Adapter`, `Capabilities`)
  - REST endpoint (cmd/usearch-api `POST /query`, `/query/stream`)
  - Config schema (`.moai/config/sections/*.yaml`)
  - Env-var 이름
  - Helm chart values.schema.json (SPEC-DEPLOY-001)
  - Database migration forward-only (SPEC-DEPLOY-001 NFR-DEPLOY-004
    + 본 SPEC MIGRATION.md §9)

- **Quality bar**:
  - Citation faithfulness ≥ 0.85 (SPEC-EVAL-001)
  - Adapter reliability dashboard live (SPEC-EVAL-002)
  - Korean benchmark sign-off (SPEC-EVAL-003)
  - Security: OWASP ASVS L1 + SLSA L2 + cosign signed + SBOM 첨부
    (SPEC-SEC-001)
  - Deploy: Helm chart deployable + multi-arch image + cosign
    signed (SPEC-DEPLOY-001)
  - Docs: Nextra bilingual EN+KO site live (SPEC-DOC-001 + DOC-002)

- **Supply chain transparency**:
  - 모든 binary + image + chart 가 cosign signed (keyless via
    GitHub Actions OIDC)
  - SLSA L2 provenance attestation 첨부
  - SPDX SBOM 첨부
  - Public verifiability via Sigstore Rekor transparency log

### 2.2 What V1 is NOT promising

- **Internal Go package stability**: `internal/` 패키지는 Go
  convention 으로 unstable (importable 가 아님). 단 외부 fork /
  vendor 시 break 가능 — acceptable.
- **Experimental adapter status**: alpha / beta 마킹된 adapter
  (SPEC-DOC-002 status taxonomy) 는 backward-incompat 변경 허용.
  현재 known alpha: SPEC-ADP-006 X (stub-only — `ErrXDisabled`).
- **AI prompt template wording**: synthesis prompt 의 정확한
  wording 은 accuracy 개선 목적 변경 frequent. user-facing
  output 의 quality 만 약속, internal prompt 는 free.
- **Internal metric label values**: cardinality allowlist 내
  addition 자유. removal 은 break.
- **Python sidecar internal API**: researcher / embedder /
  tokenizer-ko / storm / koreanews 의 inter-sidecar 통신 (사실상
  없음; 모두 Go 가 consume) free.

### 2.3 Why tag now vs wait

**Argument for tagging now (V1)**:

- M1..M8 의 implementation history (29+ SPEC, 6+ months) 가
  CHANGELOG 에 dense documentation
- M9 4-SPEC suite (DOC-001 / DOC-002 / DEPLOY-001 / SEC-001) 가
  V1 release-ready 자산을 ship
- M8 EVAL trio 가 quality bar 의 evidence 제공
- Universal Search 의 V1 positioning (product.md "auditable self-
  hosted search-as-a-service") 는 self-hosted operator 의
  **installable + verifiable + supported** artifact 가 필요. tag
  없이는 operator 가 "어느 commit 을 deploy 하는가" 결정 어려움.
- Semver 0.x-dev 시리즈는 사실상 zero external user — V1 cut 후
  community 형성 시 1.x 의 stability promise 가 evidence 로 작동.

**Argument for waiting (V2 deferred)**:

- Mobile (iOS / Android) client → roadmap §6 post-V1 backlog
- Personal context adapter (Gmail / Calendar / Drive / Obsidian /
  Slack) → post-V1
- SaaS hosted multi-tenant → post-V1 (SearXNG AGPL licensing
  review 필요)
- Federated search across multiple deployments → post-V1
- Image / video generation → post-V1
- Agentic task execution → post-V1
- Finetuned local LLM → post-V1

**결정**: V1 의 핵심은 "self-hosted operator 를 위한 verifiable +
deployable + documented retrieval/synthesis platform". 위 post-V1
items 는 V1 의 positioning 을 확장하지 않음 → V1 cut 정당화.

---

## 3. Version constant 분석

### 3.1 현재 3-binary divergence detail

`cmd/usearch/main.go:19-21`:
```go
// Version is the current release version of usearch.
// Format: semver, e.g. "0.1.0-dev".
const Version = "0.1.0-dev"
```

`cmd/usearch-api/main.go:18-24` (research §1.1 grep 결과):
```go
const version = "0.1.0-dev"
// ...
obs.Config{
    ServiceName:    "usearch-api",
    ServiceVersion: version,
    // ...
}
```

`cmd/usearch-mcp/main.go:13-30` (research §1.1):
```go
const version = "0.1.0-dev"
// ...
ServiceVersion: version,
// ...
o.Logger.Info("usearch-mcp starting", "version", version, "admin_addr", o.AdminAddr)
```

**Inconsistency observations**:
- `cmd/usearch` exports `Version` (title-case) — 외부 import 가능
- `cmd/usearch-api` + `cmd/usearch-mcp` lowercase `version` —
  package-local only
- 3개 모두 동일 literal `"0.1.0-dev"` — 현재는 drift 없음
- 1.0.0 cut 시 3개 file 동시 edit 필요 → fragility

### 3.2 `internal/version/` consolidation pattern

Go 생태계 표준 패턴 (research §11.1 charmbracelet/crush 등 참조):

```go
// internal/version/version.go
package version

import "runtime"

// Build-time variables overridden via ldflags.
var (
    Version   = "0.1.0-dev"
    Commit    = "unknown"
    BuildDate = "unknown"
)

// GoVersion returns runtime.Version() — not ldflag-injectable
// since runtime info is intrinsic.
func GoVersion() string { return runtime.Version() }

// String returns a full version identifier.
// Example: "usearch v1.0.0 (abc123, built 2026-05-22T12:00:00Z, go1.25.0)"
func String() string {
    return fmt.Sprintf("usearch v%s (%s, built %s, %s)",
        Version, Commit, BuildDate, GoVersion())
}

// Short returns only the semver.
func Short() string { return Version }
```

**ldflags injection command**:
```bash
go build -ldflags " \
  -s -w \
  -X github.com/elymas/universal-search/internal/version.Version=1.0.0 \
  -X github.com/elymas/universal-search/internal/version.Commit=$(git rev-parse HEAD) \
  -X github.com/elymas/universal-search/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  " ./cmd/usearch
```

`-s -w` 는 debug symbol strip (release binary 크기 축소). dev
build 는 strip 안 함.

### 3.3 ldflags variable target — package path 정확성

ldflags `-X <package_path>.<var_name>=<value>` 는 string 변수만
override 가능. `const` 는 컴파일타임 상수 → 변경 불가. **따라서
`const Version` → `var Version` 변경 필요** (Phase 1 implementation
detail). 이는 Go 표준 패턴이며 const → var 의 type 변경 없음
(`string` 타입 동일).

### 3.4 TestVersionFlag regex preservation

기존 regex: `^usearch v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$`

**Match cases**:
- `usearch v0.1.0-dev` ✓ (현재 default)
- `usearch v1.0.0` ✓ (v1.0.0 ldflags injected)
- `usearch v1.0.0-rc1` ✓ (pre-release)
- `usearch v1.2.3-alpha.1+build.42` — `+` build metadata 미지원
  (regex 끝 `$` 가 `+` 직전에 매칭 실패) — semver 표준은 `+` 허용
  하나 본 regex 는 보수적. V1 scope 에서 build metadata 사용 안
  함 → 무해.

**Non-match cases (intended)**:
- `usearch v1` (no patch) ✗
- `usearch v1.0` (no patch) ✗
- `usearch 1.0.0` (no `v` prefix) ✗

**Refactor 후 출력 변경 가능성**:
- 기존: `fmt.Printf("usearch v%s\n", Version)` → `"usearch v0.1.0-dev\n"`
- 신규: `fmt.Printf("usearch v%s\n", version.Version)` → 동일 출력

regex 매칭 단순 — 본 SPEC refactor 가 regex 깨지지 않음. HARD
preservation 보장.

### 3.5 Import alias 충돌 (Phase 1 risk)

`cmd/usearch-api/main.go` 의 기존 `const version` 을 `version.
Version` 패키지 참조로 변경 시 import path 와 local 변수명 충돌:

```go
import (
    "github.com/elymas/universal-search/internal/version"  // (1) package
)

// ...
ServiceVersion: version,  // (2) old local const reference
// ↓ refactor to:
ServiceVersion: version.Version,  // (3) package member access
```

**Conflict scenario**: 만약 main 함수 내에서 `version := someFunc()`
같은 short-declare 가 있다면 package alias 가 shadowed. **현재
grep 결과**: 3 file 모두 그러한 shadow 없음 (top-level `const`
only). 따라서 import path 직접 사용 가능. 보수적으로 alias
권장: `import vver "...internal/version"` 후 `vver.Version` 사용.

---

## 4. Breaking-change closure — 0.x-dev → 1.0 의 categorical sweep

### 4.1 V1 freeze scope 의 정확한 enumeration

본 SPEC MIGRATION.md §1 의 freeze scope (D9 + REQ-REL-016). 각
항목의 canonical source file 확인:

| Freeze item | Canonical source | Status at v1.0.0 cut |
|-------------|------------------|----------------------|
| CLI commands | `cmd/usearch/main.go:dispatch()` switch case | `query`, `--version`/`-v`, `--help`/`-h`/`help`. M5 의 `deep` subcommand 는 SPEC-DEEP-001..004 implementation 완료 — V1 include 확인 필요. |
| CLI flag 이름 + 의미 | `cmd/usearch/query.go` FlagSet definition | `--source`, `--format`, `--timeout`, `--no-llm`, `--no-obs`, `--json` 등. cmd/usearch/main.go:74-78 의 usage text 참조. |
| exit code | `cmd/usearch/exitcode.go` constants | `ExitSuccess`, `ExitSystemError`, 추가 항목 (exitcode.go 의 모든 export). |
| MCP protocol tool names + schemas | SPEC-MCP-001 implementation 결과 (cmd/usearch-mcp) | **DRAFT status** — V1 ship 시점 implementation 완료 후 freeze. |
| MoAI Skill manifest schema | SPEC-SKILL-001 implementation 결과 | **DRAFT status** — V1 ship 시점 implementation. |
| Adapter plugin interface | `pkg/types/Adapter`, `pkg/types/Capabilities`, `pkg/types/NormalizedDoc`, `pkg/types/Query`, `pkg/types/SourceError` | SPEC-CORE-001 implementation commit f728aa2. **stable since M2** (2026-04~05). 본 SPEC freeze. |
| REST endpoint paths | `cmd/usearch-api/handlers/` | `POST /query` (SPEC-CLI-001 path?), `POST /query/stream` (SPEC-SYN-004). schemaVersion=1 lock per `cmd/usearch/output_json.go:19`. |
| Config schema | `.moai/config/sections/*.yaml` | 27 sections (research §1.6 + ls config/sections). additive keys OK; removal/rename = breaking. |
| Env-var 이름 | grep `os.Getenv` across `cmd/`, `internal/` | `LITELLM_MASTER_KEY`, `OTLP_ENDPOINT`, `LOG_LEVEL`, `USEARCH_ADMIN_PORT`, `USEARCH_GITHUB_TOKEN`, `NAVER_CLIENT_ID`, `NAVER_CLIENT_SECRET`, `USEARCH_KNC_ENDPOINT`, `USEARCH_FAITHFULNESS_MODE`, `USEARCH_SEARXNG_URL`, `BLUESKY_BASE_URL`, `USEARCH_ADP009_RSS_ENABLED`, `KNC_ENABLED`, `DAUM_ENABLED`, `TOKENIZER_KO_BASE_URL`, `TOKENIZER_KO_TIMEOUT_MS`, `TOKENIZER_KO_MAX_RETRIES`, `EVAL_JUDGE_MODEL`, OIDC vars (SPEC-AUTH-001), etc. **completeness audit 필요** — Phase 3 의 MIGRATION.md §4 작성 시 grep audit. |
| Helm chart values.schema.json | SPEC-DEPLOY-001 deliverable | **DRAFT status** — V1 ship 시점 ship. |
| Database migration sequence | `deploy/postgres/migrations/0001..0007*.sql` | 9 SQL files (M1 BOOT, M3 IDX, M5 DEEP, M6 AUTH). forward-only. **stable**. |

### 4.2 Known breaking changes from 0.x-dev → 1.0.0

**Empty list as of HEAD 726fa3d**. zero external user 가정 시 사실상
무의미하나, framework 명시:

| Category | Known breaking change | Operator action |
|----------|----------------------|-----------------|
| CLI commands | None | None |
| CLI flag | None | None |
| Exit code | None | None |
| Config schema | None | None |
| Env-var rename | None | None |
| MCP protocol | (V1 ship 시점 확정) | (확정 후 작성) |
| Skill manifest | (V1 ship 시점 확정) | (확정 후 작성) |
| Adapter interface | None | None |
| REST endpoint | None | None |
| DB schema | None (forward-only) | None |

**Conclusion**: V1.0.0 의 MIGRATION.md 는 거의 모든 섹션이 "v1.0.0
— no breaking changes in this category" placeholder. 그러나 12-
section structure 는 V1.x 시리즈 evolution 시 채워질 framework.

### 4.3 V1.0.1+ 의 future breaking change scenarios

본 SPEC 의 future-proof 측면 — V1.x cycle 내 breaking change 도입
시 deprecation cycle (D9):

- 도입 시점: 1.X.0 (어느 minor) 에서 deprecation warning emit +
  `DEPRECATED.md` entry 추가
- alternative ship 시점: 1.X+1.0 (next minor) 에서 alternative 안내
- removal 시점: 2.0.0 (major) 에서 removal

`DEPRECATED.md` 는 본 SPEC 신설 안 함 — 첫 deprecation 발생 시
ad-hoc 신설. minimum 1 minor cycle deprecation window (≈ 3개월
가정; no hard timeline per `.moai/rules/moai/core/agent-common-
protocol.md` time estimation 금지).

---

## 5. Artifact distribution channel matrix

### 5.1 Channel A: GitHub Releases (Go binaries)

**Default channel for direct binary install**.

- URL pattern: `https://github.com/elymas/universal-search/releases/
  tag/v1.0.0`
- Asset count: 12 archives × 3 companion files each (`.tar.gz`,
  `.sig`, `.crt`, `.spdx.json`) + `SHA256SUMS` + `multiple.intoto.
  jsonl` provenance = ~50 attached files
- goreleaser produces; release.yml uploads via `gh release create
  v1.0.0 dist/*` 또는 `softprops/action-gh-release@v2`
- install snippet (README.md 추가 대상):
  ```bash
  VERSION=1.0.0
  OS=linux  # or darwin
  ARCH=amd64  # or arm64
  curl -L "https://github.com/elymas/universal-search/releases/
  download/v${VERSION}/usearch_${VERSION}_${OS}_${ARCH}.tar.gz" \
    | tar xz -C /usr/local/bin usearch
  # Verify
  curl -L "https://github.com/elymas/universal-search/releases/
  download/v${VERSION}/usearch_${VERSION}_${OS}_${ARCH}.tar.gz.crt" -O
  curl -L "https://github.com/elymas/universal-search/releases/
  download/v${VERSION}/usearch_${VERSION}_${OS}_${ARCH}.tar.gz.sig" -O
  cosign verify-blob --certificate usearch_${VERSION}_${OS}_${ARCH}.
  tar.gz.crt --signature usearch_${VERSION}_${OS}_${ARCH}.tar.gz.sig \
    --certificate-identity-regexp "https://github.com/elymas/universal-search/.*" \
    --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
    usearch_${VERSION}_${OS}_${ARCH}.tar.gz
  ```

### 5.2 Channel B: Container images via ghcr.io

SPEC-DEPLOY-001 REQ-DEPLOY-018 책임. 본 SPEC 은 verify 만 (G7).

- URL pattern: `ghcr.io/elymas/universal-search:1.0.0`,
  `ghcr.io/elymas/usearch-api:1.0.0`, `ghcr.io/elymas/usearch-mcp:
  1.0.0`, `ghcr.io/elymas/usearch-migrate:1.0.0` + 5개 Python
  sidecar
- multi-arch: amd64+arm64 (embedder amd64-only per DEPLOY-001 NFR-
  DEPLOY-007)
- cosign signed (keyless, GitHub Actions OIDC)
- SBOM attached as OCI artifact

### 5.3 Channel C: Helm chart via OCI

SPEC-DEPLOY-001 REQ-DEPLOY-017 책임. 본 SPEC 은 verify 만 (G7).

- URL pattern: `oci://ghcr.io/elymas/charts/universal-search:1.0.0`
- `helm pull oci://ghcr.io/elymas/charts/universal-search:1.0.0`
- cosign signed (chart binary signed via `cosign sign-blob`)
- chart `Chart.yaml` `appVersion: 1.0.0` 가 git tag 와 sync —
  G7 verification

### 5.4 Out-of-scope channels (post-V1)

- **Homebrew tap**: `brew install elymas/tap/universal-search` —
  formula 작성 + tap repo 관리 부담. community-driven post-V1.
- **APT / YUM packages**: `.deb` / `.rpm` 빌드 + signed repository
  운영 — Linux distribution-specific. post-V1.
- **Snap / Flatpak**: Linux desktop 환경. universal-search 는 CLI
  + daemon 성격이므로 fit 약함. post-V1 가능성 낮음.
- **Chocolatey / winget**: Windows package manager. V1 은 Windows
  binary 제외 (research §5.5) → 적용 안 됨.
- **AUR (Arch User Repository)**: community-driven. PKGBUILD
  template 만 제공 가능.
- **Go module proxy `go install`**: `go install github.com/elymas/
  universal-search/cmd/usearch@v1.0.0` 는 Go convention 으로 자동
  동작 가정. ldflags injection 은 동작 안 함 → `0.1.0-dev` 출력.
  README 에 caveat 명시 (signed binary 권장).
- **Docker Hub mirror**: ghcr.io 만 사용. Docker Hub 의 rate-limit
  + Bitnami precedent 회피. DEPLOY-001 NFR-DEPLOY-006 의 image
  mirror operator guidance 와 정렬.

### 5.5 Windows target 제외 rationale

V1 scope 밖. 이유:

- **빌드 복잡도**: goreleaser windows target 추가는 한 줄 변경이나,
  cmd/usearch-api / cmd/usearch-mcp 의 daemon 성격 (systemd unit
  expected) 가 Windows 호환 불완전. service 등록 절차 추가 부담.
- **테스트 매트릭스**: macOS + linux 만으로도 4 archives × 3
  binaries = 12; windows 추가 시 18. GitHub Actions runner cost +
  release.yml 시간 증가 (NFR-REL-001 budget 영향).
- **사용자 base**: V1 target user (small team self-hosted) 는
  대부분 Linux server 또는 macOS dev workstation. Windows server
  사용자 비율 추정 < 5%.
- **대안**: Windows 사용자는 WSL (Windows Subsystem for Linux) +
  linux/amd64 binary 사용 권장. RELEASE.md / README.md 에 명시.
- **Post-V1 path**: 사용자 요청 발생 시 별도 SPEC (예: SPEC-WIN-
  001) 에서 windows binary + signing + chocolatey/winget 통합
  ship 가능.

---

## 6. CHANGELOG automation tooling 평가

### 6.1 Option A — release-please (Google)

**Pros**:
- Conventional Commits 기반 자동 CHANGELOG + version bump
- GitHub Action 통합 ergonomic
- monorepo 지원

**Cons**:
- Conventional Commit subject 만 capture — SPEC narrative (commit
  body 에 있는 dense detail) 손실
- `Release-As: 1.0.0` footer 필요 또는 자동 bump rule (feat = minor,
  fix = patch) — V1 cut 의 explicit 의도와 mismatch
- release-please 가 자체 PR 생성 — manual review window 추가
  (workflow 단순화 의도와 충돌)

**결정**: V1 cut 부적합 (D2 rationale). V1.0.1+ patch release 자동화
시점에 재평가.

### 6.2 Option B — git-cliff (Rust-based, OSS)

**Pros**:
- Conventional Commits + custom template + section grouping
  customizable
- standalone CLI (CI 의존성 없음)
- 빠름

**Cons**:
- Conventional Commit body 의 multi-line narrative 처리 미흡
- 본 프로젝트의 CHANGELOG 의 dense per-SPEC entry (coverage %,
  metric families, REQ coverage notation) 재생산 불가
- template 작성 학습 곡선

**결정**: V1 cut 부적합. V1.0.1+ 의 hybrid (자동 conventional commit
harvest + manual SPEC narrative 보강) 시점에 평가.

### 6.3 Option C — changesets (npm 생태계)

**Pros**:
- Per-change author-authored changeset file (PR 마다)
- monorepo 우수
- npm ecosystem standard

**Cons**:
- npm/yarn/pnpm 생태계 — Go 프로젝트에는 fit 빈약
- 운영 부담: 매 PR 마다 changeset file 추가 의무

**결정**: V1 cut 부적합. 본 프로젝트는 Go primary.

### 6.4 Option D — Hand-curated CHANGELOG.md (현재 + V1 cut 채택)

**Pros**:
- SPEC narrative 보존 — 현재 CHANGELOG.md 의 dense detail level
  유지
- 6-section grouping (Added / Changed / Deprecated / Removed /
  Fixed / Security) 자유롭게 작성
- release ceremony 시 maintainer 의 deliberate work — quality
  control

**Cons**:
- 매 release 시 manual edit 부담 (V1 cut 는 1회성으로 acceptable)
- format consistency 는 maintainer self-discipline 의존

**결정**: V1 cut 채택 (D2 rationale). Post-V1 별도 SPEC 에서
hybrid 자동화 evaluation.

### 6.5 Section extraction tooling (CHANGELOG → GitHub Release body)

release.yml 의 release notes 생성 — CHANGELOG `[1.0.0]` section
추출:

```bash
# Option 1: awk (POSIX)
awk '/^## \[1\.0\.0\]/,/^## \[/' CHANGELOG.md | sed '$d' > release-notes.md

# Option 2: git-cliff section extract
git-cliff --tag v1.0.0 --strip header,footer > release-notes.md

# Option 3: sed (line-range)
sed -n '/^## \[1\.0\.0\]/,/^## \[Unreleased\]/p' CHANGELOG.md > release-notes.md
```

**결정**: Option 1 (awk + sed) — POSIX standard, external dep 없음,
release.yml shell step 으로 직접 사용 가능.

---

## 7. Pre-tag verification matrix sequencing

### 7.1 G1..G12 dependency graph

각 gate 가 다른 gate 에 depend 하는 그래프:

```
G1 (go vet + test + coverage) ─────┐
G2 (lint + pre-commit) ────────────┤
G3 (LSP gate) ─────────────────────┤
G4 (deps-audit.yml PASS) ──────────┤
G5 (security.yml PASS) ────────────┼─► pre-tag-verify job 완료
G6 (EVAL trio PASS) ───────────────┤
G7 (chart-ci.yml + image cosign) ──┤
G8 (docs.yml PASS) ────────────────┤
G9 (DOC-002 drift PASS) ───────────┤
G10 (24h CI green sustained) ──────┤
G11 (git verify-tag) ──────────────┤
G12 (--version 일치) ──────────────┘  (G12 는 build-after-test;
                                       goreleaser job 의 일부로
                                       이동 가능)
```

**parallelize-able**: G1..G11 모두 independent (각자 다른 evidence
source) — `strategy.matrix` 로 병렬화 가능. release.yml runtime 단축.

**sequence-required**:
- G12 는 goreleaser-build job 후 (binary 가 build 된 후 `--version`
  실행 가능) — pre-tag-verify 와 goreleaser-build 사이에 별도
  `version-consistency` job 으로 분리 가능.

### 7.2 G6 (EVAL trio) evidence mechanism detail

EVAL trio 의 PASS 검증은 각 EVAL SPEC implementation 결과에
의존:

- **G6-1 EVAL-001 (citation faithfulness ≥ 0.85)**:
  - SPEC-EVAL-001 implementation 결과 `eval-faithfulness.yml`
    workflow 가 main 에서 nightly 또는 PR-trigger 실행
  - 결과 artifact: `faithfulness-score-<run-id>.json` with field
    `aggregate_score: float`
  - release.yml G6-1: `gh run download <latest-run-id> --name
    faithfulness-score --dir /tmp` + `jq '.aggregate_score' /tmp/
    *.json | awk '$1 >= 0.85'` exit 0
  
- **G6-2 EVAL-002 (adapter dashboard live)**:
  - SPEC-EVAL-002 implementation 결과 — adapter reliability
    dashboard endpoint (Grafana 또는 자체 HTML)
  - Last data point timestamp within 24h verification
  - release.yml G6-2: `curl -fsSL <dashboard-status-endpoint> |
    jq '.last_data_timestamp' | awk 'system("date -d " $1 " +%s")
    > systime() - 86400'` (의사 코드 — 실제 구현은 EVAL-002 의
    status endpoint 명세에 의존)

- **G6-3 EVAL-003 (Korean benchmark manual sign-off)**:
  - SPEC-EVAL-003 implementation 결과 — manual scoring 결과를
    `.moai/reports/eval-003-korean-benchmark-<YYYY-MM-DD>.md`
    파일로 commit
  - release.yml G6-3: `ls .moai/reports/eval-003-korean-benchmark-
    *.md | tail -1` 의 mtime 이 release 30일 이내 + 파일 본문에
    "Manual sign-off by <maintainer>" line 존재

### 7.3 G3 (LSP gate) — moai lsp CLI 존재 여부

CLAUDE.md §6 의 "LSP Quality Gates" sync-phase thresholds 가
release.yml 의 G3 source. **현재 구현 상태**: `moai lsp` CLI 가
존재하는지, 또는 별도 `gopls check` 호출이 가능한지 확인 필요.

- `.claude/rules/moai/core/lsp-client.md` 의 SPEC-LSP-CORE-002
  reference: `github.com/charmbracelet/x/powernap v0.1.4` 사용
  multi-language LSP client. 운영자가 `moai` CLI 를 통해 LSP
  invoke 가능. 그러나 `moai lsp --gate sync` 같은 CLI command 는
  spec 에 명시 안 됨.
- Fallback: G3 는 `gopls check ./...` standalone command (Go LSP
  의 cmd-line interface) 또는 `gopls execute -mode...` 호출. 단
  본 SPEC scope 밖 (LSP gate enforcement 의 정확한 mechanism 은
  SPEC-LSP-CORE-002 implementation 시점에 결정).
- **결정**: G3 는 release.yml 의 optional gate — `if: hashFiles('
  $HOME/.moai/lsp-state.json') != ''` 같은 conditional 로
  graceful skip. 본 SPEC plan 의 OQ7 (open question) 후속 작업.

### 7.4 G10 (24h CI green sustained) — 정확한 query

```bash
# 24h 동안 main branch 의 go.yml run 결과 모두 success
gh run list --workflow=go.yml --branch=main \
  --created='>1 day ago' \
  --json conclusion,createdAt --jq '[.[] | .conclusion] | all(. == "success")'
```

**False positive 위험**: 24h 윈도우 내 0건 run 시 `all()` 가 `true`
반환. 추가 검증: `length > 0` 도 assert.

**결정**: G10 의 query 는 다음과 같이 conservative —
```bash
gh run list --workflow=go.yml --branch=main --created='>1 day ago' \
  --json conclusion --jq 'if length == 0 then false else all(.[].conclusion == "success") end'
```

`go.yml`, `deps-audit.yml`, `pre-commit.yml`, `web.yml`, `python.yml`
5개 workflow 각각 verify.

### 7.5 G11 (git verify-tag) — GitHub Actions runner GPG setup

`git verify-tag` 는 local GPG keyring 에 maintainer public key
import 필요. GitHub Actions runner 는 default 로 GPG 미설정.

**Option A: secrets 에 public key**:
```yaml
- name: Import maintainer GPG public key
  run: |
    echo "${{ secrets.MAINTAINER_GPG_PUBLIC_KEY }}" | gpg --import
  env:
    GNUPGHOME: ${{ runner.temp }}/gnupg
```

**Option B: repo committed public key**:
```yaml
- name: Import maintainer GPG public key
  run: |
    gpg --import .release/maintainer-public.asc
```

**결정**: Option B (committed public key). 이유: (a) public key 는
secret 아님 (public 정의), (b) repo 의 release ceremony evidence
trail, (c) secrets rotation 의존성 제거. `.release/maintainer-
public.asc` 신규 file (Phase 5 의 일부).

---

## 8. Signing infrastructure 평가

### 8.1 Tag signing: GPG vs sigstore gitsign

**GPG (D5 채택)**:

- Pros:
  - Git 표준 — `git tag -s` / `git commit -S` 기본 지원
  - maintainer 환경 setup 일회성 (GPG key gen + GitHub upload)
  - `git verify-tag` 표준 검증 명령
- Cons:
  - Key 관리 부담 (loss, rotation, expiration)
  - Web-of-trust 모델 — 일반 사용자 verify 가 maintainer key import
    선행
  - revocation 운영 복잡 (revocation cert 사전 준비 + 배포)

**sigstore gitsign (post-V1 평가)**:

- Pros:
  - Keyless — GitHub Actions OIDC identity 활용
  - cosign 과 동일 supply chain (binary/image/chart 와 도구 통일)
  - Web-of-trust 의존 없음
- Cons:
  - Tag 의 "human author" identity 와 "CI ephemeral OIDC" identity
    간 분리 모호
  - maintainer 의 PC 환경 setup (gitsign CLI install + OIDC
    configured)
  - Git 표준 외 — `git verify-tag` 대신 `gitsign verify` 별도
    명령
- **V1 decision**: GPG (D5). post-V1 별도 SPEC 에서 gitsign 전환
  evaluation 가능.

### 8.2 SLSA generator — input schema 의 정확성

`slsa-framework/slsa-github-generator/.github/workflows/generator_
generic_slsa3.yml@v2.0.0` 의 inputs 의 정확한 schema (research §8.2
follow-up — Phase 5 시작 직전 최신 docs 검증 필요):

**Typical reusable workflow call**:
```yaml
slsa-provenance:
  needs: [goreleaser-build]
  permissions:
    actions: read
    id-token: write
    contents: write
  uses: slsa-framework/slsa-github-generator/.github/workflows/
        generator_generic_slsa3.yml@v2.0.0
  with:
    base64-subjects: ${{ needs.goreleaser-build.outputs.hashes }}
    upload-assets: true
    provenance-name: multiple.intoto.jsonl
```

`base64-subjects` 는 goreleaser-build job 이 SHA256 의 base64
encoded subject 를 output 으로 emit 해야 함. goreleaser v2 의
`signs:` 또는 `artifactories:` section 으로 자동 생성 가능
(goreleaser docs 확인 필요).

**Fallback**: goreleaser 가 자동 hash subject emit 안 할 시
release.yml 의 별도 step 으로:
```bash
cd dist/
sha256sum *.tar.gz | base64 -w0 > /tmp/hashes.txt
echo "hashes=$(cat /tmp/hashes.txt)" >> $GITHUB_OUTPUT
```

### 8.3 SBOM tooling — syft vs cyclonedx-gomod vs spdx-go

**Option A — anchore/syft v1.x (D8 채택)**:

- Pros:
  - SPDX + CycloneDX 양쪽 format 지원
  - Go module + container image + filesystem 모두 scan
  - GitHub Action `anchore/sbom-action@v0.x` ergonomic
  - 컨테이너 image SBOM (SPEC-DEPLOY-001 D7) 와 도구 통일
- Cons:
  - Standalone binary (160MB+) — runner cache 미스 시 download 시간

**Option B — CycloneDX/cyclonedx-gomod**:

- Pros:
  - Go-native, 빠름
  - cyclonedx-cli 와 통합
- Cons:
  - SPDX format 미지원
  - container image 스캔 미지원 (Go module 만)
  - DEPLOY-001 의 image SBOM 과 도구 분리

**Option C — spdx/tools-golang (libraries)**:

- Pros: Go-native, library-level integration 가능
- Cons: high-level CLI 미제공, scratch implementation 필요

**결정**: Option A (syft). DEPLOY-001 의 image SBOM 도 syft 사용
가정 — 도구 통일 확보.

### 8.4 Cosign — keyless workflow detail

`sigstore/cosign-installer@v3.7.0` 가 cosign CLI 설치. release.yml
의 cosign-sign job:

```yaml
cosign-sign:
  needs: [goreleaser-build]
  runs-on: ubuntu-24.04
  permissions:
    id-token: write    # OIDC
    contents: read     # download dist artifacts
  steps:
    - uses: actions/download-artifact@v4
      with: { name: goreleaser-dist, path: dist/ }
    - uses: sigstore/cosign-installer@v3.7.0
    - name: Sign each archive
      run: |
        for archive in dist/*.tar.gz; do
          cosign sign-blob --yes \
            --output-signature "${archive}.sig" \
            --output-certificate "${archive}.crt" \
            "${archive}"
        done
    - uses: actions/upload-artifact@v4
      with: { name: cosign-signatures, path: 'dist/*.sig dist/*.crt' }
```

`--yes` flag 가 keyless transparency log entry 자동 confirm
(Rekor 에 entry 등록). interactive prompt 회피.

**Rekor downtime risk** (R3 mitigation): cosign 가 Rekor unavailable
시 retry 3회 후 fail. release.yml step 에 `continue-on-error:
false` 명시.

### 8.5 Verification command 의 reproducibility

NFR-REL-004 의 reproducibility requirement. 운영자 환경:

- macOS / Linux: cosign + slsa-verifier + git 모두 native install
  가능 (Homebrew, apt). 검증 명령 1:1 reproducible.
- Windows: WSL2 + Linux 환경 또는 Git Bash. cosign Windows binary
  존재 (sigstore/cosign releases). 검증은 가능 (signing 은 V1
  excluded).

**No local secret required**: keyless cosign + public Fulcio (sigstore.
dev) + public Rekor (rekor.sigstore.dev) — 운영자 환경에 local
secret / token 불필요. verification 의 reproducibility 100%.

---

## 9. SBOM publication

### 9.1 SPDX format selection

본 SPEC 의 SBOM format. SPDX 와 CycloneDX 양쪽 모두 industry
standard. **결정**: SPDX (per D8).

- SPDX v2.3 또는 v3.0 (syft 의 default)
- File name: `usearch_1.0.0_linux_amd64.tar.gz.spdx.json`
  per-archive + `usearch_1.0.0.spdx.json` aggregate
- Content: 모든 direct + transitive Go module dependency

### 9.2 Cross-channel SBOM consistency

- **Binary SBOM (본 SPEC)**: goreleaser archive 마다 1개 + aggregate
- **Image SBOM (DEPLOY-001 build-images.yml)**: 7 image 마다 1개,
  ghcr.io 의 OCI attached artifact
- **Chart SBOM (DEPLOY-001 chart-release.yml)**: chart artifact 자체
  의 SPDX, OCI attached

3개 채널의 SBOM 은 서로 다른 scope 다. binary SBOM 은 Go module
dependency; image SBOM 은 OS package + Go binary + Python sidecar
deps; chart SBOM 은 subchart dependency (postgresql / redis / qdrant).

### 9.3 SBOM verification by operator

운영자가 SBOM 활용:
- vulnerable dependency 발견 시 SBOM 으로 affected version
  identify
- license compliance audit (Apache-2.0 등 비호환 license 검출)
- Software supply chain audit trail

검증 명령:
```bash
# SPDX SBOM 의 dependency 목록 추출
jq '.packages[] | {name, versionInfo, licenseConcluded}' \
   usearch_1.0.0_linux_amd64.tar.gz.spdx.json
```

---

## 10. Cross-SPEC dependency closure

### 10.1 7-SPEC PASS coordination

본 SPEC `depends_on` field 의 7개 SPEC + 효과적 dependency M8
EVAL trio:

| SPEC | Status (HEAD 726fa3d) | PASS dependency for REL-001 | Gate |
|------|----------------------|-----------------------------|------|
| SPEC-DOC-001 | draft (commit 6b70742) | docs site live, link-check PASS | G8 |
| SPEC-DOC-002 | draft (commit d492f09) | adapter reference drift PASS | G9 |
| SPEC-DEPLOY-001 | draft (commit 726fa3d) | chart deployable + image cosign signed | G7 |
| SPEC-SEC-001 | draft (commit 761381d) | security workflow PASS + ASVS L1 baseline | G5 |
| SPEC-EVAL-001 | draft (commit 7fb85fc) | faithfulness ≥ 0.85 | G6-1 |
| SPEC-EVAL-002 | draft (commit a6dec5f) | adapter dashboard live | G6-2 |
| SPEC-EVAL-003 | draft (commit 9d9f996) | Korean benchmark sign-off | G6-3 |

각 SPEC 의 PASS forecast 는 owner 책임 — manager-spec 주간 status
report 가 추적.

### 10.2 Failure mode propagation

본 SPEC PASS 가 dependency PASS 에 강하게 결합 — 단일 SPEC fail 시:

- **DOC-001 / DOC-002 fail**: G8/G9 차단 → REL-001 release 차단 →
  V1 release 미수행. 운영자는 docs site 없이 V1 사용 불가.
- **DEPLOY-001 fail**: G7 차단 → 동일 결과. operator team-scale
  deploy 불가 (single-host `docker-compose up` 만 가능).
- **SEC-001 fail**: G5 차단 → 동일 결과. supply chain attestation
  + ASVS L1 baseline 미충족. V1 quality claim 의 evidence 부재.
- **EVAL trio fail**: G6 차단 → 동일 결과. quality bar 의 evidence
  부재.

**Mitigation**: 본 SPEC plan 의 Phase 5 (release.yml authoring) 가
dependency PASS 시점 확인 후 시작. dependency 지연 시 본 SPEC plan
의 Phase 1-4 (independent work) 우선 진행. R1 risk mitigation.

### 10.3 Reverse dependency — 본 SPEC PASS 가 unblock 하는 SPEC

본 SPEC `blocks: []` (terminal SPEC). 그러나 effective 다음 작업:

- **roadmap §6 post-V1 backlog 진입**: M10 planning 시작 가능
- **community 운영 도구 활성**: GitHub Security Advisories, Discussions,
  community channel
- **V1.0.1 patch release cycle**: hot-fix 발견 시 본 SPEC 의 release.
  yml 가 자동 재사용. version bump 만 manual.

---

## 11. Comparable OSS release ceremonies

### 11.1 charmbracelet/crush (Go, multi-language LSP)

- Single binary distribution via goreleaser
- GitHub Releases + ghcr.io images
- cosign keyless signing
- SemVer 2.0.0 + Keep-a-Changelog format
- Conventional commits + manual CHANGELOG promotion

본 SPEC 의 reference architecture 와 가장 가까운 OSS project.
charmbracelet ecosystem (lipgloss, bubbletea, glamour) 모두 동일
패턴.

### 11.2 golangci-lint

- Multi-binary goreleaser (linux/darwin/windows × amd64/arm64)
- Docker image distribution (Docker Hub + ghcr.io)
- CHANGELOG via hand-curated KaC format
- GPG signed tags + cosign signed binaries
- SLSA L3 (post-V1 reference)

본 SPEC 의 Windows target 제외 결정과 대조 (golangci-lint 는
Windows 포함; CLI tool 성격이라 desktop user 비율 높음). 그러나
ldflags injection pattern + goreleaser config 의 reference.

### 11.3 helm/helm

- Multi-binary goreleaser
- OCI chart distribution (helm chart 의 ground truth)
- KaC CHANGELOG + manual editing
- GPG signed tags (release manager rotation)
- Verification: sha256sums + GPG signature

본 SPEC 의 OCI chart distribution (DEPLOY-001 와 cross-link) 의
reference.

### 11.4 gohugoio/hugo

- Multi-binary goreleaser (Windows 포함; static site generator
  desktop user 많음)
- Docker image
- CHANGELOG hand-curated, dense detail per release
- Release ceremony 의 maintainer manual work 비중 큼

본 SPEC 의 hand-curated CHANGELOG 정당화의 reference. Hugo 의
release notes 의 dense detail level 이 본 프로젝트와 유사.

---

## 12. Open risks + follow-ups

### 12.1 Open risks (R1..R12 from spec.md §7)

spec.md §7 risks table 의 12개 risk 각각 mitigation 명시. plan 의
Phase exit gate 에서 risk re-evaluation.

### 12.2 Follow-up items (post-V1 별도 SPEC 후보)

- **SPEC-REL-002 (가칭) — V1.0.1+ release automation**:
  Hybrid CHANGELOG (자동 conventional commit harvest + manual
  SPEC narrative). release-please 또는 git-cliff 평가.
- **SPEC-REL-003 (가칭) — Multi-platform binary expansion**:
  Windows / FreeBSD / OpenBSD targets. Homebrew tap + AUR PKGBUILD.
- **SPEC-REL-004 (가칭) — LTS commitment + security patch cadence**:
  V1.x stable window 정의 + CVE response SLA + security advisory
  workflow.
- **SPEC-REL-005 (가칭) — Sigstore gitsign tag signing 전환**:
  GPG → gitsign keyless. maintainer environment setup 단순화.
- **SPEC-REL-006 (가칭) — Tag protection rules automation**:
  Terraform / Pulumi 로 GitHub branch protection + tag protection
  rule code-as-config.

### 12.3 External monitoring obligations

V1 ship 후 maintainer 의 ongoing 책임:

- Sigstore Rekor transparency log monitoring (signature anomaly
  detection)
- SBOM 의 dependency CVE feed monitoring (NFR-SEC-002 MTTR target)
- GitHub Security Advisories private reporting channel monitoring
- Cosign / SLSA tool version churn (quarterly audit per NFR-DEPLOY-
  005 precedent)
- Go module checksum DB monitoring (GOFLAGS=-mod=readonly enforcement)

이 항목들은 SPEC-SEC-001 의 `ops/security/runbook.md` 와 cross-
link.

### 12.4 KST timezone + maintainer ergonomics

D11 의 KST 영업시간 tag-push window. maintainer (limbowl) 가
zero-day incident response 가능 시점. 자동 enforcement 안 함
(over-engineering); RELEASE.md 의 procedural 항목.

**Future scenario**: maintainer pool 확장 시 (post-V1) 의 timezone
coordination. on-call rotation 모델 가능. 본 SPEC scope 밖.

---

## 13. Summary

본 research 는 SPEC-REL-001 (V1.0.0 release ceremony) 의 사전
분석. 핵심 발견:

1. **Single-source version package** (`internal/version/`) 는 3개
   분산 constant 의 fragility 를 해소하는 필수 consolidation.
   `TestVersionFlag` regression test 의 HARD preservation 강제.

2. **CHANGELOG.md** 는 현재 dense per-SPEC narrative 를 보유 —
   release-please / git-cliff 자동화는 narrative 손실. V1 cut 은
   hand-curated. Post-V1 hybrid 별도 SPEC.

3. **Pre-tag verification matrix (G1..G12)** 가 본 SPEC 의 핵심
   contribution — release ceremony 의 모든 quality gate 를 명문화.
   7개 dependency SPEC PASS 의 evidence mechanism 명시.

4. **Supply chain transparency** (SLSA L2 + cosign + SBOM) 는
   SPEC-SEC-001 D8 와 정렬; 본 SPEC 은 Go binary 에 동일 정책
   적용.

5. **Artifact distribution channels** 는 3-channel (GitHub Releases
   + ghcr.io + OCI chart). Homebrew / apt / Snap / Windows 는
   post-V1.

6. **Cross-SPEC dependency closure** 의 7-SPEC coordination 은 본
   SPEC PASS 의 prerequisite. manager-strategy + manager-spec 의
   주간 status report 가 추적.

7. **V1 freeze scope** (REQ-REL-016 + D9) 는 1.x.y cycle 의 stability
   promise — public API, MCP protocol, Skill manifest, adapter
   interface, REST endpoint, config schema, env-var, chart values.
   schema 의 명문화.

8. **Maintainer ergonomics** 보존 — NFR-REL-007 의 hands-on time ≤
   10분 (all gates green 가정). dry-run mode (NFR-REL-006) 가
   actual tag push 전 risk 흡수.

본 research 의 결론은 spec.md §2 의 18 EARS REQs + 7 NFRs + plan.md
의 6-phase implementation plan 으로 codification.

---

*End of SPEC-REL-001 research v0.1.0 (draft).*
