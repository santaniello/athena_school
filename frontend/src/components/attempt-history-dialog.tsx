import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { AttemptResultView } from '@/components/lab/attempt-result-view'
import { formatSessionDate, listAttemptHistory, type AttemptHistoryItem } from '@/lib/exercise'

export interface HistoryChallenge {
  id: string
  title: string
}

interface AttemptHistoryDialogProps {
  // The challenge whose attempts are listed; null keeps the dialog closed.
  challenge: HistoryChallenge | null
  onClose: () => void
}

// The finished attempts of a challenge, newest first. Choosing one opens its
// result in the same dialog; nothing here changes or deletes an attempt. See
// specs/phases/phase-03-architecture-lab/13-evaluation-and-history-screen.md.
export function AttemptHistoryDialog({ challenge, onClose }: AttemptHistoryDialogProps) {
  return challenge ? <HistoryContent challenge={challenge} onClose={onClose} /> : null
}

function HistoryContent({
  challenge,
  onClose,
}: {
  challenge: HistoryChallenge
  onClose: () => void
}) {
  const [items, setItems] = useState<AttemptHistoryItem[] | null>(null)
  const [failed, setFailed] = useState(false)
  const [selected, setSelected] = useState<string | null>(null)

  useEffect(() => {
    let stale = false
    listAttemptHistory(challenge.id)
      .then((loaded) => !stale && setItems(loaded))
      .catch(() => !stale && setFailed(true))
    // Stryker disable BooleanLiteral,BlockStatement,ArrayDeclaration: the flag only keeps a late answer from updating an unmounted dialog, and the effect runs once per challenge.
    return () => {
      stale = true
    }
  }, [challenge.id])
  // Stryker restore BooleanLiteral,BlockStatement,ArrayDeclaration

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className={cn('max-h-[90vh] overflow-y-auto', selected && 'sm:max-w-5xl')}>
        {selected ? (
          <>
            <DialogHeader className="sr-only">
              <DialogTitle>Attempt result</DialogTitle>
              <DialogDescription>The result of a finished attempt.</DialogDescription>
            </DialogHeader>
            <AttemptResultView
              attemptId={selected}
              backLabel="Back to history"
              onBack={() => setSelected(null)}
            />
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>History</DialogTitle>
              <DialogDescription>{challenge.title}</DialogDescription>
            </DialogHeader>
            <HistoryList items={items} failed={failed} onSelect={setSelected} />
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

function HistoryList({
  items,
  failed,
  onSelect,
}: {
  items: AttemptHistoryItem[] | null
  failed: boolean
  onSelect: (attemptId: string) => void
}) {
  if (failed) {
    return (
      <p role="alert" className="text-sm text-destructive">
        Could not load the history.
      </p>
    )
  }
  if (!items) return <p className="text-sm text-muted-foreground">Loading the history...</p>
  return (
    <ul className="divide-y divide-border rounded-md border border-border">
      {items.map((item) => (
        <li key={item.attemptId} className="flex items-center justify-between gap-3 px-3 py-2">
          <div className="flex flex-col text-sm">
            <span className="font-medium text-foreground">
              {formatSessionDate(item.finishedAt)}
            </span>
            <span className="text-muted-foreground">
              Diagram {item.diagramScore} · Understanding {item.understandingScore}
            </span>
          </div>
          <div className="flex items-center gap-3">
            <Badge variant={item.outcome === 'PASSED' ? 'default' : 'muted'}>
              {item.outcome === 'PASSED' ? 'Passed' : 'Failed'}
            </Badge>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => onSelect(item.attemptId)}
            >
              Open
            </Button>
          </div>
        </li>
      ))}
    </ul>
  )
}
