import { startTransition, useEffect, useMemo, useReducer } from 'react'
import type { FormEvent, ReactNode } from 'react'
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
import { FailureCopy, groupHopsByParent } from './lib/presenters'
import type { GroupedSupportHop } from './lib/presenters'
import type { Hop, TraceResult } from './lib/api/types'

const SAMPLE_PRESETS: SamplePreset[] = [
  { domain: 'example.com', qtype: 'A' },
  { domain: 'www.github.com', qtype: 'A' },
  { domain: 'nonexistent-subdomain.example.com', qtype: 'A' },
]

const TRUTH_NOTE =
  'This is a backend-run iterative trace. It is not your device resolver path, not packet capture, and not fabricated animation.'

const SCOPE_NOTE =
  'V1 supports A, AAAA, and NS. QNAME minimization is intentionally deferred and every live result can vary.'

const SOURCE_CARDS = [
  {
    title: 'RFC 1034',
    href: 'https://www.rfc-editor.org/rfc/rfc1034',
    label: 'DNS concepts, delegation, and authoritative data.',
  },
  {
    title: 'RFC 1035',
    href: 'https://www.rfc-editor.org/rfc/rfc1035',
    label: 'DNS messages, response codes, truncation, and resource records.',
  },
  {
    title: 'RFC 9210',
    href: 'https://www.rfc-editor.org/rfc/rfc9210',
    label: 'Why DNS over TCP is part of modern resolver behavior.',
  },
  {
    title: 'IANA Root Hints',
    href: 'https://www.iana.org/domains/root/files',
    label: 'The root server hints used to start an iterative trace.',
  },
  {
    title: 'WCAG 2.2',
    href: 'https://www.w3.org/TR/WCAG22/#animation-from-interactions',
    label: 'Interaction animation must be avoidable and non-essential.',
  },
  {
    title: 'MDN reduced motion',
    href: 'https://developer.mozilla.org/en-US/docs/Web/CSS/@media/prefers-reduced-motion',
    label: 'The UI respects the user motion preference.',
  },
]

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

  const statusMessage = state.trace
    ? `${state.trace.final_outcome.message} ${state.trace.summary.total_hops} trace hops.`
    : state.viewState === 'loading'
      ? 'Running DNS trace.'
      : state.viewState === 'failure'
        ? state.errorMessage ?? 'Trace request failed.'
        : 'Ready to run a DNS trace.'

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
        <section className="hero-panel" aria-labelledby="product-title">
          <p className="eyebrow">Systems Explainer Arcade flagship</p>
          <h1 id="product-title">DNS: Follow the Question</h1>
          <p className="lede">
            Ask for a record, then follow the real delegation chain from root
            hints through referrals, glue, support lookups, CNAME restarts, and
            final DNS outcomes.
          </p>
          <div className="truth-note" role="note">
            <p>{TRUTH_NOTE}</p>
            <p>{SCOPE_NOTE}</p>
          </div>
          <form
            className="query-form"
            onSubmit={onSubmit}
            aria-describedby="query-truth"
          >
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
              {state.viewState === 'loading'
                ? 'Following the question...'
                : 'Follow the question'}
            </button>
          </form>
          <p id="query-truth" className="small-note">
            Queries run from the service, not from your browser or local
            resolver.
          </p>

          <div className="sample-strip">
            <p className="section-kicker">Try a path</p>
            <div className="chip-list" aria-label="Sample trace presets">
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

        <section className="workspace-panel" aria-labelledby="workspace-title">
          <header className="workspace-header">
            <div>
              <p className="eyebrow">Trace workspace</p>
              <h2 id="workspace-title">
                {state.trace
                  ? `${state.trace.normalized_domain} / ${state.trace.qtype}`
                  : 'Run a trace to inspect the delegation path'}
              </h2>
            </div>
            <div className="toolbar" aria-label="Trace actions">
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
                {state.exportInProgress ? 'Exporting...' : 'Export raw JSON'}
              </button>
              <div className="mode-toggle" role="group" aria-label="Detail mode">
                <button
                  type="button"
                  aria-pressed={state.mode === 'beginner'}
                  data-active={state.mode === 'beginner'}
                  onClick={() => setMode('beginner')}
                >
                  Beginner
                </button>
                <button
                  type="button"
                  aria-pressed={state.mode === 'advanced'}
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

          <p className="sr-only" role="status" aria-live="polite">
            {statusMessage}
          </p>

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
      : FailureCopy[trace.final_outcome.kind] ?? FailureCopy.servfail

  return (
    <div className="workspace-grid">
      <section className="journey-panel" aria-labelledby="journey-title">
        <div className="section-heading">
          <h3 id="journey-title">Question path</h3>
          <span>{trace.total_duration_ms} ms</span>
        </div>
        <Journey trace={trace} />
        <OutcomePanel trace={trace} />
        <SourceCards />
      </section>

      <section className="timeline-panel" aria-labelledby="timeline-title">
        <div className="section-heading">
          <h3 id="timeline-title">Trace timeline</h3>
          <span>{trace.summary.total_hops} hops</span>
        </div>
        <p className="trace-note">
          Repeated traces may differ because live DNS data and network paths can
          change.
        </p>
        <ol className="timeline-list" aria-label="DNS trace hops">
          {groupedHops.map(({ parent, supportHops }) => (
            <li key={parent.index}>
              <button
                type="button"
                className="hop-card"
                data-selected={selectedHopIndex === parent.index}
                data-state={hopState(parent).kind}
                aria-current={selectedHopIndex === parent.index ? 'step' : undefined}
                aria-label={hopAriaLabel(parent)}
                onClick={() => onSelect(parent.index)}
              >
                <span className="hop-card-topline">
                  <span className="hop-index">#{parent.index}</span>
                  <span className={`role-badge role-${parent.role}`}>
                    {displayRole(parent.role)}
                  </span>
                  <span className="state-badge">{hopState(parent).label}</span>
                  <span className="latency-pill">{parent.latency_ms} ms</span>
                </span>
                <strong>{parent.server_name}</strong>
                <span>{parent.server_ip}</span>
                <span>
                  {parent.qname} / {parent.qtype}
                </span>
                <span>{displayResponseKind(parent.response_kind)}</span>
              </button>
              {supportHops.length > 0 ? (
                <details className="support-hops" open>
                  <summary>
                    {supportHops.length} real nameserver-address support step(s)
                  </summary>
                  <ul>
                    {supportHops.map((hop) => (
                      <li key={hop.hop.index}>
                        <button
                          type="button"
                          onClick={() => onSelect(hop.hop.index)}
                          data-selected={selectedHopIndex === hop.hop.index}
                          data-state={hopState(hop.hop).kind}
                          aria-current={
                            selectedHopIndex === hop.hop.index ? 'step' : undefined
                          }
                          aria-label={hopAriaLabel(hop.hop)}
                          style={{ paddingLeft: `${1 + (hop.depth - 1) * 0.85}rem` }}
                        >
                          <span>#{hop.hop.index}</span>
                          <span>{hop.hop.server_name}</span>
                          <span>{hopState(hop.hop).label}</span>
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

      <section className="details-panel" aria-labelledby="details-title">
        <div className="section-heading">
          <h3 id="details-title">Details</h3>
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
            <BeginnerDetails hop={selectedHop} />
          ) : (
            <AdvancedDetails hop={selectedHop} />
          )
        ) : (
          <p className="muted">Select a hop to inspect its details.</p>
        )}

        <div className="truth-list" role="note" aria-label="Trace truth notes">
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

function Journey({ trace }: { trace: TraceResult }) {
  const sawRoot = trace.hops.some((hop) => hop.role === 'root')
  const sawReferral = trace.hops.some((hop) => hop.response_kind === 'referral')
  const sawSupport = trace.hops.some(
    (hop) => hop.hop_purpose === 'nameserver_address_lookup',
  )
  const sawCNAME = trace.hops.some((hop) => hop.response_kind === 'cname')
  const finalHop = trace.hops.find(
    (hop) => hop.index === trace.final_outcome.terminal_hop_index,
  )

  const steps = [
    {
      title: 'Question',
      state: 'complete',
      body: `${trace.normalized_domain} / ${trace.qtype}`,
    },
    {
      title: 'Root',
      state: sawRoot ? 'complete' : 'waiting',
      body: sawRoot ? 'The trace started from configured root hints.' : 'No root hop returned.',
    },
    {
      title: 'Referral',
      state: sawReferral ? 'complete' : 'skipped',
      body: sawReferral
        ? 'Each referral narrowed the authority for the name.'
        : 'No referral was needed or returned.',
    },
    {
      title: 'Support',
      state: sawSupport ? 'complete' : 'skipped',
      body: sawSupport
        ? 'Missing glue triggered visible nameserver-address lookups.'
        : 'Glue or direct targets were enough.',
    },
    {
      title: 'CNAME',
      state: sawCNAME ? 'complete' : 'skipped',
      body: sawCNAME
        ? 'An alias restarted the question at the canonical name.'
        : 'No alias restart happened.',
    },
    {
      title: 'Stop',
      state: trace.final_outcome.kind === 'success' ? 'complete' : 'alert',
      body: finalHop
        ? `${trace.final_outcome.kind}: ${hopState(finalHop).label}`
        : trace.final_outcome.message,
    },
  ]

  return (
    <ol className="journey-list" aria-label="High-level DNS question path">
      {steps.map((step) => (
        <li key={step.title} data-state={step.state}>
          <span className="journey-dot" aria-hidden="true" />
          <div>
            <strong>{step.title}</strong>
            <p>{step.body}</p>
          </div>
        </li>
      ))}
    </ol>
  )
}

function OutcomePanel({ trace }: { trace: TraceResult }) {
  return (
    <section className="outcome-panel" aria-labelledby="outcome-title">
      <p className="section-kicker">What this run proved</p>
      <h3 id="outcome-title">{trace.summary.headline}</h3>
      <p>{trace.summary.detail}</p>
      <dl className="metric-row">
        <div>
          <dt>Answers</dt>
          <dd>{trace.summary.answer_count}</dd>
        </div>
        <div>
          <dt>CNAMEs</dt>
          <dd>{trace.summary.cname_count}</dd>
        </div>
        <div>
          <dt>Terminal hop</dt>
          <dd>#{trace.final_outcome.terminal_hop_index}</dd>
        </div>
      </dl>
    </section>
  )
}

function SourceCards() {
  return (
    <section className="source-panel" aria-labelledby="sources-title">
      <div className="section-heading">
        <h3 id="sources-title">Sources</h3>
        <span>Official</span>
      </div>
      <ul className="source-list">
        {SOURCE_CARDS.map((source) => (
          <li key={source.href}>
            <a href={source.href} target="_blank" rel="noreferrer">
              <strong>{source.title}</strong>
              <span>{source.label}</span>
            </a>
          </li>
        ))}
      </ul>
    </section>
  )
}

function BeginnerDetails({ hop }: { hop: Hop }) {
  return (
    <dl className="details-grid">
      <div>
        <dt>What happened?</dt>
        <dd>{hop.explanation}</dd>
      </div>
      <div>
        <dt>Why this server?</dt>
        <dd>{whyThisServer(hop)}</dd>
      </div>
      <div>
        <dt>Why next?</dt>
        <dd>{whyNext(hop)}</dd>
      </div>
      <div>
        <dt>Why stop?</dt>
        <dd>{whyStop(hop)}</dd>
      </div>
    </dl>
  )
}

function AdvancedDetails({ hop }: { hop: Hop }) {
  return (
    <dl className="details-grid">
      <DetailRow label="QNAME">{hop.qname}</DetailRow>
      <DetailRow label="QTYPE">{hop.qtype}</DetailRow>
      <DetailRow label="Queried server">{hop.server_name}</DetailRow>
      <DetailRow label="Queried server IP">{hop.server_ip}</DetailRow>
      <DetailRow label="Transport and latency">
        {hop.transport.toUpperCase()} / {hop.latency_ms} ms
      </DetailRow>
      <DetailRow label="RCODE">{hop.response_code}</DetailRow>
      <DetailRow label="AA / TC">
        {hop.authoritative ? 'AA=true' : 'AA=false'} /{' '}
        {hop.truncated ? 'TC=true' : 'TC=false'}
      </DetailRow>
      <DetailRow label="Hop purpose">{hop.hop_purpose}</DetailRow>
      <DetailRow label="Response kind">{hop.response_kind}</DetailRow>
      <DetailRow label="Answer RRsets">{renderRRsets(hop.answer_rrsets)}</DetailRow>
      <DetailRow label="Authority RRsets">
        {renderRRsets(hop.authority_rrsets)}
      </DetailRow>
      <DetailRow label="Additional RRsets">
        {renderRRsets(hop.additional_rrsets)}
      </DetailRow>
      <DetailRow label="Next targets">{renderTargets(hop)}</DetailRow>
      <DetailRow label="Technical note">{hop.technical_note}</DetailRow>
    </dl>
  )
}

function DetailRow({
  label,
  children,
}: {
  label: string
  children: ReactNode
}) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{children}</dd>
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
        Run a trace to see the real question path, timings, support lookups, and
        terminal outcome.
      </p>
      <p className="small-note">{SCOPE_NOTE}</p>
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
          {rrset.name} {rrset.type} TTL {rrset.ttl}: {rrset.data.join(', ')}
        </li>
      ))}
    </ul>
  )
}

function renderTargets(hop: Hop) {
  if (hop.next_targets.length === 0) {
    return 'No next targets'
  }
  return (
    <ul className="inline-list">
      {hop.next_targets.map((target) => (
        <li key={`${target.server_name}-${target.server_ip}`}>
          {target.server_name} ({target.server_ip}) / {target.reason}
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
      nodata: 'NODATA',
      error: 'Error',
    }[kind] ?? kind
  )
}

function hopState(hop: Hop) {
  if (hop.hop_purpose === 'nameserver_address_lookup') {
    return { kind: 'support', label: 'Support lookup' }
  }
  if (hop.transport === 'tcp' || hop.truncated) {
    return { kind: 'tcp', label: 'TCP fallback' }
  }
  if (hop.response_kind === 'cname') {
    return { kind: 'cname', label: 'CNAME restart' }
  }
  if (hop.response_kind === 'nodata') {
    return { kind: 'nodata', label: 'NODATA' }
  }
  if (hop.response_code === 'NXDOMAIN') {
    return { kind: 'nxdomain', label: 'NXDOMAIN' }
  }
  if (hop.response_code === 'REFUSED') {
    return { kind: 'refused', label: 'Refused' }
  }
  if (hop.response_code === 'TIMEOUT') {
    return { kind: 'timeout', label: 'Timeout' }
  }
  if (hop.response_code === 'MAX_DEPTH' || hop.response_code === 'BUDGET') {
    return { kind: 'max-depth', label: 'Max depth' }
  }
  if (hop.response_kind === 'error') {
    return { kind: 'unusable', label: 'Stopped' }
  }
  if (hop.response_kind === 'answer') {
    return { kind: 'answer', label: 'Answer' }
  }
  if (hop.next_targets.some((target) => target.reason.toLowerCase().includes('glue'))) {
    return { kind: 'glue', label: 'Referral with glue' }
  }
  if (
    hop.next_targets.some((target) =>
      target.reason.toLowerCase().includes('support lookup'),
    )
  ) {
    return { kind: 'support-referral', label: 'Referral via support' }
  }
  if (hop.response_kind === 'referral') {
    return { kind: 'referral', label: 'Referral' }
  }
  return { kind: 'neutral', label: displayResponseKind(hop.response_kind) }
}

function hopAriaLabel(hop: Hop) {
  const state = hopState(hop).label
  return `Hop ${hop.index}, ${displayRole(hop.role)}, ${state}, ${hop.qname} ${hop.qtype}, server ${hop.server_name} at ${hop.server_ip}, ${hop.latency_ms} milliseconds.`
}

function whyThisServer(hop: Hop) {
  if (hop.role === 'root') {
    return 'The backend starts from configured root hints for an iterative trace.'
  }
  if (hop.hop_purpose === 'nameserver_address_lookup') {
    return 'A previous referral named a nameserver but did not provide a usable address, so the backend resolved that nameserver.'
  }
  if (hop.hop_purpose === 'cname_follow') {
    return 'A CNAME changed the question, so the backend restarted the iterative path for the canonical name.'
  }
  return 'A previous referral selected this server as the next authority to ask.'
}

function whyNext(hop: Hop) {
  if (hop.next_targets.length > 0) {
    return hop.next_targets.map((target) => target.reason).join(' ')
  }
  if (hop.response_kind === 'cname') {
    return 'The answer was an alias, so the backend follows the canonical name.'
  }
  if (hop.response_kind === 'referral') {
    return 'The referral did not leave a safe usable target, so the trace cannot continue from this response.'
  }
  return 'No additional target was needed from this hop.'
}

function whyStop(hop: Hop) {
  if (hop.response_kind === 'answer') {
    return 'The requested data was returned.'
  }
  if (hop.response_kind === 'nodata') {
    return 'The name exists, but this record type was not present in the authoritative response.'
  }
  if (hop.response_code === 'NXDOMAIN') {
    return 'An authoritative server said the name does not exist.'
  }
  if (hop.response_code === 'REFUSED') {
    return 'The server refused the request, which is terminal for this trace path.'
  }
  if (hop.response_code === 'TIMEOUT') {
    return 'The timeout budget expired before a usable DNS response arrived.'
  }
  if (hop.response_kind === 'error') {
    return hop.technical_note
  }
  return 'The trace continues unless this is the terminal hop named in the final outcome.'
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
