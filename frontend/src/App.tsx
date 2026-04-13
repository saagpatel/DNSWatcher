import { startTransition, useEffect, useMemo, useReducer } from 'react'
import type { FormEvent } from 'react'
import './App.css'
import { runTrace, TraceRequestError } from './lib/api/client'
import { exportTraceResult } from './lib/exportTrace'
import { initialState, reducer } from './app/state'
import type { SamplePreset, TraceMode } from './app/state'
import {
  loadRecentTraces,
  saveRecentTrace,
} from './lib/storage/recentTraces'
import type { RecentTraceMetadata } from './lib/storage/recentTraces'
import {
  FailureCopy,
  groupHopsByParent,
} from './lib/presenters'
import type { GroupedSupportHop } from './lib/presenters'
import type { Hop, TraceResult } from './lib/api/types'

const SAMPLE_PRESETS: SamplePreset[] = [
  { domain: 'example.com', qtype: 'A' },
  { domain: 'www.github.com', qtype: 'A' },
  { domain: 'nonexistent-subdomain.example.com', qtype: 'A' },
]

const TRUTH_NOTE =
  'Traces are performed by the backend trace service. Timings reflect that service’s network path, not your device’s resolver path.'

function App() {
  const [state, dispatch] = useReducer(reducer, initialState)

  useEffect(() => {
    dispatch({ type: 'recentLoaded', recent: loadRecentTraces() })
  }, [])

  const groupedHops = useMemo(
    () => groupHopsByParent(state.trace?.hops ?? []),
    [state.trace?.hops],
  )

  const selectedHop = useMemo(() => {
    if (!state.trace || state.selectedHopIndex === null) {
      return null
    }
    return state.trace.hops.find((hop) => hop.index === state.selectedHopIndex) ?? null
  }, [state.selectedHopIndex, state.trace])

  const run = async (
    form: { domain: string; qtype: 'A' | 'AAAA' | 'NS' },
    rerun = false,
  ) => {
    if (!form.domain.trim()) {
      dispatch({
        type: 'validationFailed',
        message: 'Enter a domain name before running the trace.',
        errorCode: 'invalid_domain_input',
      })
      return
    }
    dispatch({ type: rerun ? 'rerunStarted' : 'submitStarted' })
    try {
      const trace = await runTrace(form)
      const recent = saveRecentTrace(
        {
          domain: trace.normalized_domain,
          qtype: trace.qtype,
          timestamp: trace.completed_at,
          total_duration_ms: trace.total_duration_ms,
          status: trace.status,
        },
        state.recent,
      )
      startTransition(() => {
        dispatch({ type: 'traceSucceeded', trace, recent })
      })
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : 'The trace request could not be completed.'
      startTransition(() => {
        dispatch({
          type: 'traceFailed',
          message,
          errorCode:
            error instanceof TraceRequestError ? error.code : 'request_failed',
        })
      })
    }
  }

  const onSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    await run(state.form)
  }

  const onPreset = async (preset: SamplePreset) => {
    dispatch({ type: 'presetApplied', preset })
    await run(preset)
  }

  const onRecent = async (recent: RecentTraceMetadata) => {
    dispatch({ type: 'recentReused', recent })
    await run({ domain: recent.domain, qtype: recent.qtype })
  }

  const onExport = async () => {
    if (!state.trace) {
      return
    }
    dispatch({ type: 'exportStarted' })
    try {
      exportTraceResult(state.trace)
    } finally {
      dispatch({ type: 'exportFinished' })
    }
  }

  const setMode = (mode: TraceMode) => dispatch({ type: 'modeChanged', mode })

  return (
    <div className="app-shell">
      <main className="app-frame">
        <section className="hero-panel">
          <p className="eyebrow">DNSWatcher</p>
          <h1>DNS at a glance</h1>
          <p className="lede">
            Run a real iterative DNS trace and follow referrals from root to
            answer.
          </p>
          <p className="truth-note">{TRUTH_NOTE}</p>
          <form className="query-form" onSubmit={onSubmit}>
            <label>
              Domain
              <input
                value={state.form.domain}
                onChange={(event) =>
                  dispatch({
                    type: 'domainChanged',
                    domain: event.currentTarget.value,
                  })
                }
                placeholder="example.com"
                autoComplete="off"
                spellCheck={false}
              />
            </label>
            <label>
              Query type
              <select
                value={state.form.qtype}
                onChange={(event) =>
                  dispatch({
                    type: 'qtypeChanged',
                    qtype: event.currentTarget.value as 'A' | 'AAAA' | 'NS',
                  })
                }
              >
                <option value="A">A</option>
                <option value="AAAA">AAAA</option>
                <option value="NS">NS</option>
              </select>
            </label>
            <button
              className="primary-action"
              type="submit"
              disabled={state.viewState === 'loading' || state.rerunInProgress}
            >
              {state.viewState === 'loading' ? 'Running trace...' : 'Run trace'}
            </button>
          </form>

          <div className="sample-strip">
            <p>Sample presets</p>
            <div className="chip-list">
              {SAMPLE_PRESETS.map((preset) => (
                <button
                  key={`${preset.domain}-${preset.qtype}`}
                  className="chip-button"
                  type="button"
                  onClick={() => void onPreset(preset)}
                >
                  {preset.domain} / {preset.qtype}
                </button>
              ))}
            </div>
          </div>

          <div className="recent-strip">
            <div className="section-heading">
              <h2>Recent traces</h2>
              <span>Metadata only</span>
            </div>
            {state.recent.length === 0 ? (
              <p className="muted">No recent traces yet.</p>
            ) : (
              <ul className="recent-list">
                {state.recent.map((recent) => (
                  <li key={`${recent.domain}-${recent.timestamp}-${recent.qtype}`}>
                    <button type="button" onClick={() => void onRecent(recent)}>
                      <span>{recent.domain}</span>
                      <span>
                        {recent.qtype} · {recent.status} · {recent.total_duration_ms}
                        ms
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>

        <section className="workspace-panel">
          <header className="workspace-header">
            <div>
              <p className="eyebrow">Trace workspace</p>
              <h2>
                {state.trace
                  ? `${state.trace.normalized_domain} / ${state.trace.qtype}`
                  : 'Run a trace to inspect the delegation path'}
              </h2>
            </div>
            <div className="toolbar">
              <button
                type="button"
                onClick={() => void run(state.form, true)}
                disabled={!state.trace}
              >
                {state.rerunInProgress ? 'Re-running...' : 'Re-run trace'}
              </button>
              <button
                type="button"
                onClick={() => void onExport()}
                disabled={!state.trace}
              >
                {state.exportInProgress ? 'Exporting...' : 'Export JSON'}
              </button>
              <div className="mode-toggle" role="group" aria-label="Detail mode">
                <button
                  type="button"
                  data-active={state.mode === 'beginner'}
                  onClick={() => setMode('beginner')}
                >
                  Beginner
                </button>
                <button
                  type="button"
                  data-active={state.mode === 'advanced'}
                  onClick={() => setMode('advanced')}
                >
                  Advanced
                </button>
              </div>
              <button type="button" onClick={() => dispatch({ type: 'backToSearch' })}>
                Back to search
              </button>
            </div>
          </header>

          {state.trace ? (
            <TraceWorkspace
              mode={state.mode}
              trace={state.trace}
              groupedHops={groupedHops}
              selectedHop={selectedHop}
              selectedHopIndex={state.selectedHopIndex}
              onSelect={(hopIndex) =>
                dispatch({ type: 'hopSelected', hopIndex: hopIndex })
              }
            />
          ) : state.viewState === 'failure' ? (
            <FailureState
              errorCode={state.errorCode}
              message={state.errorMessage}
            />
          ) : (
            <EmptyState />
          )}
        </section>
      </main>
    </div>
  )
}

type GroupedHop = {
  parent: Hop
  supportHops: GroupedSupportHop[]
}

type TraceWorkspaceProps = {
  mode: TraceMode
  trace: TraceResult
  groupedHops: GroupedHop[]
  selectedHop: Hop | null
  selectedHopIndex: number | null
  onSelect: (hopIndex: number) => void
}

function TraceWorkspace({
  mode,
  trace,
  groupedHops,
  selectedHop,
  selectedHopIndex,
  onSelect,
}: TraceWorkspaceProps) {
  const failureCopy =
    trace.final_outcome.kind === 'success'
      ? null
      : FailureCopy[trace.final_outcome.kind] ??
        FailureCopy.servfail

  return (
    <div className="workspace-grid">
      <section className="timeline-panel">
        <div className="section-heading">
          <h3>Trace timeline</h3>
          <span>{trace.total_duration_ms} ms total</span>
        </div>
        <p className="trace-note">
          Repeated traces may differ because live DNS data and network paths can
          change.
        </p>
        <ol className="timeline-list">
          {groupedHops.map(({ parent, supportHops }) => (
            <li key={parent.index}>
              <button
                type="button"
                className="hop-card"
                data-selected={selectedHopIndex === parent.index}
                onClick={() => onSelect(parent.index)}
              >
                <div className="hop-meta">
                  <span className="hop-index">#{parent.index}</span>
                  <span className={`role-badge role-${parent.role}`}>
                    {displayRole(parent.role)}
                  </span>
                  <span className="latency-pill">{parent.latency_ms} ms</span>
                </div>
                <strong>{parent.server_name}</strong>
                <p>{parent.server_ip}</p>
                <p>
                  {parent.qname} / {parent.qtype}
                </p>
                <p>{displayResponseKind(parent.response_kind)}</p>
              </button>
              {supportHops.length > 0 ? (
                <details className="support-hops" open>
                  <summary>{supportHops.length} support step(s)</summary>
                  <ul>
                    {supportHops.map((hop) => (
                      <li key={hop.hop.index}>
                        <button
                          type="button"
                          onClick={() => onSelect(hop.hop.index)}
                          data-selected={selectedHopIndex === hop.hop.index}
                          style={{ paddingLeft: `${1 + (hop.depth - 1) * 0.85}rem` }}
                        >
                          <span>#{hop.hop.index}</span>
                          <span>{hop.hop.server_name}</span>
                          <span>{displayResponseKind(hop.hop.response_kind)}</span>
                          <span>{hop.hop.latency_ms} ms</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                </details>
              ) : null}
            </li>
          ))}
        </ol>
      </section>

      <section className="details-panel">
        <div className="section-heading">
          <h3>Details</h3>
          <span>{mode === 'beginner' ? 'Beginner mode' : 'Advanced mode'}</span>
        </div>
        {failureCopy ? (
          <div className="failure-callout">
            <h4>{failureCopy.title}</h4>
            <dl>
              <div>
                <dt>What happened?</dt>
                <dd>{failureCopy.what}</dd>
              </div>
              <div>
                <dt>Where did it happen?</dt>
                <dd>{failureCopy.where}</dd>
              </div>
              <div>
                <dt>What does it mean?</dt>
                <dd>{failureCopy.meaning}</dd>
              </div>
              <div>
                <dt>Why did the trace stop?</dt>
                <dd>{failureCopy.stop}</dd>
              </div>
            </dl>
          </div>
        ) : null}

        {selectedHop ? (
          mode === 'beginner' ? (
            <dl className="details-grid">
              <div>
                <dt>What was asked?</dt>
                <dd>
                  {selectedHop.qname} / {selectedHop.qtype}
                </dd>
              </div>
              <div>
                <dt>Who answered?</dt>
                <dd>
                  {selectedHop.server_name} ({selectedHop.server_ip})
                </dd>
              </div>
              <div>
                <dt>What came back?</dt>
                <dd>{selectedHop.explanation}</dd>
              </div>
              <div>
                <dt>Why did the trace continue or stop?</dt>
                <dd>{selectedHop.technical_note}</dd>
              </div>
            </dl>
          ) : (
            <dl className="details-grid">
              <div>
                <dt>QNAME</dt>
                <dd>{selectedHop.qname}</dd>
              </div>
              <div>
                <dt>QTYPE</dt>
                <dd>{selectedHop.qtype}</dd>
              </div>
              <div>
                <dt>Queried server</dt>
                <dd>{selectedHop.server_name}</dd>
              </div>
              <div>
                <dt>Queried server IP</dt>
                <dd>{selectedHop.server_ip}</dd>
              </div>
              <div>
                <dt>Transport and latency</dt>
                <dd>
                  {selectedHop.transport.toUpperCase()} · {selectedHop.latency_ms}
                  ms
                </dd>
              </div>
              <div>
                <dt>Response code</dt>
                <dd>{selectedHop.response_code}</dd>
              </div>
              <div>
                <dt>Authoritative / truncated</dt>
                <dd>
                  {selectedHop.authoritative ? 'authoritative' : 'non-authoritative'}
                  {' / '}
                  {selectedHop.truncated ? 'truncated' : 'complete'}
                </dd>
              </div>
              <div>
                <dt>Answer summary</dt>
                <dd>{renderRRsets(selectedHop.answer_rrsets)}</dd>
              </div>
              <div>
                <dt>Authority summary</dt>
                <dd>{renderRRsets(selectedHop.authority_rrsets)}</dd>
              </div>
              <div>
                <dt>Additional summary</dt>
                <dd>{renderRRsets(selectedHop.additional_rrsets)}</dd>
              </div>
              <div>
                <dt>Next targets</dt>
                <dd>
                  {selectedHop.next_targets.length > 0 ? (
                    <ul className="inline-list">
                      {selectedHop.next_targets.map((target) => (
                        <li key={`${target.server_name}-${target.server_ip}`}>
                          {target.server_name} ({target.server_ip}) · {target.reason}
                        </li>
                      ))}
                    </ul>
                  ) : (
                    'No next targets'
                  )}
                </dd>
              </div>
              <div>
                <dt>Technical note</dt>
                <dd>{selectedHop.technical_note}</dd>
              </div>
            </dl>
          )
        ) : (
          <p className="muted">Select a hop to inspect its details.</p>
        )}

        <div className="truth-list">
          <h4>Truth notes</h4>
          <ul>
            {trace.truth_notes.map((note) => (
              <li key={note.code}>{note.message}</li>
            ))}
          </ul>
        </div>
      </section>
    </div>
  )
}

function FailureState({
  errorCode,
  message,
}: {
  errorCode: string | null
  message: string | null
}) {
  const copy = standaloneFailureCopy(errorCode, message)
  return (
    <div className="empty-state failure-state">
      <h3>{copy.title}</h3>
      <dl className="details-grid">
        <div>
          <dt>What happened?</dt>
          <dd>{copy.what}</dd>
        </div>
        <div>
          <dt>Where did it happen?</dt>
          <dd>{copy.where}</dd>
        </div>
        <div>
          <dt>What does it mean?</dt>
          <dd>{copy.meaning}</dd>
        </div>
        <div>
          <dt>Why did the trace stop?</dt>
          <dd>{copy.stop}</dd>
        </div>
      </dl>
    </div>
  )
}

function EmptyState() {
  return (
    <div className="empty-state">
      <h3>No trace yet</h3>
      <p>
        Run a trace to see the real referral path, timings, support lookups, and
        terminal outcome.
      </p>
    </div>
  )
}

function renderRRsets(rrsets: TraceResult['hops'][number]['answer_rrsets']) {
  if (rrsets.length === 0) {
    return 'None'
  }
  return (
    <ul className="inline-list">
      {rrsets.map((rrset) => (
        <li key={`${rrset.section}-${rrset.name}-${rrset.type}`}>
          {rrset.name} {rrset.type} TTL {rrset.ttl} → {rrset.data.join(', ')}
        </li>
      ))}
    </ul>
  )
}

function displayRole(role: Hop['role']) {
  return (
    {
      root: 'Root',
      tld: 'TLD',
      authoritative: 'Authoritative',
      alias: 'Alias',
      final: 'Final',
      error: 'Error',
    }[role] ?? role
  )
}

function displayResponseKind(kind: Hop['response_kind']) {
  return (
    {
      referral: 'Referral',
      answer: 'Answer',
      cname: 'CNAME',
      nodata: 'No data',
      error: 'Error',
    }[kind] ?? kind
  )
}

function standaloneFailureCopy(errorCode: string | null, message: string | null) {
  if (errorCode === 'invalid_domain_input') {
    return {
      title: 'Invalid domain input',
      what: message ?? 'The entered domain is not valid for this service.',
      where: 'Before the backend trace started.',
      meaning: 'The input failed validation and no DNS trace was run.',
      stop: 'The service rejected the request instead of sending malformed DNS queries upstream.',
    }
  }
  if (errorCode === 'rate_limited' || errorCode === 'concurrency_limited') {
    return {
      title: 'Rate limited',
      what: message ?? 'The service is receiving more trace requests than it allows right now.',
      where: 'At the API boundary before the trace started.',
      meaning: 'The backend is protecting itself from overload.',
      stop: 'The request was intentionally delayed instead of risking degraded service.',
    }
  }
  return {
    title: 'Trace request failed',
    what: message ?? 'The trace request could not be completed.',
    where: 'At the app or API boundary before a full trace result was returned.',
    meaning: 'The service did not return a modeled DNS result for this request.',
    stop: 'The request stopped before a trace could be rendered safely.',
  }
}

export default App
