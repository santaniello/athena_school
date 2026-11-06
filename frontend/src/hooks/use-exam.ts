import { useCallback, useEffect, useRef, useState } from 'react'
import {
  ATTEMPT_IN_PROGRESS,
  SUBMISSION_TOO_LATE,
  discardAttempt,
  exerciseErrorCode,
  exerciseErrorMessage,
  startAttempt,
  submitAttempt,
  type Attempt,
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
  | 'submitted'

type XmlSource = () => Promise<string>

// Owns the phase of one exam: preparing the editor, creating the attempt once
// the editor is ready, running, submitting and leaving. The attempt is created
// only after the editor reports ready, so the clock never starts on a board the
// user cannot use. See
// specs/phases/phase-03-architecture-lab/10-diagram-workspace.md.
export function useExam(challengeId: string, onExit: (notice?: string) => void) {
  const [phase, setPhaseState] = useState<ExamPhase>('preparing')
  const [attempt, setAttempt] = useState<Attempt | null>(null)
  const [hasContent, setHasContent] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  // A new key replaces the editor, which is how the user tries again.
  const [editorKey, setEditorKey] = useState(0)
  // Mirrors state so handlers fired in a burst see the latest value.
  const phaseRef = useRef<ExamPhase>('preparing')
  const attemptRef = useRef<Attempt | null>(null)

  const setPhase = useCallback((next: ExamPhase) => {
    phaseRef.current = next
    setPhaseState(next)
  }, [])

  useEffect(() => {
    if (phase !== 'preparing') return
    const timer = setTimeout(() => setPhase('editorUnavailable'), EDITOR_TIMEOUT_MS)
    return () => clearTimeout(timer)
  }, [phase, editorKey, setPhase])

  async function begin(discardInProgress: boolean) {
    setPhase('starting')
    try {
      const started = await startAttempt(challengeId, discardInProgress)
      attemptRef.current = started
      setAttempt(started)
      setPhase('running')
    } catch (err) {
      if (!discardInProgress && exerciseErrorCode(err) === ATTEMPT_IN_PROGRESS) {
        setPhase('conflict')
        return
      }
      setMessage(exerciseErrorMessage(err))
      setPhase('startFailed')
    }
  }

  function editorReady() {
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
    onExit()
  }

  function editorChanged(xml: string) {
    setHasContent(diagramHasContent(xml))
  }

  async function send(current: Attempt, xml: string) {
    try {
      await submitAttempt(current.id, xml)
      setPhase('submitted')
    } catch (err) {
      if (exerciseErrorCode(err) === SUBMISSION_TOO_LATE) {
        onExit(exerciseErrorMessage(err))
        return
      }
      setMessage(exerciseErrorMessage(err))
      setPhase('running')
    }
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
    onExit(EMPTY_BOARD_NOTICE)
  }

  async function leave() {
    if (attemptRef.current) await discard(attemptRef.current)
    onExit()
  }

  return {
    phase,
    attempt,
    hasContent,
    message,
    editorKey,
    editorReady,
    editorChanged,
    retryEditor,
    confirmReplace,
    cancelReplace,
    submit,
    expire,
    leave,
  }
}

// A failed discard is left to the startup cleanup: the user is leaving anyway.
async function discard(attempt: Attempt) {
  try {
    await discardAttempt(attempt.id)
  } catch {
    // see above
  }
}
