import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getQuizAttemptResult, listQuizAttempts } from '@/lib/quiz'
import { QuizHistoryDialog } from './quiz-history-dialog'
import { RESULT } from '@/test/quiz-fixtures'

vi.mock('@/lib/quiz', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/quiz')>()),
  listQuizAttempts: vi.fn(),
  getQuizAttemptResult: vi.fn(),
}))

const QUIZ = { id: 'quiz-1', title: 'Distributed Caching' }
const ATTEMPTS = [
  {
    id: 'a2',
    quizId: 'quiz-1',
    correctCount: 3,
    total: 3,
    score: 10,
    finishedAt: '2026-11-17T10:00:00Z',
  },
  {
    id: 'a1',
    quizId: 'quiz-1',
    correctCount: 1,
    total: 3,
    score: 3.3,
    finishedAt: '2026-11-16T09:30:00Z',
  },
]

describe('QuizHistoryDialog', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders nothing while no quiz is chosen', () => {
    const { container } = render(<QuizHistoryDialog quiz={null} onClose={vi.fn()} />)

    expect(container).toBeEmptyDOMElement()
    expect(listQuizAttempts).not.toHaveBeenCalled()
  })

  it('lists the attempts newest first with grade and count', async () => {
    vi.mocked(listQuizAttempts).mockResolvedValue(ATTEMPTS)
    render(<QuizHistoryDialog quiz={QUIZ} onClose={vi.fn()} />)

    expect(await screen.findByText('10 / 10 · 3 of 3')).toBeInTheDocument()
    expect(screen.getByText('3.3 / 10 · 1 of 3')).toBeInTheDocument()
    expect(listQuizAttempts).toHaveBeenCalledWith('quiz-1')
    const rows = screen.getAllByRole('listitem')
    expect(rows[0]).toHaveTextContent('10 / 10')
    expect(rows[1]).toHaveTextContent('3.3 / 10')
  })

  it('says when there are no attempts', async () => {
    vi.mocked(listQuizAttempts).mockResolvedValue([])
    render(<QuizHistoryDialog quiz={QUIZ} onClose={vi.fn()} />)

    expect(await screen.findByText('No attempts yet.')).toBeInTheDocument()
  })

  it('says when the attempts cannot be loaded', async () => {
    vi.mocked(listQuizAttempts).mockRejectedValue(new Error('nope'))
    render(<QuizHistoryDialog quiz={QUIZ} onClose={vi.fn()} />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the attempts.')
  })

  it('opens an attempt in the same dialog and goes back to the list', async () => {
    const user = userEvent.setup()
    vi.mocked(listQuizAttempts).mockResolvedValue(ATTEMPTS)
    vi.mocked(getQuizAttemptResult).mockResolvedValue(RESULT)
    render(<QuizHistoryDialog quiz={QUIZ} onClose={vi.fn()} />)
    await screen.findByText('3.3 / 10 · 1 of 3')

    vi.mocked(getQuizAttemptResult).mockReturnValueOnce(new Promise(() => {}))
    await user.click(screen.getAllByRole('button', { name: 'View' })[0])
    expect(screen.getByText('Loading the result...')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Back to attempts' }))

    await user.click(screen.getAllByRole('button', { name: 'View' })[1])

    expect(getQuizAttemptResult).toHaveBeenLastCalledWith('a1')
    expect(await screen.findByText('1 of 3 correct')).toBeInTheDocument()
    expect(screen.queryByText('Loading the result...')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Take again' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Back to attempts' }))
    expect(await screen.findByText('3.3 / 10 · 1 of 3')).toBeInTheDocument()
  })

  it('says when a result cannot be loaded and can retry', async () => {
    const user = userEvent.setup()
    vi.mocked(listQuizAttempts).mockResolvedValue(ATTEMPTS)
    vi.mocked(getQuizAttemptResult)
      .mockRejectedValueOnce(new Error('nope'))
      .mockResolvedValueOnce(RESULT)
    render(<QuizHistoryDialog quiz={QUIZ} onClose={vi.fn()} />)
    await user.click((await screen.findAllByRole('button', { name: 'View' }))[0])

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the result.')
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByText('1 of 3 correct')).toBeInTheDocument()
  })

  it('closes', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    vi.mocked(listQuizAttempts).mockResolvedValue(ATTEMPTS)
    render(<QuizHistoryDialog quiz={QUIZ} onClose={onClose} />)
    await waitFor(() => expect(screen.getAllByRole('button', { name: 'View' })).toHaveLength(2))

    await user.click(screen.getByRole('button', { name: 'Close' }))

    expect(onClose).toHaveBeenCalled()
  })
})
