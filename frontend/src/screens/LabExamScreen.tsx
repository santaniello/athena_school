import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Loader2 } from 'lucide-react'
import { DiagramEditor, type DiagramEditorHandle } from '@/components/lab/diagram-editor'
import { LabConfirmDialog } from '@/components/lab/lab-confirm-dialog'
import { LabLayout } from '@/components/lab/lab-layout'
import { LabLeaveDialog } from '@/components/lab/lab-leave-dialog'
import type { ExamToStart } from '@/components/lab/start-exam-dialog'
import { Button } from '@/components/ui/button'
import { useExam } from '@/hooks/use-exam'
import { MAX_HINTS } from '@/lib/lab'

interface LabExamScreenProps {
  challenge: ExamToStart
  // The user tried to navigate away through the sidebar. The screen asks the
  // same question as Leave and answers through onExit or onLeaveCancelled.
  leaveRequested: boolean
  onLeaveCancelled: () => void
  // The exam is over; the shell shows the catalog, with the notice if given.
  onExit: (notice?: string) => void
  now?: () => number
}

function noop() {}

// The exam, end to end: prepare the editor, create the attempt when it is
// ready, run the clock, submit and leave. The backend owns the clock; this
// screen only reads the attempt's deadline. See
// specs/phases/phase-03-architecture-lab/10-diagram-workspace.md.
function LabExamScreen({
  challenge,
  leaveRequested,
  onLeaveCancelled,
  onExit,
  now,
}: LabExamScreenProps) {
  const exam = useExam(challenge.id, onExit)
  const editorRef = useRef<DiagramEditorHandle>(null)
  const [finishing, setFinishing] = useState(false)
  const [connectionLost, setConnectionLost] = useState(false)
  const { attempt, phase } = exam

  const hasAttempt = attempt !== null
  useEffect(() => {
    if (leaveRequested && !hasAttempt) onExit()
  }, [leaveRequested, hasAttempt, onExit])

  function getXml(): Promise<string> {
    return editorRef.current?.getXml() ?? Promise.resolve('')
  }

  function handleExpire() {
    setFinishing(false)
    void exam.expire(getXml)
  }

  function handleConfirmFinish() {
    setFinishing(false)
    void exam.submit(getXml)
  }

  const editor = (
    <DiagramEditor
      key={exam.editorKey}
      ref={editorRef}
      onReady={exam.editorReady}
      onChange={exam.editorChanged}
      onConnectionLost={() => setConnectionLost(true)}
      onConnectionRestored={() => setConnectionLost(false)}
    />
  )

  const leaveDialog = (
    <LabLeaveDialog
      open={leaveRequested && hasAttempt}
      onCancel={onLeaveCancelled}
      onConfirm={() => void exam.leave()}
    />
  )

  if (phase === 'submitted') {
    return (
      <>
        <CenteredMessage>
          <p className="text-base font-semibold text-foreground">Diagram submitted.</p>
          <Button type="button" onClick={() => void exam.leave()}>
            Back to catalog
          </Button>
        </CenteredMessage>
        {leaveDialog}
      </>
    )
  }

  if (phase === 'editorUnavailable') {
    return (
      <CenteredMessage>
        <p className="text-sm text-foreground">
          The editor needs an internet connection. Try again.
        </p>
        <div className="flex gap-2">
          <Button type="button" onClick={exam.retryEditor}>
            Try again
          </Button>
          <Button type="button" variant="outline" onClick={() => onExit()}>
            Back to catalog
          </Button>
        </div>
      </CenteredMessage>
    )
  }

  if (phase === 'startFailed') {
    return (
      <CenteredMessage>
        <p className="text-sm text-destructive">{exam.message}</p>
        <Button type="button" variant="outline" onClick={() => onExit()}>
          Back to catalog
        </Button>
      </CenteredMessage>
    )
  }

  if (!attempt) {
    // The editor loads off screen: it must be ready before the attempt, and
    // its clock, begin.
    return (
      <>
        <CenteredMessage>
          <Loader2 className="size-5 animate-spin text-muted-foreground" aria-hidden="true" />
          <p className="text-sm text-muted-foreground">Preparing the editor</p>
        </CenteredMessage>
        <div
          aria-hidden="true"
          className="pointer-events-none fixed top-0 -left-[9999px] h-[600px] w-[800px]"
        >
          {editor}
        </div>
        <LabConfirmDialog
          open={phase === 'conflict'}
          title="Exam in progress"
          description="You have an exam in progress. Starting a new one discards it."
          confirmLabel="OK"
          onCancel={exam.cancelReplace}
          onConfirm={exam.confirmReplace}
        />
      </>
    )
  }

  return (
    <>
      <LabLayout
        title={attempt.title}
        topic={attempt.topic}
        difficulty={challenge.difficulty}
        statement={attempt.statement}
        criteria={attempt.criteria}
        deadline={attempt.deadline}
        now={now}
        hints={[]}
        hintsLeft={MAX_HINTS}
        canFinish={exam.hasContent && phase === 'running'}
        board={
          <>
            {editor}
            {connectionLost && (
              <p
                role="status"
                className="absolute inset-x-0 top-0 z-10 bg-amber-500/90 px-3 py-1 text-center text-sm text-black"
              >
                Connection lost. Reconnecting...
              </p>
            )}
            {exam.message && (
              <p
                role="alert"
                className="absolute inset-x-0 bottom-0 z-10 bg-destructive px-3 py-1 text-center text-sm text-white"
              >
                {exam.message}
              </p>
            )}
            {phase === 'submitting' && (
              <div data-testid="board-lock" className="absolute inset-0 z-20 cursor-wait" />
            )}
          </>
        }
        onExpire={handleExpire}
        onAskHint={noop}
        onFinish={() => setFinishing(true)}
        onLeave={() => void exam.leave()}
      />
      <LabConfirmDialog
        open={finishing && phase === 'running'}
        title="Finish the exam?"
        description="Finish now? You cannot edit the diagram after this."
        confirmLabel="Finish"
        onCancel={() => setFinishing(false)}
        onConfirm={handleConfirmFinish}
      />
      {leaveDialog}
    </>
  )
}

function CenteredMessage({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-full w-full flex-col items-center justify-center gap-3 text-center">
      {children}
    </div>
  )
}

export default LabExamScreen
