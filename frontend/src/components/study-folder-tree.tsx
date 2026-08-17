import { forwardRef, useEffect, useImperativeHandle, useState } from 'react'
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
  listStudySessionsByFolder,
  moveStudySession,
  startStudySession,
  type StudySession,
} from '@/lib/study'

interface FolderNode extends Folder {
  open: boolean
  sessions: StudySession[] | null
}

export interface StudyFolderTreeHandle {
  // Re-fetches folderId's session list from the backend — called after an
  // action outside this tree (ending a session in the chat view) changes a
  // session's status.
  refreshFolder: (folderId: string) => void
}

interface StudyFolderTreeProps {
  selectedSessionId: string | null
  onSelectSession: (session: StudySession) => void
  onSessionStarted: (session: StudySession) => void
}

// The Study section's sidebar navigation: folders and sessions as an
// explorer-style tree, mirroring a code editor's file tree rather than a
// separate list screen. See specs/phases/phase-01-desktop-mvp/10-study-folders.md.
const StudyFolderTree = forwardRef<StudyFolderTreeHandle, StudyFolderTreeProps>(
  function StudyFolderTree({ selectedSessionId, onSelectSession, onSessionStarted }, ref) {
    const [folders, setFolders] = useState<FolderNode[]>([])
    const [newSessionFolderId, setNewSessionFolderId] = useState<string | null>(null)
    const [newSessionTopic, setNewSessionTopic] = useState('')
    const [renamingFolderId, setRenamingFolderId] = useState<string | null>(null)
    const [renameValue, setRenameValue] = useState('')
    const [deletingFolder, setDeletingFolder] = useState<Folder | null>(null)
    const [isNewFolderDialogOpen, setIsNewFolderDialogOpen] = useState(false)
    const [newFolderName, setNewFolderName] = useState('')

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

    useImperativeHandle(ref, () => ({
      refreshFolder: (folderId: string) => {
        void loadSessions(folderId)
      },
    }))

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
      onSessionStarted(session)
    }

    async function handleMoveSession(session: StudySession, targetFolderId: string) {
      await moveStudySession(session.id, targetFolderId)
      void loadSessions(session.folderId)
      void loadSessions(targetFolderId)
    }

    return (
      <div className="flex flex-col gap-0.5 py-0.5">
        {folders.map((folder) => (
          <div key={folder.id}>
            <div
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
            </div>

            {folder.open && (
              <div className="flex flex-col gap-0.5">
                {(folder.sessions ?? []).map((session) => (
                  <div
                    key={session.id}
                    className={cn(
                      'group flex cursor-pointer items-center gap-1.5 rounded-md py-1 pr-3 pl-10 text-xs hover:bg-accent',
                      session.id === selectedSessionId && 'bg-secondary',
                    )}
                    onClick={() => onSelectSession(session)}
                  >
                    <span
                      className={cn(
                        'size-1.5 shrink-0 rounded-full',
                        session.id === selectedSessionId
                          ? 'bg-primary shadow-[0_0_6px_1px_var(--primary)]'
                          : 'bg-muted-foreground',
                      )}
                      aria-hidden="true"
                    />
                    <span className="flex-1 truncate text-foreground">{session.topic}</span>
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
                      <DropdownMenuContent
                        align="start"
                        onCloseAutoFocus={(event) => event.preventDefault()}
                      >
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
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                ))}

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
      </div>
    )
  },
)

export { StudyFolderTree }
