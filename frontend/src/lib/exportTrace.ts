import type { TraceResult } from './api/types'

function sanitizeFilenameSegment(value: string) {
  return value.replace(/[^a-z0-9.-]+/gi, '-').replace(/^-+|-+$/g, '')
}

export function exportTraceResult(trace: TraceResult) {
  const payload = JSON.stringify(trace, null, 2)
  const blob = new Blob([payload], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  const safeDomain = sanitizeFilenameSegment(trace.normalized_domain || 'trace')
  link.href = url
  link.download = `dnswatcher-${safeDomain}-${trace.qtype.toLowerCase()}.json`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
