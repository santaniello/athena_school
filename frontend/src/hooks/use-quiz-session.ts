import { useCallback, useEffect, useRef, useState } from 'react'
import {
  countBlank,
  getQuiz,
  quizErrorMessage,
  submitQuizAttempt,
  type Quiz,
  type QuizAttemptSummary,
  type QuizQuestion,
  type QuizSelection,
} from '@/lib/quiz'

export interface UseQuizSessionResult {
  state: 'loading' | 'failed' | 'ready'
  quiz: Quiz | null
  index: number
  question: QuizQuestion | null
  // One per question, in question order; selectedIndex is null while blank.
  selections: QuizSelection[]
  blankCount: number
  submitting: boolean
  submitError: string | null
  reload: () => void
  // Picks an option of the current question; null clears the answer.
  answer: (optionIndex: number | null) => void
  goTo: (index: number) => void
  next: () => void
  previous: () => void
  // Resolves with the graded attempt, or null when the submission failed (the
  // answers are kept) or is already running.
  submit: () => Promise<QuizAttemptSummary | null>
}

// useQuizSession holds one quiz being taken: the quiz, the current question
// and the answers, which live only in memory until submit(). Leaving the
// screen therefore discards them, as the spec says. See
// specs/phases/phase-03-architecture-lab/22-interactive-quiz-screen.md.
export function useQuizSession(quizId: string): UseQuizSessionResult {
  const [quiz, setQuiz] = useState<Quiz | null>(null)
  const [failed, setFailed] = useState(false)
  // Bumping this re-runs the load effect for the same quiz.
  const [reloadToken, setReloadToken] = useState(0)
  const [index, setIndex] = useState(0)
  const [chosen, setChosen] = useState<Record<string, number>>({})
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  // A ref, not the state, so two clicks in the same tick cannot both pass.
  const inFlight = useRef(false)

  useEffect(() => {
    let stale = false
    getQuiz(quizId)
      .then((loaded) => {
        if (stale) return
        setQuiz(loaded)
        setFailed(false)
      })
      .catch(() => {
        if (!stale) setFailed(true)
      })
    return () => {
      stale = true
    }
  }, [quizId, reloadToken])

  const reload = useCallback(() => {
    setFailed(false)
    setReloadToken((token) => token + 1)
  }, [])

  const questions = quiz?.questions ?? []
  const question = questions[index] ?? null
  const selections: QuizSelection[] = questions.map((item) => ({
    questionId: item.id,
    selectedIndex: chosen[item.id] ?? null,
  }))

  function answer(optionIndex: number | null) {
    if (!question) return
    setChosen((current) => {
      const next = { ...current }
      if (optionIndex === null) delete next[question.id]
      else next[question.id] = optionIndex
      return next
    })
  }

  function goTo(target: number) {
    if (target >= 0 && target < questions.length) setIndex(target)
  }

  async function submit(): Promise<QuizAttemptSummary | null> {
    if (!quiz || inFlight.current) return null
    inFlight.current = true
    setSubmitting(true)
    setSubmitError(null)
    try {
      return await submitQuizAttempt(quiz.id, selections)
    } catch (err) {
      setSubmitError(quizErrorMessage(err))
      return null
    } finally {
      inFlight.current = false
      setSubmitting(false)
    }
  }

  return {
    state: quiz ? 'ready' : failed ? 'failed' : 'loading',
    quiz,
    index,
    question,
    selections,
    blankCount: countBlank(selections),
    submitting,
    submitError,
    reload,
    answer,
    goTo,
    next: () => goTo(index + 1),
    previous: () => goTo(index - 1),
    submit,
  }
}
