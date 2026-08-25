import { useEffect, useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Progress } from '@/components/ui/progress'
import {
  importFile,
  importNotes,
  onIngestDone,
  onIngestError,
  onIngestProgress,
  type IngestProgress,
  type IngestSummary,
} from '@/lib/ingest'
import {
  onReindexDone,
  onReindexError,
  onReindexProgress,
  reindexKnowledgeItems,
  type ReindexProgress,
  type ReindexSummary,
} from '@/lib/knowledge'

type DialogKind = 'folder' | 'file' | 'reindex'

interface IngestProgressDialogProps {
  open: boolean
  kind: DialogKind
  // Required for 'folder'/'file' (the picked path); unused for 'reindex',
  // which has no path of its own — it processes every unindexed item.
  path?: string
  onClose: () => void
}

const dialogTitle: Record<DialogKind, string> = {
  folder: 'Importing notes',
  file: 'Importing notes',
  reindex: 'Indexing knowledge',
}

const processingDescription: Record<DialogKind, string> = {
  folder: 'Processing files in the selected folder.',
  file: 'Processing the selected file.',
  reindex: 'Indexing knowledge items for search.',
}

const completeDescription: Record<DialogKind, string> = {
  folder: 'Import complete.',
  file: 'Import complete.',
  reindex: 'Indexing complete.',
}

// No cancel affordance here by design: no operation in the app is
// cancellable today, and each file's replace (or each item's re-index) is
// already an isolated transaction, so worst case the user waits out the
// run. The dialog only becomes dismissible once ingest:done or
// ingest:error has fired — see
// specs/phases/phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md,
// specs/phases/phase-02-knowledge-engine/04-01-import-file.md, and
// specs/phases/phase-02-knowledge-engine/08-knowledge-item-indexing.md.
//
// 'reindex' reuses the exact same ingest:progress/ingest:done/ingest:error
// events 'folder'/'file' already stream — the UI only ever has one such
// operation active at a time — but with an items-shaped payload instead of
// a files-shaped one, so its progress/summary state is tracked separately.
export function IngestProgressDialog({ open, kind, path, onClose }: IngestProgressDialogProps) {
  const [ingestProgress, setIngestProgress] = useState<IngestProgress | null>(null)
  const [ingestSummary, setIngestSummary] = useState<IngestSummary | null>(null)
  const [reindexProgress, setReindexProgress] = useState<ReindexProgress | null>(null)
  const [reindexSummary, setReindexSummary] = useState<ReindexSummary | null>(null)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    if (!open) return
    // Guards the catch below against a stale rejection: if the dialog is
    // closed and reopened for another target before the old operation
    // settles, that old rejection must not set errorMessage on the new
    // operation's state.
    let active = true

    if (kind === 'reindex') {
      const unsubscribeProgress = onReindexProgress(setReindexProgress)
      const unsubscribeDone = onReindexDone(setReindexSummary)
      const unsubscribeError = onReindexError(setErrorMessage)

      void reindexKnowledgeItems().catch(() => {
        if (!active) return
        setErrorMessage(
          (current) => current || 'Failed to index knowledge items. Please try again.',
        )
      })

      return () => {
        active = false
        unsubscribeProgress()
        unsubscribeDone()
        unsubscribeError()
        setReindexProgress(null)
        setReindexSummary(null)
        setErrorMessage('')
      }
    }

    const unsubscribeProgress = onIngestProgress(setIngestProgress)
    const unsubscribeDone = onIngestDone(setIngestSummary)
    const unsubscribeError = onIngestError(setErrorMessage)

    // ingest:error is normally emitted with the details before this
    // rejects (see App.ImportNotes/App.ImportFile), so the catch is
    // usually just preventing an unhandled promise rejection. But if the
    // binding call itself fails before ever reaching that emit — e.g. an
    // IPC error — no ingest:error ever fires; fall back to a generic
    // message so the dialog still becomes closable rather than staying
    // stuck forever.
    const startImport = kind === 'folder' ? importNotes : importFile
    void startImport(path ?? '').catch(() => {
      if (!active) return
      setErrorMessage((current) => current || 'Failed to import notes. Please try again.')
    })

    return () => {
      active = false
      unsubscribeProgress()
      unsubscribeDone()
      unsubscribeError()
      // Reset here (on close, or right before the next open re-runs this
      // effect for a new target) rather than at the top of the effect body,
      // so a fresh run never starts by briefly rendering stale state.
      setIngestProgress(null)
      setIngestSummary(null)
      setErrorMessage('')
    }
  }, [open, kind, path])

  const finished = ingestSummary !== null || reindexSummary !== null || errorMessage !== ''

  const progressLabel =
    kind === 'reindex'
      ? reindexProgress
        ? `${reindexProgress.itemsProcessed} of ${reindexProgress.itemsTotal} items`
        : 'Starting...'
      : ingestProgress
        ? `${ingestProgress.filesProcessed} of ${ingestProgress.filesTotal} files`
        : 'Starting...'
  const currentLabel =
    kind === 'reindex' ? reindexProgress?.currentTopic : ingestProgress?.currentFile
  const percent =
    kind === 'reindex'
      ? reindexProgress && reindexProgress.itemsTotal > 0
        ? Math.round((reindexProgress.itemsProcessed / reindexProgress.itemsTotal) * 100)
        : 0
      : ingestProgress && ingestProgress.filesTotal > 0
        ? Math.round((ingestProgress.filesProcessed / ingestProgress.filesTotal) * 100)
        : 0

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && finished && onClose()}>
      <DialogContent
        className="sm:max-w-lg"
        showCloseButton={finished}
        onEscapeKeyDown={(event) => !finished && event.preventDefault()}
        onInteractOutside={(event) => !finished && event.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>{dialogTitle[kind]}</DialogTitle>
          <DialogDescription>
            {finished ? completeDescription[kind] : processingDescription[kind]}
          </DialogDescription>
        </DialogHeader>

        {!finished && (
          <div className="space-y-2">
            <Progress value={percent} />
            <p className="text-sm text-muted-foreground">{progressLabel}</p>
            {currentLabel && (
              <p className="truncate text-xs text-muted-foreground">{currentLabel}</p>
            )}
          </div>
        )}

        {errorMessage && (
          <Alert variant="destructive">
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        )}

        {ingestSummary && (
          <div className="space-y-3">
            <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
              <div>
                <dt className="text-muted-foreground">Scanned</dt>
                <dd className="font-medium text-foreground">{ingestSummary.filesScanned}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Imported</dt>
                <dd className="font-medium text-foreground">{ingestSummary.filesIngested}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Skipped</dt>
                <dd className="font-medium text-foreground">{ingestSummary.filesSkipped}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Failed</dt>
                <dd className="font-medium text-foreground">{ingestSummary.filesFailed}</dd>
              </div>
            </dl>
            {ingestSummary.failures.length > 0 && (
              <div className="thin-scroll max-h-40 space-y-1 overflow-y-auto rounded-lg border p-2">
                {ingestSummary.failures.map((failure) => (
                  <p key={failure.path} className="text-xs text-destructive">
                    <span className="font-medium">{failure.path}</span>: {failure.reason}
                  </p>
                ))}
              </div>
            )}
            {ingestSummary.indexWarnings.length > 0 && (
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">
                  Imported, but not yet searchable — retry the knowledge index from the Knowledge
                  section to fix this:
                </p>
                <div className="thin-scroll max-h-40 space-y-1 overflow-y-auto rounded-lg border p-2">
                  {ingestSummary.indexWarnings.map((warning) => (
                    <p key={warning.path} className="text-xs text-muted-foreground">
                      <span className="font-medium">{warning.path}</span>
                    </p>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {reindexSummary && (
          <div className="space-y-3">
            <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
              <div>
                <dt className="text-muted-foreground">Processed</dt>
                <dd className="font-medium text-foreground">{reindexSummary.itemsProcessed}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Indexed</dt>
                <dd className="font-medium text-foreground">{reindexSummary.itemsIndexed}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Failed</dt>
                <dd className="font-medium text-foreground">{reindexSummary.itemsFailed}</dd>
              </div>
            </dl>
            {reindexSummary.failures.length > 0 && (
              <div className="thin-scroll max-h-40 space-y-1 overflow-y-auto rounded-lg border p-2">
                {reindexSummary.failures.map((failure) => (
                  <p key={failure.itemId} className="text-xs text-destructive">
                    <span className="font-medium">{failure.topic}</span>: {failure.reason}
                  </p>
                ))}
              </div>
            )}
          </div>
        )}

        {finished && (
          <DialogFooter>
            <Button onClick={onClose}>Close</Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
