import { useEffect, useState, type ReactNode } from 'react'
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import {
  ChevronRight,
  Folder as FolderIcon,
  MoreVertical,
  Pencil,
  Plus,
  Trash2,
} from 'lucide-react'
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { createFolder, deleteFolder, listFolders, renameFolder, type Folder } from '@/lib/folder'
import {
  deleteStudySession,
  listStudySessionsByFolder,
  moveStudySession,
  startStudySession,
  type StudySession,
} from '@/lib/study'

interface FolderNode extends Folder {
  open: boolean
  sessions: StudySession[] | null
}

interface StudyFolderTreeProps {
  selectedSessionId: string | null
  onSelectSession: (session: StudySession, folderName: string) => void
  onSessionStarted: (session: StudySession, folderName: string) => void
  onSessionDeleted: (sessionId: string) => void
}

interface DraggableSessionRowProps {
  session: StudySession
  className: string
  onClick: () => void
  children: ReactNode
}

// Wraps a session row so it can be picked up and dropped onto a
// DroppableFolderHeader elsewhere in the tree. Only the outer row is a
// hook-bearing component so the surrounding .map() stays hook-free.
function DraggableSessionRow({ session, className, onClick, children }: DraggableSessionRowProps) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: session.id,
    data: { session },
  })
  return (
    <div
      ref={setNodeRef}
      {...listeners}
      {...attributes}
      onClick={onClick}
      className={cn(className, isDragging && 'opacity-40')}
    >
      {children}
    </div>
  )
}

interface DroppableFolderHeaderProps {
  folderId: string
  className: string
  onClick: () => void
  children: ReactNode
}

function DroppableFolderHeader({
  folderId,
  className,
  onClick,
  children,
}: DroppableFolderHeaderProps) {
  const { setNodeRef, isOver } = useDroppable({ id: folderId })

  return (
    <div
      ref={setNodeRef}
      onClick={onClick}
      className={cn(className, isOver && 'bg-accent ring-1 ring-inset ring-primary/50')}
    >
      {children}
    </div>
  )
}

// The Study section's sidebar navigation: folders and sessions as an
// explorer-style tree, mirroring a code editor's file tree rather than a
// separate list screen. See specs/phases/phase-01-desktop-mvp/10-study-folders.md.
function StudyFolderTree({
  selectedSessionId,
  onSelectSession,
  onSessionStarted,
  onSessionDeleted,
}: StudyFolderTreeProps) {
  const [folders, setFolders] = useState<FolderNode[]>([])
  const [newSessionFolderId, setNewSessionFolderId] = useState<string | null>(null)
  const [newSessionTopic, setNewSessionTopic] = useState('')
  const [renamingFolderId, setRenamingFolderId] = useState<string | null>(null)
  const [renameValue, setRenameValue] = useState('')
  const [deletingFolder, setDeletingFolder] = useState<Folder | null>(null)
  const [deletingSession, setDeletingSession] = useState<StudySession | null>(null)
  const [isNewFolderDialogOpen, setIsNewFolderDialogOpen] = useState(false)
  const [newFolderName, setNewFolderName] = useState('')
  const [draggingSession, setDraggingSession] = useState<StudySession | null>(null)
  // Only starts a drag past a small pointer-move threshold, so a plain
  // click still selects the session instead of always arming a drag.
  const dndSensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }))

  useEffect(() => {
    void listFolders().then((loaded) =>
      setFolders(loaded.map((folder) => ({ ...folder, open: false, sessions: null }))),
    )
  }, [])

  async function loadSessions(folderId: string) {
    const sessions = await listStudySessionsByFolder(folderId)
    setFolders((previous) =>
      previous.map((folder) => (folder.id === folderId ? { ...folder, sessions } : folder)),
    )
  }

  function toggleFolder(folder: FolderNode) {
    const willOpen = !folder.open
    setFolders((previous) =>
      previous.map((f) => (f.id === folder.id ? { ...f, open: willOpen } : f)),
    )
    if (willOpen && folder.sessions === null) void loadSessions(folder.id)
  }

  async function handleCreateFolder() {
    const name = newFolderName.trim()
    if (!name) return
    const created = await createFolder(name)
    setFolders((previous) => [...previous, { ...created, open: false, sessions: null }])
    setNewFolderName('')
    setIsNewFolderDialogOpen(false)
  }

  async function handleRenameSubmit(folderId: string) {
    const name = renameValue.trim()
    setRenamingFolderId(null)
    if (!name) return
    await renameFolder(folderId, name)
    setFolders((previous) =>
      previous.map((folder) => (folder.id === folderId ? { ...folder, name } : folder)),
    )
  }

  async function handleDeleteFolder() {
    if (!deletingFolder) return
    const id = deletingFolder.id
    const defaultFolder = folders.find((folder) => folder.isDefault)
    setDeletingFolder(null)
    await deleteFolder(id)
    setFolders((previous) => previous.filter((folder) => folder.id !== id))
    if (defaultFolder && defaultFolder.sessions !== null) void loadSessions(defaultFolder.id)
  }

  async function handleStartSession(folderId: string) {
    const topic = newSessionTopic.trim()
    if (!topic) return
    setNewSessionFolderId(null)
    setNewSessionTopic('')
    const session = await startStudySession(topic, folderId)
    setFolders((previous) =>
      previous.map((folder) =>
        folder.id === folderId
          ? { ...folder, sessions: [session, ...(folder.sessions ?? [])], open: true }
          : folder,
      ),
    )
    const folder = folders.find((f) => f.id === folderId)
    onSessionStarted(session, folder?.name ?? '')
  }

  async function handleMoveSession(session: StudySession, targetFolderId: string) {
    await moveStudySession(session.id, targetFolderId)
    void loadSessions(session.folderId)
    void loadSessions(targetFolderId)
  }

  function handleDragStart(event: DragStartEvent) {
    setDraggingSession((event.active.data.current?.session as StudySession | undefined) ?? null)
  }

  function handleDragEnd(event: DragEndEvent) {
    setDraggingSession(null)
    const session = event.active.data.current?.session as StudySession | undefined
    const targetFolderId = event.over?.id
    if (!session || typeof targetFolderId !== 'string') return
    if (targetFolderId === session.folderId) return
    void handleMoveSession(session, targetFolderId)
  }

  async function handleDeleteSession() {
    if (!deletingSession) return
    const { id, folderId } = deletingSession
    setDeletingSession(null)
    await deleteStudySession(id)
    setFolders((previous) =>
      previous.map((folder) =>
        folder.id === folderId
          ? { ...folder, sessions: (folder.sessions ?? []).filter((s) => s.id !== id) }
          : folder,
      ),
    )
    onSessionDeleted(id)
  }

  function renderSessionRow(session: StudySession, folder: FolderNode) {
    const selected = session.id === selectedSessionId
    return (
      <DraggableSessionRow
        key={session.id}
        session={session}
        className={cn(
          'group flex cursor-pointer items-center gap-1.5 rounded-md py-1 pr-3 pl-10 text-xs hover:bg-accent',
          selected && 'bg-secondary',
        )}
        onClick={() => onSelectSession(session, folder.name)}
      >
        <span
          className={cn(
            'size-1.5 shrink-0 rounded-full',
            selected ? 'bg-primary shadow-[0_0_6px_1px_var(--primary)]' : 'bg-muted-foreground',
          )}
          aria-hidden="true"
        />
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate text-foreground">{session.topic}</span>
        </span>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label={`${session.topic} options`}
              onClick={(event) => event.stopPropagation()}
              className="opacity-0 group-hover:opacity-100"
            >
              <MoreVertical aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" onCloseAutoFocus={(event) => event.preventDefault()}>
            <DropdownMenuLabel>Move to</DropdownMenuLabel>
            {folders
              .filter((f) => f.id !== folder.id)
              .map((target) => (
                <DropdownMenuItem
                  key={target.id}
                  onClick={() => void handleMoveSession(session, target.id)}
                >
                  {target.name}
                </DropdownMenuItem>
              ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={() => setDeletingSession(session)}>
              <Trash2 aria-hidden="true" />
              Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </DraggableSessionRow>
    )
  }

  return (
    <DndContext sensors={dndSensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
      <div className="flex flex-col gap-0.5 py-0.5">
        {folders.map((folder) => (
          <div key={folder.id}>
            <DroppableFolderHeader
              folderId={folder.id}
              className="group flex cursor-pointer items-center gap-1.5 rounded-md py-1 pr-3 pl-6 text-xs hover:bg-accent"
              onClick={() => toggleFolder(folder)}
            >
              <ChevronRight
                className={cn(
                  'size-3 shrink-0 text-muted-foreground transition-transform',
                  folder.open && 'rotate-90',
                )}
                aria-hidden="true"
              />
              <FolderIcon className="size-3.5 shrink-0 text-primary" aria-hidden="true" />
              {renamingFolderId === folder.id ? (
                <Input
                  autoFocus
                  value={renameValue}
                  onChange={(event) => setRenameValue(event.target.value)}
                  onClick={(event) => event.stopPropagation()}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') void handleRenameSubmit(folder.id)
                    if (event.key === 'Escape') setRenamingFolderId(null)
                  }}
                  onBlur={() => void handleRenameSubmit(folder.id)}
                  className="h-6 flex-1 text-xs"
                />
              ) : (
                <span className="flex-1 truncate font-medium text-foreground">{folder.name}</span>
              )}
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    aria-label={`${folder.name} options`}
                    onClick={(event) => event.stopPropagation()}
                    className="opacity-0 group-hover:opacity-100"
                  >
                    <MoreVertical aria-hidden="true" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent
                  align="start"
                  onCloseAutoFocus={(event) => event.preventDefault()}
                >
                  <DropdownMenuItem
                    onClick={() => {
                      // Deferred past Radix's own close/focus-return cycle for
                      // this menu — starting the rename synchronously races
                      // with it and the input loses focus (and gets
                      // blur-submitted) before the user can type.
                      setTimeout(() => {
                        setRenamingFolderId(folder.id)
                        setRenameValue(folder.name)
                      }, 0)
                    }}
                  >
                    <Pencil aria-hidden="true" />
                    Rename
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    variant="destructive"
                    disabled={folder.isDefault}
                    onClick={() => setDeletingFolder(folder)}
                  >
                    <Trash2 aria-hidden="true" />
                    Delete folder
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </DroppableFolderHeader>

            {folder.open && (
              <div className="flex flex-col gap-0.5">
                {(folder.sessions ?? [])
                  .slice()
                  .sort((a, b) => b.startedAt.localeCompare(a.startedAt))
                  .map((session) => renderSessionRow(session, folder))}

                {newSessionFolderId === folder.id ? (
                  <form
                    className="flex items-center gap-1 py-1 pr-1 pl-10"
                    onSubmit={(event) => {
                      event.preventDefault()
                      void handleStartSession(folder.id)
                    }}
                  >
                    <Input
                      autoFocus
                      value={newSessionTopic}
                      onChange={(event) => setNewSessionTopic(event.target.value)}
                      placeholder="What do you want to study?"
                      className="h-6 flex-1 text-xs"
                      onKeyDown={(event) => {
                        if (event.key === 'Escape') setNewSessionFolderId(null)
                      }}
                    />
                  </form>
                ) : (
                  <button
                    type="button"
                    className="flex cursor-pointer items-center gap-1.5 py-1 pr-1 pl-10 text-xs text-muted-foreground hover:text-primary"
                    onClick={() => {
                      setNewSessionFolderId(folder.id)
                      setNewSessionTopic('')
                    }}
                  >
                    <Plus className="size-3" aria-hidden="true" />
                    New session
                  </button>
                )}
              </div>
            )}
          </div>
        ))}

        <Button
          variant="ghost"
          size="sm"
          className="mx-1 mt-0.5 justify-start gap-1.5 px-2 text-xs text-muted-foreground"
          onClick={() => setIsNewFolderDialogOpen(true)}
        >
          <Plus className="size-3.5" aria-hidden="true" />
          New folder
        </Button>

        <Dialog open={isNewFolderDialogOpen} onOpenChange={setIsNewFolderDialogOpen}>
          <DialogContent>
            <form
              onSubmit={(event) => {
                event.preventDefault()
                void handleCreateFolder()
              }}
            >
              <DialogHeader>
                <DialogTitle>New folder</DialogTitle>
                <DialogDescription>
                  Group related study sessions together, like a ChatGPT project.
                </DialogDescription>
              </DialogHeader>
              <Input
                autoFocus
                value={newFolderName}
                onChange={(event) => setNewFolderName(event.target.value)}
                placeholder="e.g. Distributed Systems"
                className="mt-2"
              />
              <DialogFooter>
                <Button type="submit" disabled={!newFolderName.trim()}>
                  Create
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>

        <AlertDialog
          open={deletingFolder !== null}
          onOpenChange={(open) => !open && setDeletingFolder(null)}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete {deletingFolder?.name}?</AlertDialogTitle>
              <AlertDialogDescription>
                Its sessions move to General — nothing is deleted.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction onClick={() => void handleDeleteFolder()}>
                Delete folder
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

        <AlertDialog
          open={deletingSession !== null}
          onOpenChange={(open) => !open && setDeletingSession(null)}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete &quot;{deletingSession?.topic}&quot;?</AlertDialogTitle>
              <AlertDialogDescription>
                Its messages will be permanently deleted. This can&apos;t be undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction onClick={() => void handleDeleteSession()}>
                Delete session
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>

      <DragOverlay>
        {draggingSession && (
          <div className="flex items-center gap-1.5 rounded-md border bg-popover px-3 py-1 text-xs shadow-lg">
            <span
              className="size-1.5 shrink-0 rounded-full bg-muted-foreground"
              aria-hidden="true"
            />
            <span className="truncate text-foreground">{draggingSession.topic}</span>
          </div>
        )}
      </DragOverlay>
    </DndContext>
  )
}

export { StudyFolderTree }
