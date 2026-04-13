import { describe, expect, it, vi } from 'vitest'
import { exportTraceResult } from './exportTrace'
import { traceWithSupportFixture } from '../fixtures/traceFixtures'

describe('exportTraceResult', () => {
  it('creates a JSON download from the trace result', () => {
    const createObjectURL = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue('blob:test')
    const revokeObjectURL = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    exportTraceResult(traceWithSupportFixture)

    expect(createObjectURL).toHaveBeenCalled()
    expect(click).toHaveBeenCalled()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:test')
  })
})
