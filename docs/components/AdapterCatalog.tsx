'use client'

/**
 * AdapterCatalog — filterable table listing all 10 adapters.
 * Columns: Adapter, Status, Category, Auth required, Korean-locale optimized, Detail page.
 *
 * SPEC-DOC-002 REQ-ADPDOC-003
 */

import { useState } from 'react'
import { StatusBadge } from './StatusBadge'

export type AdapterCategory =
  | 'search-engine'
  | 'social'
  | 'academic'
  | 'news'
  | 'korean-locale'

export interface AdapterMeta {
  sourceID: string
  displayName: string
  category: AdapterCategory
  requiresAuth: boolean
  koreanLocaleOptimized: boolean
  detailPath: string
}

// Static catalog metadata — category and Korean-locale flags are hand-curated
// since they are not in the Capabilities struct.
const ADAPTERS: AdapterMeta[] = [
  {
    sourceID: 'arxiv',
    displayName: 'arXiv',
    category: 'academic',
    requiresAuth: false,
    koreanLocaleOptimized: false,
    detailPath: '/en/reference/adapters/arxiv',
  },
  {
    sourceID: 'reddit',
    displayName: 'Reddit',
    category: 'social',
    requiresAuth: false,
    koreanLocaleOptimized: false,
    detailPath: '/en/reference/adapters/reddit',
  },
  {
    sourceID: 'hackernews',
    displayName: 'Hacker News',
    category: 'social',
    requiresAuth: false,
    koreanLocaleOptimized: false,
    detailPath: '/en/reference/adapters/hackernews',
  },
  {
    sourceID: 'github',
    displayName: 'GitHub',
    category: 'search-engine',
    requiresAuth: true,
    koreanLocaleOptimized: false,
    detailPath: '/en/reference/adapters/github',
  },
  {
    sourceID: 'youtube',
    displayName: 'YouTube',
    category: 'social',
    requiresAuth: false,
    koreanLocaleOptimized: false,
    detailPath: '/en/reference/adapters/youtube',
  },
  {
    sourceID: 'bluesky',
    displayName: 'Bluesky',
    category: 'social',
    requiresAuth: false,
    koreanLocaleOptimized: false,
    detailPath: '/en/reference/adapters/bluesky',
  },
  {
    sourceID: 'x',
    displayName: 'X (Twitter)',
    category: 'social',
    requiresAuth: false,
    koreanLocaleOptimized: false,
    detailPath: '/en/reference/adapters/x',
  },
  {
    sourceID: 'searxng',
    displayName: 'SearXNG',
    category: 'search-engine',
    requiresAuth: false,
    koreanLocaleOptimized: false,
    detailPath: '/en/reference/adapters/searxng',
  },
  {
    sourceID: 'naver',
    displayName: 'Naver',
    category: 'korean-locale',
    requiresAuth: true,
    koreanLocaleOptimized: true,
    detailPath: '/en/reference/adapters/naver',
  },
  {
    sourceID: 'koreanews',
    displayName: 'Korean News',
    category: 'news',
    requiresAuth: false,
    koreanLocaleOptimized: true,
    detailPath: '/en/reference/adapters/koreanews',
  },
]

const CATEGORIES: Array<{ value: AdapterCategory | 'all'; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'search-engine', label: 'Search engine' },
  { value: 'social', label: 'Social' },
  { value: 'academic', label: 'Academic' },
  { value: 'news', label: 'News' },
  { value: 'korean-locale', label: 'Korean-locale' },
]

/**
 * AdapterCatalog renders a filterable table of all adapters.
 */
export function AdapterCatalog() {
  const [categoryFilter, setCategoryFilter] = useState<AdapterCategory | 'all'>('all')

  const filtered =
    categoryFilter === 'all'
      ? ADAPTERS
      : ADAPTERS.filter((a) => a.category === categoryFilter)

  return (
    <div>
      {/* Category filter buttons */}
      <div className="flex flex-wrap gap-2 mb-4">
        {CATEGORIES.map(({ value, label }) => (
          <button
            key={value}
            onClick={() => setCategoryFilter(value as AdapterCategory | 'all')}
            className={[
              'px-3 py-1 rounded-full text-xs font-medium border transition-colors',
              categoryFilter === value
                ? 'bg-blue-600 text-white border-blue-600'
                : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50',
            ].join(' ')}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Catalog table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b bg-gray-50">
              <th className="text-left py-2 px-3 font-semibold">Adapter</th>
              <th className="text-left py-2 px-3 font-semibold">Status</th>
              <th className="text-left py-2 px-3 font-semibold">Category</th>
              <th className="text-left py-2 px-3 font-semibold">Auth required</th>
              <th className="text-left py-2 px-3 font-semibold">Korean-locale</th>
              <th className="text-left py-2 px-3 font-semibold">Reference</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((adapter) => (
              <tr key={adapter.sourceID} className="border-b hover:bg-gray-50">
                <td className="py-2 px-3 font-medium">{adapter.displayName}</td>
                <td className="py-2 px-3">
                  <StatusBadge adapter={adapter.sourceID} />
                </td>
                <td className="py-2 px-3 capitalize">
                  {adapter.category.replace('-', ' ')}
                </td>
                <td className="py-2 px-3">{adapter.requiresAuth ? 'Yes' : 'No'}</td>
                <td className="py-2 px-3">
                  {adapter.koreanLocaleOptimized ? 'Yes' : 'No'}
                </td>
                <td className="py-2 px-3">
                  <a
                    href={adapter.detailPath}
                    className="text-blue-600 hover:underline"
                  >
                    Docs
                  </a>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-2 text-xs text-gray-500">
        Showing {filtered.length} of {ADAPTERS.length} adapters.
        Common error categories: see the <a href="/en/reference/adapters/errors" className="text-blue-600 hover:underline">Error taxonomy</a> page.
      </p>
    </div>
  )
}

export default AdapterCatalog
