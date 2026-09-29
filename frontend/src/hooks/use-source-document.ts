import { useCallback, useEffect, useState } from 'react'
import { getSessionSourceDocument, type SourceDocument } from '@/lib/sources'

export interface UseSourceDocumentResult {
  document: SourceDocument | null
  loading: boolean
  error: string | null
  reload: () => void
}

// useSourceDocument loads sessionId's document for itemId on demand — only
// once itemId is non-null, i.e. once the source viewer actually opens, not
// eagerly like useSessionSources' list — and reloads whenever sessionId or
// itemId changes. A response that resolves after itemId has already
// changed (a different document opened, or the viewer closed) is discarded
// rather than applied to state.
export function useSourceDocument(
  sessionId: string,
  itemId: string | null,
): UseSourceDocumentResult {
  const [document, setDocument] = useState<SourceDocument | null>(null)
  const [loading, setLoading] = useState(itemId !== null)
  const [error, setError] = useState<string | null>(null)
  // Bumping this re-runs the effect below for the same sessionId/itemId,
  // without callers needing to know anything beyond "reload".
  const [reloadToken, setReloadToken] = useState(0)

  // Resetting state happens here, during render, the instant sessionId or
  // itemId changes — not inside the effect below, which must never call
  // setState synchronously in its body (see "Adjusting state when a prop
  // changes": https://react.dev/learn/you-might-not-need-an-effect). A
  // reload() (neither changed) resets loading/error itself instead.
  // Stryker disable next-line ObjectLiteral: the initial value only matters
  // on the very first render, where document/error/loading already start
  // at exactly what the reset branch below would (re)set them to — so
  // replacing it wholesale changes nothing observable.
  const [loadedFor, setLoadedFor] = useState({ sessionId, itemId })
  if (sessionId !== loadedFor.sessionId || itemId !== loadedFor.itemId) {
    setLoadedFor({ sessionId, itemId })
    setDocument(null)
    setError(null)
    setLoading(itemId !== null)
  }

  useEffect(() => {
    if (itemId === null) return
    let cancelled = false
    getSessionSourceDocument(sessionId, itemId)
      .then((result) => {
        if (cancelled) return
        setDocument(result)
        setLoading(false)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setDocument(null)
        setError(err instanceof Error ? err.message : String(err))
        setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [sessionId, itemId, reloadToken])

  const reload = useCallback(() => {
    if (itemId === null) return
    setLoading(true)
    setError(null)
    // Stryker disable next-line ArithmeticOperator: reloadToken is never
    // read for its value, only compared for change to re-run the effect
    // above — any direction that keeps producing a new value each call
    // works identically.
    setReloadToken((token) => token + 1)
  }, [itemId])

  return { document, loading, error, reload }
}
