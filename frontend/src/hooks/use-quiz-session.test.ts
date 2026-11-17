import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getQuiz, submitQuizAttempt, type Quiz } from '@/lib/quiz'
import { useQuizSession } from './use-quiz-session'

vi.mock('@/lib/quiz', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/quiz')>()),
  getQuiz: vi.fn(),
  submitQuizAttempt: vi.fn(),
}))

const quiz: Quiz = {
  id: 'quiz-1',
  sessionTitle: 'Caching',
  difficulty: 'EASY',
  questions: [
    { id: 'q1', position: 1, kind: 'MULTIPLE_CHOICE', prompt: 'One?', options: ['A', 'B', 'C'] },
    { id: 'q2', position: 2, kind: 'TRUE_FALSE', prompt: 'Two?', options: ['True', 'False'] },
    { id: 'q3', position: 3, kind: 'MULTIPLE_CHOICE', prompt: 'Three?', options: ['X', 'Y'] },
  ],
}

async function renderLoaded() {
  vi.mocked(getQuiz).mockResolvedValue(quiz)
  const hook = renderHook(() => useQuizSession('quiz-1'))
  await waitFor(() => expect(hook.result.current.state).toBe('ready'))
  return hook
}

describe('useQuizSession', () => {
  beforeEach(() => vi.clearAllMocks())

  it('loads the quiz and starts on the first question with nothing answered', async () => {
    const { result } = await renderLoaded()

    expect(getQuiz).toHaveBeenCalledWith('quiz-1')
    expect(result.current.quiz).toEqual(quiz)
    expect(result.current.index).toBe(0)
    expect(result.current.question?.id).toBe('q1')
    expect(result.current.blankCount).toBe(3)
    expect(result.current.selections).toEqual([
      { questionId: 'q1', selectedIndex: null },
      { questionId: 'q2', selectedIndex: null },
      { questionId: 'q3', selectedIndex: null },
    ])
  })

  it('is loading first, and failed when the quiz cannot be read', async () => {
    vi.mocked(getQuiz).mockRejectedValue(new Error('nope'))

    const { result } = renderHook(() => useQuizSession('quiz-1'))
    expect(result.current.state).toBe('loading')
    expect(result.current.question).toBeNull()

    await waitFor(() => expect(result.current.state).toBe('failed'))
    expect(result.current.quiz).toBeNull()
  })

  it('reloads after a failure', async () => {
    vi.mocked(getQuiz).mockRejectedValueOnce(new Error('nope')).mockResolvedValueOnce(quiz)
    const { result } = renderHook(() => useQuizSession('quiz-1'))
    await waitFor(() => expect(result.current.state).toBe('failed'))

    act(() => result.current.reload())

    expect(result.current.state).toBe('loading')
    await waitFor(() => expect(result.current.state).toBe('ready'))
  })

  it('answers, changes and clears the answer of the current question', async () => {
    const { result } = await renderLoaded()

    act(() => result.current.answer(2))
    expect(result.current.selections[0].selectedIndex).toBe(2)
    expect(result.current.blankCount).toBe(2)

    act(() => result.current.answer(0))
    expect(result.current.selections[0].selectedIndex).toBe(0)

    act(() => result.current.answer(null))
    expect(result.current.selections[0].selectedIndex).toBeNull()
    expect(result.current.blankCount).toBe(3)
  })

  it('keeps the answers per question while moving around', async () => {
    const { result } = await renderLoaded()

    act(() => result.current.answer(1))
    act(() => result.current.next())
    act(() => result.current.answer(0))
    act(() => result.current.goTo(2))
    act(() => result.current.answer(1))
    act(() => result.current.previous())

    expect(result.current.index).toBe(1)
    expect(result.current.selections.map((selection) => selection.selectedIndex)).toEqual([1, 0, 1])
  })

  it('stays inside the quiz when moving past either end or to a bad index', async () => {
    const { result } = await renderLoaded()

    act(() => result.current.previous())
    expect(result.current.index).toBe(0)

    act(() => result.current.goTo(2))
    act(() => result.current.next())
    expect(result.current.index).toBe(2)

    act(() => result.current.goTo(3))
    expect(result.current.index).toBe(2)
    act(() => result.current.goTo(-1))
    expect(result.current.index).toBe(2)
  })

  it('submits every selection and returns the graded attempt', async () => {
    const attempt = {
      id: 'attempt-1',
      quizId: 'quiz-1',
      correctCount: 1,
      total: 3,
      score: 3.3,
      finishedAt: 'now',
    }
    vi.mocked(submitQuizAttempt).mockResolvedValue(attempt)
    const { result } = await renderLoaded()
    act(() => result.current.answer(2))

    let returned: unknown
    await act(async () => {
      returned = await result.current.submit()
    })

    expect(submitQuizAttempt).toHaveBeenCalledWith('quiz-1', [
      { questionId: 'q1', selectedIndex: 2 },
      { questionId: 'q2', selectedIndex: null },
      { questionId: 'q3', selectedIndex: null },
    ])
    expect(returned).toEqual(attempt)
    expect(result.current.submitting).toBe(false)
    expect(result.current.submitError).toBeNull()
  })

  it('shows it is submitting while the call runs', async () => {
    let finish: (value: never) => void = () => {}
    vi.mocked(submitQuizAttempt).mockReturnValue(new Promise((resolve) => (finish = resolve)))
    const { result } = await renderLoaded()

    let pending: Promise<unknown> = Promise.resolve()
    act(() => {
      pending = result.current.submit()
    })
    expect(result.current.submitting).toBe(true)

    await act(async () => {
      finish({ id: 'a' } as never)
      await pending
    })
    expect(result.current.submitting).toBe(false)
  })

  it('keeps the answers and reports the error when the submission fails, then lets it retry', async () => {
    vi.mocked(submitQuizAttempt)
      .mockRejectedValueOnce(new Error(JSON.stringify({ code: 'quiz_not_found' })))
      .mockResolvedValueOnce({ id: 'attempt-2' } as never)
    const { result } = await renderLoaded()
    act(() => result.current.answer(1))

    let first: unknown = 'unset'
    await act(async () => {
      first = await result.current.submit()
    })

    expect(first).toBeNull()
    expect(result.current.submitError).toBe('That test no longer exists.')
    expect(result.current.selections[0].selectedIndex).toBe(1)

    let second: unknown
    await act(async () => {
      second = await result.current.submit()
    })

    expect(second).toEqual({ id: 'attempt-2' })
    expect(result.current.submitError).toBeNull()
  })

  it('does not submit twice at once, nor before the quiz has loaded', async () => {
    vi.mocked(getQuiz).mockReturnValue(new Promise(() => {}))
    const loading = renderHook(() => useQuizSession('quiz-1'))
    await act(async () => {
      await loading.result.current.submit()
    })
    expect(submitQuizAttempt).not.toHaveBeenCalled()

    vi.mocked(submitQuizAttempt).mockReturnValue(new Promise(() => {}))
    const { result } = await renderLoaded()
    act(() => {
      void result.current.submit()
    })
    await act(async () => {
      await result.current.submit()
    })
    expect(submitQuizAttempt).toHaveBeenCalledTimes(1)
  })
})
