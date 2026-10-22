import type { KeyboardEvent, RefObject } from 'react'
import { ChevronDown, ChevronUp, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export interface FindBarProps {
  query: string
  matchCount: number
  // -1 when there is no match.
  activeIndex: number
  loadingHistory: boolean
  // Set when an earlier page failed: the search still covers what is loaded.
  historyError?: string | null
  inputRef: RefObject<HTMLInputElement | null>
  onQueryChange: (query: string) => void
  onNext: () => void
  onPrev: () => void
  onClose: () => void
}

function resultLabel(query: string, matchCount: number, activeIndex: number): string {
  if (query === '') return ''
  if (matchCount === 0) return 'No results'
  return `${activeIndex + 1} of ${matchCount}`
}

// FindBar is the Ctrl+F bar of the study chat. It is purely presentational:
// the search itself lives in useFindInPage.
export function FindBar({
  query,
  matchCount,
  activeIndex,
  loadingHistory,
  historyError,
  inputRef,
  onQueryChange,
  onNext,
  onPrev,
  onClose,
}: FindBarProps) {
  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Enter') {
      event.preventDefault()
      if (event.shiftKey) onPrev()
      else onNext()
    } else if (event.key === 'Escape') {
      event.preventDefault()
      onClose()
    }
  }

  const hasMatches = matchCount > 0
  return (
    <div
      role="search"
      className="flex shrink-0 items-center gap-2 rounded-lg border border-border bg-card px-2 py-1.5"
    >
      <Input
        ref={inputRef}
        value={query}
        onChange={(event) => onQueryChange(event.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="Find in conversation"
        aria-label="Find in conversation"
        className="h-7 flex-1"
      />
      <span aria-live="polite" className="min-w-16 text-right text-xs text-muted-foreground">
        {resultLabel(query, matchCount, activeIndex)}
      </span>
      {loadingHistory && <span className="text-xs text-muted-foreground">Loading history…</span>}
      {historyError && (
        <span className="text-xs text-destructive" title={historyError}>
          Could not load the full history
        </span>
      )}
      <Button
        size="icon-sm"
        variant="ghost"
        aria-label="Previous match"
        disabled={!hasMatches}
        onClick={onPrev}
      >
        <ChevronUp className="size-4" aria-hidden="true" />
      </Button>
      <Button
        size="icon-sm"
        variant="ghost"
        aria-label="Next match"
        disabled={!hasMatches}
        onClick={onNext}
      >
        <ChevronDown className="size-4" aria-hidden="true" />
      </Button>
      <Button size="icon-sm" variant="ghost" aria-label="Close find" onClick={onClose}>
        <X className="size-4" aria-hidden="true" />
      </Button>
    </div>
  )
}
