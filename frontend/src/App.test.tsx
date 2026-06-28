import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App from './App'
import { traceWithSupportFixture } from './fixtures/traceFixtures'

describe('App', () => {
  it('renders the flagship trace journey and exposes support steps', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(traceWithSupportFixture), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    render(<App />)

    expect(screen.getByText('DNS: Follow the Question')).toBeInTheDocument()
    expect(screen.getByText(/not your device resolver path/i)).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Domain'), {
      target: { value: 'service.example.com' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Follow the question' }))

    await waitFor(() => {
      expect(screen.getByText('Trace timeline')).toBeInTheDocument()
    })

    expect(screen.getByText('Question path')).toBeInTheDocument()
    expect(screen.getByText('What this run proved')).toBeInTheDocument()
    expect(screen.getByText('RFC 1034')).toBeInTheDocument()
    expect(
      screen.getByText('1 real nameserver-address support step(s)'),
    ).toBeInTheDocument()
    expect(screen.getByText('Support lookup')).toBeInTheDocument()
    expect(screen.getByText('TCP fallback')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Hop 3.*TCP fallback/i }))
    expect(screen.getByText('The requested data was returned.')).toBeInTheDocument()
    expect(
      screen.getByText(/QNAME minimization is deferred/i),
    ).toBeInTheDocument()
  })

  it('preserves raw protocol fields in advanced mode', async () => {
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
    fireEvent.click(screen.getByRole('button', { name: 'Follow the question' }))

    await waitFor(() => {
      expect(screen.getByText('Trace timeline')).toBeInTheDocument()
    })
    fireEvent.click(screen.getByRole('button', { name: 'Advanced' }))
    fireEvent.click(screen.getByRole('button', { name: /Hop 3.*TCP fallback/i }))

    expect(screen.getByText('QNAME')).toBeInTheDocument()
    expect(screen.getByText('RCODE')).toBeInTheDocument()
    expect(screen.getByText('AA / TC')).toBeInTheDocument()
    expect(screen.getByText('Answer RRsets')).toBeInTheDocument()
    expect(screen.getByText('Next targets')).toBeInTheDocument()
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
    fireEvent.click(screen.getByRole('button', { name: 'Follow the question' }))

    await waitFor(() => {
      expect(screen.getByText('Invalid domain input')).toBeInTheDocument()
    })
    expect(
      screen.getByText(/The input failed validation and no DNS trace was run/),
    ).toBeInTheDocument()
  })
})
