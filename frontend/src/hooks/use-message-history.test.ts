import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { listMessagesBefore, type StudyMessage, type StudyMessagePage } from '@/lib/study'
import { HISTORY_PAGE_SIZE, useMessageHistory } from './use-message-history'

vi.mock('@/lib/study', () => ({
  listMessagesBefore: vi.fn(),
}))

function message(id: string): StudyMessage {
  return { id, role: 'user', content: id, createdAt: '2026-08-16T10:00:00Z', sources: [] }
}

function page(ids: string[], hasMore: boolean, nextCursor: string): StudyMessagePage {
  return { messages: ids.map(message), hasMore, nextCursor }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('useMessageHistory', () => {
  it('has nothing to load until started', () => {
    // Given a hook that was never told about a resumed page
    const onPage = vi.fn()
    const { result } = renderHook(() => useMessageHistory('session-1', onPage))

    // When asking for older messages
    act(() => {
      void result.current.loadOlder()
    })

    // Then nothing is requested
    expect(result.current.hasMore).toBe(false)
    expect(result.current.status).toBe('idle')
    expect(listMessagesBefore).not.toHaveBeenCalled()
  })

  it('loads the page before the cursor, hands it over and advances the cursor', async () => {
    // Given a resumed session with older messages behind cursor m51
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(page(['m1', 'm2'], true, 'm1'))
    const onPage = vi.fn()
    const { result } = renderHook(() => useMessageHistory('session-1', onPage))
    act(() => result.current.start(true, 'm51'))
    expect(result.current.hasMore).toBe(true)

    // When loading older messages
    await act(async () => {
      await result.current.loadOlder()
    })

    // Then it requested one page at that cursor
    expect(listMessagesBefore).toHaveBeenCalledWith('session-1', 'm51', HISTORY_PAGE_SIZE)
    // And handed the fetched messages to the caller
    expect(onPage).toHaveBeenCalledWith([message('m1'), message('m2')])
    expect(result.current.status).toBe('idle')
    expect(result.current.hasMore).toBe(true)

    // And the next load continues from the returned cursor
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(page(['m0'], false, ''))
    await act(async () => {
      await result.current.loadOlder()
    })
    expect(listMessagesBefore).toHaveBeenLastCalledWith('session-1', 'm1', HISTORY_PAGE_SIZE)
  })

  it('hands the page to the latest onPage even if it changed since the hook started', async () => {
    // Given a hook started with one callback, then re-rendered with another
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(page(['m1'], false, ''))
    const first = vi.fn()
    const second = vi.fn()
    const { result, rerender } = renderHook(
      ({ onPage }) => useMessageHistory('session-1', onPage),
      {
        initialProps: { onPage: first },
      },
    )
    act(() => result.current.start(true, 'm2'))
    rerender({ onPage: second })

    // When the page arrives
    await act(async () => {
      await result.current.loadOlder()
    })

    // Then only the latest callback receives it
    expect(second).toHaveBeenCalledWith([message('m1')])
    expect(first).not.toHaveBeenCalled()
  })

  it('stops requesting once the last page was loaded', async () => {
    // Given a session whose next page is the last one
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(page(['m1'], false, ''))
    const { result } = renderHook(() => useMessageHistory('session-1', vi.fn()))
    act(() => result.current.start(true, 'm2'))
    await act(async () => {
      await result.current.loadOlder()
    })
    expect(result.current.hasMore).toBe(false)

    // When asking for more
    await act(async () => {
      await result.current.loadOlder()
    })

    // Then no further request is made
    expect(listMessagesBefore).toHaveBeenCalledTimes(1)
  })

  it('issues a single request while one is in flight', async () => {
    // Given a page request that has not resolved yet
    const inFlight = deferred<StudyMessagePage>()
    vi.mocked(listMessagesBefore).mockReturnValueOnce(inFlight.promise)
    const { result } = renderHook(() => useMessageHistory('session-1', vi.fn()))
    act(() => result.current.start(true, 'm2'))

    // When loadOlder is called repeatedly meanwhile
    act(() => {
      void result.current.loadOlder()
      void result.current.loadOlder()
    })

    // Then it is loading and only one request was made
    expect(result.current.status).toBe('loading')
    expect(listMessagesBefore).toHaveBeenCalledTimes(1)

    await act(async () => {
      inFlight.resolve(page(['m1'], false, ''))
    })
    await waitFor(() => expect(result.current.status).toBe('idle'))
  })

  it('keeps the cursor on failure so a retry requests the same page again', async () => {
    // Given a page request that fails
    vi.mocked(listMessagesBefore).mockRejectedValueOnce(new Error('boom'))
    const onPage = vi.fn()
    const { result } = renderHook(() => useMessageHistory('session-1', onPage))
    act(() => result.current.start(true, 'm2'))
    await act(async () => {
      await result.current.loadOlder()
    })

    // Then it reports the error and handed nothing over
    expect(result.current.status).toBe('error')
    expect(result.current.error).toBe('boom')
    expect(result.current.hasMore).toBe(true)
    expect(onPage).not.toHaveBeenCalled()

    // When retrying and the request now succeeds
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(page(['m1'], false, ''))
    await act(async () => {
      await result.current.loadOlder()
    })

    // Then the same cursor was requested twice and the error is gone
    expect(listMessagesBefore).toHaveBeenNthCalledWith(1, 'session-1', 'm2', HISTORY_PAGE_SIZE)
    expect(listMessagesBefore).toHaveBeenNthCalledWith(2, 'session-1', 'm2', HISTORY_PAGE_SIZE)
    expect(result.current.status).toBe('idle')
    expect(result.current.error).toBeNull()
    expect(onPage).toHaveBeenCalledWith([message('m1')])
  })

  it('falls back to a generic message when the rejection is not an Error', async () => {
    // Given a page request rejected with a plain string
    vi.mocked(listMessagesBefore).mockRejectedValueOnce('nope')
    const { result } = renderHook(() => useMessageHistory('session-1', vi.fn()))
    act(() => result.current.start(true, 'm2'))

    // When loading
    await act(async () => {
      await result.current.loadOlder()
    })

    // Then the error is the generic one
    expect(result.current.error).toBe('Failed to load earlier messages.')
  })

  it('ignores a response that arrives after the session changed', async () => {
    // Given a page request in flight for session-1
    const inFlight = deferred<StudyMessagePage>()
    vi.mocked(listMessagesBefore).mockReturnValueOnce(inFlight.promise)
    const onPage = vi.fn()
    const { result, rerender } = renderHook(({ id }) => useMessageHistory(id, onPage), {
      initialProps: { id: 'session-1' },
    })
    act(() => result.current.start(true, 'm2'))
    act(() => {
      void result.current.loadOlder()
    })

    // When the user switches to another session before it resolves
    rerender({ id: 'session-2' })
    await act(async () => {
      inFlight.resolve(page(['m1'], true, 'm1'))
    })

    // Then the stale page is neither handed over nor applied
    expect(onPage).not.toHaveBeenCalled()
    expect(result.current.hasMore).toBe(false)
    expect(result.current.status).toBe('idle')
  })

  it('ignores a failure that arrives after the session changed', async () => {
    // Given a page request in flight for session-1
    const inFlight = deferred<StudyMessagePage>()
    vi.mocked(listMessagesBefore).mockReturnValueOnce(inFlight.promise)
    const { result, rerender } = renderHook(({ id }) => useMessageHistory(id, vi.fn()), {
      initialProps: { id: 'session-1' },
    })
    act(() => result.current.start(true, 'm2'))
    act(() => {
      void result.current.loadOlder()
    })

    // When the session changes and the old request then fails
    rerender({ id: 'session-2' })
    await act(async () => {
      inFlight.reject(new Error('late'))
    })

    // Then the new session shows no error
    expect(result.current.error).toBeNull()
    expect(result.current.status).toBe('idle')
  })

  it('can load for the new session while the old one was still in flight', async () => {
    // Given session-1's request is stuck in flight
    const stuck = deferred<StudyMessagePage>()
    vi.mocked(listMessagesBefore).mockReturnValueOnce(stuck.promise)
    const onPage = vi.fn()
    const { result, rerender } = renderHook(({ id }) => useMessageHistory(id, onPage), {
      initialProps: { id: 'session-1' },
    })
    act(() => result.current.start(true, 'm2'))
    act(() => {
      void result.current.loadOlder()
    })

    // When the user switches sessions and session-2's own request is in flight
    rerender({ id: 'session-2' })
    const current = deferred<StudyMessagePage>()
    vi.mocked(listMessagesBefore).mockReturnValueOnce(current.promise)
    act(() => result.current.start(true, 'x2'))
    act(() => {
      void result.current.loadOlder()
    })
    expect(listMessagesBefore).toHaveBeenLastCalledWith('session-2', 'x2', HISTORY_PAGE_SIZE)

    // And the stale request settles meanwhile
    await act(async () => {
      stuck.resolve(page(['m1'], true, 'm1'))
    })

    // Then it neither releases session-2's in-flight guard nor hands anything over
    act(() => {
      void result.current.loadOlder()
    })
    expect(listMessagesBefore).toHaveBeenCalledTimes(2)
    expect(onPage).not.toHaveBeenCalled()

    // And session-2's own page still arrives
    await act(async () => {
      current.resolve(page(['x1'], false, ''))
    })
    expect(onPage).toHaveBeenCalledWith([message('x1')])
  })

  it('clears history state when the session changes', async () => {
    // Given a session with older messages and a failed load
    vi.mocked(listMessagesBefore).mockRejectedValueOnce(new Error('boom'))
    const { result, rerender } = renderHook(({ id }) => useMessageHistory(id, vi.fn()), {
      initialProps: { id: 'session-1' },
    })
    act(() => result.current.start(true, 'm2'))
    await act(async () => {
      await result.current.loadOlder()
    })
    expect(result.current.status).toBe('error')

    // When switching to another session
    rerender({ id: 'session-2' })

    // Then nothing is carried over
    expect(result.current.hasMore).toBe(false)
    expect(result.current.status).toBe('idle')
    expect(result.current.error).toBeNull()
  })
})
