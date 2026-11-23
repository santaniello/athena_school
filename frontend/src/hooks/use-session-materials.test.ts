import { describe, expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { useSessionMaterials } from './use-session-materials'
import { listSessionMindMaps } from '@/lib/mindmap'
import { listSessionQuizzes } from '@/lib/quiz'
import { listSessionExercises } from '@/lib/exercise'

vi.mock('@/lib/mindmap', () => ({ listSessionMindMaps: vi.fn() }))
vi.mock('@/lib/quiz', () => ({ listSessionQuizzes: vi.fn() }))
vi.mock('@/lib/exercise', () => ({ listSessionExercises: vi.fn() }))

const map = { id: 'm1', title: 'Map' }
const quiz = { id: 'q1' }
const lab = { sessionExerciseId: 'e1' }

function mockAll() {
  vi.mocked(listSessionMindMaps).mockResolvedValue([map] as never)
  vi.mocked(listSessionQuizzes).mockResolvedValue([quiz] as never)
  vi.mocked(listSessionExercises).mockResolvedValue([lab] as never)
}

describe('useSessionMaterials', () => {
  it('starts loading and then exposes the three lists of the session', async () => {
    mockAll()
    const { result } = renderHook(() => useSessionMaterials('s1', 0))
    expect(result.current.mindMaps.status).toBe('loading')

    await waitFor(() => expect(result.current.mindMaps.status).toBe('ready'))
    expect(listSessionMindMaps).toHaveBeenCalledWith('s1')
    expect(listSessionQuizzes).toHaveBeenCalledWith('s1')
    expect(listSessionExercises).toHaveBeenCalledWith('s1')
    await waitFor(() => expect(result.current.quizzes.status).toBe('ready'))
    await waitFor(() => expect(result.current.labs.status).toBe('ready'))
    expect(result.current.mindMaps.items).toEqual([map])
    expect(result.current.quizzes.items).toEqual([quiz])
    expect(result.current.labs.items).toEqual([lab])
  })

  it('reports an error for the list that failed without affecting the others', async () => {
    mockAll()
    vi.mocked(listSessionQuizzes).mockRejectedValue(new Error('boom'))
    const { result } = renderHook(() => useSessionMaterials('s1', 0))

    await waitFor(() => expect(result.current.quizzes.status).toBe('error'))
    await waitFor(() => expect(result.current.mindMaps.status).toBe('ready'))
    expect(result.current.quizzes.items).toEqual([])
  })

  it('reloads when the revision changes and keeps the lists while reloading', async () => {
    mockAll()
    const { result, rerender } = renderHook(({ revision }) => useSessionMaterials('s1', revision), {
      initialProps: { revision: 0 },
    })
    await waitFor(() => expect(result.current.quizzes.status).toBe('ready'))

    vi.mocked(listSessionQuizzes).mockResolvedValue([quiz, { id: 'q2' }] as never)
    rerender({ revision: 1 })

    expect(result.current.quizzes.items).toEqual([quiz])
    await waitFor(() => expect(result.current.quizzes.items).toHaveLength(2))
  })

  it('keeps the previous list when a reload fails', async () => {
    mockAll()
    const { result, rerender } = renderHook(({ revision }) => useSessionMaterials('s1', revision), {
      initialProps: { revision: 0 },
    })
    await waitFor(() => expect(result.current.quizzes.status).toBe('ready'))

    vi.mocked(listSessionQuizzes).mockRejectedValue(new Error('boom'))
    rerender({ revision: 1 })

    await waitFor(() => expect(listSessionQuizzes).toHaveBeenCalledTimes(2))
    expect(result.current.quizzes.status).toBe('ready')
    expect(result.current.quizzes.items).toEqual([quiz])
  })

  it('starts over when the session changes, dropping the old session lists', async () => {
    mockAll()
    const { result, rerender } = renderHook(({ id }) => useSessionMaterials(id, 0), {
      initialProps: { id: 's1' },
    })
    await waitFor(() => expect(result.current.quizzes.status).toBe('ready'))

    vi.mocked(listSessionQuizzes).mockResolvedValue([])
    rerender({ id: 's2' })

    expect(result.current.quizzes.status).toBe('loading')
    expect(result.current.quizzes.items).toEqual([])
    await waitFor(() => expect(result.current.quizzes.status).toBe('ready'))
    expect(result.current.quizzes.items).toEqual([])
  })

  it('discards an answer that arrives after the session changed', async () => {
    let resolveOld: (value: never) => void = () => {}
    vi.mocked(listSessionMindMaps).mockImplementationOnce(
      () => new Promise((resolve) => (resolveOld = resolve as never)),
    )
    vi.mocked(listSessionMindMaps).mockResolvedValue([])
    vi.mocked(listSessionQuizzes).mockResolvedValue([])
    vi.mocked(listSessionExercises).mockResolvedValue([])
    const { result, rerender } = renderHook(({ id }) => useSessionMaterials(id, 0), {
      initialProps: { id: 's1' },
    })
    rerender({ id: 's2' })
    await waitFor(() => expect(result.current.mindMaps.status).toBe('ready'))

    await act(async () => resolveOld([map] as never))

    expect(result.current.mindMaps.items).toEqual([])
  })

  it('retry reloads only the requested list', async () => {
    mockAll()
    vi.mocked(listSessionQuizzes).mockRejectedValueOnce(new Error('boom'))
    const { result } = renderHook(() => useSessionMaterials('s1', 0))
    await waitFor(() => expect(result.current.quizzes.status).toBe('error'))

    act(() => result.current.quizzes.retry())

    await waitFor(() => expect(result.current.quizzes.status).toBe('ready'))
    expect(listSessionQuizzes).toHaveBeenCalledTimes(2)
    expect(listSessionMindMaps).toHaveBeenCalledTimes(1)
  })
})
