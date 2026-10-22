export const MATCH_HIGHLIGHT_NAME = 'find-match'
export const ACTIVE_HIGHLIGHT_NAME = 'find-match-active'

// The CSS Custom Highlight API is missing from older webviews and from jsdom;
// without it the find bar still counts and navigates, only without colour.
function registry(): HighlightRegistry | undefined {
  return typeof CSS === 'undefined' ? undefined : CSS.highlights
}

export function clearFindHighlights(): void {
  const highlights = registry()
  highlights?.delete(MATCH_HIGHLIGHT_NAME)
  highlights?.delete(ACTIVE_HIGHLIGHT_NAME)
}

// paintFindHighlights colours every range and the one at activeIndex apart.
export function paintFindHighlights(ranges: Range[], activeIndex: number): void {
  const highlights = registry()
  if (!highlights) return
  clearFindHighlights()
  if (ranges.length === 0) return
  highlights.set(
    MATCH_HIGHLIGHT_NAME,
    new Highlight(...ranges.filter((_, index) => index !== activeIndex)),
  )
  const active = ranges[activeIndex]
  if (active) highlights.set(ACTIVE_HIGHLIGHT_NAME, new Highlight(active))
}
