import { useEffect, useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { IngestProgressDialog } from '@/components/ingest-progress-dialog'
import { cn } from '@/lib/utils'
import { pickNotesFolder } from '@/lib/ingest'
import { listKnowledgeItems } from '@/lib/knowledge'
import KnowledgeExplorerScreen from '@/screens/KnowledgeExplorerScreen'

interface KnowledgeSectionProps {
  // null means "All topics".
  selectedTopic: string | null
}

type Tab = 'explorer' | 'review'

// Owns the Explorer/Review tab state and the "Import notes" toolbar
// action — the main-pane counterpart to KnowledgeTopicTree in the sidebar.
// See the layout in
// specs/phases/phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md.
function KnowledgeSection({ selectedTopic }: KnowledgeSectionProps) {
  const [activeTab, setActiveTab] = useState<Tab>('explorer')
  const [draftCount, setDraftCount] = useState(0)
  const [importFolderPath, setImportFolderPath] = useState<string | null>(null)

  useEffect(() => {
    listKnowledgeItems('', 'draft')
      .then((items) => setDraftCount(items.length))
      .catch(() => {})
  }, [])

  async function handleImportClick() {
    const path = await pickNotesFolder()
    if (path) setImportFolderPath(path)
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
        <div role="tablist" aria-label="Modo de visualização" className="flex gap-1 rounded-lg border p-0.5">
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

        <Button onClick={() => void handleImportClick()}>Import notes</Button>
      </div>

      <div className="min-h-0 flex-1">
        <KnowledgeExplorerScreen selectedTopic={selectedTopic} mode={activeTab} />
      </div>

      <IngestProgressDialog
        open={importFolderPath !== null}
        folderPath={importFolderPath ?? ''}
        onClose={() => setImportFolderPath(null)}
      />
    </div>
  )
}

export { KnowledgeSection }
