import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ChallengeRow } from '@/components/challenge-row'
import { AttemptHistoryDialog, type HistoryChallenge } from '@/components/attempt-history-dialog'
import { GenerateChallengeDialog } from '@/components/generate-challenge-dialog'
import { FilterSelect } from '@/components/lab/filter-select'
import { CanonicalInterviewTab } from '@/components/lab/canonical-interview-tab'
import { StartExamDialog, type ExamToStart } from '@/components/lab/start-exam-dialog'
import { useChallengeCatalog } from '@/hooks/use-challenge-catalog'
import {
  CHALLENGE_DIFFICULTIES,
  CHALLENGE_STATUSES,
  difficultyForExperience,
  difficultyLabel,
  statusLabel,
  listChallengeTopics,
  type CanonicalProblem,
  type CatalogFilter,
  type ChallengeDifficulty,
  type ChallengeStatus,
  type GeneratedChallenge,
} from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface ArchitectureLabScreenProps {
  // The profile's experience level, which preselects the generation difficulty.
  experienceLevel?: string
  // Which list the screen shows, chosen in the sidebar's Architecture Lab submenu.
  kind: 'INTERVIEW' | 'PRACTICE'
  // The user confirmed the rules of an exam; the shell opens the exam screen.
  onStartExam: (challenge: ExamToStart) => void
}

const NO_FILTER = { query: '', status: '', difficulty: '', topic: '', kind: 'PRACTICE' } as const

// The Architecture Lab's entry screen, in two views chosen in the sidebar. Interview lists the
// canonical problems, with no study session involved. Practice lists the
// user's practice challenges with search, filters and progress at a glance,
// plus the Generate practice action. Everything shown, statuses and counts
// included, comes from the backend. Start opens the exam rules; confirming
// them hands the challenge to the shell. See
// specs/phases/phase-03-architecture-lab/07-challenge-catalog-ui.md,
// 10-diagram-workspace.md and
// specs/phases/phase-07-improve-arch-lab/03-canonical-interviews.md.
function ArchitectureLabScreen({ experienceLevel, kind, onStartExam }: ArchitectureLabScreenProps) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState<Required<CatalogFilter>>({ ...NO_FILTER })
  const { catalog, loading, error, reload } = useChallengeCatalog(filter)
  const [topics, setTopics] = useState<string[]>([])
  const [dialogOpen, setDialogOpen] = useState(false)
  const [highlightedId, setHighlightedId] = useState<string | null>(null)
  const [pendingExam, setPendingExam] = useState<ExamToStart | null>(null)
  const [historyOf, setHistoryOf] = useState<HistoryChallenge | null>(null)

  const filtersActive =
    filter.query !== '' || filter.status !== '' || filter.difficulty !== '' || filter.topic !== ''

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
      {kind === 'INTERVIEW' ? (
        <CanonicalInterviewTab
          experienceLevel={experienceLevel}
          onReady={setPendingExam}
          onHistory={(problem: CanonicalProblem) =>
            setHistoryOf({ id: problem.id, title: problem.name, canonical: true })
          }
        />
      ) : (
        <>
          <div className="flex items-center justify-between gap-4">
            {catalog ? (
              <p className="flex gap-4 text-sm text-muted-foreground">
                <span>{t('catalog.challenges', { count: catalog.total })}</span>
                <span>{t('catalog.completed', { count: catalog.completed })}</span>
                <span>{t('catalog.inProgress', { count: catalog.inProgress })}</span>
              </p>
            ) : (
              <span />
            )}
            <Button type="button" onClick={() => setDialogOpen(true)}>
              {t('catalog.generate')}
            </Button>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Input
              type="search"
              aria-label={t('catalog.searchLabel')}
              placeholder={t('catalog.searchPlaceholder')}
              className="max-w-64"
              value={filter.query}
              onChange={(event) => setFilter({ ...filter, query: event.target.value })}
            />
            <FilterSelect
              label={t('catalog.status')}
              allLabel={t('catalog.allStatuses')}
              value={filter.status}
              options={CHALLENGE_STATUSES.map((value): [ChallengeStatus, string] => [
                value,
                statusLabel(value),
              ])}
              onChange={(status) => setFilter({ ...filter, status })}
            />
            <FilterSelect
              label={t('catalog.difficulty')}
              allLabel={t('catalog.allDifficulties')}
              value={filter.difficulty}
              options={CHALLENGE_DIFFICULTIES.map((value): [ChallengeDifficulty, string] => [
                value,
                difficultyLabel(value),
              ])}
              onChange={(difficulty) => setFilter({ ...filter, difficulty })}
            />
            <FilterSelect
              label={t('catalog.topic')}
              allLabel={t('catalog.allTopics')}
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
                  {t('common.retry')}
                </Button>
              </div>
            ) : !catalog && loading ? (
              <p className="text-sm text-muted-foreground">{t('catalog.loading')}</p>
            ) : items.length > 0 ? (
              <ul className="divide-y divide-border rounded-lg border border-border">
                {items.map((item) => (
                  <ChallengeRow
                    key={item.id}
                    item={item}
                    highlighted={item.id === highlightedId}
                    onStart={setPendingExam}
                    onHistory={setHistoryOf}
                  />
                ))}
              </ul>
            ) : filtersActive ? (
              <div className="flex flex-col items-center gap-2 py-10 text-center">
                <p className="text-sm text-muted-foreground">{t('catalog.noMatch')}</p>
                <Button type="button" variant="outline" onClick={() => setFilter({ ...NO_FILTER })}>
                  {t('catalog.clearFilters')}
                </Button>
              </div>
            ) : (
              <div className="flex flex-col items-center gap-2 py-10 text-center">
                <p className="text-sm font-semibold text-foreground">{t('catalog.emptyTitle')}</p>
                <p className="max-w-80 text-sm text-muted-foreground">{t('catalog.emptyBody')}</p>
                <Button type="button" onClick={() => setDialogOpen(true)}>
                  {t('catalog.generateFirst')}
                </Button>
              </div>
            )}
          </div>
        </>
      )}

      <GenerateChallengeDialog
        open={dialogOpen}
        defaultDifficulty={difficultyForExperience(experienceLevel)}
        onClose={() => setDialogOpen(false)}
        onGenerated={handleGenerated}
        onGeneratedAfterClose={handleGeneratedAfterClose}
      />

      <AttemptHistoryDialog challenge={historyOf} onClose={() => setHistoryOf(null)} />

      <StartExamDialog
        challenge={pendingExam}
        onCancel={() => setPendingExam(null)}
        onConfirm={(challenge) => {
          setPendingExam(null)
          onStartExam(challenge)
        }}
      />
    </div>
  )
}

export default ArchitectureLabScreen
