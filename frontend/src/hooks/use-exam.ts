import { useCallback, useEffect, useRef, useState } from 'react'
import {
  ATTEMPT_IN_PROGRESS,
  SUBMISSION_TOO_LATE,
  answerDefenseQuestion,
  defenseErrorMessage,
  discardAttempt,
  evaluateAttempt,
  evaluateDefense,
  evaluationErrorMessage,
  exerciseErrorCode,
  exerciseErrorMessage,
  nextDefenseQuestion,
  startAttempt,
  submitAttempt,
  type Attempt,
  type DefenseQuestion,
} from '@/lib/exercise'
import { diagramHasContent } from '@/lib/lab'

// The editor has this long to report ready before the exam gives up on it.
const EDITOR_TIMEOUT_MS = 15000

const EMPTY_BOARD_NOTICE = 'Time is up and the board was empty. The exam was discarded.'

export type ExamPhase =
  // The editor is loading; no attempt exists and no clock runs.
  | 'preparing'
  | 'editorUnavailable'
  // The editor is ready and the attempt is being created.
  | 'starting'
  // Another exam is running and the user is asked whether to replace it.
  | 'conflict'
  | 'startFailed'
  | 'running'
  | 'submitting'
  // The diagram was submitted and the evaluation is running or failed; the
  // exam stays in this sitting. See
  // specs/phases/phase-03-architecture-lab/11-architecture-evaluator.md.
  | 'evaluating'
  | 'evaluationFailed'
  // The tutor's defense, which starts by itself once the diagram is evaluated:
  // a question is being written, failed, waits for its answer or has its
  // answer in flight. See
  // specs/phases/phase-03-architecture-lab/12-architecture-defense.md.
  | 'questioning'
  | 'questionFailed'
  | 'answering'
  | 'sendingAnswer'
  // The last answer is in and the final evaluation runs or failed.
  | 'judging'
  | 'judgingFailed'
  // Both evaluations exist: the outcome is shown.
  | 'finished'

type XmlSource = () => Promise<string>

// Owns the phase of one exam: preparing the editor, creating the attempt once
// the editor is ready, running, submitting and leaving. The attempt is created
// only after the editor reports ready, so the clock never starts on a board the
// user cannot use. See
// specs/phases/phase-03-architecture-lab/10-diagram-workspace.md.
export function useExam(
  challengeId: string,
  onExit: (notice?: string) => void,
  sessionId = '',
) {
  const [phase, setPhaseState] = useState<ExamPhase>('preparing')
  const [attempt, setAttempt] = useState<Attempt | null>(null)
  const [hasContent, setHasContent] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [question, setQuestion] = useState<DefenseQuestion | null>(null)
  // A new key replaces the editor, which is how the user tries again.
  const [editorKey, setEditorKey] = useState(0)
  // The board's own editor loads after the attempt exists; until it reports
  // ready the user cannot draw.
  const [boardReady, setBoardReady] = useState(false)
  const [boardStalled, setBoardStalled] = useState(false)
  // Mirrors state so handlers fired in a burst see the latest value.
  const phaseRef = useRef<ExamPhase>('preparing')
  const attemptRef = useRef<Attempt | null>(null)
  // The exam ends once: by leaving, by a late submission or by the expiry.
  const exitedRef = useRef(false)

  // Stryker disable ArrayDeclaration: a stable callback; the setter it wraps never changes.
  const setPhase = useCallback((next: ExamPhase) => {
    phaseRef.current = next
    setPhaseState(next)
  }, [])
  // Stryker restore ArrayDeclaration

  useEffect(() => {
    if (phase !== 'preparing') return
    const timer = setTimeout(() => setPhase('editorUnavailable'), EDITOR_TIMEOUT_MS)
    return () => clearTimeout(timer)
  }, [phase, editorKey, setPhase])

  // The board's editor is a fresh iframe, loaded while the clock already runs.
  // If it does not report ready in time it is replaced, and tried again.
  useEffect(() => {
    if (phase !== 'running' || boardReady) return
    const timer = setTimeout(() => {
      setBoardStalled(true)
      setEditorKey((key) => key + 1)
    }, EDITOR_TIMEOUT_MS)
    return () => clearTimeout(timer)
  }, [phase, boardReady, editorKey])

  function exit(notice?: string) {
    if (exitedRef.current) return
    exitedRef.current = true
    if (notice === undefined) onExit()
    else onExit(notice)
  }

  async function begin(discardInProgress: boolean) {
    setPhase('starting')
    try {
      const started = await startAttempt(challengeId, discardInProgress, sessionId)
      if (exitedRef.current) {
        // The user left while the attempt was being created.
        await discard(started)
        return
      }
      attemptRef.current = started
      setAttempt(started)
      setPhase('running')
    } catch (err) {
      if (exitedRef.current) return
      if (!discardInProgress && exerciseErrorCode(err) === ATTEMPT_IN_PROGRESS) {
        setPhase('conflict')
        return
      }
      setMessage(exerciseErrorMessage(err))
      setPhase('startFailed')
    }
  }

  function editorReady() {
    if (phaseRef.current === 'running') {
      setBoardReady(true)
      setBoardStalled(false)
      return
    }
    if (phaseRef.current !== 'preparing') return
    void begin(false)
  }

  function retryEditor() {
    setEditorKey((key) => key + 1)
    setPhase('preparing')
  }

  function confirmReplace() {
    void begin(true)
  }

  function cancelReplace() {
    exit()
  }

  function editorChanged(xml: string) {
    setHasContent(diagramHasContent(xml))
  }

  async function send(current: Attempt, xml: string) {
    try {
      await submitAttempt(current.id, xml)
    } catch (err) {
      if (exitedRef.current) return
      if (exerciseErrorCode(err) === SUBMISSION_TOO_LATE) {
        exit(exerciseErrorMessage(err))
        return
      }
      setMessage(exerciseErrorMessage(err))
      setPhase('running')
      return
    }
    if (exitedRef.current) return
    await evaluate(current)
  }

  // The evaluation is a stage of the same sitting: the user stays in the lab.
  // Once it succeeds the defense starts by itself; its result is not shown.
  async function evaluate(current: Attempt) {
    setMessage(null)
    setPhase('evaluating')
    try {
      await evaluateAttempt(current.id)
    } catch (err) {
      if (exitedRef.current) return
      setMessage(evaluationErrorMessage(err))
      setPhase('evaluationFailed')
      return
    }
    if (exitedRef.current) return
    await askNext(current)
  }

  // Manual only: there is no limit and no automatic retry.
  async function retryEvaluation() {
    const current = attemptRef.current
    if (!current || phaseRef.current !== 'evaluationFailed') return
    await evaluate(current)
  }

  async function askNext(current: Attempt) {
    setMessage(null)
    setPhase('questioning')
    try {
      const next = await nextDefenseQuestion(current.id)
      if (exitedRef.current) return
      setQuestion(next)
      setPhase('answering')
    } catch (err) {
      if (exitedRef.current) return
      setMessage(defenseErrorMessage(err))
      setPhase('questionFailed')
    }
  }

  async function judge(current: Attempt) {
    setMessage(null)
    setPhase('judging')
    try {
      await evaluateDefense(current.id)
      if (exitedRef.current) return
      setPhase('finished')
    } catch (err) {
      if (exitedRef.current) return
      setMessage(defenseErrorMessage(err))
      setPhase('judgingFailed')
    }
  }

  // The answer is stored when it is sent. A refused answer leaves the question
  // waiting, with the reason.
  async function sendAnswer(answer: string) {
    const current = attemptRef.current
    if (!current || phaseRef.current !== 'answering') return
    setPhase('sendingAnswer')
    setMessage(null)
    let complete: boolean
    try {
      complete = (await answerDefenseQuestion(current.id, answer)).complete
    } catch (err) {
      if (exitedRef.current) return
      setMessage(defenseErrorMessage(err))
      setPhase('answering')
      return
    }
    if (exitedRef.current) return
    if (complete) await judge(current)
    else await askNext(current)
  }

  // Manual only: the answers already given are kept and nothing is spent.
  async function retryDefense() {
    const current = attemptRef.current
    if (!current) return
    if (phaseRef.current === 'questionFailed') await askNext(current)
    else if (phaseRef.current === 'judgingFailed') await judge(current)
  }

  async function submit(getXml: XmlSource) {
    const current = attemptRef.current
    if (!current || phaseRef.current !== 'running') return
    setPhase('submitting')
    setMessage(null)
    await send(current, await getXml())
  }

  // Time ran out: a board with content is submitted as it is; an empty one is
  // discarded, since there is nothing to grade.
  async function expire(getXml: XmlSource) {
    const current = attemptRef.current
    if (!current || phaseRef.current !== 'running') return
    setPhase('submitting')
    setMessage(null)
    const xml = await getXml()
    if (diagramHasContent(xml)) {
      await send(current, xml)
      return
    }
    await discard(current)
    exit(EMPTY_BOARD_NOTICE)
  }

  async function leave() {
    if (exitedRef.current) return
    // Marked first: a submission or creation still in flight must not finish
    // the exam a second time or leave an attempt behind.
    exitedRef.current = true
    // A finished attempt is read-only and survives leaving the screen.
    if (attemptRef.current && phaseRef.current !== 'finished') await discard(attemptRef.current)
    onExit()
  }

  return {
    phase,
    attempt,
    hasContent,
    message,
    question,
    editorKey,
    boardStalled,
    editorReady,
    editorChanged,
    retryEditor,
    confirmReplace,
    cancelReplace,
    submit,
    expire,
    retryEvaluation,
    sendAnswer,
    retryDefense,
    leave,
  }
}

// A failed discard is left to the startup cleanup: the user is leaving anyway.
async function discard(attempt: Attempt) {
  const { id } = attempt
  try {
    await discardAttempt(id)
  } catch {
    // see above
  }
}
