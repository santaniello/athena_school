import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { SessionExerciseCard } from '@/components/session-exercise-card'
import type { SessionExerciseCard as Card } from '@/lib/exercise'

const baseCard: Card = {
  sessionExerciseId: 'l1',
  messageId: 'm1',
  challengeId: 'c1',
  title: 'Design a feed',
  difficulty: 'ADVANCED',
  timeLimitMinutes: 60,
  status: 'NOT_STARTED',
  attempts: 0,
  latest: null,
}

function setup(card: Card) {
  const onStart = vi.fn()
  const onHistory = vi.fn()
  render(<SessionExerciseCard card={card} onStart={onStart} onHistory={onHistory} />)
  return { onStart, onHistory }
}

describe('SessionExerciseCard', () => {
  it('shows the title, difficulty, time limit and progress of a challenge never tried', () => {
    setup(baseCard)

    expect(screen.getByText('Design a feed')).toBeInTheDocument()
    expect(screen.getByText('Advanced')).toBeInTheDocument()
    expect(screen.getByText('60 min')).toBeInTheDocument()
    expect(screen.getByText('Not started')).toBeInTheDocument()
  })

  it('offers only Open in Architecture Lab before a finished attempt', async () => {
    const { onStart } = setup(baseCard)

    expect(screen.queryByRole('button', { name: 'History' })).not.toBeInTheDocument()
    expect(screen.queryByText(/attempt/i)).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Open in Architecture Lab' }))

    expect(onStart).toHaveBeenCalledWith(baseCard)
  })

  it('reads Start again, shows the count and the latest failure with both scores, and offers History', async () => {
    const card: Card = {
      ...baseCard,
      status: 'NEEDS_IMPROVEMENT',
      attempts: 2,
      latest: { outcome: 'FAILED', diagramScore: 40, understandingScore: 55 },
    }
    const { onStart, onHistory } = setup(card)

    expect(screen.getByText('Needs improvement')).toBeInTheDocument()
    expect(screen.getByText('2 attempts')).toBeInTheDocument()
    expect(screen.getByText(/Failed/)).toBeInTheDocument()
    expect(screen.getByText(/Diagram 40/)).toBeInTheDocument()
    expect(screen.getByText(/Understanding 55/)).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Open in Architecture Lab' }),
    ).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Start again' }))
    await userEvent.click(screen.getByRole('button', { name: 'History' }))

    expect(onStart).toHaveBeenCalledWith(card)
    expect(onHistory).toHaveBeenCalledWith(card)
  })

  it('shows a singular attempt count and a passed result', () => {
    setup({
      ...baseCard,
      status: 'COMPLETED',
      attempts: 1,
      latest: { outcome: 'PASSED', diagramScore: 88, understandingScore: 91 },
    })

    expect(screen.getByText('Completed')).toBeInTheDocument()
    expect(screen.getByText('1 attempt')).toBeInTheDocument()
    expect(screen.getByText(/Passed/)).toBeInTheDocument()
  })
})
