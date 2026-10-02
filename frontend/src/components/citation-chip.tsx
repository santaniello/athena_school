import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { sourceLabel, type StudySource } from '@/lib/study'

interface CitationChipProps {
  // 1-based, same index into sources as the "Local sources" strip uses.
  citationIndex: number
  // The reply's full source list; an out-of-range index renders plain text
  // instead of a chip, so a model-invented number or a stale [n] in an old
  // reply never links anywhere.
  sources: StudySource[]
  // Omitted while streaming — a chip still resolves and previews on hover,
  // it just can't be clicked yet.
  onOpenCitation?: (source: StudySource) => void
}

const EXCERPT_PREVIEW_LENGTH = 300

function excerptPreview(excerpt: string): string {
  if (excerpt.length <= EXCERPT_PREVIEW_LENGTH) return excerpt
  return `${excerpt.slice(0, EXCERPT_PREVIEW_LENGTH)}…`
}

// CitationChip renders one inline [n] citation marker as a small
// hoverable/clickable superscript. See
// specs/phases/phase-02-knowledge-engine/18-01-inline-citations-in-chat.md.
function CitationChip({ citationIndex, sources, onOpenCitation }: CitationChipProps) {
  const source = sources[citationIndex - 1]
  if (!source) return <>{`[${citationIndex}]`}</>

  const { title, subtitle } = sourceLabel(source)

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <sup>
          <button
            type="button"
            data-slot="citation-chip"
            onClick={() => onOpenCitation?.(source)}
            className="mx-0.5 cursor-pointer rounded bg-primary/15 px-1 text-[0.7em] font-semibold text-primary hover:bg-primary/25"
          >
            {citationIndex}
          </button>
        </sup>
      </TooltipTrigger>
      <TooltipContent className="max-w-xs">
        <p className="font-semibold">{title}</p>
        {subtitle && <p className="text-muted-foreground">{subtitle}</p>}
        <p className="mt-1 text-muted-foreground">{excerptPreview(source.excerpt)}</p>
      </TooltipContent>
    </Tooltip>
  )
}

export { CitationChip }
