import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { ExamToStart } from '@/lib/exercise'
import { MAX_HINTS } from '@/lib/lab'
import { getPassScore } from '@/lib/passScore'

export type { ExamToStart }

interface StartExamDialogProps {
  // The challenge whose rules are shown; null keeps the dialog closed.
  challenge: ExamToStart | null
  onCancel: () => void
  onConfirm: (challenge: ExamToStart) => void
}

// The rules of the exam, shown before it starts. It never shows the scenario,
// requirements or rubric. See
// specs/phases/phase-03-architecture-lab/10-diagram-workspace.md.
export function StartExamDialog({ challenge, onCancel, onConfirm }: StartExamDialogProps) {
  return challenge ? (
    <StartExamContent challenge={challenge} onCancel={onCancel} onConfirm={onConfirm} />
  ) : null
}

function StartExamContent({
  challenge,
  onCancel,
  onConfirm,
}: Omit<StartExamDialogProps, 'challenge'> & { challenge: ExamToStart }) {
  const [passScore, setPassScore] = useState<number | null>(null)

  // Stryker disable BooleanLiteral,BlockStatement,ArrayDeclaration: the guard only keeps a late answer from updating an unmounted dialog, and the effect runs once on mount.
  useEffect(() => {
    let stale = false
    getPassScore()
      .then((score) => !stale && setPassScore(score))
      .catch(() => {
        // The rules are still true without the number; the exam can start.
      })
    return () => {
      stale = true
    }
  }, [])
  // Stryker restore BooleanLiteral,BlockStatement,ArrayDeclaration

  const minutes = challenge.timeLimitMinutes
  return (
    <Dialog open onOpenChange={(open) => !open && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{challenge.title}</DialogTitle>
          <DialogDescription>Read the rules before you start.</DialogDescription>
        </DialogHeader>
        <ul className="list-disc space-y-1 pl-5 text-sm text-foreground">
          <li>
            {minutes === 0
              ? 'No time limit'
              : `Time limit: ${minutes} ${minutes === 1 ? 'minute' : 'minutes'}`}
          </li>
          <li>There is no pause and you cannot resume.</li>
          <li>Leaving discards the exam. It will not be graded.</li>
          <li>{`You can ask the tutor for up to ${MAX_HINTS} hints.`}</li>
          {passScore !== null && <li>{`Minimum passing score: ${passScore}`}</li>}
          <li>After you submit, the tutor will ask you questions about your choices.</li>
        </ul>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="button" onClick={() => onConfirm(challenge)}>
            Start exam
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
