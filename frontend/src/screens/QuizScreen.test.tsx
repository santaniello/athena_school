import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { listSessionsForGeneration } from '@/lib/exercise'
import {
  deleteQuiz,
  generateQuiz,
  listQuizAttempts,
  listQuizzes,
  type QuizSummary,
} from '@/lib/quiz'
import { setLabLanguage } from '@/i18n'
import QuizScreen from './QuizScreen'

vi.mock('@/lib/exercise', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/exercise')>()),
  listSessionsForGeneration: vi.fn(),
}))
vi.mock('@/lib/quiz', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/quiz')>()),
  listQuizzes: vi.fn(),
  generateQuiz: vi.fn(),
  deleteQuiz: vi.fn(),
  listQuizAttempts: vi.fn(),
  cancelQuizGeneration: vi.fn(),
}))

function summary(overrides: Partial<QuizSummary>): QuizSummary {
  return {
    id: 'quiz-1',
    sessionId: 's1',
    sessionTitle: 'Distributed Caching',
    difficulty: 'MEDIUM',
    questionCount: 10,
    createdAt: '2026-11-16T09:30:00Z',
    attemptCount: 0,
    latestScore: null,
    ...overrides,
  }
}

const NEVER_TAKEN = summary({ id: 'quiz-new', sessionTitle: 'Go Concurrency', questionCount: 5 })
const TAKEN = summary({ id: 'quiz-old', attemptCount: 2, latestScore: 7.5, difficulty: 'HARD' })

function row(title: string) {
  return screen.getByRole('listitem', { name: title })
}

describe('QuizScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(listQuizzes).mockResolvedValue([NEVER_TAKEN, TAKEN])
    vi.mocked(listSessionsForGeneration).mockResolvedValue([])
  })

  it('lists the quizzes with what the user needs to choose one', async () => {
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)

    expect(screen.getByText('Loading your tests...')).toBeInTheDocument()
    const never = await screen.findByRole('listitem', { name: 'Go Concurrency' })
    expect(within(never).getByText('5 questions')).toBeInTheDocument()
    expect(within(never).getByText('Not taken yet')).toBeInTheDocument()
    expect(within(never).getByText('Medium')).toBeInTheDocument()
    expect(within(never).getByRole('button', { name: 'Take' })).toBeInTheDocument()
    expect(within(never).queryByRole('button', { name: 'History' })).not.toBeInTheDocument()

    const taken = row('Distributed Caching')
    expect(within(taken).getByText('Latest: 7.5 / 10')).toBeInTheDocument()
    expect(within(taken).getByText('2 attempts')).toBeInTheDocument()
    expect(within(taken).getByText('Hard')).toBeInTheDocument()
    expect(within(taken).getByRole('button', { name: 'Take again' })).toBeInTheDocument()
    expect(within(taken).getByRole('button', { name: 'History' })).toBeInTheDocument()
  })

  it('shows a latest score of zero, not "not taken"', async () => {
    vi.mocked(listQuizzes).mockResolvedValue([summary({ attemptCount: 1, latestScore: 0 })])
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)

    expect(await screen.findByText('Latest: 0 / 10')).toBeInTheDocument()
    expect(screen.queryByText('Not taken yet')).not.toBeInTheDocument()
  })

  it('does not call a quiz taken when its score is missing, nor show a score for an untaken one', async () => {
    vi.mocked(listQuizzes).mockResolvedValue([
      summary({ id: 'a', sessionTitle: 'Odd A', attemptCount: 2, latestScore: null }),
      summary({ id: 'b', sessionTitle: 'Odd B', attemptCount: 0, latestScore: 8 }),
    ])
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)

    const a = await screen.findByRole('listitem', { name: 'Odd A' })
    expect(within(a).getByText('Not taken yet')).toBeInTheDocument()
    expect(within(a).queryByText(/Latest/)).not.toBeInTheDocument()
    const b = screen.getByRole('listitem', { name: 'Odd B' })
    expect(within(b).getByText('Not taken yet')).toBeInTheDocument()
    expect(within(b).queryByText(/Latest/)).not.toBeInTheDocument()
  })

  it('uses the singular for one question and one attempt', async () => {
    vi.mocked(listQuizzes).mockResolvedValue([
      summary({ questionCount: 1, attemptCount: 1, latestScore: 10 }),
    ])
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)

    expect(await screen.findByText('1 question')).toBeInTheDocument()
    expect(screen.getByText('1 attempt')).toBeInTheDocument()
  })

  it('hands the chosen quiz to the shell when Take is clicked', async () => {
    const user = userEvent.setup()
    const onTake = vi.fn()
    render(<QuizScreen experienceLevel="intermediate" onTake={onTake} />)

    await user.click(
      within(await screen.findByRole('listitem', { name: 'Go Concurrency' })).getByRole('button', {
        name: 'Take',
      }),
    )

    expect(onTake).toHaveBeenCalledWith('quiz-new')
  })

  it('explains how to start when there are no quizzes', async () => {
    vi.mocked(listQuizzes).mockResolvedValue([])
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)

    expect(
      await screen.findByText('No tests yet. Generate one from a study session you already had.'),
    ).toBeInTheDocument()
  })

  it('says the quizzes could not load and retries', async () => {
    const user = userEvent.setup()
    vi.mocked(listQuizzes).mockRejectedValueOnce(new Error('nope')).mockResolvedValueOnce([TAKEN])
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load your tests.')
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByRole('listitem', { name: 'Distributed Caching' })).toBeInTheDocument()
  })

  it('opens the attempts of a quiz', async () => {
    const user = userEvent.setup()
    vi.mocked(listQuizAttempts).mockResolvedValue([])
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Distributed Caching' })

    await user.click(within(row('Distributed Caching')).getByRole('button', { name: 'History' }))

    expect(listQuizAttempts).toHaveBeenCalledWith('quiz-old')
    expect(await screen.findByText('No attempts yet.')).toBeInTheDocument()
  })

  it('asks before deleting, and deletes and reloads when confirmed', async () => {
    const user = userEvent.setup()
    vi.mocked(deleteQuiz).mockResolvedValue(undefined)
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Go Concurrency' })

    await user.click(within(row('Go Concurrency')).getByRole('button', { name: 'Delete' }))
    expect(deleteQuiz).not.toHaveBeenCalled()
    expect(screen.getByText('Delete this test?')).toBeInTheDocument()

    vi.mocked(listQuizzes).mockResolvedValue([TAKEN])
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Delete' }),
    )

    expect(deleteQuiz).toHaveBeenCalledWith('quiz-new')
    await waitFor(() =>
      expect(screen.queryByRole('listitem', { name: 'Go Concurrency' })).not.toBeInTheDocument(),
    )
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('shows loading again while a retry runs', async () => {
    const user = userEvent.setup()
    vi.mocked(listQuizzes).mockRejectedValueOnce(new Error('nope'))
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)
    await screen.findByRole('alert')

    vi.mocked(listQuizzes).mockReturnValueOnce(new Promise(() => {}))
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(screen.getByText('Loading your tests...')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('clears an old deletion error when another deletion starts', async () => {
    const user = userEvent.setup()
    vi.mocked(deleteQuiz).mockRejectedValueOnce(new Error('nope'))
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Go Concurrency' })
    await user.click(within(row('Go Concurrency')).getByRole('button', { name: 'Delete' }))
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Delete' }),
    )
    await screen.findByText('Could not delete the test.')

    await user.click(within(row('Distributed Caching')).getByRole('button', { name: 'Delete' }))

    expect(screen.queryByText('Could not delete the test.')).not.toBeInTheDocument()
  })

  it('closes the generation dialog and the history dialog on request', async () => {
    const user = userEvent.setup()
    vi.mocked(listQuizAttempts).mockResolvedValue([])
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Distributed Caching' })

    await user.click(screen.getByRole('button', { name: 'Generate test with AI' }))
    await user.click(await screen.findByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    await user.click(within(row('Distributed Caching')).getByRole('button', { name: 'History' }))
    await user.click(await screen.findByRole('button', { name: 'Close' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('keeps the quiz when the deletion is cancelled', async () => {
    const user = userEvent.setup()
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Go Concurrency' })

    await user.click(within(row('Go Concurrency')).getByRole('button', { name: 'Delete' }))
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Cancel' }),
    )

    expect(deleteQuiz).not.toHaveBeenCalled()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('says when a deletion fails', async () => {
    const user = userEvent.setup()
    vi.mocked(deleteQuiz).mockRejectedValue(new Error('nope'))
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Go Concurrency' })

    await user.click(within(row('Go Concurrency')).getByRole('button', { name: 'Delete' }))
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Delete' }),
    )

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not delete the test.')
    expect(row('Go Concurrency')).toBeInTheDocument()
  })

  it('generates a quiz, then reloads the list and highlights the new one', async () => {
    const user = userEvent.setup()
    vi.mocked(listSessionsForGeneration).mockResolvedValue([
      {
        id: 's1',
        topic: 'Distributed Caching',
        folderName: 'Backend',
        startedAt: '2026-11-10T10:00:00Z',
      },
    ])
    const created = summary({ id: 'quiz-created', sessionTitle: 'Fresh Topic' })
    vi.mocked(generateQuiz).mockResolvedValue(created)
    render(<QuizScreen experienceLevel="advanced" onTake={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Go Concurrency' })

    await user.click(screen.getByRole('button', { name: 'Generate test with AI' }))
    await user.click(await screen.findByRole('radio', { name: /Distributed Caching/ }))
    vi.mocked(listQuizzes).mockResolvedValue([created, NEVER_TAKEN, TAKEN])
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(generateQuiz).toHaveBeenCalledWith('s1', 10, 'HARD')
    const fresh = await screen.findByRole('listitem', { name: 'Fresh Topic' })
    expect(fresh).toHaveAttribute('aria-current', 'true')
    expect(row('Go Concurrency')).not.toHaveAttribute('aria-current')
    expect(row('Distributed Caching')).not.toHaveAttribute('aria-current')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('also offers Generate from the empty state', async () => {
    const user = userEvent.setup()
    vi.mocked(listQuizzes).mockResolvedValue([])
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)
    await screen.findByText('No tests yet. Generate one from a study session you already had.')

    await user.click(screen.getAllByRole('button', { name: 'Generate test with AI' })[1])

    expect(await screen.findByRole('dialog')).toBeInTheDocument()
  })

  it('speaks the profile language', async () => {
    setLabLanguage('pt')
    render(<QuizScreen experienceLevel="intermediate" onTake={vi.fn()} />)

    expect(await screen.findByText('Última: 7.5 / 10')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Gerar teste com IA' })).toBeInTheDocument()
    expect(screen.getByText('Ainda não feito')).toBeInTheDocument()
  })
})
