import { Footer, Layout, Navbar } from 'nextra-theme-docs'
import { Head } from 'nextra/components'
import { getPageMap } from 'nextra/page-map'
import type { ReactNode } from 'react'

type LayoutProps = {
  children: ReactNode
  params: Promise<{ lang: string }>
}

export default async function LocaleLayout({ children, params }: LayoutProps) {
  const { lang } = await params
  const pageMap = await getPageMap(`/${lang}`)

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
    <>
      <Head>
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      </Head>
      <Layout
        navbar={navbar}
        pageMap={pageMap}
        docsRepositoryBase="https://github.com/elymas/universal-search/tree/main/docs"
        footer={footer}
        editLink="Edit this page on GitHub"
        sidebar={{ defaultMenuCollapseLevel: 1, toggleButton: true }}
        i18n={[
          { locale: 'en', name: 'English' },
          { locale: 'ko', name: '한국어' },
        ]}
      >
        {children ?? null}
      </Layout>
    </>
  )
}
