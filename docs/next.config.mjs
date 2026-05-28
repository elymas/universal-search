import nextra from 'nextra'

const withNextra = nextra({
  theme: 'nextra-theme-docs',
  themeConfig: './theme.config.tsx',
  defaultShowCopyCode: true,
  // Note: Nextra 3 doesn't support file-system i18n like v4
  // Locale routing will be handled through page structure
})

export default withNextra({
  output: 'export',
  images: {
    unoptimized: true,
  },
})
