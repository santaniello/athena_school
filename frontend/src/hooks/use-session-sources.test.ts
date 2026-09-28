import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { listSessionSources } from '@/lib/sources'
import { useSessionSources } from './use-session-sources'

vi.mock('@/lib/sources', () => ({
  listSessionSources: vi.fn(),
}))

const sourceA = { itemId: 'item-a', title: 'Go', path: 'go.md', chunkCount: 3, ingestedAt: '2024-01-01T00:00:00Z' }
const sourceB = { itemId: 'item-b', title: 'Rust', path: 'rust.md', chunkCount: 5, ingestedAt: '2024-01-02T00:00:00Z' }

describe('useSessionSources', () => {
  it('starts loading, then exposes the loaded sources', async () => {
    // Given a session with one imported document
    vi.mocked(listSessionSources).mockResolvedValueOnce([sourceA])

    // When the hook mounts for that session
    const { result } = renderHook(() => useSessionSources('session-1'))

    // Then it starts in a loading state with no sources or error yet
    expect(result.current.loading).toBe(true)
    expect(result.current.sources).toEqual([])
    expect(result.current.error).toBeNull()

    // And settles on the loaded sources
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.sources).toEqual([sourceA])
    expect(result.current.error).toBeNull()
    expect(listSessionSources).toHaveBeenCalledWith('session-1')
  })

  it('exposes the failure message when loading fails', async () => {
    // Given a session whose sources fail to load
    vi.mocked(listSessionSources).mockRejectedValueOnce(new Error('database unavailable'))

    // When the hook mounts
    const { result } = renderHook(() => useSessionSources('session-1'))

    // Then it settles with the error message and an empty list
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('database unavailable')
    expect(result.current.sources).toEqual([])
  })

  it('reloads when the session id changes, discarding the previous session', async () => {
    // Given two sessions with different sources
    vi.mocked(listSessionSources).mockResolvedValueOnce([sourceA])
    const { result, rerender } = renderHook(({ sessionId }) => useSessionSources(sessionId), {
      initialProps: { sessionId: 'session-1' },
    })
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.sources).toEqual([sourceA])

    // When switching to a different session
    vi.mocked(listSessionSources).mockResolvedValueOnce([sourceB])
    rerender({ sessionId: 'session-2' })

    // Then it reloads for the new session
    await waitFor(() => expect(result.current.sources).toEqual([sourceB]))
    expect(listSessionSources).toHaveBeenCalledWith('session-2')
  })

  it('ignores a response that resolves after the session has already changed', async () => {
    // Given session-1's load that never resolves until told to
    let resolveSessionOne!: (sources: typeof sourceA[]) => void
    vi.mocked(listSessionSources).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveSessionOne = resolve
        }),
    )
    const { result, rerender } = renderHook(({ sessionId }) => useSessionSources(sessionId), {
      initialProps: { sessionId: 'session-1' },
    })

    // When switching away to session-2 before session-1's call resolves
    vi.mocked(listSessionSources).mockResolvedValueOnce([sourceB])
    rerender({ sessionId: 'session-2' })
    await waitFor(() => expect(result.current.sources).toEqual([sourceB]))

    // And session-1's stale response arrives late
    await act(async () => {
      resolveSessionOne([sourceA])
    })

    // Then it is ignored — session-2's sources are still shown
    expect(result.current.sources).toEqual([sourceB])
  })

  it('falls back to String(err) when the rejection is not an Error', async () => {
    // Given a rejection that is not an Error instance
    vi.mocked(listSessionSources).mockRejectedValueOnce('boom')

    // When the hook mounts
    const { result } = renderHook(() => useSessionSources('session-1'))

    // Then the error message falls back to the stringified value
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('boom')
  })

  it('ignores a rejection that resolves after the session has already changed', async () => {
    // Given session-1's load that never settles until told to
    let rejectSessionOne!: (err: unknown) => void
    vi.mocked(listSessionSources).mockImplementationOnce(
      () =>
        new Promise((_resolve, reject) => {
          rejectSessionOne = reject
        }),
    )
    const { result, rerender } = renderHook(({ sessionId }) => useSessionSources(sessionId), {
      initialProps: { sessionId: 'session-1' },
    })

    // When switching away to session-2 before session-1's call rejects
    vi.mocked(listSessionSources).mockResolvedValueOnce([sourceB])
    rerender({ sessionId: 'session-2' })
    await waitFor(() => expect(result.current.sources).toEqual([sourceB]))

    // And session-1's stale rejection arrives late
    await act(async () => {
      rejectSessionOne(new Error('too late'))
    })

    // Then it is ignored — session-2's sources are still shown, with no error
    expect(result.current.sources).toEqual([sourceB])
    expect(result.current.error).toBeNull()
  })

  it('reload re-fetches the current session', async () => {
    // Given a session already loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([sourceA])
    const { result } = renderHook(() => useSessionSources('session-1'))
    await waitFor(() => expect(result.current.loading).toBe(false))

    // When calling reload after a new document was added
    vi.mocked(listSessionSources).mockResolvedValueOnce([sourceA, sourceB])
    act(() => {
      result.current.reload()
    })

    // Then it re-fetches and exposes the updated list
    await waitFor(() => expect(result.current.sources).toEqual([sourceA, sourceB]))
    expect(listSessionSources).toHaveBeenCalledTimes(2)
  })

  it('calling reload twice re-fetches both times', async () => {
    // Given a session already loaded
    vi.mocked(listSessionSources).mockResolvedValue([sourceA])
    const { result } = renderHook(() => useSessionSources('session-1'))
    await waitFor(() => expect(result.current.loading).toBe(false))

    // When calling reload twice in a row
    act(() => {
      result.current.reload()
    })
    await waitFor(() => expect(listSessionSources).toHaveBeenCalledTimes(2))
    act(() => {
      result.current.reload()
    })

    // Then both reloads triggered their own fetch
    await waitFor(() => expect(listSessionSources).toHaveBeenCalledTimes(3))
  })

  it('resets to loading and clears a previous error the moment the session id changes', async () => {
    // Given session-1, whose load just failed
    vi.mocked(listSessionSources).mockRejectedValueOnce(new Error('boom'))
    const { result, rerender } = renderHook(({ sessionId }) => useSessionSources(sessionId), {
      initialProps: { sessionId: 'session-1' },
    })
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('boom')

    // When switching to session-2, before its own load resolves
    vi.mocked(listSessionSources).mockImplementationOnce(() => new Promise(() => {}))
    rerender({ sessionId: 'session-2' })

    // Then it immediately shows loading with no stale error from session-1
    expect(result.current.loading).toBe(true)
    expect(result.current.error).toBeNull()
  })

  it('resets to loading and clears a previous error when reload is called', async () => {
    // Given a session whose load just failed
    vi.mocked(listSessionSources).mockRejectedValueOnce(new Error('boom'))
    const { result } = renderHook(() => useSessionSources('session-1'))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('boom')

    // When reload is called, before its new call resolves
    vi.mocked(listSessionSources).mockImplementationOnce(() => new Promise(() => {}))
    act(() => {
      result.current.reload()
    })

    // Then it immediately shows loading with no stale error
    expect(result.current.loading).toBe(true)
    expect(result.current.error).toBeNull()
  })
})
