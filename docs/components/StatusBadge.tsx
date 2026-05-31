'use client'

/**
 * StatusBadge — renders a lifecycle badge for an adapter based on the
 * static adapter-status.json feed (DOC-002-owned; hand-curated from EVAL-002 dashboard).
 *
 * SPEC-DOC-002 REQ-ADPDOC-005
 *
 * Lifecycle taxonomy (D5):
 *   stable     — green  — success rate >= 0.95
 *   beta       — yellow — success rate 0.80–0.94
 *   disabled   — grey   — compile/flag-gated stub, no live V1 path (e.g. x)
 *   deprecated — red    — reserved for post-V1 adapter removal
 *
 * Falls back to "Status unknown" when the adapter key is missing or malformed.
 */

import adapterStatusRaw from '../content/en/reference/adapters/_generated/adapter-status.json'

export type Lifecycle = 'stable' | 'beta' | 'disabled' | 'deprecated'

export interface AdapterStatusEntry {
  lifecycle: Lifecycle
  successRate7d?: number
  verifiedAt?: string
}

// Re-type the imported JSON as a record we can safely query.
const adapterStatus = adapterStatusRaw as Record<string, unknown>

function parseEntry(raw: unknown): AdapterStatusEntry | null {
  if (typeof raw !== 'object' || raw === null) return null
  const obj = raw as Record<string, unknown>
  if (typeof obj.lifecycle !== 'string') return null
  const lifecycle = obj.lifecycle as string
  if (!['stable', 'beta', 'disabled', 'deprecated'].includes(lifecycle)) return null
  return {
    lifecycle: lifecycle as Lifecycle,
    successRate7d:
      typeof obj.successRate7d === 'number' ? obj.successRate7d : undefined,
    verifiedAt: typeof obj.verifiedAt === 'string' ? obj.verifiedAt : undefined,
  }
}

const BADGE_STYLES: Record<Lifecycle | 'unknown', string> = {
  stable:
    'inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800',
  beta: 'inline-flex items-center rounded-full bg-yellow-100 px-2.5 py-0.5 text-xs font-medium text-yellow-800',
  disabled:
    'inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-600',
  deprecated:
    'inline-flex items-center rounded-full bg-red-100 px-2.5 py-0.5 text-xs font-medium text-red-800',
  unknown:
    'inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-500',
}

const BADGE_LABELS: Record<Lifecycle | 'unknown', string> = {
  stable: 'stable',
  beta: 'beta',
  disabled: 'disabled — not available in V1',
  deprecated: 'deprecated',
  unknown: 'Status unknown',
}

interface Props {
  adapter: string
}

/**
 * StatusBadge renders the lifecycle badge + optional success rate + verifiedAt
 * for the given adapter SourceID.
 */
export function StatusBadge({ adapter }: Props) {
  const rawEntry = adapterStatus[adapter]
  const entry = parseEntry(rawEntry)

  if (!entry) {
    return (
      <span className={BADGE_STYLES.unknown}>
        {BADGE_LABELS.unknown}
      </span>
    )
  }

  const style = BADGE_STYLES[entry.lifecycle]
  const label = BADGE_LABELS[entry.lifecycle]

  return (
    <span className={style}>
      {label}
      {entry.successRate7d !== undefined && (
        <span className="ml-1 opacity-75">
          ({(entry.successRate7d * 100).toFixed(1)}% 7d)
        </span>
      )}
      {entry.verifiedAt && (
        <span className="ml-1 opacity-60 text-xs">
          verified {entry.verifiedAt.slice(0, 10)}
        </span>
      )}
    </span>
  )
}

export default StatusBadge
