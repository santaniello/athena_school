import { useEffect, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { formatSessionDate, listSessionsForGeneration, type SessionOption } from '@/lib/exercise'
import {
  cancelMindMapGeneration,
  generateMindMap,
  mindMapErrorCode,
  mindMapErrorMessage,
  type MindMapSummary,
} from '@/lib/mindmap'

interface GenerateMindMapDialogProps {
  open: boolean
  onClose: () => void
  // Called with the new mind map; the caller closes the dialog and reloads.
  onGenerated: (mindMap: MindMapSummary) => void
  // Called instead when the dialog was already closed by the time the map
  // arrived: it exists in the catalog, but nothing on screen is waiting for it.
  onGeneratedAfterClose: (mindMap: MindMapSummary) => void
}

// Opened by "Generate mind map with AI". The content is its own component, so
// the chosen session and any error are discarded each time the dialog closes.
export function GenerateMindMapDialog({ open, ...rest }: GenerateMindMapDialogProps) {
  return open ? <GenerateMindMapContent {...rest} /> : null
}

type Sessions =
  { state: 'loading' } | { state: 'failed' } | { state: 'ready'; list: SessionOption[] }

function GenerateMindMapContent({
  onClose,
  onGenerated,
  onGeneratedAfterClose,
}: Omit<GenerateMindMapDialogProps, 'open'>) {
  const { t } = useTranslation('mindmap')
  const [sessions, setSessions] = useState<Sessions>({ state: 'loading' })
  const [reloadToken, setReloadToken] = useState(0)
  const [search, setSearch] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // False once the dialog has closed, so a late answer is told apart from one
  // this instance is still waiting for.
  const mountedRef = useRef(false)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  useEffect(() => {
    let stale = false
    listSessionsForGeneration()
      .then((list) => {
        if (!stale) setSessions({ state: 'ready', list })
      })
      .catch(() => {
        if (!stale) setSessions({ state: 'failed' })
      })
    return () => {
      stale = true
    }
  }, [reloadToken])

  function retryLoading() {
    setSessions({ state: 'loading' })
    setReloadToken((token) => token + 1)
  }

  async function handleGenerate() {
    if (!selectedId) return
    setError(null)
    setGenerating(true)
    try {
      const mindMap = await generateMindMap(selectedId)
      if (mountedRef.current) onGenerated(mindMap)
      else onGeneratedAfterClose(mindMap)
    } catch (err) {
      // A cancelled generation is the user's own doing: back to the choice, quietly.
      if (mindMapErrorCode(err) !== 'cancelled') setError(mindMapErrorMessage(err))
      setGenerating(false)
    }
  }

  // Dismissing the dialog (Escape, the close button) must not leave a
  // generation running that nobody is waiting for.
  function handleDismiss() {
    if (generating) void cancelMindMapGeneration()
    onClose()
  }

  const list = sessions.state === 'ready' ? sessions.list : []
  const needle = search.trim().toLowerCase()
  const visible = list.filter(
    (session) =>
      session.topic.toLowerCase().includes(needle) ||
      session.folderName.toLowerCase().includes(needle),
  )

  return (
    <Dialog open onOpenChange={(nextOpen) => !nextOpen && handleDismiss()}>
      <DialogContent
        closeLabel={t('common.close')}
        className="max-h-[90vh] overflow-y-auto sm:max-w-lg"
      >
        <DialogHeader>
          <DialogTitle>{t('generate.title')}</DialogTitle>
          <DialogDescription>{t('generate.description')}</DialogDescription>
        </DialogHeader>

        {sessions.state === 'loading' && (
          <p className="text-sm text-muted-foreground">{t('generate.loadingSessions')}</p>
        )}
        {sessions.state === 'failed' && (
          <div role="alert" className="flex items-center justify-between gap-3 text-sm">
            <span className="text-destructive">{t('generate.sessionsFailed')}</span>
            <Button type="button" variant="outline" size="sm" onClick={retryLoading}>
              {t('common.retry')}
            </Button>
          </div>
        )}
        {sessions.state === 'ready' && list.length === 0 && (
          <p className="text-sm text-muted-foreground">{t('generate.noSessions')}</p>
        )}
        {sessions.state === 'ready' && list.length > 0 && (
          <div className="space-y-3">
            <Input
              type="search"
              aria-label={t('generate.searchLabel')}
              placeholder={t('generate.searchPlaceholder')}
              value={search}
              disabled={generating}
              onChange={(event) => setSearch(event.target.value)}
            />
            <div
              role="radiogroup"
              aria-label={t('generate.sessionsLabel')}
              className="thin-scroll max-h-64 space-y-1 overflow-y-auto"
            >
              {visible.map((session) => (
                // A native radio gives one tab stop and the arrow keys for free.
                <label
                  key={session.id}
                  className="flex w-full cursor-pointer flex-col rounded-lg border border-border px-3 py-2 text-left text-sm transition-colors hover:bg-accent has-[:checked]:border-primary has-[:checked]:bg-primary/10 has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-60 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring"
                >
                  <input
                    type="radio"
                    name="mindmap-study-session"
                    className="sr-only"
                    checked={session.id === selectedId}
                    disabled={generating}
                    onChange={() => setSelectedId(session.id)}
                  />
                  <span className="font-medium text-foreground">{session.topic}</span>
                  <span className="text-muted-foreground">
                    {session.folderName} · {formatSessionDate(session.startedAt)}
                  </span>
                </label>
              ))}
              {visible.length === 0 && (
                <p className="py-2 text-sm text-muted-foreground">{t('generate.noSessionMatch')}</p>
              )}
            </div>
          </div>
        )}

        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        {generating && (
          <p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            {t('generate.generating')}
          </p>
        )}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={generating ? () => void cancelMindMapGeneration() : onClose}
          >
            {t('common.cancel')}
          </Button>
          {sessions.state === 'ready' && list.length > 0 && !generating && (
            <Button type="button" disabled={!selectedId} onClick={() => void handleGenerate()}>
              {error ? t('common.tryAgain') : t('generate.submit')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
