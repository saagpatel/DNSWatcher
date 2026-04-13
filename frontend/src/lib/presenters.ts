import type { Hop } from './api/types'

export type GroupedSupportHop = {
  hop: Hop
  depth: number
}

export function groupHopsByParent(hops: Hop[]) {
  const hopByIndex = new Map(hops.map((hop) => [hop.index, hop]))
  const childrenByParent = new Map<number, Hop[]>()

  for (const hop of hops) {
    if (hop.parent_index == null) {
      continue
    }
    const existing = childrenByParent.get(hop.parent_index) ?? []
    existing.push(hop)
    childrenByParent.set(hop.parent_index, existing)
  }

  const roots = hops
    .filter((hop) => hop.parent_index == null || !hopByIndex.has(hop.parent_index))
    .sort((left, right) => left.index - right.index)

  return roots.map((parent) => ({
    parent,
    supportHops: flattenSupportHops(parent.index, childrenByParent, 1),
  }))
}

function flattenSupportHops(
  parentIndex: number,
  childrenByParent: Map<number, Hop[]>,
  depth: number,
): GroupedSupportHop[] {
  const directChildren = [...(childrenByParent.get(parentIndex) ?? [])].sort(
    (left, right) => left.index - right.index,
  )

  return directChildren.flatMap((hop) => [
    { hop, depth },
    ...flattenSupportHops(hop.index, childrenByParent, depth + 1),
  ])
}

export const FailureCopy: Record<
  string,
  { title: string; what: string; where: string; meaning: string; stop: string }
> = {
  nxdomain: {
    title: 'NXDOMAIN',
    what: 'The authoritative server said the name does not exist.',
    where: 'At the terminal authoritative hop.',
    meaning: 'The queried domain name is missing, not just the requested record type.',
    stop: 'There is no name to continue tracing for this query.',
  },
  servfail: {
    title: 'SERVFAIL',
    what: 'An upstream server reported a server failure.',
    where: 'At the hop shown as the terminal error.',
    meaning: 'The server could not complete the lookup successfully.',
    stop: 'The backend received a terminal failure instead of a usable referral or answer.',
  },
  timeout: {
    title: 'Timeout',
    what: 'The backend did not receive a response before its timeout budget expired.',
    where: 'At the terminal queried server or candidate set.',
    meaning: 'The trace could not continue because no timely answer arrived.',
    stop: 'The backend exhausted the allowed wait budget.',
  },
  refused: {
    title: 'REFUSED',
    what: 'The queried server refused the request.',
    where: 'At the terminal server shown in the trace.',
    meaning: 'The server chose not to answer this query.',
    stop: 'A refused response is terminal for this trace path.',
  },
  not_implemented: {
    title: 'Not implemented',
    what: 'The queried server reported that the operation is not implemented.',
    where: 'At the terminal server shown in the trace.',
    meaning: 'The server does not support the requested DNS behavior.',
    stop: 'The backend cannot continue from a not-implemented response.',
  },
  unusable_referral: {
    title: 'Unusable referral',
    what: 'The backend received a referral but could not derive a safe usable next hop.',
    where: 'After the referral hop shown in the timeline.',
    meaning: 'Glue was missing, blocked, or the support lookup could not produce a safe address.',
    stop: 'Continuing would require guessing or violating the destination policy.',
  },
  loop_detected: {
    title: 'Loop detected',
    what: 'The trace repeated a previously queried lookup path.',
    where: 'At the hop where the duplicate path was detected.',
    meaning: 'Continuing would likely repeat the same behavior without making progress.',
    stop: 'The backend stopped deliberately to avoid an infinite loop.',
  },
  max_depth: {
    title: 'Max depth exceeded',
    what: 'The trace exceeded its configured hop or upstream query budget.',
    where: 'Across the overall trace path.',
    meaning: 'The backend hit a guardrail before reaching a terminal answer.',
    stop: 'The trace stopped to contain cost and runaway recursion.',
  },
}
