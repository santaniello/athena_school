import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { listChallengeCatalog, type Catalog } from '@/lib/exercise'
import { useChallengeCatalog } from './use-challenge-catalog'

vi.mock('@/lib/exercise', () => ({
  listChallengeCatalog: vi.fn(),
}))

function catalogOf(...ids: string[]): Catalog {
  return {
    total: ids.length,
    completed: 0,
    inProgress: 0,
    items: ids.map((id) => ({
      id,
      title: id,
      topic: 'caching',
      difficulty: 'BEGINNER',
      timeLimitMinutes: 30,
      status: 'NOT_STARTED',
      attempts: 0,
      bestDiagramScore: 0,
    })),
  }
}

describe('useChallengeCatalog', () => {
  it('starts loading, then exposes the catalog for the filter', async () => {
    // Given a catalog with one challenge
    vi.mocked(listChallengeCatalog).mockResolvedValueOnce(catalogOf('c1'))

    // When the hook mounts with a filter
    const { result } = renderHook(() => useChallengeCatalog({ query: 'cache' }))

    // Then it starts loading with no catalog and no error
    expect(result.current.loading).toBe(true)
    expect(result.current.catalog).toBeNull()
    expect(result.current.error).toBeNull()

    // And settles on the catalog, having asked for that filter
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.catalog).toEqual(catalogOf('c1'))
    expect(listChallengeCatalog).toHaveBeenCalledWith({ query: 'cache' })
  })

  it('reports a failure to load and keeps no catalog', async () => {
    // Given a catalog that fails to load
    vi.mocked(listChallengeCatalog).mockRejectedValueOnce(new Error('boom'))

    // When the hook mounts
    const { result } = renderHook(() => useChallengeCatalog({}))

    // Then the error is exposed
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('Could not load the challenges.')
    expect(result.current.catalog).toBeNull()
  })

  it('loads again when the filter changes', async () => {
    // Given a hook already showing the unfiltered catalog
    vi.mocked(listChallengeCatalog)
      .mockResolvedValueOnce(catalogOf('c1', 'c2'))
      .mockResolvedValueOnce(catalogOf('c2'))
    const { result, rerender } = renderHook(({ query }) => useChallengeCatalog({ query }), {
      initialProps: { query: '' },
    })
    await waitFor(() => expect(result.current.catalog).toEqual(catalogOf('c1', 'c2')))

    // When the search text changes
    rerender({ query: 'two' })

    // Then the catalog for the new filter replaces it
    await waitFor(() => expect(result.current.catalog).toEqual(catalogOf('c2')))
    expect(listChallengeCatalog).toHaveBeenLastCalledWith({ query: 'two' })
  })

  it('does not load again when it re-renders with an equal filter', async () => {
    // Given a hook that loaded once
    vi.mocked(listChallengeCatalog).mockClear()
    vi.mocked(listChallengeCatalog).mockResolvedValue(catalogOf('c1'))
    const { result, rerender } = renderHook(() => useChallengeCatalog({ query: 'a', topic: 't' }))
    await waitFor(() => expect(result.current.loading).toBe(false))

    // When it re-renders with a new but equal filter object
    rerender()

    // Then nothing is fetched again
    expect(listChallengeCatalog).toHaveBeenCalledTimes(1)
  })

  it('ignores a response that no longer matches the current filter', async () => {
    // Given a slow first response and a fast second one
    let resolveFirst: (catalog: Catalog) => void = () => {}
    vi.mocked(listChallengeCatalog)
      .mockReturnValueOnce(new Promise<Catalog>((resolve) => (resolveFirst = resolve)))
      .mockResolvedValueOnce(catalogOf('new'))
    const { result, rerender } = renderHook(({ query }) => useChallengeCatalog({ query }), {
      initialProps: { query: 'old' },
    })
    rerender({ query: 'new' })
    await waitFor(() => expect(result.current.catalog).toEqual(catalogOf('new')))

    // When the first, stale response finally arrives
    await act(async () => resolveFirst(catalogOf('old')))

    // Then the catalog still belongs to the current filter
    expect(result.current.catalog).toEqual(catalogOf('new'))
  })

  it('ignores a stale failure too', async () => {
    // Given a first request that fails late, after the filter has changed
    let rejectFirst: (reason: Error) => void = () => {}
    vi.mocked(listChallengeCatalog)
      .mockReturnValueOnce(new Promise<Catalog>((_, reject) => (rejectFirst = reject)))
      .mockResolvedValueOnce(catalogOf('new'))
    const { result, rerender } = renderHook(({ query }) => useChallengeCatalog({ query }), {
      initialProps: { query: 'old' },
    })
    rerender({ query: 'new' })
    await waitFor(() => expect(result.current.catalog).toEqual(catalogOf('new')))

    // When the stale request rejects
    await act(async () => rejectFirst(new Error('late')))

    // Then no error is shown for the current filter
    expect(result.current.error).toBeNull()
  })

  it('reloads the same filter on demand, keeping the shown catalog meanwhile', async () => {
    // Given a loaded catalog
    vi.mocked(listChallengeCatalog)
      .mockResolvedValueOnce(catalogOf('c1'))
      .mockResolvedValueOnce(catalogOf('c1', 'c2'))
    const { result } = renderHook(() => useChallengeCatalog({}))
    await waitFor(() => expect(result.current.catalog).toEqual(catalogOf('c1')))

    // When reloading
    act(() => result.current.reload())

    // Then it is loading again without dropping the catalog, then shows the new one
    expect(result.current.loading).toBe(true)
    expect(result.current.catalog).toEqual(catalogOf('c1'))
    await waitFor(() => expect(result.current.catalog).toEqual(catalogOf('c1', 'c2')))
    expect(result.current.loading).toBe(false)
  })

  it('clears an earlier error when a reload succeeds', async () => {
    // Given a failed load
    vi.mocked(listChallengeCatalog)
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce(catalogOf('c1'))
    const { result } = renderHook(() => useChallengeCatalog({}))
    await waitFor(() => expect(result.current.error).not.toBeNull())

    // When retrying
    act(() => result.current.reload())

    // Then the error goes away
    expect(result.current.error).toBeNull()
    await waitFor(() => expect(result.current.catalog).toEqual(catalogOf('c1')))
    expect(result.current.error).toBeNull()
  })

  it('drops the catalog when a reload fails', async () => {
    // Given a loaded catalog
    vi.mocked(listChallengeCatalog)
      .mockResolvedValueOnce(catalogOf('c1'))
      .mockRejectedValueOnce(new Error('boom'))
    const { result } = renderHook(() => useChallengeCatalog({}))
    await waitFor(() => expect(result.current.catalog).toEqual(catalogOf('c1')))

    // When a reload fails
    act(() => result.current.reload())

    // Then no stale catalog is left beside the error
    await waitFor(() => expect(result.current.error).not.toBeNull())
    expect(result.current.catalog).toBeNull()
  })

  it('clears an earlier error when a new filter loads successfully', async () => {
    // Given a failed load
    vi.mocked(listChallengeCatalog)
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce(catalogOf('c1'))
    const { result, rerender } = renderHook(({ query }) => useChallengeCatalog({ query }), {
      initialProps: { query: 'a' },
    })
    await waitFor(() => expect(result.current.error).not.toBeNull())

    // When the filter changes and the next load succeeds
    rerender({ query: 'b' })

    // Then the error is gone
    await waitFor(() => expect(result.current.catalog).toEqual(catalogOf('c1')))
    expect(result.current.error).toBeNull()
  })

  it('reloads every time it is asked, not just the first', async () => {
    // Given a loaded catalog
    vi.mocked(listChallengeCatalog).mockClear()
    vi.mocked(listChallengeCatalog).mockResolvedValue(catalogOf('c1'))
    const { result } = renderHook(() => useChallengeCatalog({}))
    await waitFor(() => expect(result.current.loading).toBe(false))

    // When reloading twice in a row
    act(() => result.current.reload())
    await waitFor(() => expect(result.current.loading).toBe(false))
    act(() => result.current.reload())
    await waitFor(() => expect(result.current.loading).toBe(false))

    // Then each reload fetched again
    expect(listChallengeCatalog).toHaveBeenCalledTimes(3)
  })
})
