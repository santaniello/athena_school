import { useMemo, useState } from 'react'
import type { ChangeEvent } from 'react'
import { Plus, Search, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { sourceLabel, type StudySource } from '@/lib/study'
import { sourceKey } from '@/lib/study-source-key'

interface StudySourcesPanelProps {
  // Every Source that has backed a reply anywhere in the open session so
  // far (deduplicated by StudyChatScreen) — not yet the session-owned
  // source list described in
  // specs/phases/phase-02-knowledge-engine/14-study-sources-panel.md's
  // "Deferred to a future increment", which requires a session/document
  // link this phase deliberately does not add.
  sources: StudySource[]
}

// The Study screen's NotebookLM-style right rail: what LocalSourcesStrip
// shows per-message, promoted to a persistent, searchable list for the
// whole session. Add/Remove are intentionally disabled this phase — see
// specs/phases/phase-02-knowledge-engine/14-study-sources-panel.md for why
// a real attach/detach needs a session owner this increment deliberately
// does not build yet.
function StudySourcesPanel({ sources }: StudySourcesPanelProps) {
  const [query, setQuery] = useState('')

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    if (!needle) return sources
    return sources.filter((source) => {
      const { title, subtitle } = sourceLabel(source)
      return `${title} ${subtitle}`.toLowerCase().includes(needle)
    })
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
            <h2 className="font-heading text-base font-bold text-foreground">Sources</h2>
            <p className="mt-0.5 text-[11px] text-muted-foreground">Attached to this session</p>
          </div>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button size="sm" disabled aria-label="Add source">
                <Plus className="size-3.5" aria-hidden="true" />
                Add
              </Button>
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
        {filtered.length === 0 ? (
          <p className="p-4 text-center text-xs text-muted-foreground">
            {sources.length === 0
              ? 'No sources cited in this session yet.'
              : 'No sources match your search.'}
          </p>
        ) : (
          filtered.map((source) => {
            const { title, subtitle } = sourceLabel(source)
            return (
              <div
                key={sourceKey(source)}
                data-slot="study-source-row"
                className="group flex items-start gap-2 rounded-md px-2 py-2 hover:bg-accent/40"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-xs font-semibold text-foreground">{title}</p>
                  {subtitle && (
                    <p className="truncate text-[11px] text-muted-foreground">{subtitle}</p>
                  )}
                </div>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      disabled
                      aria-label={`Remove ${title}`}
                      className="opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
                    >
                      <Trash2 className="size-3.5" aria-hidden="true" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Coming soon</TooltipContent>
                </Tooltip>
              </div>
            )
          })
        )}
      </div>

      <div className="shrink-0 border-t border-border px-3 py-2 text-[11px] text-muted-foreground">
        Sources cited in a reply appear here automatically.
      </div>
    </div>
  )
}

export { StudySourcesPanel }
