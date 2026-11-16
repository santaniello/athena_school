import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getQuiz, getQuizAttemptResult, submitQuizAttempt, type Quiz } from '@/lib/quiz'
import { RESULT } from '@/test/quiz-fixtures'
import { setLabLanguage } from '@/i18n'
import QuizTakeScreen from './QuizTakeScreen'

vi.mock('@/lib/quiz', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/quiz')>()),
  getQuiz: vi.fn(),
  submitQuizAttempt: vi.fn(),
  getQuizAttemptResult: vi.fn(),
}))

const QUIZ: Quiz = {
  id: 'quiz-1',
  sessionTitle: 'Distributed Caching',
  difficulty: 'HARD',
  questions: [
    {
      id: 'q1',
      position: 1,
      kind: 'MULTIPLE_CHOICE',
      prompt: 'Which policy evicts the oldest?',
      options: ['LRU', 'FIFO', 'LFU'],
    },
    {
      id: 'q2',
      position: 2,
      kind: 'TRUE_FALSE',
      prompt: 'A cache always lowers latency.',
      options: ['True', 'False'],
    },
    {
      id: 'q3',
      position: 3,
      kind: 'MULTIPLE_CHOICE',
      prompt: 'What is a cache hit?',
      options: ['A', 'B'],
    },
  ],
}
const ATTEMPT = {
  id: 'attempt-1',
  quizId: 'quiz-1',
  correctCount: 1,
  total: 3,
  score: 3.3,
  finishedAt: '2026-11-16T09:30:00Z',
}

function renderScreen(overrides: Partial<React.ComponentProps<typeof QuizTakeScreen>> = {}) {
  const props = {
    quizId: 'quiz-1',
    returnTo: { kind: 'menu' } as const,
    leaveRequested: false,
    onLeaveCancelled: vi.fn(),
    onExit: vi.fn(),
    ...overrides,
  }
  return { ...props, ...render(<QuizTakeScreen {...props} />) }
}

async function loaded(overrides: Partial<React.ComponentProps<typeof QuizTakeScreen>> = {}) {
  const view = renderScreen(overrides)
  await screen.findByText('Which policy evicts the oldest?')
  return view
}

describe('QuizTakeScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(getQuiz).mockResolvedValue(QUIZ)
    vi.mocked(getQuizAttemptResult).mockResolvedValue(RESULT)
  })

  it('opens on the first question with its options and no answer key', async () => {
    renderScreen()
    expect(screen.getByText('Loading the test...')).toBeInTheDocument()

    expect(await screen.findByText('Which policy evicts the oldest?')).toBeInTheDocument()
    expect(getQuiz).toHaveBeenCalledWith('quiz-1')
    expect(screen.getByText('Question 1 of 3')).toBeInTheDocument()
    expect(screen.getByText('Distributed Caching')).toBeInTheDocument()
    expect(screen.getByText('Hard')).toBeInTheDocument()
    expect(
      screen.getAllByRole('radio').map((radio) => radio.closest('label')?.textContent),
    ).toEqual(['LRU', 'FIFO', 'LFU'])
    expect(
      screen.getAllByRole('radio').every((radio) => !(radio as HTMLInputElement).checked),
    ).toBe(true)
  })

  it('goes back to the study session when the quiz was started from it and could not load', async () => {
    const user = userEvent.setup()
    vi.mocked(getQuiz).mockRejectedValueOnce(new Error('nope'))
    const { onExit } = renderScreen({ returnTo: { kind: 'study', sessionId: 'session-1' } })

    await screen.findByRole('alert')
    expect(screen.queryByRole('button', { name: 'Back to tests' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Back to study session' }))

    expect(onExit).toHaveBeenCalledOnce()
  })

  it('says the quiz could not load, retries, and goes back', async () => {
    const user = userEvent.setup()
    vi.mocked(getQuiz).mockRejectedValueOnce(new Error('nope')).mockResolvedValueOnce(QUIZ)
    const { onExit } = renderScreen()

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not open the test.')
    await user.click(screen.getByRole('button', { name: 'Back to tests' }))
    expect(onExit).toHaveBeenCalledOnce()

    await user.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByText('Which policy evicts the oldest?')).toBeInTheDocument()
  })

  it('records an answer, lets it change and clears it', async () => {
    const user = userEvent.setup()
    await loaded()
    const clear = screen.getByRole('button', { name: 'Clear answer' })
    expect(clear).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Question 1' })).toBeInTheDocument()

    await user.click(screen.getByRole('radio', { name: 'FIFO' }))
    expect(screen.getByRole('radio', { name: 'FIFO' })).toBeChecked()
    expect(screen.getByRole('button', { name: 'Question 1, answered' })).toBeInTheDocument()

    await user.click(screen.getByRole('radio', { name: 'LRU' }))
    expect(screen.getByRole('radio', { name: 'LRU' })).toBeChecked()
    expect(screen.getByRole('radio', { name: 'FIFO' })).not.toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Clear answer' }))
    expect(screen.getByRole('radio', { name: 'LRU' })).not.toBeChecked()
    expect(screen.getByRole('button', { name: 'Question 1' })).toBeInTheDocument()
  })

  it('moves with Previous and Next and keeps each answer', async () => {
    const user = userEvent.setup()
    await loaded()
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled()
    await user.click(screen.getByRole('radio', { name: 'FIFO' }))

    await user.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('Question 2 of 3')).toBeInTheDocument()
    expect(screen.getAllByRole('radio')).toHaveLength(2)
    await user.click(screen.getByRole('radio', { name: 'False' }))

    await user.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('Question 3 of 3')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: 'Previous' }))
    expect(screen.getByRole('radio', { name: 'False' })).toBeChecked()
    await user.click(screen.getByRole('button', { name: 'Previous' }))
    expect(screen.getByRole('radio', { name: 'FIFO' })).toBeChecked()
  })

  it('jumps to a question from the numbered strip and marks the current one', async () => {
    const user = userEvent.setup()
    await loaded()
    expect(screen.getByRole('button', { name: 'Question 1' })).toHaveAttribute(
      'aria-current',
      'step',
    )
    expect(screen.getByRole('button', { name: 'Question 3' })).not.toHaveAttribute('aria-current')
    expect(screen.getByRole('navigation')).toHaveTextContent('123')

    await user.click(screen.getByRole('button', { name: 'Question 3' }))

    expect(screen.getByText('What is a cache hit?')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Question 3' })).toHaveAttribute(
      'aria-current',
      'step',
    )
  })

  async function answerEverything(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByRole('radio', { name: 'FIFO' }))
    await user.click(screen.getByRole('button', { name: 'Question 2' }))
    await user.click(screen.getByRole('radio', { name: 'False' }))
    await user.click(screen.getByRole('button', { name: 'Question 3' }))
    await user.click(screen.getByRole('radio', { name: 'A' }))
  }

  it('submits without asking when every question is answered, then shows the result', async () => {
    const user = userEvent.setup()
    vi.mocked(submitQuizAttempt).mockResolvedValue(ATTEMPT)
    await loaded()
    await answerEverything(user)

    await user.click(screen.getByRole('button', { name: 'Finish' }))

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(submitQuizAttempt).toHaveBeenCalledWith('quiz-1', [
      { questionId: 'q1', selectedIndex: 1 },
      { questionId: 'q2', selectedIndex: 1 },
      { questionId: 'q3', selectedIndex: 0 },
    ])
    expect(await screen.findByText('1 of 3 correct')).toBeInTheDocument()
    expect(getQuizAttemptResult).toHaveBeenCalledWith('attempt-1')
  })

  it('warns about blank questions, and lets the user keep answering', async () => {
    const user = userEvent.setup()
    await loaded()
    await user.click(screen.getByRole('radio', { name: 'FIFO' }))

    await user.click(screen.getByRole('button', { name: 'Finish' }))

    const dialog = screen.getByRole('alertdialog')
    expect(
      within(dialog).getByText('2 questions are blank and count as wrong.'),
    ).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Keep answering' }))
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(submitQuizAttempt).not.toHaveBeenCalled()
  })

  it('submits anyway when the user insists, and says one blank question in the singular', async () => {
    const user = userEvent.setup()
    vi.mocked(submitQuizAttempt).mockResolvedValue(ATTEMPT)
    await loaded()
    await user.click(screen.getByRole('radio', { name: 'FIFO' }))
    await user.click(screen.getByRole('button', { name: 'Question 2' }))
    await user.click(screen.getByRole('radio', { name: 'False' }))

    await user.click(screen.getByRole('button', { name: 'Finish' }))
    expect(screen.getByText('1 question is blank and counts as wrong.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Finish anyway' }))

    expect(submitQuizAttempt).toHaveBeenCalledWith('quiz-1', [
      { questionId: 'q1', selectedIndex: 1 },
      { questionId: 'q2', selectedIndex: 1 },
      { questionId: 'q3', selectedIndex: null },
    ])
    expect(await screen.findByText('1 of 3 correct')).toBeInTheDocument()
  })

  it('shows progress while grading and keeps the answers when the submission fails', async () => {
    const user = userEvent.setup()
    let fail: (reason: unknown) => void = () => {}
    vi.mocked(submitQuizAttempt)
      .mockReturnValueOnce(new Promise((_, reject) => (fail = reject)))
      .mockResolvedValueOnce(ATTEMPT)
    await loaded()
    await answerEverything(user)

    await user.click(screen.getByRole('button', { name: 'Finish' }))
    expect(screen.getByRole('status')).toHaveTextContent('Grading your answers...')
    expect(screen.getByRole('button', { name: 'Finish' })).toBeDisabled()

    fail(new Error('boom'))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Something went wrong. Please try again.',
    )
    expect(screen.getByRole('radio', { name: 'A' })).toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Finish' }))
    expect(await screen.findByText('1 of 3 correct')).toBeInTheDocument()
  })

  it('asks before leaving, says the answers are lost, and stays when cancelled', async () => {
    const user = userEvent.setup()
    const { onExit } = await loaded()

    await user.click(screen.getByRole('button', { name: 'Leave' }))
    const dialog = screen.getByRole('alertdialog')
    expect(
      within(dialog).getByText('Your answers are not saved. Leaving discards this attempt.'),
    ).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(onExit).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Leave' }))
    await user.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Leave' }))
    expect(onExit).toHaveBeenCalledOnce()
  })

  it('asks the same question when the shell wants to navigate away, and reports a cancel', async () => {
    const user = userEvent.setup()
    const view = await loaded()
    view.rerender(<QuizTakeScreen {...view} leaveRequested />)
    const { onExit, onLeaveCancelled } = view

    const dialog = screen.getByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(onLeaveCancelled).toHaveBeenCalledOnce()
    expect(onExit).not.toHaveBeenCalled()
  })

  it('leaves for the shell once the user confirms', async () => {
    const user = userEvent.setup()
    const view = await loaded()
    view.rerender(<QuizTakeScreen {...view} leaveRequested />)
    const { onExit } = view

    await user.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Leave' }))

    expect(onExit).toHaveBeenCalledOnce()
    expect(view.onLeaveCancelled).not.toHaveBeenCalled()
  })

  it('lets the shell navigate away once it asks while the quiz is still loading', async () => {
    vi.mocked(getQuiz).mockReturnValueOnce(new Promise(() => {}))
    const view = renderScreen()
    expect(view.onExit).not.toHaveBeenCalled()

    view.rerender(<QuizTakeScreen {...view} leaveRequested />)

    await waitFor(() => expect(view.onExit).toHaveBeenCalledOnce())
  })

  it('lets the shell navigate away at once while the quiz is still loading or failed to load', async () => {
    vi.mocked(getQuiz).mockReturnValueOnce(new Promise(() => {}))
    const loading = renderScreen({ leaveRequested: true })
    await waitFor(() => expect(loading.onExit).toHaveBeenCalledOnce())
    loading.unmount()

    vi.mocked(getQuiz).mockRejectedValueOnce(new Error('nope'))
    const failed = renderScreen({ leaveRequested: true })
    await waitFor(() => expect(failed.onExit).toHaveBeenCalledOnce())
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  describe('after the quiz is graded', () => {
    async function finishQuiz(
      user: ReturnType<typeof userEvent.setup>,
      overrides: Partial<React.ComponentProps<typeof QuizTakeScreen>> = {},
    ) {
      vi.mocked(submitQuizAttempt).mockResolvedValue(ATTEMPT)
      const view = await loaded(overrides)
      await answerEverything(user)
      await user.click(screen.getByRole('button', { name: 'Finish' }))
      await screen.findByText('1 of 3 correct')
      return view
    }

    it('says the result is loading, and when it cannot be read', async () => {
      const user = userEvent.setup()
      vi.mocked(getQuizAttemptResult)
        .mockRejectedValueOnce(new Error('nope'))
        .mockResolvedValueOnce(RESULT)
      vi.mocked(submitQuizAttempt).mockResolvedValue(ATTEMPT)
      await loaded()
      await answerEverything(user)
      await user.click(screen.getByRole('button', { name: 'Finish' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the result.')
      await user.click(screen.getByRole('button', { name: 'Retry' }))
      expect(await screen.findByText('1 of 3 correct')).toBeInTheDocument()
    })

    it('shows the result only once it has loaded', async () => {
      const user = userEvent.setup()
      vi.mocked(getQuizAttemptResult).mockReturnValueOnce(new Promise(() => {}))
      vi.mocked(submitQuizAttempt).mockResolvedValue(ATTEMPT)
      await loaded()
      await answerEverything(user)
      await user.click(screen.getByRole('button', { name: 'Finish' }))

      expect(await screen.findByText('Loading the result...')).toBeInTheDocument()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      expect(screen.queryByText('1 of 3 correct')).not.toBeInTheDocument()
    })

    it('goes back to the study session when the quiz was started from it', async () => {
      const user = userEvent.setup()
      const { onExit } = await finishQuiz(user, {
        returnTo: { kind: 'study', sessionId: 'session-1' },
      })

      expect(screen.queryByRole('button', { name: 'Back to tests' })).not.toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Back to study session' }))

      expect(onExit).toHaveBeenCalledOnce()
    })

    it('goes back to the catalog', async () => {
      const user = userEvent.setup()
      const { onExit } = await finishQuiz(user)

      await user.click(screen.getByRole('button', { name: 'Back to tests' }))

      expect(onExit).toHaveBeenCalledOnce()
    })

    it('takes the quiz again from scratch', async () => {
      const user = userEvent.setup()
      await finishQuiz(user)

      await user.click(screen.getByRole('button', { name: 'Take again' }))

      expect(await screen.findByText('Question 1 of 3')).toBeInTheDocument()
      expect(getQuiz).toHaveBeenCalledTimes(2)
      expect(
        screen.getAllByRole('radio').every((radio) => !(radio as HTMLInputElement).checked),
      ).toBe(true)
    })

    it('lets the shell navigate away at once: there is nothing left to lose', async () => {
      const user = userEvent.setup()
      const view = await finishQuiz(user)

      view.rerender(<QuizTakeScreen {...view} leaveRequested />)

      await waitFor(() => expect(view.onExit).toHaveBeenCalledOnce())
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    })
  })

  it('speaks the profile language', async () => {
    setLabLanguage('pt')
    renderScreen()

    expect(await screen.findByText('Questão 1 de 3')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Finalizar' })).toBeInTheDocument()
  })
})
