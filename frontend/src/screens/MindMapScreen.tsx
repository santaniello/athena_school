import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { GenerateMindMapDialog } from '@/components/mindmap/generate-mindmap-dialog'
import { MindMapRow } from '@/components/mindmap/mindmap-row'
import { deleteMindMap, listMindMaps, mindMapErrorCode, type MindMapSummary } from '@/lib/mindmap'

interface MindMapScreenProps {
  // Open was clicked on a map in the list; the shell opens the editor. A map that
  // was just generated is only highlighted in the list.
  onOpen: (mindMapId: string) => void
}

type MindMaps =
  { state: 'loading' } | { state: 'failed' } | { state: 'ready'; list: MindMapSummary[] }

// The Mind Map section: the user's mind maps, newest first, with the Generate
// mind map with AI action. Everything shown comes from the backend. See
// specs/phases/phase-03-architecture-lab/24-mind-map.md.
function MindMapScreen({ onOpen }: MindMapScreenProps) {
  const { t } = useTranslation('mindmap')
  const [mindMaps, setMindMaps] = useState<MindMaps>({ state: 'loading' })
  // Bumping this reloads the list.
  const [reloadToken, setReloadToken] = useState(0)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [highlightedId, setHighlightedId] = useState<string | null>(null)
  const [toDelete, setToDelete] = useState<MindMapSummary | null>(null)
  const [deleteFailed, setDeleteFailed] = useState(false)
  // A second click while the deletion is in flight must not delete again.
  const [deleting, setDeleting] = useState(false)

  useEffect(() => {
    let stale = false
    listMindMaps()
      .then((list) => {
        if (!stale) setMindMaps({ state: 'ready', list })
      })
      .catch(() => {
        if (!stale) setMindMaps({ state: 'failed' })
      })
    return () => {
      stale = true
    }
  }, [reloadToken])

  const reload = useCallback(() => setReloadToken((token) => token + 1), [])

  function retry() {
    setMindMaps({ state: 'loading' })
    reload()
  }

  function handleGenerated(mindMap: MindMapSummary) {
    setDialogOpen(false)
    setHighlightedId(mindMap.id)
    reload()
  }

  async function confirmDelete() {
    if (!toDelete || deleting) return
    setDeleting(true)
    try {
      await deleteMindMap(toDelete.id)
      setToDelete(null)
      reload()
    } catch (err) {
      setToDelete(null)
      // A map that is already gone is what the user wanted.
      if (mindMapErrorCode(err) === 'mind_map_not_found') reload()
      else setDeleteFailed(true)
    } finally {
      setDeleting(false)
    }
  }

  const list = mindMaps.state === 'ready' ? mindMaps.list : []

  return (
    <div className="flex min-h-0 w-full flex-col gap-4">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-foreground">{t('catalog.title')}</h2>
          <p className="text-sm text-muted-foreground">{t('catalog.description')}</p>
        </div>
        <Button type="button" onClick={() => setDialogOpen(true)}>
          {t('catalog.generate')}
        </Button>
      </div>

      {deleteFailed && (
        <p role="alert" className="text-sm text-destructive">
          {t('catalog.deleteFailed')}
        </p>
      )}

      <div className="thin-scroll min-h-0 flex-1 overflow-y-auto">
        {mindMaps.state === 'failed' ? (
          <div role="alert" className="flex items-center gap-3 text-sm">
            <span className="text-destructive">{t('catalog.loadFailed')}</span>
            <Button type="button" variant="outline" size="sm" onClick={retry}>
              {t('common.retry')}
            </Button>
          </div>
        ) : mindMaps.state === 'loading' ? (
          <p className="text-sm text-muted-foreground">{t('catalog.loading')}</p>
        ) : list.length > 0 ? (
          <ul className="divide-y divide-border rounded-lg border border-border">
            {list.map((mindMap) => (
              <MindMapRow
                key={mindMap.id}
                mindMap={mindMap}
                highlighted={mindMap.id === highlightedId}
                onOpen={(chosen) => onOpen(chosen.id)}
                onDelete={(chosen) => {
                  setDeleteFailed(false)
                  setToDelete(chosen)
                }}
              />
            ))}
          </ul>
        ) : (
          <div className="flex flex-col items-center gap-2 py-10 text-center">
            <p className="max-w-80 text-sm text-muted-foreground">{t('catalog.empty')}</p>
            <Button type="button" onClick={() => setDialogOpen(true)}>
              {t('catalog.generate')}
            </Button>
          </div>
        )}
      </div>

      <GenerateMindMapDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        onGenerated={handleGenerated}
        onGeneratedAfterClose={reload}
      />

      <AlertDialog open={toDelete !== null} onOpenChange={(open) => !open && setToDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('catalog.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('catalog.deleteDescription')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              // The action closes the dialog on click unless prevented; the
              // dialog closes once the deletion has an answer.
              onClick={(event) => {
                event.preventDefault()
                void confirmDelete()
              }}
            >
              {t('catalog.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

export default MindMapScreen
