# Universal Search Documentation

This directory contains the documentation site for Universal Search, built with [Nextra v4](https://nextra.site/).

## Development

```bash
# Install dependencies
pnpm install

# Start development server
pnpm dev

# Build for production
pnpm build

# Preview production build
pnpm start
```

## Project Structure

```
docs/
├── content/          # Documentation content (MDX files)
│   ├── en/          # English content
│   └── ko/          # Korean content
├── public/          # Static assets
├── package.json     # Dependencies
├── next.config.mjs # Next.js configuration
└── theme.config.tsx # Nextra theme configuration
```

## Content Organization

The documentation is organized into 7 main sections:

1. **Introduction** - What is Universal Search, personas, comparison
2. **Getting Started** - Prerequisites, setup, first query
3. **End Users** - CLI, Web UI, Claude Skill, MCP integration
4. **Operators** - Deployment, auth, RBAC, observability, security
5. **Reference** - Architecture, dependencies, CLI, MCP tools
6. **Troubleshooting** - Common issues and solutions
7. **Legal** - Licenses, changelog, security, accessibility

## Adding New Content

1. Create MDX files in `content/en/` or `content/ko/`
2. Add corresponding entries in `_meta.json` navigation files
3. Follow the [MDX format](https://mdxjs.com/)
4. Include frontmatter with title, description, and lastUpdated

## Build and Deployment

The documentation site is automatically built and deployed via GitHub Actions:

- **GitHub Pages**: https://elymas.github.io/universal-search/
- **Container Image**: `ghcr.io/elymas/usearch-docs:latest`

## Localization

The documentation is bilingual (English + Korean). When adding new content:

1. Add English content to `content/en/`
2. Add Korean translation to `content/ko/`
3. Maintain ≥90% bilingual coverage per Tier-1 pages

## Quality Checks

All documentation changes must pass:

- Build check: `pnpm build` (zero errors)
- Link check: `lychee` (internal links 100% resolvable)
- Bilingual coverage: ≥90% KO page parity
- Screenshot freshness: UI screenshots ≤90 days old

## Related Files

- `.github/workflows/docs.yml` - CI/CD pipeline
- `scripts/gen-cli-reference.sh` - CLI reference generator
- `scripts/check-bilingual-coverage.sh` - Translation coverage checker
- `scripts/check-screenshot-freshness.sh` - Image freshness validator
