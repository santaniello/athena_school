import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ChallengeRow } from '@/components/challenge-row'
import { GenerateChallengeDialog } from '@/components/generate-challenge-dialog'
import { useChallengeCatalog } from '@/hooks/use-challenge-catalog'
import {
  DIFFICULTY_LABELS,
  STATUS_LABELS,
  difficultyForExperience,
  listChallengeTopics,
  type CatalogFilter,
  type ChallengeDifficulty,
  type ChallengeStatus,
  type GeneratedChallenge,
} from '@/lib/exercise'

interface ArchitectureLabScreenProps {
  // The profile's experience level, which preselects the generation difficulty.
  experienceLevel?: string
}

const ALL = 'all'

const NO_FILTER = { query: '', status: '', difficulty: '', topic: '' } as const

function plural(count: number, noun: string): string {
  return `${count} ${noun}${count === 1 ? '' : 's'}`
}

// The Architecture Lab's entry screen: the user's challenges with search,
// filters and progress at a glance, plus the Generate challenge with AI
// action. Everything shown, statuses and counts included, comes from the
// backend. Starting an exam arrives with the diagram editor. See
// specs/phases/phase-03-architecture-lab/07-challenge-catalog-ui.md.
function ArchitectureLabScreen({ experienceLevel }: ArchitectureLabScreenProps) {
  const [filter, setFilter] = useState<Required<CatalogFilter>>({ ...NO_FILTER })
  const { catalog, loading, error, reload } = useChallengeCatalog(filter)
  const [topics, setTopics] = useState<string[]>([])
  const [dialogOpen, setDialogOpen] = useState(false)
  const [highlightedId, setHighlightedId] = useState<string | null>(null)

  const filtersActive = Object.values(filter).some((value) => value !== '')

  function loadTopics() {
    listChallengeTopics()
      .then(setTopics)
      .catch(() => setTopics([]))
  }

  // Stryker disable next-line ArrayDeclaration: mount-once effect.
  useEffect(loadTopics, [])

  function handleGenerated(challenge: GeneratedChallenge) {
    setDialogOpen(false)
    setHighlightedId(challenge.id)
    // Filters that could hide the new challenge are cleared; a reload covers
    // the case where they were already clear.
    setFilter({ ...NO_FILTER })
    reload()
    loadTopics()
  }

  // The dialog was closed before the challenge arrived. The challenge is in
  // the catalog, so the list refreshes, but the filters and any dialog opened
  // since are left alone.
  function handleGeneratedAfterClose() {
    reload()
    loadTopics()
  }

  const items = catalog?.items ?? []

  return (
    <div className="flex min-h-0 w-full flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        {catalog ? (
          <p className="flex gap-4 text-sm text-muted-foreground">
            <span>{plural(catalog.total, 'challenge')}</span>
            <span>{catalog.completed} completed</span>
            <span>{catalog.inProgress} in progress</span>
          </p>
        ) : (
          <span />
        )}
        <Button type="button" onClick={() => setDialogOpen(true)}>
          Generate challenge with AI
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Input
          type="search"
          aria-label="Search challenges"
          placeholder="Search"
          className="max-w-64"
          value={filter.query}
          onChange={(event) => setFilter({ ...filter, query: event.target.value })}
        />
        <FilterSelect
          label="Status"
          allLabel="All statuses"
          value={filter.status}
          options={Object.entries(STATUS_LABELS) as [ChallengeStatus, string][]}
          onChange={(status) => setFilter({ ...filter, status })}
        />
        <FilterSelect
          label="Difficulty"
          allLabel="All difficulties"
          value={filter.difficulty}
          options={Object.entries(DIFFICULTY_LABELS) as [ChallengeDifficulty, string][]}
          onChange={(difficulty) => setFilter({ ...filter, difficulty })}
        />
        <FilterSelect
          label="Topic"
          allLabel="All topics"
          value={filter.topic}
          options={topics.map((topic): [string, string] => [topic, topic])}
          onChange={(topic) => setFilter({ ...filter, topic })}
        />
      </div>

      <div className="thin-scroll min-h-0 flex-1 overflow-y-auto">
        {error ? (
          <div role="alert" className="flex items-center gap-3 text-sm">
            <span className="text-destructive">{error}</span>
            <Button type="button" variant="outline" size="sm" onClick={reload}>
              Retry
            </Button>
          </div>
        ) : !catalog && loading ? (
          <p className="text-sm text-muted-foreground">Loading challenges…</p>
        ) : items.length > 0 ? (
          <ul className="divide-y divide-border rounded-lg border border-border">
            {items.map((item) => (
              <ChallengeRow key={item.id} item={item} highlighted={item.id === highlightedId} />
            ))}
          </ul>
        ) : filtersActive ? (
          <div className="flex flex-col items-center gap-2 py-10 text-center">
            <p className="text-sm text-muted-foreground">No challenges match your filters.</p>
            <Button type="button" variant="outline" onClick={() => setFilter({ ...NO_FILTER })}>
              Clear filters
            </Button>
          </div>
        ) : (
          <div className="flex flex-col items-center gap-2 py-10 text-center">
            <p className="text-sm font-semibold text-foreground">No challenges yet</p>
            <p className="max-w-80 text-sm text-muted-foreground">
              Study a topic, then generate one from a study session. Each challenge is built from
              what you studied.
            </p>
            <Button type="button" onClick={() => setDialogOpen(true)}>
              Generate your first challenge
            </Button>
          </div>
        )}
      </div>

      <GenerateChallengeDialog
        open={dialogOpen}
        defaultDifficulty={difficultyForExperience(experienceLevel)}
        onClose={() => setDialogOpen(false)}
        onGenerated={handleGenerated}
        onGeneratedAfterClose={handleGeneratedAfterClose}
      />
    </div>
  )
}

interface FilterSelectProps<T extends string> {
  label: string
  allLabel: string
  value: T | ''
  options: [T, string][]
  onChange: (value: T | '') => void
}

// A filter dropdown whose first option clears the filter. Radix cannot hold an
// empty item value, so "all" is stood in by a sentinel.
function FilterSelect<T extends string>({
  label,
  allLabel,
  value,
  options,
  onChange,
}: FilterSelectProps<T>) {
  return (
    <Select
      value={value === '' ? ALL : value}
      onValueChange={(next) => onChange(next === ALL ? '' : (next as T))}
    >
      <SelectTrigger aria-label={label}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>{allLabel}</SelectItem>
        {options.map(([optionValue, optionLabel]) => (
          <SelectItem key={optionValue} value={optionValue}>
            {optionLabel}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

export default ArchitectureLabScreen
