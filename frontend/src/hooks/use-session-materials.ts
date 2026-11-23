import { useCallback, useEffect, useState } from 'react'
import { listSessionExercises, type SessionExerciseCard } from '@/lib/exercise'
import { listSessionMindMaps, type MindMapSummary } from '@/lib/mindmap'
import { listSessionQuizzes, type QuizSummary } from '@/lib/quiz'

export type MaterialListStatus = 'loading' | 'ready' | 'error'

export interface MaterialList<T> {
  items: T[]
  status: MaterialListStatus
  retry: () => void
}

export interface SessionMaterials {
  mindMaps: MaterialList<MindMapSummary>
  quizzes: MaterialList<QuizSummary>
  labs: MaterialList<SessionExerciseCard>
}

interface ListState<T> {
  sessionId: string
  items: T[]
  status: MaterialListStatus
}

// One session's list, loaded on mount and again whenever the revision moves
// or retry() is called. A reload that fails keeps what the list already
// showed; only a first load that fails is an error. An answer for a session
// that is no longer the current one is discarded.
function useMaterialList<T>(
  sessionId: string,
  revision: number,
  load: (sessionId: string) => Promise<T[]>,
): MaterialList<T> {
  const [state, setState] = useState<ListState<T>>({ sessionId, items: [], status: 'loading' })
  const [retryToken, setRetryToken] = useState(0)

  // Adjusting state during render, not in an effect: the old session's items
  // must never be visible for the one render after the session changed.
  if (state.sessionId !== sessionId) {
    setState({ sessionId, items: [], status: 'loading' })
  }

  useEffect(() => {
    let cancelled = false
    load(sessionId)
      .then((items) => {
        if (!cancelled) setState({ sessionId, items, status: 'ready' })
      })
      .catch(() => {
        if (cancelled) return
        setState((current) =>
          current.sessionId === sessionId && current.status === 'ready'
            ? current
            : { sessionId, items: [], status: 'error' },
        )
      })
    return () => {
      cancelled = true
    }
  }, [sessionId, revision, retryToken, load])

  const retry = useCallback(() => {
    setState((current) => ({ ...current, status: 'loading' }))
    setRetryToken((token) => token + 1)
  }, [])

  const visible = state.sessionId === sessionId
  return { items: visible ? state.items : [], status: visible ? state.status : 'loading', retry }
}

// The mind maps, quizzes and Arch Labs of a study session, for the Session
// materials panel. See
// specs/phases/phase-04-tutor/05-session-materials-panel.md.
export function useSessionMaterials(sessionId: string, revision: number): SessionMaterials {
  return {
    mindMaps: useMaterialList(sessionId, revision, listSessionMindMaps),
    quizzes: useMaterialList(sessionId, revision, listSessionQuizzes),
    labs: useMaterialList(sessionId, revision, listSessionExercises),
  }
}
