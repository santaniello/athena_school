import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { GenerateQuizDialog } from '@/components/quiz/generate-quiz-dialog'
import { QuizHistoryDialog, type HistoryQuiz } from '@/components/quiz/quiz-history-dialog'
import { QuizRow } from '@/components/quiz/quiz-row'
import { deleteQuiz, difficultyForExperience, listQuizzes, type QuizSummary } from '@/lib/quiz'

interface QuizScreenProps {
  // The profile's experience level, which preselects the difficulty.
  experienceLevel: string | undefined
  // Take or Take again was clicked; the shell opens the quiz full screen.
  onTake: (quizId: string) => void
}

type Quizzes = { state: 'loading' } | { state: 'failed' } | { state: 'ready'; list: QuizSummary[] }

// The Interactive Test section: the user's quizzes, newest first, with the
// Generate test with AI action. Everything shown comes from the backend. See
// specs/phases/phase-03-architecture-lab/22-interactive-quiz-screen.md.
function QuizScreen({ experienceLevel, onTake }: QuizScreenProps) {
  const { t } = useTranslation('quiz')
  const [quizzes, setQuizzes] = useState<Quizzes>({ state: 'loading' })
  // Bumping this reloads the list.
  const [reloadToken, setReloadToken] = useState(0)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [highlightedId, setHighlightedId] = useState<string | null>(null)
  const [historyOf, setHistoryOf] = useState<HistoryQuiz | null>(null)
  const [toDelete, setToDelete] = useState<QuizSummary | null>(null)
  const [deleteFailed, setDeleteFailed] = useState(false)

  useEffect(() => {
    let stale = false
    listQuizzes()
      .then((list) => {
        if (!stale) setQuizzes({ state: 'ready', list })
      })
      .catch(() => {
        if (!stale) setQuizzes({ state: 'failed' })
      })
    return () => {
      stale = true
    }
  }, [reloadToken])

  const reload = useCallback(() => setReloadToken((token) => token + 1), [])

  function retry() {
    setQuizzes({ state: 'loading' })
    reload()
  }

  function handleGenerated(quiz: QuizSummary) {
    setDialogOpen(false)
    setHighlightedId(quiz.id)
    reload()
  }

  async function confirmDelete() {
    if (!toDelete) return
    try {
      await deleteQuiz(toDelete.id)
      setToDelete(null)
      reload()
    } catch {
      setToDelete(null)
      setDeleteFailed(true)
    }
  }

  const list = quizzes.state === 'ready' ? quizzes.list : []

  return (
    <div className="flex min-h-0 w-full flex-col gap-4">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-foreground">{t('catalog.title')}</h2>
          <p className="text-sm text-muted-foreground">{t('catalog.description')}</p>
        </div>
        <Button type="button" onClick={() => setDialogOpen(true)}>
          {t('catalog.generate')}
        </Button>
      </div>

      {deleteFailed && (
        <p role="alert" className="text-sm text-destructive">
          {t('catalog.deleteFailed')}
        </p>
      )}

      <div className="thin-scroll min-h-0 flex-1 overflow-y-auto">
        {quizzes.state === 'failed' ? (
          <div role="alert" className="flex items-center gap-3 text-sm">
            <span className="text-destructive">{t('catalog.loadFailed')}</span>
            <Button type="button" variant="outline" size="sm" onClick={retry}>
              {t('common.retry')}
            </Button>
          </div>
        ) : quizzes.state === 'loading' ? (
          <p className="text-sm text-muted-foreground">{t('catalog.loading')}</p>
        ) : list.length > 0 ? (
          <ul className="divide-y divide-border rounded-lg border border-border">
            {list.map((quiz) => (
              <QuizRow
                key={quiz.id}
                quiz={quiz}
                highlighted={quiz.id === highlightedId}
                onTake={(chosen) => onTake(chosen.id)}
                onHistory={(chosen) => setHistoryOf({ id: chosen.id, title: chosen.sessionTitle })}
                onDelete={(chosen) => {
                  setDeleteFailed(false)
                  setToDelete(chosen)
                }}
              />
            ))}
          </ul>
        ) : (
          <div className="flex flex-col items-center gap-2 py-10 text-center">
            <p className="max-w-80 text-sm text-muted-foreground">{t('catalog.empty')}</p>
            <Button type="button" onClick={() => setDialogOpen(true)}>
              {t('catalog.generate')}
            </Button>
          </div>
        )}
      </div>

      <GenerateQuizDialog
        open={dialogOpen}
        defaultDifficulty={difficultyForExperience(experienceLevel)}
        onClose={() => setDialogOpen(false)}
        onGenerated={handleGenerated}
        onGeneratedAfterClose={reload}
      />

      <QuizHistoryDialog quiz={historyOf} onClose={() => setHistoryOf(null)} />

      <AlertDialog open={toDelete !== null} onOpenChange={(open) => !open && setToDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('catalog.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('catalog.deleteDescription')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              // The action closes the dialog on click unless prevented; the
              // dialog closes once the deletion has an answer.
              onClick={(event) => {
                event.preventDefault()
                void confirmDelete()
              }}
            >
              {t('catalog.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

export default QuizScreen
