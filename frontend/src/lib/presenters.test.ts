import { describe, expect, it } from 'vitest'
import { traceWithSupportFixture } from '../fixtures/traceFixtures'
import type { Hop } from './api/types'
import { groupHopsByParent } from './presenters'

describe('groupHopsByParent', () => {
  it('groups support lookups under their triggering hop', () => {
    const grouped = groupHopsByParent(traceWithSupportFixture.hops)
    const targetGroup = grouped.find((group) => group.parent.index === 1)
    expect(targetGroup?.supportHops).toHaveLength(1)
    expect(targetGroup?.supportHops[0]?.hop.hop_purpose).toBe(
      'nameserver_address_lookup',
    )
  })

  it('keeps nested support hops visible under the top-level triggering hop', () => {
    const baseSupportHop = traceWithSupportFixture.hops[2] as Hop
    const nestedSupportHop: Hop = {
      ...baseSupportHop,
      index: 4,
      parent_index: 2,
      qname: 'ns.deep.outside.net.',
      server_name: 'ns.deep.outside.net.',
    }

    const grouped = groupHopsByParent([
      ...traceWithSupportFixture.hops,
      nestedSupportHop,
    ])

    const targetGroup = grouped.find((group) => group.parent.index === 1)
    expect(targetGroup?.supportHops).toHaveLength(2)
    expect(targetGroup?.supportHops[1]?.hop.index).toBe(4)
    expect(targetGroup?.supportHops[1]?.depth).toBe(2)
  })
})
