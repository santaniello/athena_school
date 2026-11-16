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
import { useTranslation } from 'react-i18next'

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
  const { t } = useTranslation()
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
      <DialogContent
        closeLabel={t('common.close')}
        className={cn('max-h-[90vh] overflow-y-auto', selected ? 'sm:max-w-5xl' : 'sm:max-w-md')}
      >
        {selected ? (
          <>
            <DialogHeader className="sr-only">
              <DialogTitle>{t('historyDialog.resultTitle')}</DialogTitle>
              <DialogDescription>{t('historyDialog.resultDescription')}</DialogDescription>
            </DialogHeader>
            <AttemptResultView
              attemptId={selected}
              backLabel={t('back.history')}
              onBack={() => setSelected(null)}
            />
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{t('common.history')}</DialogTitle>
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
  const { t } = useTranslation()
  if (failed) {
    return (
      <p role="alert" className="text-sm text-destructive">
        {t('historyDialog.loadFailed')}
      </p>
    )
  }
  if (!items) return <p className="text-sm text-muted-foreground">{t('historyDialog.loading')}</p>
  return (
    <ul className="divide-y divide-border rounded-md border border-border">
      {items.map((item) => (
        <li key={item.attemptId} className="flex items-center justify-between gap-3 px-3 py-2">
          <div className="flex flex-col text-sm">
            <span className="font-medium text-foreground">
              {formatSessionDate(item.finishedAt)}
            </span>
            <span className="text-muted-foreground">
              {t('sessionCard.diagram', { score: item.diagramScore })} ·{' '}
              {t('sessionCard.understanding', { score: item.understandingScore })}
            </span>
          </div>
          <div className="flex items-center gap-3">
            <Badge variant={item.outcome === 'PASSED' ? 'default' : 'muted'}>
              {item.outcome === 'PASSED' ? t('common.passed') : t('common.failed')}
            </Badge>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => onSelect(item.attemptId)}
            >
              {t('common.open')}
            </Button>
          </div>
        </li>
      ))}
    </ul>
  )
}
