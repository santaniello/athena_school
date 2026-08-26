import { useMemo, useState } from 'react'
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
import {
  discardExtraction,
  saveAndApproveExtractedKnowledge,
  saveExtractedKnowledge,
  type KnowledgeItem,
} from '@/lib/knowledge'

interface KnowledgeExtractionDialogProps {
  open: boolean
  batchId: string
  items: KnowledgeItem[]
  onClose: () => void
  // Fired after a successful "Save as drafts" that actually persisted at
  // least one candidate — the only save mode that adds to the review queue.
  // Lets AppShell refresh the sidebar/Review-tab badge without a reload. See
  // specs/phases/phase-02-knowledge-engine/07-knowledge-review.md.
  onKnowledgeChanged?: () => void
}

// The three options from specs/Athena.md §12 ("Knowledge Promotion"):
// "Save as knowledge" (save directly as approved), "Save as drafts" (save
// as draft, the default review flow), "Dismiss" (discard).
type SaveMode = 'draft' | 'approve'

export function KnowledgeExtractionDialog({
  open,
  batchId,
  items,
  onClose,
  onKnowledgeChanged,
}: KnowledgeExtractionDialogProps) {
  const [selected, setSelected] = useState<Set<number>>(
    () => new Set(items.map((_, index) => index)),
  )
  const [saved, setSaved] = useState<Set<number>>(new Set())
  const [isSaving, setIsSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  // Tracks which button triggered the in-flight/last-failed save, so retry
  // re-attempts the same mode and the other button stays at its default label.
  const [lastMode, setLastMode] = useState<SaveMode | null>(null)

  const pendingIndices = useMemo(
    () => [...selected].filter((index) => !saved.has(index)).sort((a, b) => a - b),
    [saved, selected],
  )

  function toggle(index: number) {
    setSelected((previous) => {
      const next = new Set(previous)
      if (next.has(index)) next.delete(index)
      else next.add(index)
      return next
    })
  }

  // handleClose covers every true dialog-close path — the explicit Dismiss
  // button, clicking outside, Escape, and closing right after a fully
  // successful save — and discards whatever candidates in this batch were
  // never attempted (deselected, or not yet reached). It is never called
  // from handleSave's error branch, so a partial-save failure always leaves
  // the remaining receipts retryable.
  async function handleClose() {
    if (batchId) {
      try {
        await discardExtraction(batchId)
      } catch {
        // Best-effort: an unreachable backend during close should not block
        // the dialog from dismissing. The receipt simply outlives this
        // session's batch and stays orphaned until the app restarts.
      }
    }
    onClose()
  }

  async function handleSave(mode: SaveMode) {
    if (pendingIndices.length === 0) return
    setIsSaving(true)
    setSaveError('')
    setLastMode(mode)
    try {
      const save = mode === 'approve' ? saveAndApproveExtractedKnowledge : saveExtractedKnowledge
      const result = await save(
        batchId,
        pendingIndices.map((index) => items[index]),
      )
      const persistedIndices = result.savedIndices
        .map((index) => pendingIndices[index])
        .filter((index): index is number => index !== undefined)
      if (persistedIndices.length > 0) {
        setSaved((previous) => new Set([...previous, ...persistedIndices]))
        if (mode === 'draft') onKnowledgeChanged?.()
      }
      if (result.error) {
        setSaveError(result.error)
        return
      }
      void handleClose()
    } catch (caught) {
      const fallbackMessage =
        mode === 'approve' ? 'Failed to save as knowledge.' : 'Failed to save drafts.'
      const error = caught instanceof Error ? caught : new Error(fallbackMessage)
      setSaveError(error.message)
    } finally {
      setIsSaving(false)
    }
  }

  function saveButtonLabel(mode: SaveMode, idleLabel: string) {
    if (lastMode !== mode) return idleLabel
    if (isSaving) return 'Saving...'
    if (saveError) return 'Try again'
    return idleLabel
  }

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && void handleClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>New knowledge found</DialogTitle>
          <DialogDescription>
            Review the concepts and choose which ones to save as drafts.
          </DialogDescription>
        </DialogHeader>

        {items.length === 0 ? (
          <p className="py-4 text-sm text-muted-foreground">No new knowledge found</p>
        ) : (
          <div className="thin-scroll max-h-[50vh] space-y-3 overflow-y-auto">
            {items.map((item, index) => (
              <label key={item.id || index} className="flex gap-3 rounded-lg border p-3">
                <input
                  type="checkbox"
                  aria-label={`Select ${item.concept}`}
                  checked={selected.has(index)}
                  disabled={saved.has(index) || isSaving}
                  onChange={() => toggle(index)}
                  className="mt-1 size-4 accent-primary"
                />
                <span className="min-w-0">
                  <span className="block font-medium text-foreground">{item.concept}</span>
                  <span className="mt-1 block text-sm text-muted-foreground">
                    {item.definition}
                  </span>
                  {saved.has(index) && (
                    <span className="mt-1 block text-xs text-primary">Saved</span>
                  )}
                </span>
              </label>
            ))}
          </div>
        )}

        {saveError && (
          <Alert variant="destructive">
            <AlertDescription>{saveError}</AlertDescription>
          </Alert>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => void handleClose()} disabled={isSaving}>
            Dismiss
          </Button>
          {items.length > 0 && (
            <>
              <Button
                variant="outline"
                onClick={() => void handleSave('draft')}
                disabled={pendingIndices.length === 0 || isSaving}
              >
                {saveButtonLabel('draft', 'Save as drafts')}
              </Button>
              <Button
                onClick={() => void handleSave('approve')}
                disabled={pendingIndices.length === 0 || isSaving}
              >
                {saveButtonLabel('approve', 'Save as knowledge')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
