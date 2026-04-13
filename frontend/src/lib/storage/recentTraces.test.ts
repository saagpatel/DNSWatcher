import { beforeEach, describe, expect, it } from 'vitest'
import {
  clearRecentTraces,
  loadRecentTraces,
  saveRecentTrace,
} from './recentTraces'

describe('recentTraces storage', () => {
  beforeEach(() => {
    clearRecentTraces()
  })

  it('stores metadata only and caps the list', () => {
    let current = loadRecentTraces()
    for (let index = 0; index < 12; index += 1) {
      current = saveRecentTrace(
        {
          domain: `example-${index}.com`,
          qtype: 'A',
          timestamp: new Date().toISOString(),
          total_duration_ms: index,
          status: 'success',
        },
        current,
      )
    }
    expect(loadRecentTraces()).toHaveLength(10)
    expect(loadRecentTraces()[0]).toHaveProperty('domain')
    expect(loadRecentTraces()[0]).not.toHaveProperty('hops')
  })
})
