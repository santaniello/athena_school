import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { listHints, requestHint } from '@/lib/exercise'
import { useHints } from './use-hints'

vi.mock('@/lib/exercise', () => ({ listHints: vi.fn(), requestHint: vi.fn() }))

const first = { id: 'h1', text: 'What if the cache is down?' }
const second = { id: 'h2', text: 'Who owns the write path?' }

const getXml = vi.fn<() => Promise<string>>()

function setup(attemptId: string | null = 'a1') {
  return renderHook(() => useHints(attemptId, getXml))
}

describe('useHints', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getXml.mockResolvedValue('<mxGraphModel/>')
    vi.mocked(listHints).mockResolvedValue([])
  })

  it('starts with no hints and all of them left', () => {
    const { result } = setup()

    expect(result.current.hints).toEqual([])
    expect(result.current.hintsLeft).toBe(3)
    expect(result.current.asking).toBe(false)
    expect(result.current.failed).toBe(false)
  })

  it('loads the hints already given for the attempt', async () => {
    vi.mocked(listHints).mockResolvedValue([first])

    const { result } = setup()

    await waitFor(() => expect(result.current.hints).toEqual([first]))
    expect(listHints).toHaveBeenCalledExactlyOnceWith('a1')
    expect(result.current.hintsLeft).toBe(2)
  })

  it('loads nothing before the attempt exists', () => {
    setup(null)

    expect(listHints).not.toHaveBeenCalled()
  })

  it('keeps the counter when loading the earlier hints fails', async () => {
    vi.mocked(listHints).mockRejectedValue(new Error('boom'))

    const { result } = setup()

    await waitFor(() => expect(listHints).toHaveBeenCalled())
    expect(result.current.hints).toEqual([])
    expect(result.current.failed).toBe(false)
  })

  it('asks with the current board and appends the hint, counting 3, 2, 1, none', async () => {
    vi.mocked(requestHint)
      .mockResolvedValueOnce(first)
      .mockResolvedValueOnce(second)
      .mockResolvedValueOnce({ id: 'h3', text: 'third' })
    const { result } = setup()

    await act(() => result.current.askHint())
    expect(requestHint).toHaveBeenLastCalledWith('a1', '<mxGraphModel/>')
    expect(result.current.hintsLeft).toBe(2)
    await act(() => result.current.askHint())
    expect(result.current.hintsLeft).toBe(1)
    await act(() => result.current.askHint())

    expect(result.current.hintsLeft).toBe(0)
    expect(result.current.hints.map((hint) => hint.id)).toEqual(['h1', 'h2', 'h3'])
  })

  it('shows progress while a request runs and refuses a second one', async () => {
    let finish: (hint: typeof first) => void = () => {}
    vi.mocked(requestHint).mockReturnValue(new Promise((resolve) => (finish = resolve)))
    const { result } = setup()

    act(() => void result.current.askHint())
    await waitFor(() => expect(result.current.asking).toBe(true))
    await act(() => result.current.askHint())
    expect(requestHint).toHaveBeenCalledOnce()
    await act(async () => finish(first))

    expect(result.current.asking).toBe(false)
    expect(result.current.hints).toEqual([first])
  })

  it('flags a failure, leaves the counter alone, and clears the flag on the next try', async () => {
    vi.mocked(requestHint).mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce(first)
    const { result } = setup()

    await act(() => result.current.askHint())
    expect(result.current.failed).toBe(true)
    expect(result.current.asking).toBe(false)
    expect(result.current.hintsLeft).toBe(3)
    await act(() => result.current.askHint())

    expect(result.current.failed).toBe(false)
    expect(result.current.hints).toEqual([first])
  })

  it('does not ask when there is no attempt yet', async () => {
    const { result } = setup(null)

    await act(() => result.current.askHint())

    expect(requestHint).not.toHaveBeenCalled()
  })

  it('does not ask once none are left', async () => {
    vi.mocked(listHints).mockResolvedValue([first, second, { id: 'h3', text: 'third' }])
    const { result } = setup()
    await waitFor(() => expect(result.current.hintsLeft).toBe(0))

    await act(() => result.current.askHint())

    expect(requestHint).not.toHaveBeenCalled()
  })
})
