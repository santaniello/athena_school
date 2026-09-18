import { useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { IngestProgressDialog } from '@/components/ingest-progress-dialog'
import { PendingReconciliationSection } from '@/components/pending-reconciliation-section'
import { cn } from '@/lib/utils'
import { pickNotesFile } from '@/lib/ingest'
import KnowledgeExplorerScreen from '@/screens/KnowledgeExplorerScreen'

interface KnowledgeSectionProps {
  // null means "All topics".
  selectedTopic: string | null
  // True while the knowledge index is retrying — import/edit/approve/
  // deprecate/delete are rejected by a backend guard during a retry, so the
  // UI disables them too rather than letting a call fail confusingly.
  mutationsDisabled: boolean
  // The count of drafts pending review, owned by AppShell (see
  // specs/phases/phase-02-knowledge-engine/07-knowledge-review.md) — shown
  // on the Review tab. Not fetched locally, to avoid a second independent
  // count diverging from the sidebar badge.
  draftCount: number
  // Fired after an action inside the Explorer/Review screen changes the
  // draft count, so AppShell can refresh both badges without a reload.
  onKnowledgeChanged: () => void
  // Fired after an Explorer action that can change which topics exist
  // (delete, or an edit that changes the topic field), so AppShell can
  // refresh the sidebar's KnowledgeTopicTree without a reload.
  onTopicsChanged?: () => void
}

type Tab = 'explorer' | 'review'

const pickerErrorMessage = 'Failed to open the notes picker. Please try again.'

// Owns the Explorer/Review tab state and the "Import notes" toolbar action
// (a single file) — the main-pane counterpart to KnowledgeTopicTree in the
// sidebar. See the layout in
// specs/phases/phase-02-knowledge-engine/04-01-import-file.md.
function KnowledgeSection({
  selectedTopic,
  mutationsDisabled,
  draftCount,
  onKnowledgeChanged,
  onTopicsChanged,
}: KnowledgeSectionProps) {
  const [activeTab, setActiveTab] = useState<Tab>('explorer')
  const [importPath, setImportPath] = useState<string | null>(null)
  const [pickerError, setPickerError] = useState('')

  async function handleImportClick() {
    setPickerError('')
    try {
      const path = await pickNotesFile()
      if (path) setImportPath(path)
    } catch {
      setPickerError(pickerErrorMessage)
    }
  }

  function tabClassName(tab: Tab) {
    return cn(
      'flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1 text-sm text-muted-foreground hover:text-foreground',
      activeTab === tab && 'bg-secondary text-foreground',
    )
  }

  // Stryker disable StringLiteral: only read when importPath is null, i.e.
  // IngestProgressDialog's own `open` prop is false — its effect bails via
  // `if (!open) return` before path ever drives anything observable, so
  // this fallback exists purely to satisfy the required (non-optional) prop
  // type.
  const importDialogPath = importPath ?? ''
  // Stryker restore StringLiteral

  return (
    <div className="flex h-full w-full flex-col gap-4">
      <div className="flex items-center justify-between gap-2">
        <div role="tablist" aria-label="View mode" className="flex gap-1 rounded-lg border p-0.5">
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'explorer'}
            className={tabClassName('explorer')}
            onClick={() => setActiveTab('explorer')}
          >
            Explorer
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'review'}
            className={tabClassName('review')}
            onClick={() => setActiveTab('review')}
          >
            Review
            {draftCount > 0 && <Badge>{draftCount}</Badge>}
          </button>
        </div>

        <Button disabled={mutationsDisabled} onClick={() => void handleImportClick()}>
          Import notes
        </Button>
      </div>

      {pickerError && <p className="text-sm text-destructive">{pickerError}</p>}

      {activeTab === 'review' && (
        <PendingReconciliationSection onKnowledgeChanged={onKnowledgeChanged} />
      )}

      <div className="min-h-0 flex-1">
        <KnowledgeExplorerScreen
          selectedTopic={selectedTopic}
          mode={activeTab}
          mutationsDisabled={mutationsDisabled}
          onKnowledgeChanged={onKnowledgeChanged}
          onTopicsChanged={onTopicsChanged}
        />
      </div>

      <IngestProgressDialog
        open={importPath !== null}
        kind="file"
        path={importDialogPath}
        onClose={() => setImportPath(null)}
      />
    </div>
  )
}

export { KnowledgeSection }
