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

interface IngestProgressDialogProps {
  open: boolean
  kind: 'folder' | 'file'
  path: string
  onClose: () => void
}

const processingDescription: Record<'folder' | 'file', string> = {
  folder: 'Processing files in the selected folder.',
  file: 'Processing the selected file.',
}

// No cancel affordance here by design: no operation in the app is
// cancellable today, and each file's replace is already an isolated
// transaction, so worst case the user waits out the import. The dialog
// only becomes dismissible once ingest:done or ingest:error has fired —
// see specs/phases/phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md
// and specs/phases/phase-02-knowledge-engine/04-01-import-file.md.
export function IngestProgressDialog({ open, kind, path, onClose }: IngestProgressDialogProps) {
  const [progress, setProgress] = useState<IngestProgress | null>(null)
  const [summary, setSummary] = useState<IngestSummary | null>(null)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    if (!open) return
    // Guards the catch below against a stale rejection: if the dialog is
    // closed and reopened for another target before the old import call
    // settles, that old rejection must not set errorMessage on the new
    // import's state.
    let active = true

    const unsubscribeProgress = onIngestProgress(setProgress)
    const unsubscribeDone = onIngestDone(setSummary)
    const unsubscribeError = onIngestError(setErrorMessage)

    // ingest:error is normally emitted with the details before this
    // rejects (see App.ImportNotes/App.ImportFile), so the catch is
    // usually just preventing an unhandled promise rejection. But if the
    // binding call itself fails before ever reaching that emit — e.g. an
    // IPC error — no ingest:error ever fires; fall back to a generic
    // message so the dialog still becomes closable rather than staying
    // stuck forever.
    const startImport = kind === 'folder' ? importNotes : importFile
    void startImport(path).catch(() => {
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
      // so a fresh import never starts by briefly rendering stale state.
      setProgress(null)
      setSummary(null)
      setErrorMessage('')
    }
  }, [open, kind, path])

  const finished = summary !== null || errorMessage !== ''
  const percent =
    progress && progress.filesTotal > 0
      ? Math.round((progress.filesProcessed / progress.filesTotal) * 100)
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
            {finished ? 'Import complete.' : processingDescription[kind]}
          </DialogDescription>
        </DialogHeader>

        {!finished && (
          <div className="space-y-2">
            <Progress value={percent} />
            <p className="text-sm text-muted-foreground">
              {progress
                ? `${progress.filesProcessed} of ${progress.filesTotal} files`
                : 'Starting...'}
            </p>
            {progress?.currentFile && (
              <p className="truncate text-xs text-muted-foreground">{progress.currentFile}</p>
            )}
          </div>
        )}

        {errorMessage && (
          <Alert variant="destructive">
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        )}

        {summary && (
          <div className="space-y-3">
            <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
              <div>
                <dt className="text-muted-foreground">Scanned</dt>
                <dd className="font-medium text-foreground">{summary.filesScanned}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Imported</dt>
                <dd className="font-medium text-foreground">{summary.filesIngested}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Skipped</dt>
                <dd className="font-medium text-foreground">{summary.filesSkipped}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Failed</dt>
                <dd className="font-medium text-foreground">{summary.filesFailed}</dd>
              </div>
            </dl>
            {summary.failures.length > 0 && (
              <div className="thin-scroll max-h-40 space-y-1 overflow-y-auto rounded-lg border p-2">
                {summary.failures.map((failure) => (
                  <p key={failure.path} className="text-xs text-destructive">
                    <span className="font-medium">{failure.path}</span>: {failure.reason}
                  </p>
                ))}
              </div>
            )}
            {summary.indexWarnings.length > 0 && (
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">
                  Imported, but not yet searchable — retry the knowledge index from the Knowledge
                  section to fix this:
                </p>
                <div className="thin-scroll max-h-40 space-y-1 overflow-y-auto rounded-lg border p-2">
                  {summary.indexWarnings.map((warning) => (
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
