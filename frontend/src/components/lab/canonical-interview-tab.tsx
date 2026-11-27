import { useCallback, useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { FilterSelect } from '@/components/lab/filter-select'
import { CanonicalProblemRow } from '@/components/lab/canonical-problem-row'
import { StartCanonicalDialog } from '@/components/lab/start-canonical-dialog'
import {
  CHALLENGE_STATUSES,
  difficultyForExperience,
  statusLabel,
  listCanonicalProblems,
  type CanonicalProblem,
  type ChallengeStatus,
  type ExamToStart,
} from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface CanonicalInterviewTabProps {
  // The profile's experience level, which preselects the difficulty.
  experienceLevel?: string
  // The challenge is ready: the screen shows the exam rules.
  onReady: (exam: ExamToStart) => void
  // History was clicked on a problem.
  onHistory: (problem: CanonicalProblem) => void
}

// The Interview tab: the fixed list of canonical problems, read with no model
// call and no generation. See
// specs/phases/phase-07-improve-arch-lab/03-canonical-interviews.md.
function CanonicalInterviewTab({
  experienceLevel,
  onReady,
  onHistory,
}: CanonicalInterviewTabProps) {
  const { t } = useTranslation()
  const [problems, setProblems] = useState<CanonicalProblem[] | null>(null)
  const [failed, setFailed] = useState(false)
  const [reloadToken, setReloadToken] = useState(0)
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState<ChallengeStatus | ''>('')
  const [starting, setStarting] = useState<CanonicalProblem | null>(null)

  // Stryker disable BooleanLiteral,BlockStatement: the flag only keeps a late answer from updating an unmounted tab.
  useEffect(() => {
    let stale = false
    listCanonicalProblems()
      .then((list) => {
        if (stale) return
        setProblems(list)
        setFailed(false)
      })
      .catch(() => {
        if (!stale) setFailed(true)
      })
    return () => {
      stale = true
    }
  }, [reloadToken])
  // Stryker restore BooleanLiteral,BlockStatement

  const reload = useCallback(() => setReloadToken((token) => token + 1), [])

  const hasStatus = (problem: CanonicalProblem, wanted: ChallengeStatus) =>
    problem.difficulties.some((state) => state.generated && state.status === wanted)
  const needle = query.trim().toLowerCase()
  const visible = (problems ?? []).filter(
    (problem) =>
      (needle === '' ||
        problem.name.toLowerCase().includes(needle) ||
        problem.summary.toLowerCase().includes(needle)) &&
      (status === '' || hasStatus(problem, status)),
  )
  const filtersActive = needle !== '' || status !== ''

  return (
    <>
      <div className="flex items-center justify-between gap-4">
        {problems ? (
          <p className="flex gap-4 text-sm text-muted-foreground">
            <span>{t('canonical.problems', { count: problems.length })}</span>
            <span>
              {t('catalog.completed', {
                count: problems.filter((problem) => hasStatus(problem, 'COMPLETED')).length,
              })}
            </span>
            <span>
              {t('catalog.inProgress', {
                count: problems.filter((problem) => hasStatus(problem, 'IN_PROGRESS')).length,
              })}
            </span>
          </p>
        ) : (
          <span />
        )}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Input
          type="search"
          aria-label={t('canonical.searchLabel')}
          placeholder={t('catalog.searchPlaceholder')}
          className="max-w-64"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <FilterSelect
          label={t('catalog.status')}
          allLabel={t('catalog.allStatuses')}
          value={status}
          options={CHALLENGE_STATUSES.map((value): [ChallengeStatus, string] => [
            value,
            statusLabel(value),
          ])}
          onChange={setStatus}
        />
      </div>

      <div className="thin-scroll min-h-0 flex-1 overflow-y-auto">
        {failed ? (
          <div role="alert" className="flex items-center gap-3 text-sm">
            <span className="text-destructive">{t('canonical.loadFailed')}</span>
            <Button type="button" variant="outline" size="sm" onClick={reload}>
              {t('common.retry')}
            </Button>
          </div>
        ) : !problems ? (
          <p className="text-sm text-muted-foreground">{t('canonical.loading')}</p>
        ) : visible.length > 0 ? (
          <ul className="divide-y divide-border rounded-lg border border-border">
            {visible.map((problem) => (
              <CanonicalProblemRow
                key={problem.id}
                problem={problem}
                onStart={setStarting}
                onHistory={onHistory}
              />
            ))}
          </ul>
        ) : (
          <div className="flex flex-col items-center gap-2 py-10 text-center">
            <p className="text-sm text-muted-foreground">{t('catalog.noMatch')}</p>
            {filtersActive && (
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setQuery('')
                  setStatus('')
                }}
              >
                {t('catalog.clearFilters')}
              </Button>
            )}
          </div>
        )}
      </div>

      <StartCanonicalDialog
        problem={starting}
        defaultDifficulty={difficultyForExperience(experienceLevel)}
        onClose={() => setStarting(null)}
        onReady={(exam) => {
          setStarting(null)
          reload()
          onReady(exam)
        }}
        onGeneratedAfterClose={reload}
      />
    </>
  )
}

export { CanonicalInterviewTab }
