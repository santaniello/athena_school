import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { SourceMode } from '@/lib/study'

interface SourceModeOption {
  value: SourceMode
  label: string
  description: string
}

// Descriptions spell out each policy, and Web's explicitly disclaims live
// internet search — see
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
      'Answers only from your approved local knowledge, and says so when it cannot fully answer.',
  },
  {
    value: 'web',
    label: 'Web',
    description:
      "Ignores local sources. May use the model's general knowledge — not necessarily a live internet search.",
  },
]

interface SourceModeSelectProps {
  value: SourceMode
  onValueChange: (value: SourceMode) => void
  disabled?: boolean
}

// The composer's source-mode selector: Notes, Strict notes, or Web,
// defaulting to Notes on every new or resumed chat. Disabled while a
// response streams.
function SourceModeSelect({ value, onValueChange, disabled }: SourceModeSelectProps) {
  return (
    <div className="flex flex-col gap-1">
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
              <div className="flex flex-col py-0.5">
                <span>{option.label}</span>
                <span className="text-xs text-muted-foreground">{option.description}</span>
              </div>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

export { SourceModeSelect }
