import { Footer, Layout, Navbar } from 'nextra-theme-docs'
import { Head } from 'nextra/components'
import { getPageMap } from 'nextra/page-map'
import 'nextra-theme-docs/style.css'
import type { Metadata } from 'next'

export const metadata: Metadata = {
  title: {
    default: 'Universal Search Documentation',
    template: '%s – Universal Search Docs',
  },
  description: 'Operator and end-user documentation for Universal Search — a hybrid AI-powered search engine',
}

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const navbar = (
    <Navbar
      logo={<strong>Universal Search</strong>}
      projectLink="https://github.com/elymas/universal-search"
    />
  )

  const footer = (
    <Footer>
      <span>
        Universal Search Docs — Apache-2.0 ·{' '}
        <a
          href="https://github.com/elymas/universal-search/blob/main/CHANGELOG.md"
          target="_blank"
          rel="noopener noreferrer"
        >
          CHANGELOG
        </a>
      </span>
    </Footer>
  )

  return (
    <html lang="en" dir="ltr" suppressHydrationWarning>
      <Head>
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      </Head>
      <body>
        <Layout
          navbar={navbar}
          pageMap={await getPageMap()}
          docsRepositoryBase="https://github.com/elymas/universal-search/tree/main/docs"
          footer={footer}
          editLink="Edit this page on GitHub"
          sidebar={{
            defaultMenuCollapseLevel: 1,
            toggleButton: true,
          }}
        >
          {children}
        </Layout>
      </body>
    </html>
  )
}
