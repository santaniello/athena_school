import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { AlertCircle, BookOpen, FolderOpen, Layers, TrendingUp } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ProgressTopicDetail } from '@/components/progress-topic-detail'
import { StartExamDialog, type ExamToStart } from '@/components/lab/start-exam-dialog'
import { cn } from '@/lib/utils'
import {
  getProgressDashboard,
  type ProgressDashboard,
  type ProgressChallengeResult,
  type ProgressFolder,
  type ProgressTopic,
  type TopicStatus,
} from '@/lib/progress'

type Dashboard =
  { state: 'loading' } | { state: 'failed' } | { state: 'ready'; data: ProgressDashboard }

const STATUS_STYLE: Record<TopicStatus, string> = {
  INSUFFICIENT_EVIDENCE: 'border-border bg-muted text-muted-foreground',
  NEEDS_REINFORCEMENT: 'border-[#e8875a]/40 bg-[#e8875a]/10 text-[#e8875a]',
  IMPROVING: 'border-[#5fbf8f]/40 bg-[#5fbf8f]/10 text-[#5fbf8f]',
  STABLE: 'border-primary/40 bg-primary/10 text-primary',
}

interface ProgressScreenProps {
  onStartQuiz: (quizId: string) => void
  onStartExam: (exam: ExamToStart) => void
}

// The Progress section: every folder that has study sessions, and each session
// as a topic with its latest quiz and Arch Lab scores and a status. All of it is
// derived by the backend on read, with no LLM call; selecting a topic
// shows its detail on the right (spec 5.2); a quiz opens the quiz and a challenge shows the exam
// rules first, like the session materials panel. See
// specs/phases/phase-05-progress/01-progress-dashboard.md.
function ProgressScreen({ onStartQuiz, onStartExam }: ProgressScreenProps) {
  const { t } = useTranslation('progressTab')
  const [dashboard, setDashboard] = useState<Dashboard>({ state: 'loading' })
  // Bumping this reloads the dashboard.
  const [reloadToken, setReloadToken] = useState(0)
  const [folderId, setFolderId] = useState<string | null>(null)
  const [topicId, setTopicId] = useState<string | null>(null)
  // The rules of the exam of the challenge that was clicked; null keeps them closed.
  const [examRules, setExamRules] = useState<ExamToStart | null>(null)

  useEffect(() => {
    let stale = false
    getProgressDashboard()
      .then((data) => {
        if (!stale) setDashboard({ state: 'ready', data })
      })
      .catch(() => {
        if (!stale) setDashboard({ state: 'failed' })
      })
    return () => {
      stale = true
    }
  }, [reloadToken])

  function retry() {
    setDashboard({ state: 'loading' })
    setReloadToken((token) => token + 1)
  }

  function pickFolder(id: string) {
    setFolderId(id)
    setTopicId(null)
  }

  const selectedFolder =
    dashboard.state === 'ready'
      ? (dashboard.data.folders.find((folder) => folder.id === folderId) ??
        dashboard.data.folders[0])
      : undefined

  return (
    <div className="thin-scroll mx-auto flex w-full max-w-6xl flex-col gap-6 overflow-y-auto pr-2">
      <header className="flex items-center gap-4 border-b border-border pb-5">
        <div className="flex size-13 items-center justify-center rounded-xl bg-muted">
          <TrendingUp className="size-6 text-primary" aria-hidden="true" />
        </div>
        <div>
          <h2 className="font-heading text-3xl font-bold text-foreground">{t('title')}</h2>
          <p className="text-sm text-muted-foreground">{t('subtitle')}</p>
        </div>
      </header>

      {dashboard.state === 'loading' && (
        <p className="text-sm text-muted-foreground">{t('loading')}</p>
      )}

      {dashboard.state === 'failed' && (
        <div role="alert" className="flex flex-col items-start gap-3">
          <p className="flex items-center gap-2 text-sm text-destructive">
            <AlertCircle className="size-4" aria-hidden="true" />
            {t('loadFailed')}
          </p>
          <Button variant="outline" onClick={retry}>
            {t('retry')}
          </Button>
        </div>
      )}

      {dashboard.state === 'ready' && !selectedFolder && (
        <div className="m-auto flex flex-col items-center gap-2 py-12 text-center">
          <BookOpen className="size-8 text-muted-foreground" aria-hidden="true" />
          <p className="text-sm font-semibold text-foreground">{t('empty.title')}</p>
          <p className="max-w-80 text-sm text-muted-foreground">{t('empty.description')}</p>
        </div>
      )}

      {dashboard.state === 'ready' && selectedFolder && (
        <>
          <section className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <Counter
              icon={<FolderOpen aria-hidden="true" />}
              value={dashboard.data.counters.folders}
              label={t('counters.folders')}
            />
            <Counter
              icon={<Layers aria-hidden="true" />}
              value={dashboard.data.counters.topics}
              label={t('counters.topics')}
            />
            <Counter
              icon={<AlertCircle aria-hidden="true" />}
              value={dashboard.data.counters.topicsToReview}
              label={t('counters.toReview')}
              warning
            />
          </section>

          <section className="flex flex-col gap-5 rounded-2xl border border-border bg-card p-5">
            <h2 className="text-lg font-extrabold text-foreground">{t('folders.title')}</h2>
            <div className="flex flex-wrap gap-2.5">
              {dashboard.data.folders.map((folder) => (
                <FolderChip
                  key={folder.id}
                  folder={folder}
                  selected={folder.id === selectedFolder.id}
                  onPick={() => pickFolder(folder.id)}
                />
              ))}
            </div>

            <div className="border-t border-border pt-4">
              <h3 className="font-heading text-xl font-bold text-foreground">
                {selectedFolder.name}
              </h3>
              <p className="text-xs text-muted-foreground">{t('topics.hint')}</p>
            </div>

            <div className="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
              <div className="flex flex-col gap-3">
                <ul className="flex flex-col gap-2.5">
                  {selectedFolder.topics.map((topic) => (
                    <li key={topic.sessionId}>
                      <TopicRow
                        topic={topic}
                        selected={topic.sessionId === topicId}
                        onPick={() => setTopicId(topic.sessionId)}
                      />
                    </li>
                  ))}
                </ul>
                <p className="text-xs text-muted-foreground">{t('topics.note')}</p>
              </div>
              <aside
                aria-label={t('detail.title')}
                className="rounded-xl border border-border p-4 xl:self-start"
              >
                {topicId ? (
                  <ProgressTopicDetail
                    sessionId={topicId}
                    onOpenQuiz={onStartQuiz}
                    onOpenChallenge={(challenge) => setExamRules(toExam(challenge))}
                  />
                ) : (
                  <p className="text-sm text-muted-foreground">{t('detail.placeholder')}</p>
                )}
              </aside>
            </div>
          </section>
        </>
      )}

      <StartExamDialog
        challenge={examRules}
        onCancel={() => setExamRules(null)}
        onConfirm={(exam) => {
          setExamRules(null)
          onStartExam(exam)
        }}
      />
    </div>
  )
}

// What the exam rules dialog and the exam need of a challenge.
function toExam(challenge: ProgressChallengeResult): ExamToStart {
  return {
    id: challenge.challengeId,
    title: challenge.title,
    difficulty: challenge.difficulty,
    timeLimitMinutes: challenge.timeLimitMinutes,
  }
}

function Counter({
  icon,
  value,
  label,
  warning = false,
}: {
  icon: ReactNode
  value: number
  label: string
  warning?: boolean
}) {
  return (
    <div className="flex items-center gap-4 rounded-xl border border-border bg-card px-5 py-4">
      <span className={cn('size-7 [&>svg]:size-7', warning ? 'text-[#e8875a]' : 'text-primary')}>
        {icon}
      </span>
      <div>
        <div className="text-3xl leading-none font-extrabold text-foreground">{value}</div>
        <div className="mt-1 text-sm text-muted-foreground">{label}</div>
      </div>
    </div>
  )
}

function FolderChip({
  folder,
  selected,
  onPick,
}: {
  folder: ProgressFolder
  selected: boolean
  onPick: () => void
}) {
  const { t } = useTranslation('progressTab')
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onPick}
      className={cn(
        'flex cursor-pointer items-center gap-2.5 rounded-xl border px-3.5 py-2.5 text-left transition-colors',
        selected
          ? 'border-primary bg-primary/10 text-primary'
          : 'border-border text-muted-foreground hover:bg-accent',
      )}
    >
      <FolderOpen className="size-5 shrink-0" aria-hidden="true" />
      <span className="flex flex-col">
        <span className="text-sm font-extrabold text-foreground">{folder.name}</span>
        <span className="text-xs text-muted-foreground">
          {t('folders.topics', { count: folder.topicCount })}
        </span>
      </span>
    </button>
  )
}

function TopicRow({
  topic,
  selected,
  onPick,
}: {
  topic: ProgressTopic
  selected: boolean
  onPick: () => void
}) {
  const { t } = useTranslation('progressTab')
  const score = (value: number | null) => (value === null ? t('topics.noScore') : `${value}%`)
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onPick}
      className={cn(
        'flex w-full cursor-pointer items-center justify-between gap-3 rounded-xl border px-4 py-3 text-left transition-colors',
        selected ? 'border-primary bg-primary/10' : 'border-border hover:bg-accent',
      )}
    >
      <span className="flex min-w-0 flex-col gap-1">
        <span className="truncate text-base font-extrabold text-foreground">{topic.name}</span>
        <span className="text-xs text-muted-foreground">
          {t('topics.quiz')}: <b className="text-foreground">{score(topic.quizScore)}</b>
          {' · '}
          {t('topics.lab')}: <b className="text-foreground">{score(topic.labScore)}</b>
        </span>
      </span>
      <span
        className={cn(
          'shrink-0 rounded-full border px-2.5 py-1 text-xs font-bold',
          STATUS_STYLE[topic.status],
        )}
      >
        {t(`status.${topic.status}`)}
      </span>
    </button>
  )
}

export default ProgressScreen
