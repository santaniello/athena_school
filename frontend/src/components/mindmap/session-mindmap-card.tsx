import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { formatMindMapDate, type MindMapSummary } from '@/lib/mindmap'

interface SessionMindMapCardProps {
  mindMap: MindMapSummary
  onOpen: (mindMap: MindMapSummary) => void
}

// The accepted mind map, kept under the tutor's message that offered it: its
// title, when it was last updated and the button to open it. It never shows the
// diagram. See specs/phases/phase-03-architecture-lab/24-mind-map.md.
function SessionMindMapCard({ mindMap, onOpen }: SessionMindMapCardProps) {
  const { t } = useTranslation('mindmap')

  return (
    <div className="flex flex-col items-start gap-3 rounded-lg border bg-card p-3 text-sm">
      <p className="font-medium">{mindMap.title}</p>
      <p className="text-muted-foreground">
        {t('catalog.updated', { date: formatMindMapDate(mindMap.updatedAt) })}
      </p>
      <Button size="sm" onClick={() => onOpen(mindMap)}>
        {t('sessionCard.open')}
      </Button>
    </div>
  )
}

export { SessionMindMapCard }
