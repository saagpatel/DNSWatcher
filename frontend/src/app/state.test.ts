import { describe, expect, it } from 'vitest'
import { initialState, reducer } from './state'
import { traceWithSupportFixture } from '../fixtures/traceFixtures'

describe('reducer', () => {
  it('transitions into success with the first hop selected', () => {
    const state = reducer(initialState, {
      type: 'traceSucceeded',
      trace: traceWithSupportFixture,
      recent: [],
    })

    expect(state.viewState).toBe('success')
    expect(state.selectedHopIndex).toBe(0)
  })

  it('tracks rerun and export flags', () => {
    let state = reducer(initialState, { type: 'rerunStarted' })
    expect(state.rerunInProgress).toBe(true)
    state = reducer(state, { type: 'exportStarted' })
    expect(state.exportInProgress).toBe(true)
    state = reducer(state, { type: 'exportFinished' })
    expect(state.exportInProgress).toBe(false)
  })

  it('stores structured failure codes for non-trace failures', () => {
    const state = reducer(initialState, {
      type: 'validationFailed',
      message: 'invalid domain input',
      errorCode: 'invalid_domain_input',
    })

    expect(state.viewState).toBe('failure')
    expect(state.errorCode).toBe('invalid_domain_input')
    expect(state.trace).toBeNull()
  })
})
