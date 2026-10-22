// The divider shown in the transcript right after the last message folded into
// the session's summary. Orange, like the other context notices, so the cut
// stands out while scrolling. The summary itself is opened from the session
// header. See specs/phases/phase-02-knowledge-engine/28-compaction-ux.md.
function SummaryMarker() {
  return (
    <div className="flex shrink-0 items-center gap-3 text-xs">
      <span className="h-px flex-1 bg-orange-600/60 dark:bg-orange-400/60" aria-hidden="true" />
      <span className="text-orange-600 dark:text-orange-400">Earlier messages were summarized</span>
      <span className="h-px flex-1 bg-orange-600/60 dark:bg-orange-400/60" aria-hidden="true" />
    </div>
  )
}

export { SummaryMarker }
