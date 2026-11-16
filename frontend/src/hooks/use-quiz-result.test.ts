import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getQuizAttemptResult } from '@/lib/quiz'
import { RESULT } from '@/test/quiz-fixtures'
import { useQuizResult } from './use-quiz-result'

vi.mock('@/lib/quiz', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/quiz')>()),
  getQuizAttemptResult: vi.fn(),
}))

describe('useQuizResult', () => {
  beforeEach(() => vi.clearAllMocks())

  it('loads the result of an attempt', async () => {
    vi.mocked(getQuizAttemptResult).mockResolvedValue(RESULT)

    const { result } = renderHook(() => useQuizResult('attempt-1'))
    expect(result.current.state).toBe('loading')
    expect(result.current.result).toBeNull()

    await waitFor(() => expect(result.current.state).toBe('ready'))
    expect(getQuizAttemptResult).toHaveBeenCalledWith('attempt-1')
    expect(result.current.result).toEqual(RESULT)
  })

  it('fails when the result cannot be read and reloads on request', async () => {
    vi.mocked(getQuizAttemptResult)
      .mockRejectedValueOnce(new Error('nope'))
      .mockResolvedValueOnce(RESULT)
    const { result } = renderHook(() => useQuizResult('attempt-1'))
    await waitFor(() => expect(result.current.state).toBe('failed'))

    act(() => result.current.reload())

    expect(result.current.state).toBe('loading')
    await waitFor(() => expect(result.current.state).toBe('ready'))
  })

  it('reads the new attempt when the id changes and drops the old result meanwhile', async () => {
    vi.mocked(getQuizAttemptResult).mockResolvedValue(RESULT)
    const { result, rerender } = renderHook(({ id }) => useQuizResult(id), {
      initialProps: { id: 'attempt-1' },
    })
    await waitFor(() => expect(result.current.state).toBe('ready'))

    rerender({ id: 'attempt-2' })

    expect(result.current.state).toBe('loading')
    await waitFor(() => expect(result.current.state).toBe('ready'))
    expect(getQuizAttemptResult).toHaveBeenLastCalledWith('attempt-2')
  })
})
