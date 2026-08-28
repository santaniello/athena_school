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
  MATCH_EXACT,
  MATCH_SEMANTIC,
  saveAndApproveExtractedKnowledge,
  saveExtractedKnowledge,
  type DuplicateMatch,
  type KnowledgeItem,
} from '@/lib/knowledge'

// exactDuplicate returns item's exact match, if any — 10-duplicate-detection.md
// disables direct Create whenever one exists, whatever its status.
function exactDuplicate(item: KnowledgeItem): DuplicateMatch | undefined {
  return (item.duplicates ?? []).find((match) => match.matchType === MATCH_EXACT)
}

// semanticMatches returns item's semantic matches — warnings, never a block;
// the user opts in explicitly by checking the candidate ("Create separately").
function semanticMatches(item: KnowledgeItem): DuplicateMatch[] {
  return (item.duplicates ?? []).filter((match) => match.matchType === MATCH_SEMANTIC)
}

// defaultSelected is which candidate indices start checked: everything
// except an exact duplicate (Create disabled) or a semantic-only match
// (creating it separately is an explicit opt-in, not a default).
function defaultSelected(items: KnowledgeItem[]): Set<number> {
  return new Set(
    items
      .map((_, index) => index)
      .filter(
        (index) => !exactDuplicate(items[index]) && semanticMatches(items[index]).length === 0,
      ),
  )
}

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
  const [selected, setSelected] = useState<Set<number>>(() => defaultSelected(items))
  const [saved, setSaved] = useState<Set<number>>(new Set())
  const [isSaving, setIsSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  // Tracks which button triggered the in-flight/last-failed save, so retry
  // re-attempts the same mode and the other button stays at its default label.
  const [lastMode, setLastMode] = useState<SaveMode | null>(null)

  // The caller currently remounts this component fresh per batch (see
  // StudyChatScreen's `{showExtractionDialog && <KnowledgeExtractionDialog
  // .../>}`), so useState's initializers above already run once per batch in
  // practice. Adjusting state during render on a batchId change (React's own
  // documented pattern for this — see "Resetting all state when a prop
  // changes" — rather than a useEffect, which would cost an extra render and
  // trips react-hooks/set-state-in-effect) makes that a guarantee of the
  // component itself rather than an assumption about how any given caller
  // renders it: if a future caller instead kept one instance mounted and
  // only toggled `open` (the pattern `open` as a prop invites, and the one
  // TranscriptTruncationDialog already uses), a new batchId with a stale
  // `selected` set could carry an old selection into a new batch — e.g.
  // silently including a semantic-match candidate that was never explicitly
  // re-confirmed for this batch ("Create separately" is supposed to be a
  // per-batch, explicit choice). batchId, not items, is the actual "this is
  // a new batch" signal — items is a fresh array reference most renders
  // regardless.
  const [resetForBatchId, setResetForBatchId] = useState(batchId)
  if (batchId !== resetForBatchId) {
    setResetForBatchId(batchId)
    setSelected(defaultSelected(items))
    setSaved(new Set())
    setSaveError('')
    setLastMode(null)
  }

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
            {items.map((item, index) => {
              const exactMatch = exactDuplicate(item)
              const semantic = semanticMatches(item)
              return (
                <label key={item.id || index} className="flex gap-3 rounded-lg border p-3">
                  <input
                    type="checkbox"
                    aria-label={`Select ${item.concept}`}
                    checked={selected.has(index)}
                    disabled={Boolean(exactMatch) || saved.has(index) || isSaving}
                    onChange={() => toggle(index)}
                    className="mt-1 size-4 accent-primary"
                  />
                  <span className="min-w-0">
                    <span className="block font-medium text-foreground">{item.concept}</span>
                    <span className="mt-1 block text-sm text-muted-foreground">
                      {item.definition}
                    </span>
                    {exactMatch && (
                      <span className="mt-1 block text-xs text-destructive">
                        Already exists as &ldquo;{exactMatch.concept}&rdquo; ({exactMatch.status}).
                        Edit the existing item instead of creating a duplicate.
                      </span>
                    )}
                    {!exactMatch && semantic.length > 0 && (
                      <span className="mt-1 block text-xs text-amber-600 dark:text-amber-500">
                        Similar to &ldquo;{semantic[0].concept}&rdquo; ({semantic[0].status},{' '}
                        {Math.round(semantic[0].score * 100)}% match).
                        {!selected.has(index) && ' Check the box to create it separately anyway.'}
                      </span>
                    )}
                    {!exactMatch && item.semanticCheckUnavailable && (
                      <span className="mt-1 block text-xs text-muted-foreground">
                        Semantic duplicate check unavailable — only exact matches were checked.
                      </span>
                    )}
                    {saved.has(index) && (
                      <span className="mt-1 block text-xs text-primary">Saved</span>
                    )}
                  </span>
                </label>
              )
            })}
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
