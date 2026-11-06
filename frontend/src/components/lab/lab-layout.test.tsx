import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LabLayout } from './lab-layout'

const NOW = new Date('2026-11-06T12:00:00Z').getTime()

function renderLayout(overrides: Partial<Parameters<typeof LabLayout>[0]> = {}) {
  const handlers = {
    onExpire: vi.fn(),
    onAskHint: vi.fn(),
    onFinish: vi.fn(),
    onLeave: vi.fn(),
  }
  render(
    <LabLayout
      title="Cache and invalidation in a catalog"
      topic="Caching"
      difficulty="INTERMEDIATE"
      statement={{
        scenario: 'A catalog service serves millions of reads per day.',
        functionalRequirements: ['Browse products'],
        nonFunctionalRequirements: [],
        constraints: [],
        objectives: [],
      }}
      criteria={[
        {
          id: 'k1',
          name: 'Consistency',
          description: 'Reads stay correct.',
          weight: 20,
          mandatory: true,
        },
      ]}
      deadline={new Date(NOW + 44 * 60 * 1000 + 12 * 1000).toISOString()}
      now={() => Date.now()}
      hints={[]}
      hintsLeft={3}
      canFinish
      board={<p>the editor</p>}
      {...handlers}
      {...overrides}
    />,
  )
  return handlers
}

describe('LabLayout', () => {
  const innerWidth = window.innerWidth

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.setSystemTime(NOW)
    window.innerWidth = 1600
  })
  afterEach(() => {
    vi.useRealTimers()
    window.innerWidth = innerWidth
  })

  it('shows the breadcrumb, title, difficulty and topic chips and the countdown in the top bar', () => {
    renderLayout()

    expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toHaveTextContent(
      'Architecture Lab›Caching›Cache and invalidation in a catalog',
    )
    expect(
      screen.getByRole('heading', { level: 1, name: 'Cache and invalidation in a catalog' }),
    ).toBeInTheDocument()
    expect(screen.getByText('Intermediate')).toBeInTheDocument()
    expect(screen.getAllByText('Caching').length).toBeGreaterThan(0)
    expect(screen.getByRole('timer')).toHaveTextContent('44:12')
  })

  it('shows the statement, the board the caller provides, the tutor and the rubric at once', () => {
    renderLayout()

    expect(screen.getByText('A catalog service serves millions of reads per day.')).toBeVisible()
    expect(screen.getByRole('region', { name: 'Board' })).toHaveTextContent('the editor')
    expect(screen.getByRole('heading', { name: 'Tutor Athena' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Rubric' })).toBeInTheDocument()
  })

  it('places the tutor above the rubric', () => {
    renderLayout()

    const tutor = screen.getByRole('heading', { name: 'Tutor Athena' })
    const rubric = screen.getByRole('heading', { name: 'Rubric' })
    expect(tutor.compareDocumentPosition(rubric) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('has no tabs and no pause control', () => {
    renderLayout()

    expect(screen.queryByRole('tab')).not.toBeInTheDocument()
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /pause/i })).not.toBeInTheDocument()
  })

  it('enables Finish and evaluate when the board has content and calls onFinish', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const handlers = renderLayout({ canFinish: true })

    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))

    expect(handlers.onFinish).toHaveBeenCalledOnce()
  })

  it('disables Finish and evaluate while the board is empty', () => {
    renderLayout({ canFinish: false })

    expect(screen.getByRole('button', { name: 'Finish and evaluate' })).toBeDisabled()
  })

  it('asks for confirmation before leaving and calls onLeave only when confirmed', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const handlers = renderLayout()

    await user.click(screen.getByRole('button', { name: 'Leave' }))
    const dialog = screen.getByRole('alertdialog')
    expect(handlers.onLeave).not.toHaveBeenCalled()

    await user.click(within(dialog).getByRole('button', { name: 'Leave' }))

    expect(handlers.onLeave).toHaveBeenCalledOnce()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('closes the leave confirmation on Cancel without calling onLeave', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const handlers = renderLayout()

    await user.click(screen.getByRole('button', { name: 'Leave' }))
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Cancel' }),
    )

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(handlers.onLeave).not.toHaveBeenCalled()
  })

  it('passes the hint request to the caller', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const handlers = renderLayout()

    await user.click(screen.getByRole('button', { name: 'Ask for a hint' }))

    expect(handlers.onAskHint).toHaveBeenCalledOnce()
  })

  it('passes the expiry to the caller', () => {
    const handlers = renderLayout({ deadline: new Date(NOW - 1000).toISOString() })

    expect(handlers.onExpire).toHaveBeenCalledOnce()
  })

  it('toggles the side panels from the top bar', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    renderLayout()

    await user.click(screen.getByRole('button', { name: 'Hide statement' }))
    expect(screen.getByRole('button', { name: 'Show statement' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Hide tutor and rubric' }))
    expect(screen.getByRole('button', { name: 'Show tutor and rubric' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Show statement' }))
    expect(screen.getByRole('button', { name: 'Hide statement' })).toBeInTheDocument()
  })

  it('starts with the side panels collapsed on a narrow window', () => {
    window.innerWidth = 900

    renderLayout()

    expect(screen.getByRole('button', { name: 'Show statement' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Show tutor and rubric' })).toBeInTheDocument()
  })

  it('starts with the side panels open on a wide window', () => {
    renderLayout()

    expect(screen.getByRole('button', { name: 'Hide statement' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Hide tutor and rubric' })).toBeInTheDocument()
  })

  it('starts with the side panels open at exactly the minimum width', () => {
    window.innerWidth = 1100

    renderLayout()

    expect(screen.getByRole('button', { name: 'Hide statement' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Hide tutor and rubric' })).toBeInTheDocument()
  })
})
