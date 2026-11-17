import { useCallback, useEffect, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
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
import { DiagramEditor, type DiagramEditorHandle } from '@/components/diagram/diagram-editor'
import {
  cancelMindMapGeneration,
  deleteMindMap,
  getMindMap,
  mindMapErrorCode,
  mindMapErrorMessage,
  regenerateMindMap,
  saveMindMapDiagram,
  type MindMap,
  type MindMapReturn,
} from '@/lib/mindmap'

// How long the editor has to report it is loaded before the screen says it
// most likely needs the internet.
const EDITOR_LOAD_TIMEOUT_MS = 15000

interface MindMapViewScreenProps {
  mindMapId: string
  // Where the map goes back to when it is closed; it only names the button.
  returnTo: MindMapReturn
  // The shell wants to navigate away; the unsaved-changes question is asked here.
  leaveRequested: boolean
  onLeaveCancelled: () => void
  // The user is done: went back, or deleted the map.
  onExit: () => void
}

type Load = { state: 'loading' } | { state: 'failed' } | { state: 'ready'; mindMap: MindMap }

// One mind map, full screen, in the draw.io editor: edit by hand and Save, redraw
// it from its session with Update, or Delete it. Leaving with unsaved edits asks
// first. See specs/phases/phase-03-architecture-lab/24-mind-map.md.
function MindMapViewScreen(props: MindMapViewScreenProps) {
  const { t } = useTranslation('mindmap')
  const [load, setLoad] = useState<Load>({ state: 'loading' })
  // Bumping this reloads the map.
  const [reloadToken, setReloadToken] = useState(0)

  useEffect(() => {
    let stale = false
    getMindMap(props.mindMapId)
      .then((mindMap) => {
        if (!stale) setLoad({ state: 'ready', mindMap })
      })
      .catch(() => {
        if (!stale) setLoad({ state: 'failed' })
      })
    return () => {
      stale = true
    }
  }, [props.mindMapId, reloadToken])

  // Nothing is open, so a navigation request is granted.
  const { leaveRequested, onExit } = props
  const hasMap = load.state === 'ready'
  useEffect(() => {
    if (leaveRequested && !hasMap) onExit()
  }, [leaveRequested, hasMap, onExit])

  if (load.state === 'ready') {
    return <Editing key={load.mindMap.id} {...props} initial={load.mindMap} />
  }
  return (
    <div className="flex h-full w-full flex-col items-center justify-center gap-3 text-center">
      {load.state === 'loading' ? (
        <p className="text-sm text-muted-foreground">{t('view.loading')}</p>
      ) : (
        <>
          <p role="alert" className="text-sm text-destructive">
            {t('view.loadFailed')}
          </p>
          <div className="flex gap-2">
            <Button
              type="button"
              onClick={() => {
                setLoad({ state: 'loading' })
                setReloadToken((token) => token + 1)
              }}
            >
              {t('common.retry')}
            </Button>
            <Button type="button" variant="outline" onClick={props.onExit}>
              {backLabel(t, props.returnTo)}
            </Button>
          </div>
        </>
      )}
    </div>
  )
}

function backLabel(t: (key: string) => string, returnTo: MindMapReturn): string {
  return returnTo.kind === 'study' ? t('view.backToSession') : t('view.back')
}

type Busy = 'saving' | 'regenerating' | 'deleting' | null
type Confirming = 'regenerate' | 'delete' | 'leave' | null

interface EditingProps extends MindMapViewScreenProps {
  initial: MindMap
}

function Editing({ initial, returnTo, leaveRequested, onLeaveCancelled, onExit }: EditingProps) {
  const { t } = useTranslation('mindmap')
  const editorRef = useRef<DiagramEditorHandle>(null)
  const [mindMap, setMindMap] = useState(initial)
  // A new key replaces the editor, which is how a regenerated map, or a retry,
  // is shown.
  const [editorKey, setEditorKey] = useState(0)
  const [editorReady, setEditorReady] = useState(false)
  const [editorUnavailable, setEditorUnavailable] = useState(!navigator.onLine)
  const [connectionLost, setConnectionLost] = useState(false)
  const [dirty, setDirty] = useState(false)
  // Counts the editor's change events, so a Save can tell whether the user edited
  // again while it was in flight: that edit is not in the saved map.
  const editsRef = useRef(0)
  const [busy, setBusy] = useState<Busy>(null)
  const [message, setMessage] = useState<string | null>(null)
  // The question the user started: Update, Delete or Back with unsaved edits.
  const [asked, setAsked] = useState<Confirming>(null)
  // A navigation request from the sidebar asks the leave question as well.
  const confirming: Confirming = asked ?? (leaveRequested && dirty ? 'leave' : null)

  // Without the editor reporting in time, the screen says it needs the internet.
  useEffect(() => {
    if (editorReady || editorUnavailable) return
    const timer = setTimeout(() => setEditorUnavailable(true), EDITOR_LOAD_TIMEOUT_MS)
    return () => clearTimeout(timer)
  }, [editorReady, editorUnavailable, editorKey])

  // A navigation request from the sidebar is granted at once when nothing is
  // unsaved; otherwise the leave question above is asked.
  useEffect(() => {
    if (leaveRequested && !dirty) onExit()
  }, [leaveRequested, dirty, onExit])

  const reloadEditor = useCallback(() => {
    setEditorReady(false)
    setEditorUnavailable(!navigator.onLine)
    setEditorKey((key) => key + 1)
  }, [])

  function handleBack() {
    if (dirty) setAsked('leave')
    else onExit()
  }

  function closeConfirm() {
    if (confirming === 'leave' && leaveRequested) onLeaveCancelled()
    setAsked(null)
  }

  async function handleSave() {
    const editor = editorRef.current
    if (!editor || busy) return
    setMessage(null)
    setBusy('saving')
    const editsBeforeSave = editsRef.current
    try {
      const saved = await saveMindMapDiagram(mindMap.id, await editor.getXml())
      setMindMap(saved)
      setDirty(editsRef.current !== editsBeforeSave)
    } catch (err) {
      setMessage(`${t('view.saveFailed')} ${mindMapErrorMessage(err)}`)
    } finally {
      setBusy(null)
    }
  }

  async function handleRegenerate() {
    setAsked(null)
    setMessage(null)
    setBusy('regenerating')
    try {
      const regenerated = await regenerateMindMap(mindMap.id)
      setMindMap(regenerated)
      setDirty(false)
      reloadEditor()
    } catch (err) {
      // A cancelled regeneration is the user's own doing: back to the map, quietly.
      if (mindMapErrorCode(err) !== 'cancelled') {
        setMessage(`${t('view.regenerateFailed')} ${mindMapErrorMessage(err)}`)
      }
    } finally {
      setBusy(null)
    }
  }

  async function handleDelete() {
    setAsked(null)
    setMessage(null)
    setBusy('deleting')
    try {
      await deleteMindMap(mindMap.id)
      onExit()
    } catch {
      setMessage(t('view.deleteFailed'))
      setBusy(null)
    }
  }

  const working = busy !== null

  return (
    <div className="flex h-full min-h-0 w-full flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="truncate text-lg font-semibold text-foreground">{mindMap.title}</h2>
          <p className="truncate text-sm text-muted-foreground">{mindMap.sessionTitle}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span role="status" className="text-sm text-muted-foreground">
            {busy === 'saving' ? t('view.saving') : dirty ? t('view.unsaved') : ''}
          </span>
          <Button type="button" variant="outline" size="sm" disabled={working} onClick={handleBack}>
            {backLabel(t, returnTo)}
          </Button>
          <Button
            type="button"
            size="sm"
            disabled={working || !editorReady || !dirty}
            onClick={() => void handleSave()}
          >
            {t('view.save')}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={working}
            onClick={() => setAsked('regenerate')}
          >
            {t('view.regenerate')}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={working}
            onClick={() => setAsked('delete')}
          >
            {t('view.delete')}
          </Button>
        </div>
      </div>

      {message && (
        <p role="alert" className="text-sm text-destructive">
          {message}
        </p>
      )}

      <div className="relative min-h-0 flex-1 overflow-hidden rounded-md border border-border">
        {editorUnavailable ? (
          <div className="flex size-full flex-col items-center justify-center gap-3 p-4 text-center">
            <p className="text-sm text-foreground">{t('view.editorNeedsInternet')}</p>
            <Button type="button" onClick={reloadEditor}>
              {t('common.tryAgain')}
            </Button>
          </div>
        ) : (
          <>
            <DiagramEditor
              key={editorKey}
              ref={editorRef}
              xml={mindMap.diagramXml}
              onReady={() => {
                setEditorReady(true)
                setConnectionLost(false)
              }}
              onChange={() => {
                editsRef.current += 1
                setDirty(true)
              }}
              onConnectionLost={() => setConnectionLost(true)}
              onConnectionRestored={() => setConnectionLost(false)}
            />
            {!editorReady && (
              <p
                role="status"
                className="absolute inset-0 z-10 flex items-center justify-center gap-2 bg-background text-sm text-muted-foreground"
              >
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
                {t('view.preparing')}
              </p>
            )}
            {connectionLost && (
              <p
                role="status"
                className="absolute inset-x-0 top-0 z-10 bg-amber-500/90 px-3 py-1 text-center text-sm text-black"
              >
                {t('view.editorNeedsInternet')}
              </p>
            )}
          </>
        )}
        {busy === 'regenerating' && (
          <div
            role="status"
            className="absolute inset-0 z-20 flex flex-col items-center justify-center gap-3 bg-background/90 text-sm text-muted-foreground"
          >
            <span className="flex items-center gap-2">
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              {t('view.regenerating')}
            </span>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => void cancelMindMapGeneration()}
            >
              {t('common.cancel')}
            </Button>
          </div>
        )}
      </div>

      <AlertDialog open={confirming !== null} onOpenChange={(open) => !open && closeConfirm()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirmTitle(t, confirming)}</AlertDialogTitle>
            <AlertDialogDescription>{confirmDescription(t, confirming)}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {confirming === 'leave' ? t('view.keepEditing') : t('common.cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (confirming === 'regenerate') void handleRegenerate()
                else if (confirming === 'delete') void handleDelete()
                else onExit()
              }}
            >
              {confirmAction(t, confirming)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function confirmTitle(t: (key: string) => string, confirming: Confirming): string {
  if (confirming === 'regenerate') return t('view.regenerateTitle')
  if (confirming === 'delete') return t('view.deleteTitle')
  return t('view.leaveTitle')
}

function confirmDescription(t: (key: string) => string, confirming: Confirming): string {
  if (confirming === 'regenerate') return t('view.regenerateDescription')
  if (confirming === 'delete') return t('view.deleteDescription')
  return t('view.leaveDescription')
}

function confirmAction(t: (key: string) => string, confirming: Confirming): string {
  if (confirming === 'regenerate') return t('view.regenerateConfirm')
  if (confirming === 'delete') return t('view.delete')
  return t('view.leave')
}

export default MindMapViewScreen
