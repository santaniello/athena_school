import { useEffect, useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  AlertCircle,
  CheckCircle2,
  ChevronRight,
  Code2,
  HelpCircle,
  Layers,
  MinusCircle,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { PROGRESS_NAMESPACE } from '@/i18n'
import { quizDifficultyLabel } from '@/lib/quiz'
import { ProgressEvolutionChart } from '@/components/progress-evolution-chart'
import {
  getProgressTopic,
  type LabResult,
  type ProgressChallengeResult,
  type ProgressConcept,
  type ProgressDifficulty,
  type ProgressProof,
  type ProgressQuizResult,
  type ProgressTopicDetail as TopicDetail,
} from '@/lib/progress'

interface ProgressTopicDetailProps {
  sessionId: string
  onOpenQuiz: (quizId: string) => void
  onOpenChallenge: (challenge: ProgressChallengeResult) => void
  // The status pill of the topic, shown beside its name.
  status?: ReactNode
}

type DetailState =
  { state: 'loading' } | { state: 'failed' } | { state: 'ready'; detail: TopicDetail }

type BadgeVariant = 'beginner' | 'intermediate' | 'advanced' | 'muted'

const RESULT_VARIANT: Record<LabResult, BadgeVariant> = {
  PASSED: 'beginner',
  FAILED: 'advanced',
  PENDING: 'muted',
}

const ASSESSMENTS: Record<string, { label: string; variant: BadgeVariant }> = {
  CORRECT: { label: 'assessment.correct', variant: 'beginner' },
  PARTIALLY_CORRECT: { label: 'assessment.partiallyCorrect', variant: 'intermediate' },
  INCORRECT: { label: 'assessment.incorrect', variant: 'advanced' },
}

// The right panel of Progress: what supports the status of a topic. It reads
// the topic on mount and again when another one is selected; the backend
// derives everything on read, with no LLM call. Names and descriptions come
// from the model, so they are only ever rendered as text. See
// specs/phases/phase-05-progress/02-topic-detail.md.
export function ProgressTopicDetail(props: ProgressTopicDetailProps) {
  // Keyed by the session, so selecting another topic starts from "loading"
  // and a late answer for the previous one lands in an unmounted body.
  return <TopicBody key={props.sessionId} {...props} />
}

function TopicBody({ sessionId, onOpenQuiz, onOpenChallenge, status }: ProgressTopicDetailProps) {
  const { t } = useTranslation('progressTab')
  const [state, setState] = useState<DetailState>({ state: 'loading' })

  useEffect(() => {
    let stale = false
    getProgressTopic(sessionId)
      .then((detail) => {
        if (!stale) setState({ state: 'ready', detail })
      })
      .catch(() => {
        if (!stale) setState({ state: 'failed' })
      })
    return () => {
      stale = true
    }
  }, [sessionId])

  if (state.state === 'loading') {
    return <p className="text-sm text-muted-foreground">{t('detail.loading')}</p>
  }
  if (state.state === 'failed') {
    return (
      <p role="alert" className="flex items-center gap-2 text-sm text-destructive">
        <AlertCircle className="size-4" aria-hidden="true" />
        {t('detail.loadFailed')}
      </p>
    )
  }
  const { detail } = state
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex items-center gap-3.5">
          <Layers className="size-8 shrink-0 text-primary" aria-hidden="true" />
          <div>
            <h3 className="font-heading text-2xl font-bold break-words text-foreground">
              {detail.name}
            </h3>
            <span className="text-xs text-muted-foreground">
              <b className="font-normal">{detail.folderName}</b> · {t('detail.session')}
            </span>
          </div>
        </div>
        {status}
      </header>
      <Section title={t('detail.activities.title')}>
        <ul className="grid grid-cols-3 gap-3">
          <Stat
            value={detail.quizCount}
            label={t('detail.activities.quizzes', { count: detail.quizCount })}
          />
          <Stat
            value={detail.challengeCount}
            label={t('detail.activities.labs', { count: detail.challengeCount })}
          />
          <Stat
            value={detail.conceptCount}
            label={t('detail.activities.concepts', { count: detail.conceptCount })}
          />
        </ul>
      </Section>
      <Results detail={detail} onOpenQuiz={onOpenQuiz} onOpenChallenge={onOpenChallenge} />
      <Section title={t('detail.evolution.title')}>
        <ProgressEvolutionChart evolution={detail.evolution} />
      </Section>
      <Learning detail={detail} />
    </div>
  )
}

function Stat({ value, label }: { value: number; label: string }) {
  return (
    <li className="rounded-xl bg-muted px-4 py-3.5">
      <div className="text-2xl font-extrabold text-foreground">{value}</div>
      <div className="text-xs text-muted-foreground">{label}</div>
    </li>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  const headingId = useId()
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-2.5">
      <h4 id={headingId} className="text-base font-extrabold text-foreground">
        {title}
      </h4>
      {children}
    </section>
  )
}

function Empty({ children }: { children: ReactNode }) {
  return <p className="text-sm text-muted-foreground">{children}</p>
}

function Results({
  detail,
  onOpenQuiz,
  onOpenChallenge,
}: {
  detail: TopicDetail
  onOpenQuiz: (quizId: string) => void
  onOpenChallenge: (challenge: ProgressChallengeResult) => void
}) {
  const { t } = useTranslation('progressTab')
  return (
    <Section title={t('detail.results.title')}>
      <h5 className="text-xs font-bold text-muted-foreground uppercase">
        {t('detail.results.quizzes')}
      </h5>
      {detail.quizzes.length === 0 ? (
        <Empty>{t('detail.results.none')}</Empty>
      ) : (
        <ul className="flex flex-col gap-2">
          {detail.quizzes.map((quiz) => (
            <li key={quiz.quizId}>
              <QuizRow quiz={quiz} onOpen={() => onOpenQuiz(quiz.quizId)} />
            </li>
          ))}
        </ul>
      )}
      <h5 className="mt-2 text-xs font-bold text-muted-foreground uppercase">
        {t('detail.results.labs')}
      </h5>
      {detail.challenges.length === 0 ? (
        <Empty>{t('detail.results.none')}</Empty>
      ) : (
        <ul className="flex flex-col gap-2">
          {detail.challenges.map((challenge) => (
            <li key={challenge.challengeId}>
              <ChallengeRow challenge={challenge} onOpen={() => onOpenChallenge(challenge)} />
            </li>
          ))}
        </ul>
      )}
      <p className="text-xs text-muted-foreground">{t('detail.results.note')}</p>
    </Section>
  )
}

const ROW_CLASS =
  'flex w-full cursor-pointer items-center gap-3 rounded-xl bg-muted px-4 py-3 text-left transition-colors hover:bg-accent'

function QuizRow({ quiz, onOpen }: { quiz: ProgressQuizResult; onOpen: () => void }) {
  const { t } = useTranslation('progressTab')
  return (
    <button type="button" onClick={onOpen} className={ROW_CLASS}>
      <HelpCircle className="size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm font-bold text-foreground">
          {t('detail.results.quizTitle', {
            number: quiz.number,
            difficulty: quizDifficultyLabel(quiz.difficulty),
          })}
        </span>
        <span className="text-xs text-muted-foreground">
          {quiz.percent === null
            ? t('detail.results.notTaken')
            : t('detail.results.attempts', { count: quiz.attempts })}
        </span>
      </span>
      {quiz.percent !== null && (
        <span className="shrink-0 text-base font-extrabold text-foreground">{quiz.percent}%</span>
      )}
    </button>
  )
}

function ChallengeRow({
  challenge,
  onOpen,
}: {
  challenge: ProgressChallengeResult
  onOpen: () => void
}) {
  const { t } = useTranslation('progressTab')
  const score = (value: number | null) => (value === null ? t('topics.noScore') : `${value}%`)
  return (
    <button type="button" onClick={onOpen} className={ROW_CLASS}>
      <Code2 className="size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-sm font-bold text-foreground">{challenge.title}</span>
        <span className="text-xs text-muted-foreground">
          {t('detail.results.diagram')} {score(challenge.diagramScore)}
          {' · '}
          {challenge.defenseScore === null
            ? t('detail.results.defensePending')
            : `${t('detail.results.defense')} ${challenge.defenseScore}%`}
          {' · '}
          {t('detail.results.attempts', { count: challenge.attempts })}
        </span>
      </span>
      <Badge variant={RESULT_VARIANT[challenge.result]} className="shrink-0 px-2">
        {t(`detail.results.outcome.${challenge.result}`)}
      </Badge>
    </button>
  )
}

function Learning({ detail }: { detail: TopicDetail }) {
  const { t } = useTranslation('progressTab')
  return (
    <Section title={t('detail.learning.title')}>
      <Group
        icon={<CheckCircle2 className="text-[#5fbf8f]" aria-hidden="true" />}
        title={t('detail.learning.understood')}
        empty={t('detail.learning.noUnderstood')}
      >
        {detail.understood.map((concept) => (
          <ConceptEntry key={concept.name} concept={concept} />
        ))}
      </Group>
      <Group
        icon={<AlertCircle className="text-[#e8875a]" aria-hidden="true" />}
        title={t('detail.learning.difficulties')}
        empty={t('detail.learning.noDifficulty')}
      >
        {detail.difficulties.map((difficulty) => (
          <ConceptEntry
            key={`${difficulty.source}-${difficulty.challengeTitle}-${difficulty.name}`}
            concept={difficulty}
            difficulty={difficulty}
          />
        ))}
      </Group>
      <Group
        icon={<MinusCircle className="text-muted-foreground" aria-hidden="true" />}
        title={t('detail.learning.explained')}
        empty={t('detail.learning.noExplained')}
      >
        {detail.explainedOnly.map((concept) => (
          <ConceptEntry key={concept.name} concept={concept} />
        ))}
      </Group>
    </Section>
  )
}

// A group is a labelled region whose list is replaced by a sentence when empty.
function Group({
  icon,
  title,
  empty,
  children,
}: {
  icon: ReactNode
  title: string
  empty: string
  children: ReactNode[]
}) {
  const headingId = useId()
  return (
    <section aria-labelledby={headingId} className="flex gap-3">
      <span className="mt-0.5 shrink-0 [&>svg]:size-6">{icon}</span>
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <h5 id={headingId} className="text-base font-extrabold text-foreground">
          {title}
        </h5>
        {children.length === 0 ? (
          <Empty>{empty}</Empty>
        ) : (
          <ul className="flex flex-col gap-2">{children}</ul>
        )}
      </div>
    </section>
  )
}

function ConceptEntry({
  concept,
  difficulty,
}: {
  concept: ProgressConcept
  difficulty?: ProgressDifficulty
}) {
  const { t } = useTranslation('progressTab')
  const [open, setOpen] = useState(false)
  const proofsId = useId()
  return (
    <li className="flex flex-col gap-2 rounded-xl bg-muted px-4 py-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <span className="text-base font-bold break-words text-foreground">{concept.name}</span>
        {difficulty && (
          <span className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            {difficulty.source === 'TUTOR'
              ? t('detail.learning.sourceTutor')
              : t('detail.learning.sourceLab', { title: difficulty.challengeTitle })}
            {difficulty.toReview && (
              <Badge variant="advanced" className="px-2">
                {t('detail.learning.toReview')}
              </Badge>
            )}
          </span>
        )}
      </div>
      {concept.proofs.length > 0 && (
        <>
          <button
            type="button"
            aria-expanded={open}
            aria-controls={proofsId}
            onClick={() => setOpen((current) => !current)}
            className="flex w-fit cursor-pointer items-center gap-1 text-sm text-primary"
          >
            <ChevronRight
              className={cn('size-3.5 transition-transform', open && 'rotate-90')}
              aria-hidden="true"
            />
            {open ? t('detail.learning.hideProofs') : t('detail.learning.showProofs')} (
            {concept.proofs.length})
          </button>
          {open && (
            <ul id={proofsId} className="flex flex-col gap-1.5 border-t border-border pt-2">
              {concept.proofs.map((proof, index) => (
                <Proof key={`${proof.createdAt}-${index}`} proof={proof} />
              ))}
            </ul>
          )}
        </>
      )}
    </li>
  )
}

function Proof({ proof }: { proof: ProgressProof }) {
  const { t } = useTranslation(PROGRESS_NAMESPACE)
  const assessment = ASSESSMENTS[proof.assessment]
  return (
    <li className="flex flex-col gap-1 text-sm">
      <div className="flex flex-wrap items-center gap-1.5">
        <Badge variant={assessment?.variant ?? 'muted'} className="px-2">
          {assessment ? t(assessment.label) : proof.assessment}
        </Badge>
        {proof.assisted && (
          <Badge variant="muted" className="px-2">
            {t('assisted')}
          </Badge>
        )}
      </div>
      {proof.description && (
        <p className="break-words text-muted-foreground">{proof.description}</p>
      )}
    </li>
  )
}
