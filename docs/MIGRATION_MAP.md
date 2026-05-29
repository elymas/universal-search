# SPEC-DOC-001 Migration Map

Generated: 2026-05-27
Purpose: Track migration of existing docs assets to Nextra content structure

## Page → Source → Strategy Mapping

| Page | Source File | Strategy | Owner | Status |
|------|-------------|----------|-------|--------|
| introduction/index.mdx | .moai/project/product.md §1 | PRESERVE | manager-docs | TODO |
| introduction/personas.mdx | .moai/project/product.md §3 | PRESERVE | manager-docs | TODO |
| introduction/comparison.mdx | .moai/project/product.md §7 | PRESERVE | manager-docs | TODO |
| getting-started/prerequisites.mdx | README.md Prerequisites table | PRESERVE | manager-docs | TODO |
| getting-started/compose-setup.mdx | README.md Quickstart steps 1-3 | PRESERVE | manager-docs | TODO |
| getting-started/build-binary.mdx | README.md Quickstart steps 4-5 | PRESERVE | manager-docs | TODO |
| reference/architecture.mdx | .moai/project/tech.md | PRESERVE | manager-docs | TODO |
| reference/dependencies.mdx | docs/dependencies.md | LINK | manager-docs | TODO |
| operators/auth-setup.mdx | .moai/docs/MCP_OAUTH_SETUP.md | PRESERVE | manager-docs | TODO |
| operators/security/runbook.mdx | ops/security/runbook.md | CROSS-LINK | manager-docs | TODO |
| operators/security/owasp-checklist.mdx | ops/security/owasp-asvs-checklist.md | CROSS-LINK | manager-docs | TODO |
| operators/security/threat-model.mdx | ops/security/threat-model.md | CROSS-LINK | manager-docs | TODO |
| legal/licenses.mdx | .moai/project/product.md §8 + LICENSE | PRESERVE | manager-docs | TODO |
| legal/changelog.mdx | CHANGELOG.md | EMBED/LINK | manager-docs | TODO |

## Strategy Definitions

- **PRESERVE**: Copy content with byte-fidelity, add MDX wrapper + frontmatter
- **LINK**: Add cross-link to canonical source (no content duplication)
- **CROSS-LINK**: Reference to canonical file in ops/security/
- **GENERATE**: Auto-generated from scripts (CLI reference, MCP tools)
- **HAND-WRITE**: Net-new content with no source (narrative, tutorials)

## Screenshots Needed

List of `screenshot:ui:*` placeholders to be filled in Phase 4:

- end-users/web-ui-tour.mdx:
  - screenshot:ui:home - Query interface homepage
  - screenshot:ui:source-detail - Source detail view
  - screenshot:ui:history - Query history page

## Notes

- All migrations preserve original content - no rewriting in Phase 1
- Phase 3 (PRESERVE) focuses on byte-fidelity migration
- Phase 4 (HAND-WRITE) creates net-new narrative content
- KO placeholder pages created in Phase 3, filled in Phase 4
