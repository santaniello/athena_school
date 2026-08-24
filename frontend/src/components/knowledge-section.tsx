import { useState } from 'react'
import { ChevronDownIcon } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { IngestProgressDialog } from '@/components/ingest-progress-dialog'
import { cn } from '@/lib/utils'
import { pickNotesFile, pickNotesFolder } from '@/lib/ingest'
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
}

type Tab = 'explorer' | 'review'

// The single active import target, whichever picker produced it — the
// progress dialog drives both flows off this one piece of state.
interface ImportTarget {
  kind: 'folder' | 'file'
  path: string
}

const pickerErrorMessage = 'Failed to open the notes picker. Please try again.'

// Owns the Explorer/Review tab state and the "Import notes" toolbar
// dropdown (folder or single file) — the main-pane counterpart to
// KnowledgeTopicTree in the sidebar. See the layout in
// specs/phases/phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md
// and specs/phases/phase-02-knowledge-engine/04-01-import-file.md.
function KnowledgeSection({
  selectedTopic,
  mutationsDisabled,
  draftCount,
  onKnowledgeChanged,
}: KnowledgeSectionProps) {
  const [activeTab, setActiveTab] = useState<Tab>('explorer')
  const [importTarget, setImportTarget] = useState<ImportTarget | null>(null)
  const [pickerError, setPickerError] = useState('')

  async function handleImportFolderClick() {
    setPickerError('')
    try {
      const path = await pickNotesFolder()
      if (path) setImportTarget({ kind: 'folder', path })
    } catch {
      setPickerError(pickerErrorMessage)
    }
  }

  async function handleImportFileClick() {
    setPickerError('')
    try {
      const path = await pickNotesFile()
      if (path) setImportTarget({ kind: 'file', path })
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

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button disabled={mutationsDisabled}>
              Import notes
              <ChevronDownIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => void handleImportFolderClick()}>
              Import folder...
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => void handleImportFileClick()}>
              Import file...
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {pickerError && <p className="text-sm text-destructive">{pickerError}</p>}

      <div className="min-h-0 flex-1">
        <KnowledgeExplorerScreen
          selectedTopic={selectedTopic}
          mode={activeTab}
          mutationsDisabled={mutationsDisabled}
          onKnowledgeChanged={onKnowledgeChanged}
        />
      </div>

      <IngestProgressDialog
        open={importTarget !== null}
        kind={importTarget?.kind ?? 'folder'}
        path={importTarget?.path ?? ''}
        onClose={() => setImportTarget(null)}
      />
    </div>
  )
}

export { KnowledgeSection }
