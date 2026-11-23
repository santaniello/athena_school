import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import type { MaterialList } from '@/hooks/use-session-materials'
import {
  difficultyLabel,
  formatSessionDate,
  formatTimeLimit,
  type SessionExerciseCard,
} from '@/lib/exercise'
import { formatMindMapDate, type MindMapSummary } from '@/lib/mindmap'
import { formatScore, quizDifficultyLabel, scoreBand, type QuizSummary } from '@/lib/quiz'
import { cn } from '@/lib/utils'

type Tone = 'good' | 'warn' | 'bad' | 'neutral'

const TONE_CLASSES: Record<Tone, string> = {
  good: 'border-emerald-500/50 text-emerald-400',
  warn: 'border-amber-500/50 text-amber-400',
  bad: 'border-destructive/60 text-destructive',
  neutral: 'border-border text-muted-foreground',
}

interface MaterialRowProps {
  title: string
  subtitle: string
  badge?: string
  tone?: Tone
  onClick: () => void
}

// One clickable row: icon-less, title and a one-line subtitle on the left, a
// status badge on the right.
function MaterialRow({ title, subtitle, badge, tone = 'neutral', onClick }: MaterialRowProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex w-full items-start gap-2 rounded-md px-2 py-2 text-left hover:bg-accent/40"
    >
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-semibold text-foreground">{title}</span>
        <span className="block truncate text-xs text-muted-foreground">{subtitle}</span>
      </span>
      {badge && (
        <span
          className={cn(
            'shrink-0 rounded-full border px-2 text-[11px] tabular-nums',
            TONE_CLASSES[tone],
          )}
        >
          {badge}
        </span>
      )}
    </button>
  )
}

interface ListShellProps<T> {
  list: MaterialList<T>
  emptyKey: 'mindMaps' | 'labs' | 'quizzes'
  children: ReactNode
}

// What every list shows before it has rows: loading, a failure with Retry or
// an empty message.
function ListShell<T>({ list, emptyKey, children }: ListShellProps<T>) {
  const { t } = useTranslation('materials')

  if (list.status === 'loading') {
    return <p className="p-4 text-center text-sm text-muted-foreground">{t('loading')}</p>
  }
  if (list.status === 'error') {
    return (
      <div className="flex flex-col items-center gap-2 p-4 text-center">
        <p className="text-sm text-destructive">{t('loadFailed')}</p>
        <Button size="sm" variant="outline" onClick={list.retry}>
          {t('retry')}
        </Button>
      </div>
    )
  }
  if (list.items.length === 0) {
    return <p className="p-4 text-center text-sm text-muted-foreground">{t(`empty.${emptyKey}`)}</p>
  }
  return <div className="flex flex-col gap-0.5">{children}</div>
}

interface ListProps<T> {
  list: MaterialList<T>
  onOpen: (item: T) => void
}

function MindMapList({ list, onOpen }: ListProps<MindMapSummary>) {
  const { t } = useTranslation('materials')
  return (
    <ListShell list={list} emptyKey="mindMaps">
      {list.items.map((mindMap) => (
        <MaterialRow
          key={mindMap.id}
          title={mindMap.title}
          subtitle={t('mindMap.updated', { date: formatMindMapDate(mindMap.updatedAt) })}
          onClick={() => onOpen(mindMap)}
        />
      ))}
    </ListShell>
  )
}

const SCORE_TONES = { low: 'bad', mid: 'warn', high: 'good' } as const

function QuizList({ list, onOpen }: ListProps<QuizSummary>) {
  const { t } = useTranslation('materials')
  return (
    <ListShell list={list} emptyKey="quizzes">
      {list.items.map((quiz, index) => {
        const taken = quiz.attemptCount > 0 && quiz.latestScore !== null
        const subtitle = [
          t('quiz.questions', { count: quiz.questionCount }),
          formatSessionDate(quiz.createdAt),
          taken ? t('quiz.attempts', { count: quiz.attemptCount }) : t('quiz.notTaken'),
        ].join(' · ')
        return (
          <MaterialRow
            key={quiz.id}
            title={t('quiz.title', {
              number: index + 1,
              difficulty: quizDifficultyLabel(quiz.difficulty),
            })}
            subtitle={subtitle}
            badge={taken ? formatScore(quiz.latestScore as number) : t('quiz.new')}
            tone={taken ? SCORE_TONES[scoreBand(quiz.latestScore as number)] : 'neutral'}
            onClick={() => onOpen(quiz)}
          />
        )
      })}
    </ListShell>
  )
}

function LabList({ list, onOpen }: ListProps<SessionExerciseCard>) {
  const { t } = useTranslation('materials')
  return (
    <ListShell list={list} emptyKey="labs">
      {list.items.map((card) => {
        const subtitle = [
          difficultyLabel(card.difficulty),
          formatTimeLimit(card.timeLimitMinutes),
          card.attempts > 0 ? t('lab.attempts', { count: card.attempts }) : t('lab.notAttempted'),
        ].join(' · ')
        const passed = card.latest?.outcome === 'PASSED'
        return (
          <MaterialRow
            key={card.sessionExerciseId}
            title={card.title}
            subtitle={subtitle}
            badge={card.latest ? (passed ? t('lab.passed') : t('lab.failed')) : t('lab.new')}
            tone={card.latest ? (passed ? 'good' : 'bad') : 'neutral'}
            onClick={() => onOpen(card)}
          />
        )
      })}
    </ListShell>
  )
}

export { LabList, MindMapList, QuizList }
