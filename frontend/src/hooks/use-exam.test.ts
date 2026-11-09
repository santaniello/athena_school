import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import {
  answerDefenseQuestion,
  discardAttempt,
  evaluateAttempt,
  evaluateDefense,
  nextDefenseQuestion,
  startAttempt,
  submitAttempt,
  type Attempt,
  type DefenseQuestion,
  type Evaluation,
  type Outcome,
} from '@/lib/exercise'
import { useExam } from './use-exam'

vi.mock('@/lib/exercise', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/exercise')>()
  return {
    ...original,
    startAttempt: vi.fn(),
    submitAttempt: vi.fn(),
    discardAttempt: vi.fn(),
    evaluateAttempt: vi.fn(),
    nextDefenseQuestion: vi.fn(),
    answerDefenseQuestion: vi.fn(),
    evaluateDefense: vi.fn(),
  }
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

const evaluation: Evaluation = {
  score: 72,
  summary: 'Solid.',
  strengths: [],
  gaps: [],
  recommendations: [],
  results: [],
}

const firstQuestion: DefenseQuestion = { position: 1, text: 'Why a cache?', total: 3 }
const secondQuestion: DefenseQuestion = { position: 2, text: 'What if it fails?', total: 3 }

const outcome: Outcome = {
  result: 'PASSED',
  passScore: 80,
  diagramScore: 85,
  understandingScore: 90,
  reasons: [],
}

const EMPTY_XML =
  '<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/></root></mxGraphModel>'
const FULL_XML = `<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/><mxCell id="2" vertex="1" parent="1"/></root></mxGraphModel>`

function coded(code: string): Error {
  return new Error(JSON.stringify({ code, message: code }))
}

function setup(sessionId?: string) {
  const onExit = vi.fn()
  const hook = renderHook(() => useExam('c1', onExit, sessionId))
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
    vi.mocked(evaluateAttempt).mockResolvedValue(evaluation)
    vi.mocked(nextDefenseQuestion).mockResolvedValue(firstQuestion)
    vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 3, complete: false })
    vi.mocked(evaluateDefense).mockResolvedValue(outcome)
  })
  afterEach(() => vi.useRealTimers())

  it('starts preparing the editor and creates no attempt', () => {
    const { result } = setup()

    expect(result.current.phase).toBe('preparing')
    expect(result.current.attempt).toBeNull()
    expect(startAttempt).not.toHaveBeenCalled()
  })

  it('links the attempt to the study session it was started from', async () => {
    vi.mocked(startAttempt).mockResolvedValue(attempt)
    const { result } = setup('s1')

    await act(async () => result.current.editorReady())

    expect(startAttempt).toHaveBeenCalledExactlyOnceWith('c1', false, 's1')
  })

  it('creates the attempt only once the editor is ready, then runs', async () => {
    vi.mocked(startAttempt).mockResolvedValue(attempt)
    const { result } = setup()

    await act(async () => result.current.editorReady())

    expect(startAttempt).toHaveBeenCalledExactlyOnceWith('c1', false, '')
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

    expect(startAttempt).toHaveBeenLastCalledWith('c1', true, '')
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

  it('submits the XML the editor returns, then evaluates the attempt', async () => {
    const { result } = await running()

    await act(async () => result.current.submit(async () => FULL_XML))

    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith('a1', FULL_XML)
    expect(evaluateAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(result.current.phase).toBe('answering')
  })

  it('is evaluating while the evaluation is in flight', async () => {
    const { result } = await running()
    vi.mocked(evaluateAttempt).mockReturnValue(new Promise(() => {}))

    act(() => {
      void result.current.submit(async () => FULL_XML)
    })

    await vi.waitFor(() => expect(result.current.phase).toBe('evaluating'))
  })

  it('does not evaluate a diagram that could not be submitted', async () => {
    const { result } = await running()
    vi.mocked(submitAttempt).mockRejectedValue(coded('submission_too_large'))

    await act(async () => result.current.submit(async () => FULL_XML))

    expect(evaluateAttempt).not.toHaveBeenCalled()
  })

  it('shows why the evaluation failed and keeps the submission', async () => {
    const { result, onExit } = await running()
    vi.mocked(evaluateAttempt).mockRejectedValue(coded('empty_diagram'))

    await act(async () => result.current.submit(async () => FULL_XML))

    expect(result.current.phase).toBe('evaluationFailed')
    expect(result.current.message).toBe('The diagram is empty.')
    expect(submitAttempt).toHaveBeenCalledOnce()
    expect(discardAttempt).not.toHaveBeenCalled()
    expect(onExit).not.toHaveBeenCalled()
  })

  it('evaluates again on request, without submitting again, and can succeed', async () => {
    const { result } = await running()
    vi.mocked(evaluateAttempt).mockRejectedValueOnce(coded('internal'))
    await act(async () => result.current.submit(async () => FULL_XML))

    await act(async () => result.current.retryEvaluation())

    expect(evaluateAttempt).toHaveBeenCalledTimes(2)
    expect(submitAttempt).toHaveBeenCalledOnce()
    expect(result.current.phase).toBe('answering')
    expect(result.current.message).toBeNull()
  })

  it('is evaluating again while a retry is in flight', async () => {
    const { result } = await running()
    vi.mocked(evaluateAttempt).mockRejectedValueOnce(coded('internal'))
    await act(async () => result.current.submit(async () => FULL_XML))
    vi.mocked(evaluateAttempt).mockReturnValue(new Promise(() => {}))

    act(() => {
      void result.current.retryEvaluation()
    })

    await vi.waitFor(() => expect(result.current.phase).toBe('evaluating'))
    expect(result.current.message).toBeNull()
  })

  it('ignores a retry unless the evaluation failed', async () => {
    const { result } = await running()
    await act(async () => result.current.retryEvaluation())
    expect(evaluateAttempt).not.toHaveBeenCalled()

    await act(async () => result.current.submit(async () => FULL_XML))
    await act(async () => result.current.retryEvaluation())

    expect(evaluateAttempt).toHaveBeenCalledOnce()
  })

  it('discards the attempt and drops the result when the user leaves while it is evaluated', async () => {
    const { result, onExit } = await running()
    let resolve: (value: Evaluation) => void = () => {}
    vi.mocked(evaluateAttempt).mockReturnValue(new Promise<Evaluation>((r) => (resolve = r)))
    act(() => {
      void result.current.submit(async () => FULL_XML)
    })
    await vi.waitFor(() => expect(result.current.phase).toBe('evaluating'))

    await act(async () => result.current.leave())
    await act(async () => resolve(evaluation))

    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(onExit).toHaveBeenCalledOnce()
    expect(result.current.phase).toBe('evaluating')
    expect(nextDefenseQuestion).not.toHaveBeenCalled()
  })

  it('drops a failure that arrives after the user left', async () => {
    const { result } = await running()
    let reject: (reason: Error) => void = () => {}
    vi.mocked(evaluateAttempt).mockReturnValue(new Promise<Evaluation>((_, r) => (reject = r)))
    act(() => {
      void result.current.submit(async () => FULL_XML)
    })
    await vi.waitFor(() => expect(result.current.phase).toBe('evaluating'))
    await act(async () => result.current.leave())

    await act(async () => reject(coded('internal')))

    expect(result.current.phase).toBe('evaluating')
    expect(result.current.message).toBeNull()
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
    expect(result.current.phase).toBe('answering')
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
    expect(result.current.phase).toBe('answering')
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

  it('is submitting while the expiry reads and sends the diagram', async () => {
    const { result } = await running()
    vi.mocked(submitAttempt).mockReturnValue(new Promise(() => {}))

    act(() => {
      void result.current.expire(async () => FULL_XML)
    })

    await vi.waitFor(() => expect(result.current.phase).toBe('submitting'))
  })

  it('clears an earlier message when the expiry takes over', async () => {
    const { result } = await running()
    vi.mocked(submitAttempt).mockRejectedValueOnce(coded('submission_too_large'))
    await act(async () => result.current.submit(async () => FULL_XML))
    expect(result.current.message).not.toBeNull()
    vi.mocked(submitAttempt).mockReturnValue(new Promise(() => {}))

    act(() => {
      void result.current.expire(async () => FULL_XML)
    })

    await vi.waitFor(() => expect(result.current.phase).toBe('submitting'))
    expect(result.current.message).toBeNull()
  })

  it('clears an earlier message while the next submission is in flight', async () => {
    const { result } = await running()
    vi.mocked(submitAttempt).mockRejectedValueOnce(coded('submission_too_large'))
    await act(async () => result.current.submit(async () => FULL_XML))
    vi.mocked(submitAttempt).mockReturnValue(new Promise(() => {}))

    act(() => {
      void result.current.submit(async () => FULL_XML)
    })

    await vi.waitFor(() => expect(result.current.phase).toBe('submitting'))
    expect(result.current.message).toBeNull()
  })

  it('ignores a submission error that arrives after the user left', async () => {
    const { result } = await running()
    let reject: (reason: Error) => void = () => {}
    vi.mocked(submitAttempt).mockReturnValue(new Promise<void>((_, r) => (reject = r)))
    act(() => {
      void result.current.submit(async () => FULL_XML)
    })
    await vi.waitFor(() => expect(submitAttempt).toHaveBeenCalled())
    await act(async () => result.current.leave())

    await act(async () => reject(coded('submission_too_large')))

    expect(result.current.phase).toBe('submitting')
    expect(result.current.message).toBeNull()
  })

  it('exits once when the user leaves while a late submission is in flight', async () => {
    const { result, onExit } = await running()
    let reject: (reason: Error) => void = () => {}
    vi.mocked(submitAttempt).mockReturnValue(new Promise<void>((_, r) => (reject = r)))
    let sending: Promise<void> = Promise.resolve()
    act(() => {
      sending = result.current.submit(async () => FULL_XML)
    })
    await vi.waitFor(() => expect(submitAttempt).toHaveBeenCalled())

    await act(async () => result.current.leave())
    await act(async () => {
      reject(coded('submission_too_late'))
      await sending
    })

    expect(onExit).toHaveBeenCalledExactlyOnceWith()
    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
  })

  it('does not start the evaluation when the user left before the submission finished', async () => {
    const { result } = await running()
    let resolve: () => void = () => {}
    vi.mocked(submitAttempt).mockReturnValue(new Promise<void>((r) => (resolve = r)))
    let sending: Promise<void> = Promise.resolve()
    act(() => {
      sending = result.current.submit(async () => FULL_XML)
    })
    await vi.waitFor(() => expect(submitAttempt).toHaveBeenCalled())
    await act(async () => result.current.leave())

    await act(async () => {
      resolve()
      await sending
    })

    expect(result.current.phase).toBe('submitting')
    expect(evaluateAttempt).not.toHaveBeenCalled()
  })

  it('leaves only once when asked twice', async () => {
    const { result, onExit } = await running()

    await act(async () => {
      await Promise.all([result.current.leave(), result.current.leave()])
    })

    expect(onExit).toHaveBeenCalledOnce()
    expect(discardAttempt).toHaveBeenCalledOnce()
  })

  it('discards an attempt that is created after the user left', async () => {
    let resolve: (value: Attempt) => void = () => {}
    vi.mocked(startAttempt).mockReturnValue(new Promise<Attempt>((r) => (resolve = r)))
    const { result, onExit } = setup()
    act(() => result.current.editorReady())

    await act(async () => result.current.leave())
    await act(async () => resolve(attempt))

    expect(onExit).toHaveBeenCalledOnce()
    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(result.current.phase).toBe('starting')
  })

  it('ignores a start failure that arrives after the user left', async () => {
    let reject: (reason: Error) => void = () => {}
    vi.mocked(startAttempt).mockReturnValue(new Promise<Attempt>((_, r) => (reject = r)))
    const { result } = setup()
    act(() => result.current.editorReady())
    await act(async () => result.current.leave())

    await act(async () => reject(coded('attempt_in_progress')))

    expect(result.current.phase).toBe('starting')
  })

  it('replaces the board editor when it is not ready 15 seconds after the attempt began', async () => {
    vi.useFakeTimers()
    const { result } = await running()
    const key = result.current.editorKey

    act(() => vi.advanceTimersByTime(15000))

    expect(result.current.editorKey).toBe(key + 1)
    expect(result.current.boardStalled).toBe(true)
  })

  it('tries the board editor again every 15 seconds until it is ready', async () => {
    vi.useFakeTimers()
    const { result } = await running()

    act(() => vi.advanceTimersByTime(15000))
    act(() => vi.advanceTimersByTime(15000))

    expect(result.current.editorKey).toBe(2)
  })

  it('does not replace the board editor once it is ready, and clears the stall', async () => {
    vi.useFakeTimers()
    const { result } = await running()
    act(() => vi.advanceTimersByTime(15000))

    act(() => result.current.editorReady())
    act(() => vi.advanceTimersByTime(60000))

    expect(result.current.boardStalled).toBe(false)
    expect(result.current.editorKey).toBe(1)
  })
  describe('the defense', () => {
    // Gets to the first question.
    async function answering() {
      const view = await running()
      await act(async () => view.result.current.submit(async () => FULL_XML))
      return view
    }

    it('asks the first question by itself once the diagram is evaluated', async () => {
      const { result } = await answering()

      expect(nextDefenseQuestion).toHaveBeenCalledExactlyOnceWith('a1')
      expect(result.current.phase).toBe('answering')
      expect(result.current.question).toBe(firstQuestion)
    })

    it('is questioning while the next question is being written', async () => {
      const { result } = await running()
      vi.mocked(nextDefenseQuestion).mockReturnValue(new Promise(() => {}))

      act(() => {
        void result.current.submit(async () => FULL_XML)
      })

      await vi.waitFor(() => expect(result.current.phase).toBe('questioning'))
      expect(result.current.question).toBeNull()
    })

    it('shows why a question could not be written and keeps the exam open', async () => {
      const { result, onExit } = await running()
      vi.mocked(nextDefenseQuestion).mockRejectedValue(coded('internal'))

      await act(async () => result.current.submit(async () => FULL_XML))

      expect(result.current.phase).toBe('questionFailed')
      expect(result.current.message).toBe('The tutor could not continue the defense.')
      expect(discardAttempt).not.toHaveBeenCalled()
      expect(onExit).not.toHaveBeenCalled()
    })

    it('asks again on request after a failed question, and can succeed', async () => {
      const { result } = await running()
      vi.mocked(nextDefenseQuestion).mockRejectedValueOnce(coded('internal'))
      await act(async () => result.current.submit(async () => FULL_XML))

      await act(async () => result.current.retryDefense())

      expect(nextDefenseQuestion).toHaveBeenCalledTimes(2)
      expect(evaluateAttempt).toHaveBeenCalledOnce()
      expect(result.current.phase).toBe('answering')
      expect(result.current.message).toBeNull()
      expect(result.current.question).toBe(firstQuestion)
    })

    it('is questioning again while a retry is in flight', async () => {
      const { result } = await running()
      vi.mocked(nextDefenseQuestion).mockRejectedValueOnce(coded('internal'))
      await act(async () => result.current.submit(async () => FULL_XML))
      vi.mocked(nextDefenseQuestion).mockReturnValue(new Promise(() => {}))

      act(() => {
        void result.current.retryDefense()
      })

      await vi.waitFor(() => expect(result.current.phase).toBe('questioning'))
      expect(result.current.message).toBeNull()
    })

    it('ignores a defense retry unless something failed', async () => {
      const { result } = await answering()

      await act(async () => result.current.retryDefense())

      expect(nextDefenseQuestion).toHaveBeenCalledOnce()
      expect(evaluateDefense).not.toHaveBeenCalled()
    })

    it('sends the answer and asks the next question while the defense goes on', async () => {
      const { result } = await answering()
      vi.mocked(nextDefenseQuestion).mockResolvedValue(secondQuestion)

      await act(async () => result.current.sendAnswer('To cut reads.'))

      expect(answerDefenseQuestion).toHaveBeenCalledExactlyOnceWith('a1', 'To cut reads.')
      expect(nextDefenseQuestion).toHaveBeenCalledTimes(2)
      expect(evaluateDefense).not.toHaveBeenCalled()
      expect(result.current.phase).toBe('answering')
      expect(result.current.question).toBe(secondQuestion)
    })

    it('is sending while the answer is in flight, and clears an earlier message', async () => {
      const { result } = await answering()
      vi.mocked(answerDefenseQuestion).mockRejectedValueOnce(coded('internal'))
      await act(async () => result.current.sendAnswer('first try'))
      expect(result.current.message).not.toBeNull()
      vi.mocked(answerDefenseQuestion).mockReturnValue(new Promise(() => {}))

      act(() => {
        void result.current.sendAnswer('second try')
      })

      await vi.waitFor(() => expect(result.current.phase).toBe('sendingAnswer'))
      expect(result.current.message).toBeNull()
    })

    it('keeps the question and says why when the answer is refused', async () => {
      const { result } = await answering()
      vi.mocked(answerDefenseQuestion).mockRejectedValue(coded('invalid_answer'))

      await act(async () => result.current.sendAnswer('x'))

      expect(result.current.phase).toBe('answering')
      expect(result.current.question).toBe(firstQuestion)
      expect(result.current.message).toBe('The answer must be between 1 and 4,000 characters.')
      expect(nextDefenseQuestion).toHaveBeenCalledOnce()
    })

    it('evaluates the defense after the last answer and shows the outcome', async () => {
      const { result } = await answering()
      vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 3, complete: true })

      await act(async () => result.current.sendAnswer('The last one.'))

      expect(nextDefenseQuestion).toHaveBeenCalledOnce()
      expect(evaluateDefense).toHaveBeenCalledExactlyOnceWith('a1')
      expect(result.current.phase).toBe('finished')
    })

    it('is judging while the final evaluation is in flight', async () => {
      const { result } = await answering()
      vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 3, complete: true })
      vi.mocked(evaluateDefense).mockReturnValue(new Promise(() => {}))

      act(() => {
        void result.current.sendAnswer('The last one.')
      })

      await vi.waitFor(() => expect(result.current.phase).toBe('judging'))
    })

    it('keeps the answers and shows why the final evaluation failed', async () => {
      const { result, onExit } = await answering()
      vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 3, complete: true })
      vi.mocked(evaluateDefense).mockRejectedValue(coded('invalid_evaluation'))

      await act(async () => result.current.sendAnswer('The last one.'))

      expect(result.current.phase).toBe('judgingFailed')
      expect(result.current.message).toBe('The tutor returned an invalid evaluation.')
      expect(discardAttempt).not.toHaveBeenCalled()
      expect(onExit).not.toHaveBeenCalled()
    })

    it('evaluates again on request without sending the answer again, and can succeed', async () => {
      const { result } = await answering()
      vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 3, complete: true })
      vi.mocked(evaluateDefense).mockRejectedValueOnce(coded('internal'))
      await act(async () => result.current.sendAnswer('The last one.'))

      await act(async () => result.current.retryDefense())

      expect(evaluateDefense).toHaveBeenCalledTimes(2)
      expect(answerDefenseQuestion).toHaveBeenCalledOnce()
      expect(result.current.phase).toBe('finished')
      expect(result.current.message).toBeNull()
    })

    it('ignores an answer unless a question is waiting', async () => {
      const { result } = await running()
      await act(async () => result.current.sendAnswer('too early'))
      expect(answerDefenseQuestion).not.toHaveBeenCalled()

      vi.mocked(answerDefenseQuestion).mockReturnValue(new Promise(() => {}))
      await act(async () => result.current.submit(async () => FULL_XML))
      act(() => {
        void result.current.sendAnswer('once')
      })
      await act(async () => result.current.sendAnswer('twice'))

      expect(answerDefenseQuestion).toHaveBeenCalledExactlyOnceWith('a1', 'once')
    })

    it('discards the attempt when the user leaves during the defense, dropping a late question', async () => {
      const { result, onExit } = await running()
      let resolve: (value: DefenseQuestion) => void = () => {}
      vi.mocked(nextDefenseQuestion).mockReturnValue(
        new Promise<DefenseQuestion>((r) => (resolve = r)),
      )
      act(() => {
        void result.current.submit(async () => FULL_XML)
      })
      await vi.waitFor(() => expect(result.current.phase).toBe('questioning'))

      await act(async () => result.current.leave())
      await act(async () => resolve(firstQuestion))

      expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
      expect(onExit).toHaveBeenCalledOnce()
      expect(result.current.phase).toBe('questioning')
      expect(result.current.question).toBeNull()
    })

    it('drops a question failure that arrives after the user left', async () => {
      const { result } = await running()
      let reject: (reason: Error) => void = () => {}
      vi.mocked(nextDefenseQuestion).mockReturnValue(
        new Promise<DefenseQuestion>((_, r) => (reject = r)),
      )
      act(() => {
        void result.current.submit(async () => FULL_XML)
      })
      await vi.waitFor(() => expect(result.current.phase).toBe('questioning'))
      await act(async () => result.current.leave())

      await act(async () => reject(coded('internal')))

      expect(result.current.phase).toBe('questioning')
      expect(result.current.message).toBeNull()
    })

    it('drops an answer result that arrives after the user left', async () => {
      const { result } = await answering()
      let resolve: (value: { total: number; complete: boolean }) => void = () => {}
      vi.mocked(answerDefenseQuestion).mockReturnValue(new Promise((r) => (resolve = r)))
      act(() => {
        void result.current.sendAnswer('late')
      })
      await vi.waitFor(() => expect(result.current.phase).toBe('sendingAnswer'))
      await act(async () => result.current.leave())

      await act(async () => resolve({ total: 3, complete: true }))

      expect(evaluateDefense).not.toHaveBeenCalled()
      expect(nextDefenseQuestion).toHaveBeenCalledOnce()
    })

    it('drops an answer failure that arrives after the user left', async () => {
      const { result } = await answering()
      let reject: (reason: Error) => void = () => {}
      vi.mocked(answerDefenseQuestion).mockReturnValue(new Promise((_, r) => (reject = r)))
      act(() => {
        void result.current.sendAnswer('late')
      })
      await vi.waitFor(() => expect(result.current.phase).toBe('sendingAnswer'))
      await act(async () => result.current.leave())

      await act(async () => reject(coded('internal')))

      expect(result.current.phase).toBe('sendingAnswer')
      expect(result.current.message).toBeNull()
    })

    it('drops the outcome that arrives after the user left, and the failure too', async () => {
      const { result } = await answering()
      vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 3, complete: true })
      let resolve: (value: Outcome) => void = () => {}
      vi.mocked(evaluateDefense).mockReturnValue(new Promise<Outcome>((r) => (resolve = r)))
      act(() => {
        void result.current.sendAnswer('The last one.')
      })
      await vi.waitFor(() => expect(result.current.phase).toBe('judging'))
      await act(async () => result.current.leave())

      await act(async () => resolve(outcome))

      expect(result.current.phase).toBe('judging')
    })

    it('drops a final evaluation failure that arrives after the user left', async () => {
      const { result } = await answering()
      vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 3, complete: true })
      let reject: (reason: Error) => void = () => {}
      vi.mocked(evaluateDefense).mockReturnValue(new Promise<Outcome>((_, r) => (reject = r)))
      act(() => {
        void result.current.sendAnswer('The last one.')
      })
      await vi.waitFor(() => expect(result.current.phase).toBe('judging'))
      await act(async () => result.current.leave())

      await act(async () => reject(coded('internal')))

      expect(result.current.phase).toBe('judging')
      expect(result.current.message).toBeNull()
    })

    it('leaves a finished attempt alone: it is read-only and survives', async () => {
      const { result, onExit } = await answering()
      vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 3, complete: true })
      await act(async () => result.current.sendAnswer('The last one.'))

      await act(async () => result.current.leave())

      expect(discardAttempt).not.toHaveBeenCalled()
      expect(onExit).toHaveBeenCalledOnce()
    })
  })
})
