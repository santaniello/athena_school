import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { formatMindMapDate, type MindMapSummary } from '@/lib/mindmap'
import { cn } from '@/lib/utils'

interface MindMapRowProps {
  mindMap: MindMapSummary
  // True for the mind map that was just generated.
  highlighted: boolean
  onOpen: (mindMap: MindMapSummary) => void
  onDelete: (mindMap: MindMapSummary) => void
}

// One catalog row: the map's title, the session it is about and when it was
// last updated. It never shows the diagram.
function MindMapRow({ mindMap, highlighted, onOpen, onDelete }: MindMapRowProps) {
  const { t } = useTranslation('mindmap')

  return (
    <li
      aria-label={mindMap.title}
      aria-current={highlighted ? 'true' : undefined}
      className={cn(
        'flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-3',
        highlighted && 'bg-primary/10 ring-1 ring-primary/40',
      )}
    >
      <div className="min-w-0 flex-1 basis-48">
        <p className="truncate text-sm font-semibold text-foreground">{mindMap.title}</p>
        <p className="flex flex-wrap gap-x-2 text-sm text-muted-foreground">
          <span>{mindMap.sessionTitle}</span>
          <span>{t('catalog.updated', { date: formatMindMapDate(mindMap.updatedAt) })}</span>
        </p>
      </div>
      <div className="flex flex-wrap items-center justify-end gap-3">
        <Button type="button" variant="outline" size="sm" onClick={() => onOpen(mindMap)}>
          {t('catalog.open')}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={() => onDelete(mindMap)}>
          {t('catalog.delete')}
        </Button>
      </div>
    </li>
  )
}

export { MindMapRow }
