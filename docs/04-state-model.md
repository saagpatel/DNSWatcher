# Frontend State Model

## View state

- `idle`
- `validating`
- `loading`
- `success`
- `failure`

## UI mode

- `beginner`
- `advanced`

## Transient flags

- `rerunInProgress`
- `exportInProgress`

## Recent traces

Stored under `dnswatcher.recent_traces.v1` with metadata only:

- `domain`
- `qtype`
- `timestamp`
- `total_duration_ms`
- `status`

Selecting a recent trace repopulates the form and performs a new trace request.
