import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App from './App'
import { traceWithSupportFixture } from './fixtures/traceFixtures'

describe('App', () => {
  it('renders a trace and exposes support steps', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(traceWithSupportFixture), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    render(<App />)

    fireEvent.change(screen.getByLabelText('Domain'), {
      target: { value: 'service.example.com' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Run trace' }))

    await waitFor(() => {
      expect(screen.getByText('Trace timeline')).toBeInTheDocument()
    })

    expect(screen.getByText('1 support step(s)')).toBeInTheDocument()
    fireEvent.click(
      screen.getAllByRole('button', { name: /ns\.outside\.net\./i })[0]!,
    )
    expect(screen.getByText(/The UDP response was truncated/)).toBeInTheDocument()
  })

  it('renders a structured invalid-domain failure before any trace result exists', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify({
          error: 'invalid_domain_input',
          message: 'invalid domain input',
        }),
        {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        },
      ),
    )

    render(<App />)
    fireEvent.change(screen.getByLabelText('Domain'), {
      target: { value: 'bad_domain.example.com' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Run trace' }))

    await waitFor(() => {
      expect(screen.getByText('Invalid domain input')).toBeInTheDocument()
    })
    expect(
      screen.getByText(/The input failed validation and no DNS trace was run/),
    ).toBeInTheDocument()
  })
})
