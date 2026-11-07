import { useCallback, useEffect, useRef, useState } from 'react'
import { listHints, requestHint } from '@/lib/exercise'
import { MAX_HINTS, type LabHint } from '@/lib/lab'

// The tutor's hints for one attempt. The backend counts and enforces the limit;
// this hook only shows the hints it returns, one request at a time, and counts
// what is left from them. A failed request spends nothing. See
// specs/phases/phase-03-architecture-lab/10-01-tutor-hints.md.
export function useHints(attemptId: string | null, getXml: () => Promise<string>) {
  const [hints, setHints] = useState<LabHint[]>([])
  const [asking, setAsking] = useState(false)
  const [failed, setFailed] = useState(false)
  // Mirrors asking so a burst of clicks sends one request.
  const askingRef = useRef(false)

  useEffect(() => {
    if (attemptId === null) return
    // Stryker disable BooleanLiteral,ConditionalExpression: the guard only skips a state update after unmount, which React ignores silently, so it cannot be observed.
    let cancelled = false
    listHints(attemptId)
      .then((given) => {
        if (!cancelled) setHints(given)
      })
      // Stryker disable next-line BlockStatement: the swallow is the behavior; an empty handler and none are the same to a test.
      .catch(() => {
        // The panel starts empty; the backend still enforces the limit.
      })
    return () => {
      cancelled = true
    }
    // Stryker restore BooleanLiteral,ConditionalExpression
  }, [attemptId])

  const hintsLeft = MAX_HINTS - hints.length

  const askHint = useCallback(async () => {
    if (attemptId === null || hintsLeft <= 0 || askingRef.current) return
    askingRef.current = true
    setAsking(true)
    setFailed(false)
    try {
      const hint = await requestHint(attemptId, await getXml())
      setHints((given) => [...given, hint])
    } catch {
      setFailed(true)
    } finally {
      askingRef.current = false
      setAsking(false)
    }
  }, [attemptId, hintsLeft, getXml])

  return { hints, hintsLeft, asking, failed, askHint }
}
