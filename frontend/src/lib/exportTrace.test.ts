import { describe, expect, it, vi } from 'vitest'
import { exportTraceResult } from './exportTrace'
import { traceWithSupportFixture } from '../fixtures/traceFixtures'

describe('exportTraceResult', () => {
  it('creates a JSON download from the raw normalized trace result', async () => {
    const createObjectURL = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue('blob:test')
    const revokeObjectURL = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    exportTraceResult(traceWithSupportFixture)

    expect(createObjectURL).toHaveBeenCalled()
    const blob = createObjectURL.mock.calls[0]?.[0] as Blob
    const payload = JSON.parse(await blob.text())
    expect(payload).toEqual(traceWithSupportFixture)
    expect(payload.summary.headline).toBe('Authoritative answer returned.')
    expect(JSON.stringify(payload)).not.toContain('What happened?')
    expect(click).toHaveBeenCalled()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:test')
  })
})
