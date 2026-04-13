import type { TraceError, TraceRequest, TraceResult } from './types'

export class TraceRequestError extends Error {
  code: string
  status: number

  constructor(message: string, code: string, status: number) {
    super(message)
    this.name = 'TraceRequestError'
    this.code = code
    this.status = status
  }
}

export async function runTrace(request: TraceRequest): Promise<TraceResult> {
  const response = await fetch('/api/v1/traces', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  })

  if (!response.ok) {
    const error = (await safeJSON(response)) as TraceError | null
    throw new TraceRequestError(
      error?.message ?? 'The trace request failed.',
      error?.error ?? 'unknown_error',
      response.status,
    )
  }

  return (await response.json()) as TraceResult
}

async function safeJSON(response: Response) {
  try {
    return await response.json()
  } catch {
    return null
  }
}
