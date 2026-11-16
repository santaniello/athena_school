import { useEffect, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { formatSessionDate, listSessionsForGeneration, type SessionOption } from '@/lib/exercise'
import {
  DEFAULT_QUESTIONS,
  MAX_QUESTIONS,
  MIN_QUESTIONS,
  QUIZ_DIFFICULTIES,
  cancelQuizGeneration,
  generateQuiz,
  parseQuestionCount,
  quizDifficultyLabel,
  quizErrorCode,
  quizErrorMessage,
  type QuizDifficulty,
  type QuizSummary,
} from '@/lib/quiz'

interface GenerateQuizDialogProps {
  open: boolean
  // Preselected from the profile's experience level.
  defaultDifficulty: QuizDifficulty
  onClose: () => void
  // Called with the new quiz; the caller closes the dialog and reloads.
  onGenerated: (quiz: QuizSummary) => void
  // Called instead when the dialog was already closed by the time the quiz
  // arrived: it exists in the catalog, but nothing on screen is waiting for it.
  onGeneratedAfterClose: (quiz: QuizSummary) => void
}

// Opened by "Generate test with AI". The content is its own component, so the
// chosen session, the count and any error are discarded each time the dialog
// closes. See specs/phases/phase-03-architecture-lab/22-interactive-quiz-screen.md.
export function GenerateQuizDialog({ open, ...rest }: GenerateQuizDialogProps) {
  return open ? <GenerateQuizContent {...rest} /> : null
}

type Sessions =
  { state: 'loading' } | { state: 'failed' } | { state: 'ready'; list: SessionOption[] }

function GenerateQuizContent({
  defaultDifficulty,
  onClose,
  onGenerated,
  onGeneratedAfterClose,
}: Omit<GenerateQuizDialogProps, 'open'>) {
  const { t } = useTranslation('quiz')
  const [sessions, setSessions] = useState<Sessions>({ state: 'loading' })
  const [reloadToken, setReloadToken] = useState(0)
  const [search, setSearch] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [countText, setCountText] = useState(String(DEFAULT_QUESTIONS))
  const [difficulty, setDifficulty] = useState(defaultDifficulty)
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // False once the dialog has closed, so a late answer is told apart from one
  // this instance is still waiting for.
  const mountedRef = useRef(false)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  useEffect(() => {
    let stale = false
    listSessionsForGeneration()
      .then((list) => {
        if (!stale) setSessions({ state: 'ready', list })
      })
      .catch(() => {
        if (!stale) setSessions({ state: 'failed' })
      })
    return () => {
      stale = true
    }
  }, [reloadToken])

  function retryLoading() {
    setSessions({ state: 'loading' })
    setReloadToken((token) => token + 1)
  }

  const count = parseQuestionCount(countText)

  async function handleGenerate() {
    if (!selectedId || count === null) return
    setError(null)
    setGenerating(true)
    try {
      const quiz = await generateQuiz(selectedId, count, difficulty)
      if (mountedRef.current) onGenerated(quiz)
      else onGeneratedAfterClose(quiz)
    } catch (err) {
      // A cancelled generation is the user's own doing: back to the choice, quietly.
      if (quizErrorCode(err) !== 'cancelled') setError(quizErrorMessage(err))
      setGenerating(false)
    }
  }

  // Dismissing the dialog (Escape, the close button) must not leave a
  // generation running that nobody is waiting for.
  function handleDismiss() {
    if (generating) void cancelQuizGeneration()
    onClose()
  }

  const list = sessions.state === 'ready' ? sessions.list : []
  const needle = search.trim().toLowerCase()
  const visible = list.filter(
    (session) =>
      session.topic.toLowerCase().includes(needle) ||
      session.folderName.toLowerCase().includes(needle),
  )

  return (
    <Dialog open onOpenChange={(nextOpen) => !nextOpen && handleDismiss()}>
      <DialogContent
        closeLabel={t('common.close')}
        className="max-h-[90vh] overflow-y-auto sm:max-w-lg"
      >
        <DialogHeader>
          <DialogTitle>{t('generate.title')}</DialogTitle>
          <DialogDescription>{t('generate.description')}</DialogDescription>
        </DialogHeader>

        {sessions.state === 'loading' && (
          <p className="text-sm text-muted-foreground">{t('generate.loadingSessions')}</p>
        )}
        {sessions.state === 'failed' && (
          <div role="alert" className="flex items-center justify-between gap-3 text-sm">
            <span className="text-destructive">{t('generate.sessionsFailed')}</span>
            <Button type="button" variant="outline" size="sm" onClick={retryLoading}>
              {t('common.retry')}
            </Button>
          </div>
        )}
        {sessions.state === 'ready' && list.length === 0 && (
          <p className="text-sm text-muted-foreground">{t('generate.noSessions')}</p>
        )}
        {sessions.state === 'ready' && list.length > 0 && (
          <div className="space-y-3">
            <Input
              type="search"
              aria-label={t('generate.searchLabel')}
              placeholder={t('generate.searchPlaceholder')}
              value={search}
              disabled={generating}
              onChange={(event) => setSearch(event.target.value)}
            />
            <div
              role="radiogroup"
              aria-label={t('generate.sessionsLabel')}
              className="thin-scroll max-h-64 space-y-1 overflow-y-auto"
            >
              {visible.map((session) => (
                // A native radio gives one tab stop and the arrow keys for free.
                <label
                  key={session.id}
                  className="flex w-full cursor-pointer flex-col rounded-lg border border-border px-3 py-2 text-left text-sm transition-colors hover:bg-accent has-[:checked]:border-primary has-[:checked]:bg-primary/10 has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-60 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring"
                >
                  <input
                    type="radio"
                    name="quiz-study-session"
                    className="sr-only"
                    checked={session.id === selectedId}
                    disabled={generating}
                    onChange={() => setSelectedId(session.id)}
                  />
                  <span className="font-medium text-foreground">{session.topic}</span>
                  <span className="text-muted-foreground">
                    {session.folderName} · {formatSessionDate(session.startedAt)}
                  </span>
                </label>
              ))}
              {visible.length === 0 && (
                <p className="py-2 text-sm text-muted-foreground">{t('generate.noSessionMatch')}</p>
              )}
            </div>
            <div className="flex items-center gap-2">
              <label htmlFor="quiz-count" className="text-sm text-muted-foreground">
                {t('generate.questions')}
              </label>
              <Input
                id="quiz-count"
                type="number"
                min={MIN_QUESTIONS}
                max={MAX_QUESTIONS}
                className="w-24"
                value={countText}
                disabled={generating}
                onChange={(event) => setCountText(event.target.value)}
              />
            </div>
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">{t('generate.difficulty')}</span>
              <Select
                value={difficulty}
                disabled={generating}
                onValueChange={(value) => setDifficulty(value as QuizDifficulty)}
              >
                <SelectTrigger aria-label={t('generate.difficulty')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {QUIZ_DIFFICULTIES.map((value) => (
                    <SelectItem key={value} value={value}>
                      {quizDifficultyLabel(value)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        )}

        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        {generating && (
          <p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            {t('generate.generating')}
          </p>
        )}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={generating ? () => void cancelQuizGeneration() : onClose}
          >
            {t('common.cancel')}
          </Button>
          {sessions.state === 'ready' && list.length > 0 && !generating && (
            <Button
              type="button"
              disabled={!selectedId || count === null}
              onClick={() => void handleGenerate()}
            >
              {error ? t('common.tryAgain') : t('generate.submit')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
