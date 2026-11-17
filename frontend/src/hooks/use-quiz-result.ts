import { useCallback, useEffect, useState } from 'react'
import { getQuizAttemptResult, type QuizAttemptResult } from '@/lib/quiz'

export interface UseQuizResultResult {
  state: 'loading' | 'failed' | 'ready'
  result: QuizAttemptResult | null
  reload: () => void
}

// useQuizResult loads the graded result of one attempt: the answer key, the
// explanations and what the user chose. A new attempt id starts a new load,
// and an answer that arrives for an old one is discarded.
export function useQuizResult(attemptId: string): UseQuizResultResult {
  const [loaded, setLoaded] = useState<{ attemptId: string; result: QuizAttemptResult } | null>(
    null,
  )
  const [failed, setFailed] = useState(false)
  // Bumping this re-runs the load effect for the same attempt.
  const [reloadToken, setReloadToken] = useState(0)

  useEffect(() => {
    let stale = false
    getQuizAttemptResult(attemptId)
      .then((result) => {
        if (stale) return
        setLoaded({ attemptId, result })
        setFailed(false)
      })
      .catch(() => {
        if (!stale) setFailed(true)
      })
    return () => {
      stale = true
    }
  }, [attemptId, reloadToken])

  const reload = useCallback(() => {
    setFailed(false)
    setReloadToken((token) => token + 1)
  }, [])

  // A result of another attempt is not shown while the new one loads.
  const result = loaded?.attemptId === attemptId ? loaded.result : null
  return { state: result ? 'ready' : failed ? 'failed' : 'loading', result, reload }
}
