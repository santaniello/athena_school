import { useCallback, useEffect, useRef, useState } from 'react'
import { listMessagesBefore, type StudyMessage } from '@/lib/study'

// How many older messages each request asks for (the backend default).
export const HISTORY_PAGE_SIZE = 50

export type MessageHistoryStatus = 'idle' | 'loading' | 'error'

export interface UseMessageHistoryResult {
  hasMore: boolean
  status: MessageHistoryStatus
  error: string | null
  // Seeds the hook with the cursor the resumed latest page returned.
  start: (hasMore: boolean, nextCursor: string) => void
  // Fetches the next older page. Doubles as the retry after a failure: the
  // cursor only advances on success, so it asks for the same page again.
  loadOlder: () => Promise<void>
}

// useMessageHistory owns the pagination state of a session's older messages:
// the cursor, whether more exist, and the loading / failed status. Fetched
// pages are handed to onPage — the caller keeps the message list itself. At
// most one request is in flight, and a response that resolves after the
// session changed is discarded.
export function useMessageHistory(
  sessionId: string,
  onPage: (messages: StudyMessage[]) => void,
): UseMessageHistoryResult {
  const [hasMore, setHasMore] = useState(false)
  const [status, setStatus] = useState<MessageHistoryStatus>('idle')
  const [error, setError] = useState<string | null>(null)

  // Resets during render the instant sessionId changes, not in an effect (see
  // "Adjusting state when a prop changes":
  // https://react.dev/learn/you-might-not-need-an-effect).
  const [stateSessionId, setStateSessionId] = useState(sessionId)
  if (sessionId !== stateSessionId) {
    setStateSessionId(sessionId)
    setHasMore(false)
    setStatus('idle')
    setError(null)
  }

  // Empty means there is nothing older to load.
  const cursorRef = useRef('')
  const inFlightRef = useRef(false)
  // Bumped on every session change; a request compares it on arrival.
  const generationRef = useRef(0)
  const onPageRef = useRef(onPage)

  useEffect(() => {
    onPageRef.current = onPage
  }, [onPage])

  useEffect(() => {
    generationRef.current += 1
    cursorRef.current = ''
    inFlightRef.current = false
  }, [sessionId])

  const start = useCallback((more: boolean, nextCursor: string) => {
    cursorRef.current = more ? nextCursor : ''
    setHasMore(more)
  }, [])

  const loadOlder = useCallback(async () => {
    if (!cursorRef.current || inFlightRef.current) return
    inFlightRef.current = true
    const generation = generationRef.current
    setStatus('loading')
    setError(null)
    try {
      const page = await listMessagesBefore(sessionId, cursorRef.current, HISTORY_PAGE_SIZE)
      if (generation !== generationRef.current) return
      cursorRef.current = page.hasMore ? page.nextCursor : ''
      setHasMore(page.hasMore)
      setStatus('idle')
      onPageRef.current(page.messages)
    } catch (err) {
      if (generation !== generationRef.current) return
      setError(err instanceof Error ? err.message : 'Failed to load earlier messages.')
      setStatus('error')
    } finally {
      if (generation === generationRef.current) inFlightRef.current = false
    }
  }, [sessionId])

  return { hasMore, status, error, start, loadOlder }
}
