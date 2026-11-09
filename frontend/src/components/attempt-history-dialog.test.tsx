import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { listAttemptHistory, type AttemptHistoryItem } from '@/lib/exercise'
import { AttemptHistoryDialog } from './attempt-history-dialog'

vi.mock('@/lib/exercise', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/exercise')>()
  return { ...original, listAttemptHistory: vi.fn() }
})

// The real view is covered by its own tests.
vi.mock('@/components/lab/attempt-result-view', () => ({
  AttemptResultView: (props: { attemptId: string; backLabel: string; onBack: () => void }) => (
    <div>
      <span>result of {props.attemptId}</span>
      <button onClick={props.onBack}>{props.backLabel}</button>
    </div>
  ),
}))

const items: AttemptHistoryItem[] = [
  {
    attemptId: 'a2',
    finishedAt: '2026-11-08T12:00:00Z',
    diagramScore: 90,
    understandingScore: 85,
    outcome: 'PASSED',
  },
  {
    attemptId: 'a1',
    finishedAt: '2026-11-07T12:00:00Z',
    diagramScore: 60,
    understandingScore: 40,
    outcome: 'FAILED',
  },
]

const challenge = { id: 'c1', title: 'Cache and invalidation' }

describe('AttemptHistoryDialog', () => {
  beforeEach(() => vi.clearAllMocks())

  it('stays closed and reads nothing without a challenge', () => {
    render(<AttemptHistoryDialog challenge={null} onClose={vi.fn()} />)

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(listAttemptHistory).not.toHaveBeenCalled()
  })

  it('lists the attempts in the order received with date, scores and outcome', async () => {
    vi.mocked(listAttemptHistory).mockResolvedValue(items)

    render(<AttemptHistoryDialog challenge={challenge} onClose={vi.fn()} />)

    expect(screen.getByRole('dialog')).toHaveTextContent('Cache and invalidation')
    expect(screen.getByText('Loading the history...')).toBeInTheDocument()
    const rows = await screen.findAllByRole('listitem')
    expect(listAttemptHistory).toHaveBeenCalledExactlyOnceWith('c1')
    expect(within(rows[0]).getByText('Nov 8, 2026')).toBeInTheDocument()
    expect(within(rows[0]).getByText('Diagram 90 · Understanding 85')).toBeInTheDocument()
    expect(within(rows[0]).getByText('Passed')).toBeInTheDocument()
    expect(within(rows[1]).getByText('Nov 7, 2026')).toBeInTheDocument()
    expect(within(rows[1]).getByText('Diagram 60 · Understanding 40')).toBeInTheDocument()
    expect(within(rows[1]).getByText('Failed')).toBeInTheDocument()
  })

  it('tells a passed attempt from a failed one at a glance', async () => {
    vi.mocked(listAttemptHistory).mockResolvedValue(items)

    render(<AttemptHistoryDialog challenge={challenge} onClose={vi.fn()} />)
    await screen.findAllByRole('listitem')

    expect(screen.getByText('Passed')).toHaveClass('bg-primary')
    expect(screen.getByText('Failed')).toHaveClass('bg-muted')
  })

  it('offers no action that changes or deletes an attempt', async () => {
    vi.mocked(listAttemptHistory).mockResolvedValue(items)

    render(<AttemptHistoryDialog challenge={challenge} onClose={vi.fn()} />)
    await screen.findAllByRole('listitem')

    const names = screen.getAllByRole('button').map((button) => button.textContent)
    expect(names.filter((name) => name !== 'Close')).toEqual(['Open', 'Open'])
  })

  it('opens the chosen attempt and comes back to the list', async () => {
    vi.mocked(listAttemptHistory).mockResolvedValue(items)
    const user = userEvent.setup()
    render(<AttemptHistoryDialog challenge={challenge} onClose={vi.fn()} />)
    const rows = await screen.findAllByRole('listitem')

    await user.click(within(rows[1]).getByRole('button', { name: 'Open' }))
    expect(screen.getByText('result of a1')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Back to history' }))

    expect(await screen.findAllByRole('listitem')).toHaveLength(2)
    expect(screen.queryByText('result of a1')).not.toBeInTheDocument()
  })

  it('says the history could not be loaded', async () => {
    vi.mocked(listAttemptHistory).mockRejectedValue(new Error('boom'))

    render(<AttemptHistoryDialog challenge={challenge} onClose={vi.fn()} />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the history.')
  })

  it('closes from the dialog', async () => {
    vi.mocked(listAttemptHistory).mockResolvedValue(items)
    const onClose = vi.fn()
    render(<AttemptHistoryDialog challenge={challenge} onClose={onClose} />)
    await screen.findAllByRole('listitem')

    await userEvent.keyboard('{Escape}')

    expect(onClose).toHaveBeenCalledOnce()
  })
})
