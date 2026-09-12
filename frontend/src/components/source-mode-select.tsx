import { CircleHelp } from 'lucide-react'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import type { SourceMode } from '@/lib/study'

interface SourceModeOption {
  value: SourceMode
  label: string
  description: string
}

// Descriptions spell out each policy — see
// specs/phases/phase-02-knowledge-engine/05-rag-integration.md.
const SOURCE_MODE_OPTIONS: SourceModeOption[] = [
  {
    value: 'notes',
    label: 'Notes',
    description:
      'Uses your approved local knowledge as the primary source, filling gaps with general knowledge when needed.',
  },
  {
    value: 'strict-notes',
    label: 'Strict notes',
    description:
      'Answers only from your approved local knowledge, and says so when it finds nothing relevant.',
  },
]

interface SourceModeSelectProps {
  value: SourceMode
  onValueChange: (value: SourceMode) => void
  disabled?: boolean
}

// The composer's source-mode selector: Notes or Strict notes, defaulting
// to Notes on every new or resumed chat. Disabled while a
// response streams. The dropdown lists plain labels only — what each mode
// means lives in the "?" tooltip beside it, not stacked under every item.
function SourceModeSelect({ value, onValueChange, disabled }: SourceModeSelectProps) {
  return (
    <div className="flex items-center gap-1.5">
      <Label htmlFor="source-mode-select" className="sr-only">
        Source mode
      </Label>
      <Select
        value={value}
        onValueChange={(next) => onValueChange(next as SourceMode)}
        disabled={disabled}
      >
        <SelectTrigger id="source-mode-select" size="sm" aria-label="Source mode">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SOURCE_MODE_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Tooltip>
        <TooltipTrigger aria-label="What do the source modes mean?">
          <CircleHelp className="size-4 text-muted-foreground" aria-hidden="true" />
        </TooltipTrigger>
        <TooltipContent className="flex flex-col gap-1.5 py-2">
          {SOURCE_MODE_OPTIONS.map((option) => (
            <p key={option.value}>
              <span className="font-semibold">{option.label}:</span> {option.description}
            </p>
          ))}
        </TooltipContent>
      </Tooltip>
    </div>
  )
}

export { SourceModeSelect }
