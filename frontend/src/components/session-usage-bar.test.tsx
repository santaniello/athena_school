import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SessionUsageBar } from './session-usage-bar'
import type { StudySessionUsage } from '@/lib/study'

type UsageTotals = Omit<StudySessionUsage, 'context'>

const NO_CONTEXT = { state: 'normal' as const, usedTokens: 0, contextLength: 0, estimated: false }

function usage(overrides: Partial<UsageTotals>): UsageTotals {
  return { inputTokens: 0, outputTokens: 0, cost: 0, unpricedCalls: 0, calls: 0, ...overrides }
}

describe('SessionUsageBar context meter', () => {
  it('shows used / limit and the percentage for a resolved limit', () => {
    // Given a measured context of 32k out of 128k
    render(
      <SessionUsageBar
        context={{ state: 'normal', usedTokens: 32_000, contextLength: 128_000, estimated: false }}
        usage={null}
      />,
    )

    // Then the meter shows the exact figures and 25%
    expect(screen.getByText('32k / 128k tokens · 25%')).toBeInTheDocument()
    expect(screen.getByRole('progressbar', { name: 'Context' })).toBeInTheDocument()
  })

  it('marks an estimated measurement with ~', () => {
    // Given an estimated measurement
    render(
      <SessionUsageBar
        context={{ state: 'normal', usedTokens: 32_000, contextLength: 128_000, estimated: true }}
        usage={null}
      />,
    )

    // Then the used tokens are prefixed with ~
    expect(screen.getByText('~32k / 128k tokens · 25%')).toBeInTheDocument()
  })

  it('shows only text, with no bar or percentage, when the limit is unresolved', () => {
    // Given tokens in use but no resolved context length
    render(
      <SessionUsageBar
        context={{ state: 'normal', usedTokens: 12_400, contextLength: 0, estimated: true }}
        usage={null}
      />,
    )

    // Then there is no progress bar and no percentage
    expect(screen.getByText('~12.4k tokens · limit unknown')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    expect(screen.queryByText(/%/)).not.toBeInTheDocument()
  })

  it('shows a dash when nothing has been measured yet', () => {
    // Given a never-measured context
    render(<SessionUsageBar context={NO_CONTEXT} usage={null} />)

    // Then the meter has no figures
    expect(screen.getByText('Context')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    expect(screen.getAllByText('—').length).toBeGreaterThan(0)
  })

  it('caps the bar at 100% when usage exceeds the limit', () => {
    // Given usage above the limit
    render(
      <SessionUsageBar
        context={{
          state: 'blocked',
          usedTokens: 130_000,
          contextLength: 100_000,
          estimated: false,
        }}
        usage={null}
      />,
    )

    // Then the bar is full while the text keeps the real percentage
    expect(screen.getByRole('progressbar', { name: 'Context' })).toHaveAttribute(
      'aria-valuenow',
      '100',
    )
    expect(screen.getByText('130k / 100k tokens · 130%')).toBeInTheDocument()
  })
})

describe('SessionUsageBar session usage', () => {
  it('labels the usage as covering completed replies only', () => {
    // Given any usage
    render(<SessionUsageBar context={NO_CONTEXT} usage={usage({ calls: 1, inputTokens: 5 })} />)

    // Then the visible label carries the caveat without needing a hover
    expect(screen.getByText('Usage (completed replies)')).toBeInTheDocument()
  })

  it('explains the caveat in a tooltip', async () => {
    // Given any usage
    render(<SessionUsageBar context={NO_CONTEXT} usage={usage({ calls: 1, inputTokens: 5 })} />)

    // When hovering the info icon
    await userEvent.hover(screen.getByLabelText('About session usage'))

    // Then the tooltip explains what is not counted
    expect(
      (await screen.findAllByText(/Interrupted replies and failed calls are not counted/))[0],
    ).toBeInTheDocument()
  })

  it('shows a dash for a session with no recorded calls', () => {
    // Given zero recorded calls
    render(<SessionUsageBar context={NO_CONTEXT} usage={usage({})} />)

    // Then neither tokens nor cost are shown
    expect(screen.queryByText(/US\$/)).not.toBeInTheDocument()
    expect(screen.queryByText(/tokens/)).not.toBeInTheDocument()
  })

  it('shows the total tokens and the exact cost when every call reported a cost', () => {
    // Given priced calls
    render(
      <SessionUsageBar
        context={NO_CONTEXT}
        usage={usage({ calls: 3, inputTokens: 10_000, outputTokens: 2_400, cost: 0.424 })}
      />,
    )

    // Then input + output tokens and the exact cost appear
    expect(screen.getByText('12.4k tokens · US$ 0.42')).toBeInTheDocument()
  })

  it('shows the cost as a lower bound when some calls have no reported cost', () => {
    // Given an unpriced call
    render(
      <SessionUsageBar
        context={NO_CONTEXT}
        usage={usage({ calls: 3, inputTokens: 100, cost: 0.42, unpricedCalls: 1 })}
      />,
    )

    // Then the cost is prefixed with ≥
    expect(screen.getByText('100 tokens · ≥ US$ 0.42')).toBeInTheDocument()
  })

  it('never mixes the two metrics', () => {
    // Given a context of 5k and an accumulated usage of 90k
    render(
      <SessionUsageBar
        context={{ state: 'normal', usedTokens: 5_000, contextLength: 100_000, estimated: false }}
        usage={usage({ calls: 9, inputTokens: 80_000, outputTokens: 10_000, cost: 1 })}
      />,
    )

    // Then each figure appears only in its own metric
    expect(screen.getByText('5k / 100k tokens · 5%')).toBeInTheDocument()
    expect(screen.getByText('90k tokens · US$ 1.00')).toBeInTheDocument()
  })
})
