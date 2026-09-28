import { useCallback, useEffect, useState } from 'react'
import { listSessionSources, type SessionSource } from '@/lib/sources'

export interface UseSessionSourcesResult {
  sources: SessionSource[]
  loading: boolean
  error: string | null
  reload: () => void
}

// useSessionSources owns the Sources panel's data: it loads sessionId's
// imported documents on mount and whenever sessionId changes, and exposes
// reload() for after an Add/Remove. A response that resolves after the
// session has already changed — the user switched sessions, or this
// component unmounted — is discarded rather than applied to state.
export function useSessionSources(sessionId: string): UseSessionSourcesResult {
  const [sources, setSources] = useState<SessionSource[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  // Bumping this re-runs the effect below for the same sessionId, without
  // callers needing to know anything beyond "reload".
  const [reloadToken, setReloadToken] = useState(0)

  // Resetting to a loading state happens here, during render, the instant
  // sessionId changes — not inside the effect below, which must never call
  // setState synchronously in its body (see "Adjusting state when a prop
  // changes": https://react.dev/learn/you-might-not-need-an-effect). A
  // reload() (sessionId unchanged) resets loading itself instead.
  const [loadedForSessionId, setLoadedForSessionId] = useState(sessionId)
  if (sessionId !== loadedForSessionId) {
    setLoadedForSessionId(sessionId)
    setLoading(true)
    setError(null)
  }

  useEffect(() => {
    let cancelled = false
    listSessionSources(sessionId)
      .then((result) => {
        if (cancelled) return
        setSources(result)
        setLoading(false)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setSources([])
        setError(err instanceof Error ? err.message : String(err))
        setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [sessionId, reloadToken])

  const reload = useCallback(() => {
    setLoading(true)
    setError(null)
    setReloadToken((token) => token + 1)
  }, [])

  return { sources, loading, error, reload }
}
