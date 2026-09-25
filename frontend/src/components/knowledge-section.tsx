import { useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { PendingReconciliationSection } from '@/components/pending-reconciliation-section'
import { cn } from '@/lib/utils'
import KnowledgeExplorerScreen from '@/screens/KnowledgeExplorerScreen'

interface KnowledgeSectionProps {
  // null means "All topics".
  selectedTopic: string | null
  // True while the knowledge index is retrying — edit/approve/deprecate/
  // delete are rejected by a backend guard during a retry, so the UI
  // disables them too rather than letting a call fail confusingly.
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

// Owns the Explorer/Review tab state — the main-pane counterpart to
// KnowledgeTopicTree in the sidebar. Importing is not offered here: knowledge
// is owned by a study session, so importing belongs to that session's Sources
// panel. See specs/phases/phase-02-knowledge-engine/15-session-scoped-knowledge.md.
function KnowledgeSection({
  selectedTopic,
  mutationsDisabled,
  draftCount,
  onKnowledgeChanged,
  onTopicsChanged,
}: KnowledgeSectionProps) {
  const [activeTab, setActiveTab] = useState<Tab>('explorer')

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
      </div>

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
    </div>
  )
}

export { KnowledgeSection }
