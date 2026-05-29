# SPEC-DOC-001 Progress

## Date: 2026-05-28

### Phase 1: Infrastructure Setup ✅

**Issue**: Nextra 4.x incompatible with Next.js 16's Turbopack (Turbopack panic errors during build)

**Solution**: Downgraded docs site to Nextra 3.1.1 + Next.js 14.2.21 with webpack

#### Completed Tasks:

1. ✅ **Package Configuration** (`docs/package.json`)
   - Downgraded from Next.js 15.0.0 → Next.js 14.2.21 (last secure Next.js 14 version)
   - Downgraded from Nextra 3.0.0 → Nextra 3.1.1 (stable webpack-compatible version)
   - Added `lint` script for development workflow
   - Updated to compatible React 18.3.1 and @types packages

2. ✅ **Next.js Configuration** (`docs/next.config.mjs`)
   - Removed Turbopack experimental configuration
   - Added Nextra 3 theme configuration (`theme: 'nextra-theme-docs'`, `themeConfig: './theme.config.tsx'`)
   - Configured static export (`output: 'export'`)
   - Disabled image optimization for static export (`images.unoptimized: true`)
   - Note: Nextra 3 uses `pages/` directory structure (not `content/` like v4)

3. ✅ **Workspace Isolation** (`pnpm-workspace.yaml`)
   - Created pnpm workspace configuration to isolate docs package from root
   - Prevents dependency conflicts between docs (Next.js 14) and web app (Next.js 16)
   - Each package manages its own dependencies independently

4. ✅ **Content Structure**
   - Migrated from incorrect `pages/content/en/` to proper `pages/en/` structure for Nextra 3
   - Fixed frontmatter format from JSON to YAML for MDX parsing
   - Created directory structure for bilingual content: `pages/en/` and `pages/ko/`

5. ✅ **Build Verification**
   - Successful static export build generating `docs/out/` directory
   - Zero build errors or type errors
   - Optimized production bundle generated successfully

#### Current State:

- **Build Status**: ✅ Working
- **Output Directory**: `docs/out/` contains static HTML/CSS/JS
- **Bundle Size**: 151 kB First Load JS (within acceptable limits)
- **Pages Generated**: 3 pages (404 + index + CSS)

#### Architecture Decisions:

1. **Next.js 14 vs 16**: Chose stability and webpack compatibility over latest Turbopack features
2. **Nextra 3 vs 4**: Nextra 3 required for Next.js 14 compatibility (Nextra 4 requires Next.js 15+)
3. **Workspace Isolation**: Prevents monorepo dependency conflicts
4. **Static Export**: Meets SPEC requirement for air-gapped deployments

#### Remaining Work (from SPEC):

**Phase 2: Content Creation** (Next Priority)
- Create complete bilingual content tree per REQ-DOC-003:
  - `pages/en/introduction/*.mdx` (3 pages)
  - `pages/en/getting-started/*.mdx` (5 pages)
  - `pages/en/end-users/*.mdx` (6 pages)
  - `pages/en/operators/*.mdx` (8+ pages)
  - `pages/en/reference/*.mdx` (CLI, architecture, dependencies)
  - `pages/en/troubleshooting/*.mdx` (10+ entries)
  - `pages/en/legal/*.mdx` (4 pages)
  - Mirror structure in `pages/ko/` for Korean content (90% coverage per REQ-DOC-016)

**Phase 3: Automation Scripts**
- `scripts/gen-cli-reference.sh` (REQ-DOC-007)
- `scripts/check-screenshot-freshness.sh` (REQ-DOC-014)
- `scripts/check-bilingual-coverage.sh` (REQ-DOC-016)
- `scripts/check-doc-claims.sh` (REQ-DOC-018)

**Phase 4: CI/CD Pipeline**
- `.github/workflows/docs.yml` with 5 jobs (REQ-DOC-012)
- `docs/lychee.toml` for link checking (REQ-DOC-013)
- `Dockerfile.docs` multi-stage build (REQ-DOC-015)

**Phase 5: Navigation & IA**
- Update `theme.config.tsx` with proper sidebar navigation
- Configure 7-section information architecture (REQ-DOC-003)
- Add locale switcher for EN ↔ KO

#### Technical Notes:

- **Security**: Next.js 14.2.21 addresses the security vulnerability mentioned in npm warnings (version > 14.2.19)
- **Performance**: Static export with webpack is slower than Turbopack but more reliable
- **Compatibility**: This setup will work until Nextra v4 becomes webpack-compatible or Next.js 15+ stabilizes

#### Build Command:
```bash
pnpm --filter usearch-docs build
```

#### Dev Server:
```bash
pnpm --filter usearch-docs dev
```

---

## Next Session Priorities:

1. **Create Introduction Section** (3 pages EN + KO)
2. **Create Getting Started Section** (5 pages EN + KO)  
3. **Set up proper sidebar navigation** in `theme.config.tsx`
4. **Configure locale switcher** for bilingual support

## Status: Phase 1 Complete ✅
## Next Phase: Content Creation (Phase 2)
