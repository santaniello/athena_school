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
  onIngestDone,
  onIngestError,
  onIngestProgress,
  type IngestProgress,
  type IngestSummary,
} from '@/lib/ingest'

interface IngestProgressDialogProps {
  open: boolean
  // The study session that will own the imported knowledge, and the picked
  // path.
  sessionId?: string
  path?: string
  onClose: () => void
}

// No cancel affordance here by design: no operation in the app is
// cancellable today, and each file's replace is already an isolated
// transaction, so worst case the user waits out the run. The dialog only
// becomes dismissible once ingest:done or ingest:error has fired — see
// specs/phases/phase-02-knowledge-engine/04-01-import-file.md.
export function IngestProgressDialog({
  open,
  sessionId,
  path,
  onClose,
}: IngestProgressDialogProps) {
  const [ingestProgress, setIngestProgress] = useState<IngestProgress | null>(null)
  const [ingestSummary, setIngestSummary] = useState<IngestSummary | null>(null)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    if (!open) return
    // Guards the catch below against a stale rejection: if the dialog is
    // closed and reopened for another target before the old operation
    // settles, that old rejection must not set errorMessage on the new
    // operation's state.
    let active = true

    const unsubscribeProgress = onIngestProgress(setIngestProgress)
    const unsubscribeDone = onIngestDone(setIngestSummary)
    const unsubscribeError = onIngestError(setErrorMessage)

    // ingest:error is normally emitted with the details before this
    // rejects (see App.ImportFile), so the catch is usually just
    // preventing an unhandled promise rejection. But if the binding call
    // itself fails before ever reaching that emit — e.g. an IPC error —
    // no ingest:error ever fires; fall back to a generic message so the
    // dialog still becomes closable rather than staying stuck forever.
    void importFile(sessionId ?? '', path ?? '').catch(() => {
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
  }, [open, sessionId, path])

  const finished = ingestSummary !== null || errorMessage !== ''

  const progressLabel = ingestProgress
    ? `${ingestProgress.filesProcessed} of ${ingestProgress.filesTotal} files`
    : 'Starting...'
  const currentLabel = ingestProgress?.currentFile
  const percent =
    ingestProgress && ingestProgress.filesTotal > 0
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
          <DialogTitle>Importing notes</DialogTitle>
          <DialogDescription>
            {finished ? 'Import complete.' : 'Processing the selected file.'}
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
                <p className="text-xs text-muted-foreground">Imported, but not yet searchable:</p>
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

        {finished && (
          <DialogFooter>
            <Button onClick={onClose}>Close</Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
