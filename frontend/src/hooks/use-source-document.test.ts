import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { getSessionSourceDocument } from '@/lib/sources'
import { useSourceDocument } from './use-source-document'

vi.mock('@/lib/sources', () => ({
  getSessionSourceDocument: vi.fn(),
}))

const documentA = {
  itemId: 'item-a',
  title: 'Go',
  path: 'go.md',
  segments: [{ text: 'Go is a language.', chunkId: 'chunk-a' }],
}
const documentB = {
  itemId: 'item-b',
  title: 'Rust',
  path: 'rust.md',
  segments: [{ text: 'Rust is a language.', chunkId: 'chunk-b' }],
}

describe('useSourceDocument', () => {
  it('does not load anything while itemId is null', () => {
    // Given no item requested (the viewer is closed)
    const { result } = renderHook(() => useSourceDocument('session-1', null))

    // Then nothing loads
    expect(result.current.loading).toBe(false)
    expect(result.current.document).toBeNull()
    expect(result.current.error).toBeNull()
    expect(getSessionSourceDocument).not.toHaveBeenCalled()
  })

  it('starts loading, then exposes the loaded document, once itemId is set', async () => {
    // Given a document that resolves
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentA)

    // When the hook mounts with an item id
    const { result } = renderHook(() => useSourceDocument('session-1', 'item-a'))

    // Then it starts loading with no document or error yet
    expect(result.current.loading).toBe(true)
    expect(result.current.document).toBeNull()
    expect(result.current.error).toBeNull()

    // And settles on the loaded document
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.document).toEqual(documentA)
    expect(getSessionSourceDocument).toHaveBeenCalledWith('session-1', 'item-a')
  })

  it('exposes the failure message when loading fails', async () => {
    // Given a document that fails to load
    vi.mocked(getSessionSourceDocument).mockRejectedValueOnce(new Error('ingest: source not found'))

    // When the hook mounts with an item id
    const { result } = renderHook(() => useSourceDocument('session-1', 'item-a'))

    // Then it settles with the error message and no document
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('ingest: source not found')
    expect(result.current.document).toBeNull()
  })

  it('falls back to String(err) when the rejection is not an Error', async () => {
    // Given a rejection that is not an Error instance
    vi.mocked(getSessionSourceDocument).mockRejectedValueOnce('boom')

    // When the hook mounts with an item id
    const { result } = renderHook(() => useSourceDocument('session-1', 'item-a'))

    // Then the error message falls back to the stringified value
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('boom')
  })

  it('reloads when itemId changes to a different document', async () => {
    // Given item-a already loaded
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentA)
    const { result, rerender } = renderHook(
      ({ itemId }) => useSourceDocument('session-1', itemId),
      {
        initialProps: { itemId: 'item-a' as string | null },
      },
    )
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.document).toEqual(documentA)

    // When switching to a different item
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentB)
    rerender({ itemId: 'item-b' })

    // Then it reloads for the new item
    expect(result.current.loading).toBe(true)
    await waitFor(() => expect(result.current.document).toEqual(documentB))
    expect(getSessionSourceDocument).toHaveBeenCalledWith('session-1', 'item-b')
  })

  it('clears the document without calling the API again when itemId goes back to null', async () => {
    // Given item-a already loaded (the viewer was open)
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentA)
    const { result, rerender } = renderHook(
      ({ itemId }) => useSourceDocument('session-1', itemId),
      {
        initialProps: { itemId: 'item-a' as string | null },
      },
    )
    await waitFor(() => expect(result.current.loading).toBe(false))

    // When the viewer closes (itemId back to null)
    rerender({ itemId: null })

    // Then the document is cleared and no new call is made
    expect(result.current.document).toBeNull()
    expect(result.current.loading).toBe(false)
    expect(getSessionSourceDocument).toHaveBeenCalledTimes(1)
  })

  it('ignores a response that resolves after itemId has already changed', async () => {
    // Given item-a's load that never resolves until told to
    let resolveItemA!: (doc: typeof documentA) => void
    vi.mocked(getSessionSourceDocument).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveItemA = resolve
        }),
    )
    const { result, rerender } = renderHook(
      ({ itemId }) => useSourceDocument('session-1', itemId),
      {
        initialProps: { itemId: 'item-a' as string | null },
      },
    )

    // When switching to item-b before item-a's call resolves
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentB)
    rerender({ itemId: 'item-b' })
    await waitFor(() => expect(result.current.document).toEqual(documentB))

    // And item-a's stale response arrives late
    await act(async () => {
      resolveItemA(documentA)
    })

    // Then it is ignored — item-b's document is still shown
    expect(result.current.document).toEqual(documentB)
  })

  it('ignores a rejection that resolves after itemId has already changed', async () => {
    // Given item-a's load that never settles until told to
    let rejectItemA!: (err: unknown) => void
    vi.mocked(getSessionSourceDocument).mockImplementationOnce(
      () =>
        new Promise((_resolve, reject) => {
          rejectItemA = reject
        }),
    )
    const { result, rerender } = renderHook(
      ({ itemId }) => useSourceDocument('session-1', itemId),
      {
        initialProps: { itemId: 'item-a' as string | null },
      },
    )

    // When switching to item-b before item-a's call rejects
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentB)
    rerender({ itemId: 'item-b' })
    await waitFor(() => expect(result.current.document).toEqual(documentB))

    // And item-a's stale rejection arrives late
    await act(async () => {
      rejectItemA(new Error('too late'))
    })

    // Then it is ignored — item-b's document is still shown, with no error
    expect(result.current.document).toEqual(documentB)
    expect(result.current.error).toBeNull()
  })

  it('reloads when sessionId changes while itemId stays the same', async () => {
    // Given item-a loaded under session-1
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentA)
    const { result, rerender } = renderHook(
      ({ sessionId }) => useSourceDocument(sessionId, 'item-a'),
      { initialProps: { sessionId: 'session-1' } },
    )
    await waitFor(() => expect(result.current.loading).toBe(false))

    // When the session id changes but the item id does not
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentB)
    rerender({ sessionId: 'session-2' })

    // Then it reloads under the new session
    expect(result.current.loading).toBe(true)
    await waitFor(() => expect(result.current.document).toEqual(documentB))
    expect(getSessionSourceDocument).toHaveBeenCalledWith('session-2', 'item-a')
  })

  it('reload re-fetches the current document', async () => {
    // Given a document already loaded
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentA)
    const { result } = renderHook(() => useSourceDocument('session-1', 'item-a'))
    await waitFor(() => expect(result.current.loading).toBe(false))

    // When calling reload after the document changed on re-import
    const updated = { ...documentA, title: 'Go (updated)' }
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(updated)
    act(() => {
      result.current.reload()
    })

    // Then it re-fetches and exposes the updated document
    await waitFor(() => expect(result.current.document).toEqual(updated))
    expect(getSessionSourceDocument).toHaveBeenCalledTimes(2)
  })

  it('clears the previously loaded document when a reload fails', async () => {
    // Given a document already loaded successfully
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentA)
    const { result } = renderHook(() => useSourceDocument('session-1', 'item-a'))
    await waitFor(() => expect(result.current.document).toEqual(documentA))

    // When reload is called and the new call fails (e.g. the source was
    // removed in the meantime)
    vi.mocked(getSessionSourceDocument).mockRejectedValueOnce(new Error('ingest: source not found'))
    act(() => {
      result.current.reload()
    })

    // Then the stale document is cleared, not kept stale alongside the error
    await waitFor(() => expect(result.current.error).toBe('ingest: source not found'))
    expect(result.current.document).toBeNull()
  })

  it('reload reflects the current itemId, not a stale one from before it changed', async () => {
    // Given item-a loaded, and a captured reload reference
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(documentA)
    const { result, rerender } = renderHook(
      ({ itemId }) => useSourceDocument('session-1', itemId),
      {
        initialProps: { itemId: 'item-a' as string | null },
      },
    )
    await waitFor(() => expect(result.current.loading).toBe(false))

    // When the viewer closes (itemId becomes null) and the latest reload is
    // called
    rerender({ itemId: null })
    act(() => {
      result.current.reload()
    })

    // Then it is a no-op — reload must see the current itemId (null), not a
    // closure captured back when itemId was still "item-a"
    expect(getSessionSourceDocument).toHaveBeenCalledTimes(1)
    expect(result.current.loading).toBe(false)
  })

  it('calling reload twice re-fetches both times', async () => {
    // Given a document already loaded
    vi.mocked(getSessionSourceDocument).mockResolvedValue(documentA)
    const { result } = renderHook(() => useSourceDocument('session-1', 'item-a'))
    await waitFor(() => expect(result.current.loading).toBe(false))

    // When calling reload twice in a row
    act(() => {
      result.current.reload()
    })
    await waitFor(() => expect(getSessionSourceDocument).toHaveBeenCalledTimes(2))
    act(() => {
      result.current.reload()
    })

    // Then both reloads triggered their own fetch
    await waitFor(() => expect(getSessionSourceDocument).toHaveBeenCalledTimes(3))
  })

  it('resets to loading and clears a previous error the moment reload is called', async () => {
    // Given a document whose load just failed
    vi.mocked(getSessionSourceDocument).mockRejectedValueOnce(new Error('boom'))
    const { result } = renderHook(() => useSourceDocument('session-1', 'item-a'))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('boom')

    // When reload is called, before its new call resolves
    vi.mocked(getSessionSourceDocument).mockImplementationOnce(() => new Promise(() => {}))
    act(() => {
      result.current.reload()
    })

    // Then it immediately shows loading with no stale error
    expect(result.current.loading).toBe(true)
    expect(result.current.error).toBeNull()
  })

  it('reload is a no-op while itemId is null', () => {
    // Given no item requested
    const { result } = renderHook(() => useSourceDocument('session-1', null))

    // When calling reload anyway
    act(() => {
      result.current.reload()
    })

    // Then nothing happens
    expect(result.current.loading).toBe(false)
    expect(getSessionSourceDocument).not.toHaveBeenCalled()
  })

  it('resets to loading and clears a previous error the moment itemId changes', async () => {
    // Given item-a, whose load just failed
    vi.mocked(getSessionSourceDocument).mockRejectedValueOnce(new Error('boom'))
    const { result, rerender } = renderHook(
      ({ itemId }) => useSourceDocument('session-1', itemId),
      {
        initialProps: { itemId: 'item-a' as string | null },
      },
    )
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('boom')

    // When switching to item-b, before its own load resolves
    vi.mocked(getSessionSourceDocument).mockImplementationOnce(() => new Promise(() => {}))
    rerender({ itemId: 'item-b' })

    // Then it immediately shows loading with no stale error from item-a
    expect(result.current.loading).toBe(true)
    expect(result.current.error).toBeNull()
  })
})
