import nextra from 'nextra'

const withNextra = nextra({
  defaultShowCopyCode: true,
})

/** @type {import('next').NextConfig} */
const nextConfig = {
  // output: 'export' — NOTE: static export mode
  // nextra v4 + next 16 static export requires pagefind postbuild step
  // Uncomment for gh-pages deployment:
  // output: 'export',
  // distDir: 'out',
  images: {
    unoptimized: true,
  },
  trailingSlash: true,
}

export default withNextra(nextConfig)
