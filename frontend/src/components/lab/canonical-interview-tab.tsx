import { useCallback, useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { CanonicalProblemRow } from '@/components/lab/canonical-problem-row'
import { StartCanonicalDialog } from '@/components/lab/start-canonical-dialog'
import {
  difficultyForExperience,
  listCanonicalProblems,
  type CanonicalProblem,
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

  if (failed) {
    return (
      <div role="alert" className="flex items-center gap-3 text-sm">
        <span className="text-destructive">{t('canonical.loadFailed')}</span>
        <Button type="button" variant="outline" size="sm" onClick={reload}>
          {t('common.retry')}
        </Button>
      </div>
    )
  }
  if (!problems) return <p className="text-sm text-muted-foreground">{t('canonical.loading')}</p>
  return (
    <>
      <ul className="divide-y divide-border rounded-lg border border-border">
        {problems.map((problem) => (
          <CanonicalProblemRow
            key={problem.id}
            problem={problem}
            onStart={setStarting}
            onHistory={onHistory}
          />
        ))}
      </ul>
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
