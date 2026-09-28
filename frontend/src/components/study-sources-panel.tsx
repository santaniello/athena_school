import { useMemo, useState } from 'react'
import type { ChangeEvent, ReactNode } from 'react'
import { MoreVertical, Plus, Search, Trash2 } from 'lucide-react'
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { IngestProgressDialog } from '@/components/ingest-progress-dialog'
import { useSessionSources } from '@/hooks/use-session-sources'
import { pickNotesFile } from '@/lib/ingest'
import { removeSessionSource, type SessionSource } from '@/lib/sources'

interface StudySourcesPanelProps {
  sessionId: string
  // True while the knowledge index is retrying — Add is rejected by the
  // same backend guard ImportFile's index reservation goes through during a
  // retry, so the UI disables it too rather than letting the call fail
  // confusingly. Defaults to false (every call site before AppShell wires
  // it, and every test that isn't specifically about this state).
  mutationsDisabled?: boolean
}

// The existing inline error copy for a picker that failed to open, carried
// over from the pre-2.17 Knowledge screen's own "Import notes" action.
const PICKER_ERROR_MESSAGE = 'Failed to open the notes picker. Please try again.'

// chunkCountLabel pluralizes "chunk" for a row's subtitle — "1 chunk",
// "12 chunks".
function chunkCountLabel(chunkCount: number): string {
  return `${chunkCount} chunk${chunkCount === 1 ? '' : 's'}`
}

interface AddSourceButtonProps {
  onClick: () => void
  disabled: boolean
  children: ReactNode
  variant?: 'default' | 'outline'
  // The header's icon-and-label button needs an explicit accessible name
  // distinct from its visible "Add" text (to tell it apart from the
  // empty state's own, differently-labeled action); the empty state's
  // "Add a source" button is fine relying on its own visible text.
  ariaLabel?: string
}

// Add is disabled only while mutationsDisabled — Remove stays disabled
// unconditionally until its own increment wires it up (see the file
// comment). Sharing this between the header and empty-state actions keeps
// their disabled/tooltip behavior from drifting apart.
function AddSourceButton({
  onClick,
  disabled,
  children,
  variant = 'default',
  ariaLabel,
}: AddSourceButtonProps) {
  const button = (
    <Button
      size="sm"
      variant={variant}
      disabled={disabled}
      onClick={onClick}
      aria-label={ariaLabel}
    >
      <Plus className="size-3.5" aria-hidden="true" />
      {children}
    </Button>
  )
  if (!disabled) return button
  return (
    <Tooltip>
      {/* A disabled <button> takes pointer-events: none (buttonVariants) and
          drops out of the tab order, so neither hover nor focus ever
          reaches it — the tooltip could never open with asChild directly on
          the Button. Wrapping it in a plain, enabled <span> gives the
          trigger something that actually receives those events. */}
      <TooltipTrigger asChild>
        <span>{button}</span>
      </TooltipTrigger>
      <TooltipContent>Rebuilding knowledge index…</TooltipContent>
    </Tooltip>
  )
}

// The Study screen's NotebookLM-style right rail: the session's imported
// documents, from the moment each is imported (not a per-message dedupe of
// sources cited so far — see StudyChatScreen's per-message "Local sources"
// strip for that). See
// specs/phases/phase-02-knowledge-engine/17-session-sources-panel.md.
function StudySourcesPanel({ sessionId, mutationsDisabled = false }: StudySourcesPanelProps) {
  const { sources, loading, error, reload } = useSessionSources(sessionId)
  const [query, setQuery] = useState('')
  const [importPath, setImportPath] = useState<string | null>(null)
  const [pickerError, setPickerError] = useState('')
  // The document a Remove confirmation targets — non-null opens the
  // AlertDialog. removeError is scoped to that one confirmation, not the
  // whole panel, so opening it for another document always starts clean.
  const [removeTarget, setRemoveTarget] = useState<SessionSource | null>(null)
  const [removing, setRemoving] = useState(false)
  // Stryker disable next-line StringLiteral: only rendered while the
  // AlertDialog is open (removeTarget set), and every path that opens it
  // (handleRemoveClick) clears this fresh first — so this initial value is
  // never itself observable.
  const [removeError, setRemoveError] = useState('')

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    // Stryker disable next-line ConditionalExpression: an empty needle
    // matches every source via .includes('') below anyway, so skipping the
    // early return produces the exact same result — a genuine equivalent
    // mutant, not a gap in test coverage.
    if (!needle) return sources
    return sources.filter((source) =>
      `${source.title} ${source.path}`.toLowerCase().includes(needle),
    )
  }, [sources, query])

  function handleQueryChange(event: ChangeEvent<HTMLInputElement>) {
    setQuery(event.target.value)
  }

  async function handleAddClick() {
    setPickerError('')
    try {
      const path = await pickNotesFile()
      if (path) setImportPath(path)
    } catch {
      setPickerError(PICKER_ERROR_MESSAGE)
    }
  }

  function handleImportDialogClose() {
    setImportPath(null)
    reload()
  }

  function handleRemoveClick(source: SessionSource) {
    setRemoveError('')
    setRemoveTarget(source)
  }

  // No AlertDialogTrigger is ever rendered — this panel opens the
  // confirmation itself, via handleRemoveClick — so Radix only ever calls
  // this to report a close (Cancel or Escape), never an open. Closing
  // without confirming always just clears the target; a confirmed click
  // goes through handleConfirmRemove instead, which decides for itself
  // whether to close.
  function handleRemoveDialogOpenChange() {
    setRemoveTarget(null)
  }

  async function handleConfirmRemove() {
    // Stryker disable next-line ConditionalExpression: structurally
    // unreachable — the Remove button that calls this only ever renders
    // inside this same AlertDialog, which is only open while removeTarget
    // is set.
    if (!removeTarget) return
    setRemoving(true)
    setRemoveError('')
    try {
      await removeSessionSource(sessionId, removeTarget.itemId)
      setRemoveTarget(null)
      reload()
    } catch (err) {
      setRemoveError(err instanceof Error ? err.message : 'Failed to remove the source.')
    } finally {
      setRemoving(false)
    }
  }

  // Stryker disable next-line StringLiteral: only read when importPath is
  // null, i.e. IngestProgressDialog's own `open` prop is false — its effect
  // bails via `if (!open) return` before path ever drives anything
  // observable, so this fallback exists purely to satisfy the required
  // (non-optional) prop type.
  const importDialogPath = importPath ?? ''

  return (
    <>
      {/* Same raw background as the sidebar (app-shell.tsx's <nav>) — one
          step darker than --card, so this panel reads as chrome alongside
          the sidebar rather than a card floating in the chat's canvas. */}
      <div className="flex h-full w-full flex-col overflow-hidden bg-[oklch(0.115_0.014_50)]">
        <div className="flex flex-col gap-3 border-b border-border p-3">
          <div className="flex items-start justify-between gap-2">
            <div>
              <h2 className="font-heading text-base font-bold text-foreground">
                Sources ({sources.length})
              </h2>
              <p className="mt-0.5 text-[11px] text-muted-foreground">Imported into this session</p>
            </div>
            <AddSourceButton
              onClick={() => void handleAddClick()}
              disabled={mutationsDisabled}
              ariaLabel="Add source"
            >
              Add
            </AddSourceButton>
          </div>

          <label className="relative block">
            <span className="sr-only">Search sources</span>
            <Search
              className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              type="text"
              value={query}
              onChange={handleQueryChange}
              placeholder="Search sources"
              className="pl-8 text-xs"
            />
          </label>

          {pickerError && <p className="text-xs text-destructive">{pickerError}</p>}
        </div>

        <div className="thin-scroll flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto p-2">
          {loading ? (
            <p className="p-4 text-center text-xs text-muted-foreground">Loading sources…</p>
          ) : error ? (
            <div className="flex flex-col items-center gap-2 p-4 text-center">
              <p className="text-xs text-destructive">{error}</p>
              <Button size="sm" variant="outline" onClick={reload}>
                Retry
              </Button>
            </div>
          ) : filtered.length === 0 ? (
            sources.length === 0 ? (
              <div className="flex flex-col items-center gap-2 p-4 text-center">
                <p className="text-xs font-semibold text-foreground">No sources imported yet.</p>
                <p className="text-[11px] text-muted-foreground">
                  The chat searches these documents when answering.
                </p>
                <AddSourceButton
                  onClick={() => void handleAddClick()}
                  disabled={mutationsDisabled}
                  variant="outline"
                >
                  Add a source
                </AddSourceButton>
              </div>
            ) : (
              <p className="p-4 text-center text-xs text-muted-foreground">
                No sources match your search.
              </p>
            )
          ) : (
            filtered.map((source) => (
              <div
                key={source.itemId}
                data-slot="study-source-row"
                className="group flex items-start gap-2 rounded-md px-2 py-2 hover:bg-accent/40"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-xs font-semibold text-foreground">{source.title}</p>
                  <p className="truncate text-[11px] text-muted-foreground">
                    {source.path} · {chunkCountLabel(source.chunkCount)}
                  </p>
                </div>
                {mutationsDisabled ? (
                  <Tooltip>
                    {/* Same disabled-button-can't-trigger-a-tooltip issue as
                        AddSourceButton above — see its comment. */}
                    <TooltipTrigger asChild>
                      <span className="opacity-0 group-hover:opacity-100">
                        <Button
                          size="icon-sm"
                          variant="ghost"
                          disabled
                          aria-label={`${source.title} options`}
                        >
                          <MoreVertical className="size-3.5" aria-hidden="true" />
                        </Button>
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>Rebuilding knowledge index…</TooltipContent>
                  </Tooltip>
                ) : (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        size="icon-sm"
                        variant="ghost"
                        aria-label={`${source.title} options`}
                        className="opacity-0 group-hover:opacity-100"
                      >
                        <MoreVertical className="size-3.5" aria-hidden="true" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem
                        variant="destructive"
                        onClick={() => handleRemoveClick(source)}
                      >
                        <Trash2 className="size-3.5" aria-hidden="true" />
                        Remove
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
            ))
          )}
        </div>
      </div>
      <IngestProgressDialog
        open={importPath !== null}
        sessionId={sessionId}
        path={importDialogPath}
        onClose={handleImportDialogClose}
      />
      <AlertDialog open={removeTarget !== null} onOpenChange={handleRemoveDialogOpenChange}>
        {/* No onInteractOutside guard here: unlike Dialog, AlertDialog
            never closes on an outside click by design — only Cancel,
            Escape, or a confirmed Remove can close it. */}
        <AlertDialogContent onEscapeKeyDown={(event) => removing && event.preventDefault()}>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove {removeTarget?.title}?</AlertDialogTitle>
            <AlertDialogDescription>
              It will no longer be searched in this session. The file on your computer is not
              changed.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {removeError && <p className="text-sm text-destructive">{removeError}</p>}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removing}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              disabled={removing}
              onClick={(event) => {
                // AlertDialogAction auto-closes on click by default; this
                // panel instead drives open/close entirely off removeTarget,
                // so a failed removal can keep the confirmation open.
                event.preventDefault()
                void handleConfirmRemove()
              }}
            >
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

export { StudySourcesPanel }
