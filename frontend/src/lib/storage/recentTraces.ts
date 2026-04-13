export type RecentTraceMetadata = {
  domain: string
  qtype: 'A' | 'AAAA' | 'NS'
  timestamp: string
  total_duration_ms: number
  status: string
}

const STORAGE_KEY = 'dnswatcher.recent_traces.v1'
const STORAGE_LIMIT = 10

export function loadRecentTraces(): RecentTraceMetadata[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      return []
    }
    const parsed = JSON.parse(raw) as RecentTraceMetadata[]
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

export function saveRecentTrace(
  item: RecentTraceMetadata,
  current: RecentTraceMetadata[],
) {
  const filtered = current.filter(
    (entry) => !(entry.domain === item.domain && entry.qtype === item.qtype),
  )
  const next = [item, ...filtered].slice(0, STORAGE_LIMIT)
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
  } catch {
    return current
  }
  return next
}

export function clearRecentTraces() {
  window.localStorage.removeItem(STORAGE_KEY)
}
