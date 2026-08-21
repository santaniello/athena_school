import { useState } from 'react'
import { ChevronDown, ChevronUp } from 'lucide-react'
import type { StudySource } from '@/lib/study'

interface LocalSourcesStripProps {
  sources: StudySource[]
}

// Formats a cosine similarity as a fixed decimal — never a confidence
// percentage, so it can't be mistaken for one.
function formatScore(score: number): string {
  return score.toFixed(2)
}

// Maps a Source to its display title/subtitle per
// specs/phases/phase-02-knowledge-engine/05-rag-integration.md's three
// source-type label variants.
function sourceLabel(source: StudySource): { title: string; subtitle: string } {
  switch (source.sourceType) {
    case 'user_note':
      return { title: 'User note', subtitle: source.concept }
    case 'athena':
      return { title: 'Athena Knowledge', subtitle: source.concept }
    default:
      return { title: source.filePath, subtitle: source.heading }
  }
}

// A collapsed "Local sources (N)" strip shown under a completed assistant
// bubble, expanding to the exact post-cap sources in retrieval order.
// Renders nothing when there are no sources — calling it "Local sources"
// (rather than "citations") makes clear a `notes` answer may also draw on
// the model's general knowledge.
function LocalSourcesStrip({ sources }: LocalSourcesStripProps) {
  const [expanded, setExpanded] = useState(false)
  if (sources.length === 0) return null

  return (
    <div data-slot="local-sources-strip" className="mt-1 max-w-[75%] self-start text-xs">
      <button
        type="button"
        onClick={() => setExpanded((previous) => !previous)}
        aria-expanded={expanded}
        className="flex cursor-pointer items-center gap-1 text-muted-foreground hover:text-foreground"
      >
        {expanded ? (
          <ChevronUp className="size-3" aria-hidden="true" />
        ) : (
          <ChevronDown className="size-3" aria-hidden="true" />
        )}
        Local sources ({sources.length})
      </button>
      {expanded && (
        <ul className="mt-1 flex flex-col gap-1 pl-4">
          {sources.map((source, index) => {
            const { title, subtitle } = sourceLabel(source)
            return (
              <li key={index} className="text-muted-foreground">
                <span className="font-medium text-foreground">{title}</span>
                {subtitle && <span> · {subtitle}</span>}
                <span> · {formatScore(source.score)}</span>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}

export { LocalSourcesStrip }
