import type { TraceResult } from '../lib/api/types'
import type { RecentTraceMetadata } from '../lib/storage/recentTraces'

export type TraceMode = 'beginner' | 'advanced'
export type ViewState = 'idle' | 'validating' | 'loading' | 'success' | 'failure'

export type SamplePreset = {
  domain: string
  qtype: 'A' | 'AAAA' | 'NS'
}

export type AppState = {
  viewState: ViewState
  form: SamplePreset
  mode: TraceMode
  trace: TraceResult | null
  errorMessage: string | null
  errorCode: string | null
  rerunInProgress: boolean
  exportInProgress: boolean
  recent: RecentTraceMetadata[]
  selectedHopIndex: number | null
}

export const EMPTY_FORM: SamplePreset = {
  domain: '',
  qtype: 'A',
}

export const initialState: AppState = {
  viewState: 'idle',
  form: EMPTY_FORM,
  mode: 'beginner',
  trace: null,
  errorMessage: null,
  errorCode: null,
  rerunInProgress: false,
  exportInProgress: false,
  recent: [],
  selectedHopIndex: null,
}

type Action =
  | { type: 'domainChanged'; domain: string }
  | { type: 'qtypeChanged'; qtype: 'A' | 'AAAA' | 'NS' }
  | { type: 'submitStarted' }
  | { type: 'traceSucceeded'; trace: TraceResult; recent: RecentTraceMetadata[] }
  | { type: 'traceFailed'; message: string; errorCode: string }
  | { type: 'validationFailed'; message: string; errorCode: string }
  | { type: 'modeChanged'; mode: TraceMode }
  | { type: 'hopSelected'; hopIndex: number }
  | { type: 'rerunStarted' }
  | { type: 'exportStarted' }
  | { type: 'exportFinished' }
  | { type: 'recentLoaded'; recent: RecentTraceMetadata[] }
  | { type: 'presetApplied'; preset: SamplePreset }
  | { type: 'recentReused'; recent: RecentTraceMetadata }
  | { type: 'backToSearch' }

export function reducer(state: AppState, action: Action): AppState {
  switch (action.type) {
    case 'domainChanged':
      return { ...state, form: { ...state.form, domain: action.domain } }
    case 'qtypeChanged':
      return { ...state, form: { ...state.form, qtype: action.qtype } }
    case 'submitStarted':
      return {
        ...state,
        viewState: 'loading',
        errorMessage: null,
        rerunInProgress: false,
      }
    case 'traceSucceeded':
      return {
        ...state,
        viewState: 'success',
        trace: action.trace,
        errorMessage: null,
        errorCode: null,
        rerunInProgress: false,
        recent: action.recent,
        selectedHopIndex: action.trace.hops[0]?.index ?? null,
      }
    case 'traceFailed':
      return {
        ...state,
        viewState: 'failure',
        errorMessage: action.message,
        errorCode: action.errorCode,
        rerunInProgress: false,
        trace: null,
      }
    case 'validationFailed':
      return {
        ...state,
        viewState: 'failure',
        errorMessage: action.message,
        errorCode: action.errorCode,
        trace: null,
      }
    case 'modeChanged':
      return { ...state, mode: action.mode }
    case 'hopSelected':
      return { ...state, selectedHopIndex: action.hopIndex }
    case 'rerunStarted':
      return {
        ...state,
        rerunInProgress: true,
        viewState: 'loading',
      }
    case 'exportStarted':
      return { ...state, exportInProgress: true }
    case 'exportFinished':
      return { ...state, exportInProgress: false }
    case 'recentLoaded':
      return { ...state, recent: action.recent }
    case 'presetApplied':
      return { ...state, form: action.preset }
    case 'recentReused':
      return {
        ...state,
        form: { domain: action.recent.domain, qtype: action.recent.qtype },
      }
    case 'backToSearch':
      return {
        ...state,
        trace: null,
        viewState: 'idle',
        selectedHopIndex: null,
        errorMessage: null,
        errorCode: null,
      }
    default:
      return state
  }
}
