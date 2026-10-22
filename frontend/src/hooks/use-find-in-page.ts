import { useCallback, useEffect, useRef, useState, type RefObject } from 'react'
import { clearFindHighlights, paintFindHighlights } from '@/lib/find-highlight'
import { findTextRanges } from '@/lib/find-in-page'

// Content changes (a streamed chunk, a prepended page) are coalesced so the
// text is not re-scanned on every mutation.
const RECOMPUTE_DEBOUNCE_MS = 100

export interface UseFindInPageOptions {
  containerRef: RefObject<HTMLElement | null>
  // Called when the user jumps to a match, so the screen can stop following
  // the bottom of the transcript.
  onNavigate?: () => void
}

export interface UseFindInPageResult {
  isOpen: boolean
  query: string
  matchCount: number
  // -1 when there is no match.
  activeIndex: number
  inputRef: RefObject<HTMLInputElement | null>
  open: () => void
  close: () => void
  setQuery: (query: string) => void
  next: () => void
  prev: () => void
}

// useFindInPage owns the state of the find bar: it scans the container's text
// for the query, paints the matches through the Custom Highlight API and moves
// between them. Everything happens in the DOM layer, so the message bubbles
// never re-render because of a search.
export function useFindInPage({
  containerRef,
  onNavigate,
}: UseFindInPageOptions): UseFindInPageResult {
  const [isOpen, setIsOpen] = useState(false)
  const [query, setQueryState] = useState('')
  const [matchCount, setMatchCount] = useState(0)
  const [activeIndex, setActiveIndex] = useState(-1)
  const [focusTick, setFocusTick] = useState(0)

  const inputRef = useRef<HTMLInputElement | null>(null)
  const queryRef = useRef('')
  const rangesRef = useRef<Range[]>([])
  const activeRef = useRef(-1)
  const onNavigateRef = useRef(onNavigate)

  useEffect(() => {
    onNavigateRef.current = onNavigate
  }, [onNavigate])

  const commit = useCallback((ranges: Range[], index: number) => {
    rangesRef.current = ranges
    activeRef.current = index
    setMatchCount(ranges.length)
    setActiveIndex(index)
    paintFindHighlights(ranges, index)
  }, [])

  // Re-scans the container. The active match is kept when it still exists
  // (same text node and offset), so a page prepended above does not move it.
  const recompute = useCallback(() => {
    const container = containerRef.current
    const ranges = container ? findTextRanges(container, queryRef.current) : []
    if (ranges.length === 0) {
      commit([], -1)
      return
    }
    const previous = rangesRef.current[activeRef.current]
    const kept = previous
      ? ranges.findIndex(
          (range) =>
            range.startContainer === previous.startContainer &&
            range.startOffset === previous.startOffset,
        )
      : -1
    commit(ranges, kept !== -1 ? kept : Math.min(Math.max(activeRef.current, 0), ranges.length - 1))
  }, [containerRef, commit])

  const setQuery = useCallback(
    (value: string) => {
      queryRef.current = value
      setQueryState(value)
      // A new term always starts at its first match.
      rangesRef.current = []
      activeRef.current = -1
      recompute()
    },
    [recompute],
  )

  const open = useCallback(() => {
    setIsOpen(true)
    setFocusTick((tick) => tick + 1)
  }, [])

  const close = useCallback(() => {
    setIsOpen(false)
    queryRef.current = ''
    setQueryState('')
    rangesRef.current = []
    activeRef.current = -1
    setMatchCount(0)
    setActiveIndex(-1)
    clearFindHighlights()
  }, [])

  const move = useCallback(
    (delta: number) => {
      const ranges = rangesRef.current
      if (ranges.length === 0) return
      const index = (activeRef.current + delta + ranges.length) % ranges.length
      commit(ranges, index)
      ranges[index].startContainer.parentElement?.scrollIntoView({ block: 'center' })
      onNavigateRef.current?.()
    },
    [commit],
  )
  const next = useCallback(() => move(1), [move])
  const prev = useCallback(() => move(-1), [move])

  useEffect(() => {
    if (!isOpen) return
    inputRef.current?.focus()
    inputRef.current?.select()
  }, [isOpen, focusTick])

  useEffect(() => {
    const container = containerRef.current
    if (!isOpen || !container) return
    let timer: ReturnType<typeof setTimeout> | undefined
    const observer = new MutationObserver(() => {
      clearTimeout(timer)
      timer = setTimeout(recompute, RECOMPUTE_DEBOUNCE_MS)
    })
    observer.observe(container, { childList: true, subtree: true, characterData: true })
    return () => {
      clearTimeout(timer)
      observer.disconnect()
    }
  }, [isOpen, containerRef, recompute])

  useEffect(() => clearFindHighlights, [])

  return { isOpen, query, matchCount, activeIndex, inputRef, open, close, setQuery, next, prev }
}
