import { useMemo, useState } from 'react'
import type { ChangeEvent } from 'react'
import { Plus, Search, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useSessionSources } from '@/hooks/use-session-sources'

interface StudySourcesPanelProps {
  sessionId: string
}

// chunkCountLabel pluralizes "chunk" for a row's subtitle — "1 chunk",
// "12 chunks".
function chunkCountLabel(chunkCount: number): string {
  return `${chunkCount} chunk${chunkCount === 1 ? '' : 's'}`
}

// The Study screen's NotebookLM-style right rail: the session's imported
// documents, from the moment each is imported (not a per-message dedupe of
// sources cited so far — see StudyChatScreen's per-message "Local sources"
// strip for that). Add/Remove are disabled this increment; wiring them is
// specs/phases/phase-02-knowledge-engine/17-session-sources-panel.md's next
// two slices.
function StudySourcesPanel({ sessionId }: StudySourcesPanelProps) {
  const { sources, loading, error, reload } = useSessionSources(sessionId)
  const [query, setQuery] = useState('')

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    if (!needle) return sources
    return sources.filter((source) =>
      `${source.title} ${source.path}`.toLowerCase().includes(needle),
    )
  }, [sources, query])

  function handleQueryChange(event: ChangeEvent<HTMLInputElement>) {
    setQuery(event.target.value)
  }

  return (
    // Same raw background as the sidebar (app-shell.tsx's <nav>) — one step
    // darker than --card, so this panel reads as chrome alongside the
    // sidebar rather than a card floating in the chat's canvas.
    <div className="flex h-full w-full flex-col overflow-hidden bg-[oklch(0.115_0.014_50)]">
      <div className="flex flex-col gap-3 border-b border-border p-3">
        <div className="flex items-start justify-between gap-2">
          <div>
            <h2 className="font-heading text-base font-bold text-foreground">
              Sources ({sources.length})
            </h2>
            <p className="mt-0.5 text-[11px] text-muted-foreground">Imported into this session</p>
          </div>
          <Tooltip>
            {/* A disabled <button> takes pointer-events: none (buttonVariants)
                and drops out of the tab order, so neither hover nor focus
                ever reaches it — the tooltip could never open with asChild
                directly on the Button. Wrapping it in a plain, enabled <span>
                gives the trigger something that actually receives those
                events. */}
            <TooltipTrigger asChild>
              <span>
                <Button size="sm" disabled aria-label="Add source">
                  <Plus className="size-3.5" aria-hidden="true" />
                  Add
                </Button>
              </span>
            </TooltipTrigger>
            <TooltipContent>Coming soon</TooltipContent>
          </Tooltip>
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
              <Tooltip>
                <TooltipTrigger asChild>
                  <span>
                    <Button size="sm" variant="outline" disabled>
                      <Plus className="size-3.5" aria-hidden="true" />
                      Add a source
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>Coming soon</TooltipContent>
              </Tooltip>
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
              <Tooltip>
                {/* Same disabled-button-can't-trigger-a-tooltip issue as
                    the Add button above — see its comment. */}
                <TooltipTrigger asChild>
                  <span className="opacity-0 group-hover:opacity-100">
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      disabled
                      aria-label={`Remove ${source.title}`}
                    >
                      <Trash2 className="size-3.5" aria-hidden="true" />
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>Coming soon</TooltipContent>
              </Tooltip>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

export { StudySourcesPanel }
