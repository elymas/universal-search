import nextra from 'nextra'

const withNextra = nextra({
  defaultShowCopyCode: true,
})

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  distDir: 'out',
  images: {
    unoptimized: true,
  },
  trailingSlash: true,
  i18n: {
    locales: ['en', 'ko'],
    defaultLocale: 'en',
  },
}

export default withNextra(nextConfig)
