import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { discardAttempt, startAttempt, submitAttempt, type Attempt } from '@/lib/exercise'
import { useExam } from './use-exam'

vi.mock('@/lib/exercise', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/exercise')>()
  return { ...original, startAttempt: vi.fn(), submitAttempt: vi.fn(), discardAttempt: vi.fn() }
})

const attempt: Attempt = {
  id: 'a1',
  challengeId: 'c1',
  title: 'Cache',
  topic: 'caching',
  deadline: '2026-11-06T13:00:00Z',
  statement: {
    scenario: 's',
    functionalRequirements: [],
    nonFunctionalRequirements: [],
    constraints: [],
    objectives: [],
  },
  criteria: [],
}

const EMPTY_XML =
  '<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/></root></mxGraphModel>'
const FULL_XML = `<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/><mxCell id="2" vertex="1" parent="1"/></root></mxGraphModel>`

function coded(code: string): Error {
  return new Error(JSON.stringify({ code, message: code }))
}

function setup() {
  const onExit = vi.fn()
  const hook = renderHook(() => useExam('c1', onExit))
  return { onExit, ...hook }
}

// Gets to a running exam.
async function running() {
  vi.mocked(startAttempt).mockResolvedValue(attempt)
  const view = setup()
  await act(async () => view.result.current.editorReady())
  return view
}

describe('useExam', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(submitAttempt).mockResolvedValue()
    vi.mocked(discardAttempt).mockResolvedValue()
  })
  afterEach(() => vi.useRealTimers())

  it('starts preparing the editor and creates no attempt', () => {
    const { result } = setup()

    expect(result.current.phase).toBe('preparing')
    expect(result.current.attempt).toBeNull()
    expect(startAttempt).not.toHaveBeenCalled()
  })

  it('creates the attempt only once the editor is ready, then runs', async () => {
    vi.mocked(startAttempt).mockResolvedValue(attempt)
    const { result } = setup()

    await act(async () => result.current.editorReady())

    expect(startAttempt).toHaveBeenCalledExactlyOnceWith('c1', false)
    expect(result.current.phase).toBe('running')
    expect(result.current.attempt).toBe(attempt)
  })

  it('creates a single attempt when the editor reports ready twice', async () => {
    vi.mocked(startAttempt).mockResolvedValue(attempt)
    const { result } = setup()

    await act(async () => {
      result.current.editorReady()
      result.current.editorReady()
    })

    expect(startAttempt).toHaveBeenCalledOnce()
  })

  it('gives up on the editor when it is not ready in 15 seconds, with no attempt', () => {
    vi.useFakeTimers()
    const { result } = setup()

    act(() => vi.advanceTimersByTime(14999))
    expect(result.current.phase).toBe('preparing')
    act(() => vi.advanceTimersByTime(1))

    expect(result.current.phase).toBe('editorUnavailable')
    expect(startAttempt).not.toHaveBeenCalled()
  })

  it('does not give up while the attempt is being created', async () => {
    vi.useFakeTimers()
    vi.mocked(startAttempt).mockReturnValue(new Promise(() => {}))
    const { result } = setup()
    act(() => result.current.editorReady())

    act(() => vi.advanceTimersByTime(20000))

    expect(result.current.phase).toBe('starting')
  })

  it('tries the editor again with a fresh one', () => {
    vi.useFakeTimers()
    const { result } = setup()
    act(() => vi.advanceTimersByTime(15000))
    const key = result.current.editorKey

    act(() => result.current.retryEditor())

    expect(result.current.phase).toBe('preparing')
    expect(result.current.editorKey).toBe(key + 1)
    act(() => vi.advanceTimersByTime(14999))
    expect(result.current.phase).toBe('preparing')
  })

  it('asks before replacing an exam that is already running', async () => {
    vi.mocked(startAttempt).mockRejectedValue(coded('attempt_in_progress'))
    const { result } = setup()

    await act(async () => result.current.editorReady())

    expect(result.current.phase).toBe('conflict')
  })

  it('replaces the running exam when the user confirms', async () => {
    vi.mocked(startAttempt).mockRejectedValueOnce(coded('attempt_in_progress'))
    vi.mocked(startAttempt).mockResolvedValueOnce(attempt)
    const { result } = setup()
    await act(async () => result.current.editorReady())

    await act(async () => result.current.confirmReplace())

    expect(startAttempt).toHaveBeenLastCalledWith('c1', true)
    expect(result.current.phase).toBe('running')
  })

  it('leaves the other exam alone when the user cancels', async () => {
    vi.mocked(startAttempt).mockRejectedValue(coded('attempt_in_progress'))
    const { result, onExit } = setup()
    await act(async () => result.current.editorReady())

    act(() => result.current.cancelReplace())

    expect(onExit).toHaveBeenCalledExactlyOnceWith()
    expect(startAttempt).toHaveBeenCalledOnce()
  })

  it('shows why the exam could not start when the attempt cannot be created', async () => {
    vi.mocked(startAttempt).mockRejectedValue(coded('challenge_has_no_rubric'))
    const { result } = setup()

    await act(async () => result.current.editorReady())

    expect(result.current.phase).toBe('startFailed')
    expect(result.current.message).toBe('Something went wrong. Please try again.')
  })

  it('tells the user when the replacement could not start', async () => {
    vi.mocked(startAttempt).mockRejectedValueOnce(coded('attempt_in_progress'))
    vi.mocked(startAttempt).mockRejectedValueOnce(coded('internal'))
    const { result } = setup()
    await act(async () => result.current.editorReady())

    await act(async () => result.current.confirmReplace())

    expect(result.current.phase).toBe('startFailed')
  })

  it('enables Finish only once the board has content', async () => {
    const { result } = await running()
    expect(result.current.hasContent).toBe(false)

    act(() => result.current.editorChanged(FULL_XML))
    expect(result.current.hasContent).toBe(true)

    act(() => result.current.editorChanged(EMPTY_XML))
    expect(result.current.hasContent).toBe(false)
  })

  it('submits the XML the editor returns and shows the submitted state', async () => {
    const { result } = await running()

    await act(async () => result.current.submit(async () => FULL_XML))

    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith('a1', FULL_XML)
    expect(result.current.phase).toBe('submitted')
  })

  it('is submitting while the submission is in flight', async () => {
    const { result } = await running()
    vi.mocked(submitAttempt).mockReturnValue(new Promise(() => {}))

    act(() => {
      void result.current.submit(async () => FULL_XML)
    })

    await vi.waitFor(() => expect(result.current.phase).toBe('submitting'))
  })

  it('submits only once even if asked twice', async () => {
    const { result } = await running()

    await act(async () => {
      void result.current.submit(async () => FULL_XML)
      void result.current.submit(async () => FULL_XML)
    })

    expect(submitAttempt).toHaveBeenCalledOnce()
  })

  it('keeps the exam open and says why when the diagram is too large', async () => {
    const { result, onExit } = await running()
    vi.mocked(submitAttempt).mockRejectedValue(coded('submission_too_large'))

    await act(async () => result.current.submit(async () => FULL_XML))

    expect(result.current.phase).toBe('running')
    expect(result.current.message).toBe('The diagram is too large to submit. Remove some elements.')
    expect(onExit).not.toHaveBeenCalled()
  })

  it('clears the message on the next submission', async () => {
    const { result } = await running()
    vi.mocked(submitAttempt).mockRejectedValueOnce(coded('submission_too_large'))
    await act(async () => result.current.submit(async () => FULL_XML))

    await act(async () => result.current.submit(async () => FULL_XML))

    expect(result.current.message).toBeNull()
    expect(result.current.phase).toBe('submitted')
  })

  it('returns to the catalog with a notice when the submission came too late', async () => {
    const { result, onExit } = await running()
    vi.mocked(submitAttempt).mockRejectedValue(coded('submission_too_late'))

    await act(async () => result.current.submit(async () => FULL_XML))

    expect(onExit).toHaveBeenCalledExactlyOnceWith(
      'The time was over and the exam could not be submitted.',
    )
  })

  it('submits at expiry when the board has content, without asking', async () => {
    const { result } = await running()

    await act(async () => result.current.expire(async () => FULL_XML))

    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith('a1', FULL_XML)
    expect(result.current.phase).toBe('submitted')
  })

  it('discards at expiry when the board is empty, and says so', async () => {
    const { result, onExit } = await running()

    await act(async () => result.current.expire(async () => EMPTY_XML))

    expect(submitAttempt).not.toHaveBeenCalled()
    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(onExit).toHaveBeenCalledExactlyOnceWith(
      'Time is up and the board was empty. The exam was discarded.',
    )
  })

  it('still returns to the catalog when the empty board could not be discarded', async () => {
    const { result, onExit } = await running()
    vi.mocked(discardAttempt).mockRejectedValue(new Error('boom'))

    await act(async () => result.current.expire(async () => EMPTY_XML))

    expect(onExit).toHaveBeenCalledOnce()
  })

  it('ignores the expiry once the diagram was submitted', async () => {
    const { result } = await running()
    await act(async () => result.current.submit(async () => FULL_XML))
    vi.mocked(submitAttempt).mockClear()

    await act(async () => result.current.expire(async () => FULL_XML))

    expect(submitAttempt).not.toHaveBeenCalled()
  })

  it('discards the attempt on leave and returns to the catalog', async () => {
    const { result, onExit } = await running()

    await act(async () => result.current.leave())

    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(onExit).toHaveBeenCalledExactlyOnceWith()
  })

  it('discards a submitted attempt on leave too', async () => {
    const { result, onExit } = await running()
    await act(async () => result.current.submit(async () => FULL_XML))

    await act(async () => result.current.leave())

    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(onExit).toHaveBeenCalledOnce()
  })

  it('leaves without discarding when no attempt exists', async () => {
    const { result, onExit } = setup()

    await act(async () => result.current.leave())

    expect(discardAttempt).not.toHaveBeenCalled()
    expect(onExit).toHaveBeenCalledOnce()
  })

  it('returns to the catalog even when the discard fails', async () => {
    const { result, onExit } = await running()
    vi.mocked(discardAttempt).mockRejectedValue(new Error('boom'))

    await act(async () => result.current.leave())

    expect(onExit).toHaveBeenCalledOnce()
  })
})
