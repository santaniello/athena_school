import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { setLabLanguage } from '@/i18n'
import type { QuizSummary } from '@/lib/quiz'
import { SessionQuizCard } from './session-quiz-card'

const NEVER_TAKEN: QuizSummary = {
  id: 'quiz-1',
  sessionId: 's1',
  sessionTitle: 'Distributed Caching',
  difficulty: 'HARD',
  questionCount: 8,
  createdAt: '2026-11-16T09:30:00Z',
  attemptCount: 0,
  latestScore: null,
  messageId: 'm1',
}
const TAKEN: QuizSummary = { ...NEVER_TAKEN, attemptCount: 2, latestScore: 7.5 }

function setup(quiz: QuizSummary, language?: string) {
  setLabLanguage(language)
  const onStart = vi.fn()
  const onHistory = vi.fn()
  render(<SessionQuizCard quiz={quiz} onStart={onStart} onHistory={onHistory} />)
  return { onStart, onHistory, user: userEvent.setup() }
}

describe('SessionQuizCard', () => {
  it('shows the title, difficulty and size of a test that was not taken', () => {
    setup(NEVER_TAKEN)

    expect(screen.getByText('Distributed Caching')).toBeInTheDocument()
    expect(screen.getByText('Hard')).toBeInTheDocument()
    expect(screen.getByText('8 questions')).toBeInTheDocument()
    expect(screen.getByText('Not taken yet')).toBeInTheDocument()
  })

  it('offers Take test and no History before the first attempt', async () => {
    const { user, onStart } = setup(NEVER_TAKEN)

    expect(screen.queryByRole('button', { name: 'History' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Take test' }))

    expect(onStart).toHaveBeenCalledWith(NEVER_TAKEN)
  })

  it('shows the attempts and the latest grade once it was taken', () => {
    setup(TAKEN)

    expect(screen.getByText('Latest: 7.5 / 10')).toBeInTheDocument()
    expect(screen.getByText('2 attempts')).toBeInTheDocument()
    expect(screen.queryByText('Not taken yet')).not.toBeInTheDocument()
  })

  it('keeps a latest grade of zero as a grade, not as "not taken"', () => {
    setup({ ...TAKEN, attemptCount: 1, latestScore: 0 })

    expect(screen.getByText('Latest: 0 / 10')).toBeInTheDocument()
  })

  it('offers Take again and History once it was taken', async () => {
    const { user, onStart, onHistory } = setup(TAKEN)

    await user.click(screen.getByRole('button', { name: 'Take again' }))
    await user.click(screen.getByRole('button', { name: 'History' }))

    expect(onStart).toHaveBeenCalledWith(TAKEN)
    expect(onHistory).toHaveBeenCalledWith(TAKEN)
    expect(screen.queryByRole('button', { name: 'Take test' })).not.toBeInTheDocument()
  })

  it('speaks Portuguese for a Portuguese profile', () => {
    setup(TAKEN, 'pt')

    expect(screen.getByRole('button', { name: 'Fazer de novo' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Histórico' })).toBeInTheDocument()
  })
})
