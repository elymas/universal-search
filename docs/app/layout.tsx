import type { Metadata } from 'next'
import 'nextra-theme-docs/style.css'

export const metadata: Metadata = {
  title: {
    default: 'Universal Search Documentation',
    template: '%s – Universal Search Docs',
  },
  description: 'Operator and end-user documentation for Universal Search — a hybrid AI-powered search engine',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html suppressHydrationWarning>
      <head />
      <body>{children}</body>
    </html>
  )
}
