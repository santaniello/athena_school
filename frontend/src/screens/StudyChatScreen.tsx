import { memo, useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { ChangeEvent, KeyboardEvent, UIEvent } from 'react'
import { ArrowUp } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { MessageBubble } from '@/components/message-bubble'
import { ThinkingIndicator } from '@/components/thinking-indicator'
import { SourceModeSelect } from '@/components/source-mode-select'
import { LocalSourcesStrip } from '@/components/local-sources-strip'
import { SessionUsageBar } from '@/components/session-usage-bar'
import { ToastViewport, type ToastItem } from '@/components/toast'
import { useMessageHistory } from '@/hooks/use-message-history'
import {
  compactStudySession,
  getStudySessionUsage,
  onStudyChunk,
  onStudyCompactionFailed,
  onStudyContextCompacted,
  onStudyContextCompacting,
  onStudyContextLimitReached,
  onStudyContextLimitUnavailable,
  onStudyContextNormal,
  onStudyContextWarning,
  onStudyDone,
  onStudyError,
  onStudySources,
  requestOpeningTurn,
  resumeStudySession,
  sendStudyMessage,
  type SourceMode,
  type StudyContextEvent,
  type StudyMessage,
  type StudySessionUsage,
  type StudySource,
} from '@/lib/study'

interface StudyChatScreenProps {
  sessionId: string
  initialTopic: string
  // 'new' sessions request the opening turn immediately; 'resume' sessions
  // (picked from the sidebar tree) load their prior history instead.
  mode: 'new' | 'resume'
  // Resumed sessions hydrate their real topic from the backend, which may
  // differ from the sidebar's cached copy — this reports it up so the
  // AppShell topbar (which owns the title) can stay in sync.
  onTopicResolved?: (topic: string) => void
  // Starts a fresh session on the same topic/folder, offered by the
  // warning/blocked context-limit banners below. Owned by AppShell (see
  // specs/phases/phase-02-knowledge-engine/06-study-context-limits.md).
  onStartNewSession: () => void | Promise<void>
  startingNewSession: boolean
  // AppShell's topbar node the source-mode selector portals into, keeping
  // it out of the composer's textarea. Undefined/null (e.g. a standalone
  // render in tests) falls back to rendering it inline over the composer.
  sourceModeSlot?: HTMLElement | null
  // Opens the Sources panel at a citation's source document, at the exact
  // cited passage. Owned by AppShell (see spec 2.18 decision 11). Omitted
  // in contexts that never open the panel (e.g. this screen rendered
  // standalone, as in its own tests).
  onOpenCitation?: (source: StudySource) => void
}

// ContextState mirrors the persisted study.ContextState the backend tracks
// per session (see StudyContextUsage in lib/study.ts).
type ContextState = 'normal' | 'warning' | 'blocked'

// A failed or empty fetch resolves to null so callers keep the previous
// totals — usage is informational and must never break the chat.
async function fetchSessionUsage(sessionId: string): Promise<StudySessionUsage | null> {
  try {
    return (await getStudySessionUsage(sessionId)) ?? null
  } catch {
    return null
  }
}

function contextTokensFrom(
  event: Pick<StudyContextEvent, 'usedTokens' | 'contextLength' | 'estimated'>,
) {
  return {
    usedTokens: event.usedTokens,
    contextLength: event.contextLength,
    estimated: event.estimated,
  }
}

// Error codes carried by StudyErrorEvent that mean the failure happened
// before the user message was persisted — see StudyErrorEvent.code in
// internal/interfaces/desktop/study.go.
const PRE_PERSISTENCE_ERROR_CODES = new Set([
  'context_limit_reached',
  'turn_in_progress',
  'budget_impossible',
  'message_too_large',
])

// Of those, the codes whose rejection has no dedicated alert of its own (the
// blocked alert already explains context_limit_reached, and a turn already
// in progress needs no explanation): the user must be told why the message
// came back.
const REJECTION_MESSAGES: Record<string, string> = {
  budget_impossible:
    "This conversation no longer fits the model's context window, even summarized. Start a new session on the same topic to continue.",
  message_too_large:
    "This message is too long for the model's context window. Shorten it and try again.",
}

// A failure toast stays long enough to read; the compaction success notice
// has no timeout and stays until the user closes it.
const FAILURE_TOAST_MS = 10_000

interface ChatMessage {
  // Persisted id for resumed messages; client-generated for the optimistic
  // user message and the streamed assistant message. Used as the list key.
  id: string
  role: 'user' | 'assistant'
  content: string
  // Only ever set on a completed assistant message; undefined for the
  // opening turn (no retrieval) and for user messages.
  sources?: StudySource[]
}

function toChatMessage(message: StudyMessage): ChatMessage {
  return {
    id: message.id,
    role: message.role as ChatMessage['role'],
    content: message.content,
    sources: message.sources,
  }
}

// One finished message plus, when the reply used local knowledge, its sources
// strip. Memoized so a finished message never re-renders while another one
// streams: its props only change if the message itself does.
const MessageRow = memo(function MessageRow({
  message,
  onOpenCitation,
}: {
  message: ChatMessage
  onOpenCitation?: (source: StudySource) => void
}) {
  if (message.role === 'assistant' && message.sources && message.sources.length > 0) {
    return (
      <div className="flex flex-col items-start gap-1">
        <MessageBubble
          role={message.role}
          content={message.content}
          sources={message.sources}
          onOpenCitation={onOpenCitation}
        />
        <LocalSourcesStrip sources={message.sources} onOpenCitation={onOpenCitation} />
      </div>
    )
  }
  return <MessageBubble role={message.role} content={message.content} />
})

// The in-flight assistant turn. It owns the accumulated text, so each chunk
// re-renders only this component instead of the whole screen. The parent
// pushes chunks through `registerPush` and keeps its own ref for the final
// text handed to study:done.
function StreamingTurn({
  sources,
  registerPush,
  onContentChange,
  compacting,
}: {
  sources: StudySource[]
  registerPush: (push: ((text: string) => void) | null) => void
  onContentChange: () => void
  compacting: boolean
}) {
  const [text, setText] = useState('')

  useEffect(() => {
    registerPush(setText)
    return () => registerPush(null)
  }, [registerPush])

  useEffect(() => {
    onContentChange()
  }, [text, onContentChange])

  if (!text) {
    return <ThinkingIndicator caption={compacting ? 'Summarizing earlier messages…' : undefined} />
  }
  return <MessageBubble role="assistant" content={text} isStreaming sources={sources} />
}

// How close to the bottom of the transcript the user has to be for it to
// count as "following along", and keep auto-scrolling with the stream.
const STICKY_BOTTOM_THRESHOLD_PX = 80

// How far above the transcript's top edge the sentinel counts as reached, so
// the next page of earlier messages starts loading before the user hits it.
const HISTORY_TRIGGER_MARGIN_PX = 200

// The chat-style Study Mode screen: exchange messages with the LLM, whose
// reply streams in over Wails events (see lib/study.ts) rather than
// arriving all at once. Session creation itself now happens in the sidebar
// tree (see study-folder-tree.tsx) — this screen only ever renders once a
// session already exists, either freshly started or resumed from history.
// See specs/phases/phase-01-desktop-mvp/06-study-mode.md and
// specs/phases/phase-01-desktop-mvp/10-study-folders.md.
function StudyChatScreen({
  sessionId,
  initialTopic,
  mode,
  onTopicResolved,
  onStartNewSession,
  startingNewSession,
  sourceModeSlot,
  onOpenCitation,
}: StudyChatScreenProps) {
  const [sessionTopic, setSessionTopic] = useState(initialTopic)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [isStreaming, setIsStreaming] = useState(mode === 'new')
  const [sourceMode, setSourceMode] = useState<SourceMode>('notes')
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | null>(null)
  // Persistent context-limit state, restored on resume and updated live by
  // the study:context-* events below — unlike `error`, never cleared by the
  // next handleSend. See
  // specs/phases/phase-02-knowledge-engine/06-study-context-limits.md.
  const [contextState, setContextState] = useState<ContextState>('normal')
  // Numeric context occupancy behind the meter; the unresolved-limit case
  // is contextLength === 0. Never fed from the accumulated usage below.
  const [contextTokens, setContextTokens] = useState({
    usedTokens: 0,
    contextLength: 0,
    estimated: false,
  })
  // Accumulated consumption of the session's completed LLM calls; null until
  // the first fetch resolves or when it failed (the bar then shows "—").
  const [sessionUsage, setSessionUsage] = useState<StudySessionUsage | null>(null)
  const [contextUnavailable, setContextUnavailable] = useState<string | null>(null)
  // True from study:context-compacting until the compaction succeeds or fails.
  const [compacting, setCompacting] = useState(false)
  // Short reason of the last failed compaction, shown by the blocked alert;
  // null when none failed since the last success (or after a reload).
  const [compactionFailure, setCompactionFailure] = useState<string | null>(null)
  const [toasts, setToasts] = useState<ToastItem[]>([])
  // Mirrors contextState for event handlers subscribed once per session.
  const contextStateRef = useRef<ContextState>('normal')
  // Set when a study:compaction-failed arrives during a manual retry, so the
  // retry does not toast a second, generic failure on top of it.
  const retryFailureReportedRef = useRef(false)
  const streamingTextRef = useRef('')
  // Setter of the mounted StreamingTurn, if any; chunks are pushed through it.
  const streamPushRef = useRef<((text: string) => void) | null>(null)
  // Holds the sources emitted by "study:sources" for the turn currently in
  // flight, until "study:done" attaches them to the completed assistant
  // message and clears this back to empty.
  const pendingSourcesRef = useRef<StudySource[]>([])
  // Mirrors pendingSourcesRef as state, so the streaming bubble's own render
  // can resolve a citation before study:done — reading ref.current during
  // render is flagged by react-hooks/refs. onStudyDone still reads the ref
  // directly (a synchronous read, to attach sources to the persisted message
  // object); only the streaming bubble's render uses this state instead.
  const [streamingSources, setStreamingSources] = useState<StudySource[]>([])
  // Holds the draft text of the currently in-flight optimistic send, so a
  // study:error carrying a pre-persistence code (context_limit_reached,
  // turn_in_progress) can pop the optimistic bubble and restore it.
  const pendingSendRef = useRef<string | null>(null)
  // The unavailable notice shows at most once per mounted screen instance.
  const contextUnavailableShownRef = useRef(false)
  // Bumped by every study:context-* event. A usage snapshot fetched before a
  // newer event must not overwrite that event's state with older context.
  const contextVersionRef = useRef(0)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const transcriptRef = useRef<HTMLDivElement>(null)
  // Starts pinned so a resumed session opens on its most recent message.
  const followTranscriptRef = useRef(true)
  // Tracks the sessionId already sent to requestOpeningTurn, so React 18
  // StrictMode's dev-only double-invoke of this effect (mount → cleanup →
  // mount) doesn't fire two independent LLM generations for one session.
  const openingTurnRequestedRef = useRef<string | null>(null)
  // Empty element at the top of the transcript, watched to load earlier messages.
  const sentinelRef = useRef<HTMLDivElement>(null)
  // Scroll metrics captured just before an older page is prepended, consumed
  // by the layout effect below to keep the viewport on the same message.
  const pendingPrependRef = useRef<{ scrollHeight: number; scrollTop: number } | null>(null)
  const firstMessageIdRef = useRef<string | undefined>(undefined)

  // Older pages are added above the existing messages; one already shown
  // (same id) is skipped.
  const prependPage = useCallback((page: StudyMessage[]) => {
    const el = transcriptRef.current
    if (el) pendingPrependRef.current = { scrollHeight: el.scrollHeight, scrollTop: el.scrollTop }
    setMessages((previous) => {
      const known = new Set(previous.map((message) => message.id))
      const fresh = page.filter((message) => !known.has(message.id)).map(toChatMessage)
      return fresh.length === 0 ? previous : [...fresh, ...previous]
    })
  }, [])
  const {
    hasMore: hasEarlierMessages,
    status: earlierStatus,
    error: earlierError,
    start: startHistory,
    loadOlder: loadEarlierMessages,
  } = useMessageHistory(sessionId, prependPage)

  // Fetched on resume and again when a turn ends (study:done / study:error):
  // a failed turn may still have recorded its embedding / HyDE calls.
  const refreshSessionUsage = useCallback(() => {
    const versionAtRequest = contextVersionRef.current
    void fetchSessionUsage(sessionId).then((usage) => {
      if (!usage) return
      setSessionUsage(usage)
      if (contextVersionRef.current !== versionAtRequest) return
      // The persisted context is the source of truth after a turn:
      // study:context-* events only fire on state transitions, so a turn
      // that stays "normal" would otherwise leave the meter stale.
      setContextState(usage.context.state)
      setContextTokens(contextTokensFrom(usage.context))
    })
  }, [sessionId])

  const registerStreamPush = useCallback((push: ((text: string) => void) | null) => {
    streamPushRef.current = push
    // Replays chunks that arrived before the turn mounted: sendStudyMessage is
    // invoked before StreamingTurn's effect registers, and the binding may
    // already have emitted study:chunk by then.
    if (push) push(streamingTextRef.current)
  }, [])

  useEffect(() => {
    contextStateRef.current = contextState
  }, [contextState])

  const pushToast = useCallback((tone: ToastItem['tone'], message: string, durationMs?: number) => {
    setToasts((previous) => [...previous, { id: crypto.randomUUID(), tone, message, durationMs }])
  }, [])

  const dismissToast = useCallback((id: string) => {
    setToasts((previous) => previous.filter((toast) => toast.id !== id))
  }, [])

  function clearContextUnavailable() {
    contextUnavailableShownRef.current = false
    setContextUnavailable(null)
  }

  // Keeps the newest content in view as messages arrive and the answer
  // streams in — but only while the user is still parked at the bottom, so
  // scrolling up to re-read an earlier explanation isn't yanked back down by
  // the next chunk. StreamingTurn calls this on every chunk.
  const scrollToBottomIfFollowing = useCallback(() => {
    const el = transcriptRef.current
    if (!el || !followTranscriptRef.current) return
    el.scrollTop = el.scrollHeight
  }, [])

  // A prepended page must leave the viewport on the message the user was
  // reading, and must never drag them to the bottom; everything else (the
  // initial hydration, a sent message, the stream) follows the bottom.
  useLayoutEffect(() => {
    const pending = pendingPrependRef.current
    pendingPrependRef.current = null
    const firstId = messages[0]?.id
    const previousFirstId = firstMessageIdRef.current
    firstMessageIdRef.current = firstId
    const el = transcriptRef.current
    if (pending && el && previousFirstId !== undefined && firstId !== previousFirstId) {
      el.scrollTop = pending.scrollTop + (el.scrollHeight - pending.scrollHeight)
      return
    }
    scrollToBottomIfFollowing()
  }, [messages, scrollToBottomIfFollowing])

  // Loads the next older page whenever the sentinel is in view. The observer
  // is re-created after every load (status goes loading -> idle), which makes
  // it report the sentinel's current visibility again: a page too short to
  // push the sentinel out of view keeps loading without any scroll event. It
  // is not armed after a failure, so only the Retry button asks again.
  useEffect(() => {
    const sentinel = sentinelRef.current
    if (!sentinel || !hasEarlierMessages || earlierStatus !== 'idle') return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) void loadEarlierMessages()
      },
      { root: transcriptRef.current, rootMargin: `${HISTORY_TRIGGER_MARGIN_PX}px 0px 0px 0px` },
    )
    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [hasEarlierMessages, earlierStatus, loadEarlierMessages])

  // Focuses the reply textarea as soon as the chat view mounts, so the user
  // can start typing immediately instead of clicking into it first.
  useEffect(() => {
    textareaRef.current?.focus()
  }, [])

  useEffect(() => {
    if (mode === 'new') {
      if (openingTurnRequestedRef.current === sessionId) return
      openingTurnRequestedRef.current = sessionId
      requestOpeningTurn(sessionId, initialTopic).catch((err: unknown) => {
        setIsStreaming(false)
        setError(err instanceof Error ? err.message : 'Failed to start the session.')
      })
      return
    }
    let cancelled = false
    resumeStudySession(sessionId)
      .then((history) => {
        if (cancelled) return
        setSessionTopic(history.session.topic)
        onTopicResolved?.(history.session.topic)
        setContextState(history.session.context.state)
        setContextTokens({
          usedTokens: history.session.context.usedTokens,
          contextLength: history.session.context.contextLength,
          estimated: history.session.context.estimated,
        })
        refreshSessionUsage()
        setMessages(history.messages.map(toChatMessage))
        startHistory(history.hasMore, history.nextCursor)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(err instanceof Error ? err.message : 'Failed to load the session.')
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId])

  // Subscribed on mount, not gated on any streaming flag: the
  // "study:chunk"/"study:done" events for a session's opening turn are
  // emitted by the Go binding *before* requestOpeningTurn's own promise
  // resolves (it blocks until the whole stream finishes, then returns).
  // Every event carries a sessionId; one whose sessionId doesn't match the
  // session this screen instance displays is ignored, so a stale listener
  // from a just-abandoned session can never bleed into this one.
  useEffect(() => {
    const offChunk = onStudyChunk((event) => {
      if (event.sessionId !== sessionId) return
      streamingTextRef.current += event.content
      streamPushRef.current?.(streamingTextRef.current)
    })
    const offDone = onStudyDone((event) => {
      if (event.sessionId !== sessionId) return
      const content = streamingTextRef.current
      streamingTextRef.current = ''
      const sources = pendingSourcesRef.current
      pendingSourcesRef.current = []
      setStreamingSources([])
      pendingSendRef.current = null
      setMessages((previous) => [
        ...previous,
        { id: crypto.randomUUID(), role: 'assistant', content, sources },
      ])
      setIsStreaming(false)
      setCompacting(false)
      refreshSessionUsage()
    })
    const offError = onStudyError((event) => {
      if (event.sessionId !== sessionId) return
      streamingTextRef.current = ''
      setIsStreaming(false)
      setCompacting(false)
      refreshSessionUsage()
      // A pre-persistence error (the turn was rejected before the user
      // message was ever saved) only applies to a send actually in flight —
      // an opening turn never sets pendingSendRef, so it falls through to
      // the normal error path below with nothing to reconcile.
      if (PRE_PERSISTENCE_ERROR_CODES.has(event.code) && pendingSendRef.current !== null) {
        const restored = pendingSendRef.current
        pendingSendRef.current = null
        setMessages((previous) => previous.slice(0, -1))
        setDraft(restored)
        const rejection = REJECTION_MESSAGES[event.code]
        if (rejection) setError(rejection)
        return
      }
      pendingSendRef.current = null
      setError(event.message)
    })
    const offSources = onStudySources((event) => {
      if (event.sessionId !== sessionId) return
      pendingSourcesRef.current = event.sources
      setStreamingSources(event.sources)
    })
    const offContextNormal = onStudyContextNormal((event) => {
      if (event.sessionId !== sessionId) return
      contextVersionRef.current += 1
      setContextState('normal')
      setContextTokens(contextTokensFrom(event))
      clearContextUnavailable()
    })
    const offContextWarning = onStudyContextWarning((event) => {
      if (event.sessionId !== sessionId) return
      contextVersionRef.current += 1
      setContextState('warning')
      setContextTokens(contextTokensFrom(event))
      clearContextUnavailable()
    })
    const offContextLimitReached = onStudyContextLimitReached((event) => {
      if (event.sessionId !== sessionId) return
      contextVersionRef.current += 1
      setContextState('blocked')
      setContextTokens(contextTokensFrom(event))
      clearContextUnavailable()
    })
    const offContextLimitUnavailable = onStudyContextLimitUnavailable((event) => {
      if (event.sessionId !== sessionId) return
      if (contextUnavailableShownRef.current) return
      contextUnavailableShownRef.current = true
      setContextUnavailable(event.message)
    })
    const offCompacting = onStudyContextCompacting((event) => {
      if (event.sessionId !== sessionId) return
      setCompacting(true)
    })
    const offCompacted = onStudyContextCompacted((event) => {
      if (event.sessionId !== sessionId) return
      contextVersionRef.current += 1
      setCompacting(false)
      setCompactionFailure(null)
      setContextState('normal')
      setContextTokens({
        usedTokens: event.usedTokens,
        contextLength: event.contextLength,
        estimated: true,
      })
      pushToast('info', 'Older messages were summarized to keep the conversation going')
      // The persisted state is the source of truth (a compacted prompt can
      // still sit above 80% on a tiny window).
      refreshSessionUsage()
    })
    const offCompactionFailed = onStudyCompactionFailed((event) => {
      if (event.sessionId !== sessionId) return
      retryFailureReportedRef.current = true
      setCompacting(false)
      setCompactionFailure(event.reason)
      const continuation =
        contextStateRef.current === 'blocked' ? '' : ' Continuing with the full history.'
      pushToast(
        'warning',
        `Could not summarize the conversation (${event.reason}).${continuation}`,
        FAILURE_TOAST_MS,
      )
    })

    // First use of Wails runtime events in this codebase: EventsOn returns
    // its own unsubscribe function, which must run on unmount to avoid
    // leaking a listener.
    return () => {
      offChunk()
      offDone()
      offError()
      offSources()
      offContextNormal()
      offContextWarning()
      offContextLimitReached()
      offContextLimitUnavailable()
      offCompacting()
      offCompacted()
      offCompactionFailed()
    }
  }, [sessionId, refreshSessionUsage, pushToast])

  // The blocked alert's primary action: compaction progress and outcome come
  // back as events; a rejection without a failure event (nothing to compact,
  // a turn in progress) still needs to reach the user.
  async function handleRetryCompaction() {
    retryFailureReportedRef.current = false
    try {
      await compactStudySession(sessionId)
    } catch (err) {
      setCompacting(false)
      if (!retryFailureReportedRef.current) {
        pushToast(
          'warning',
          err instanceof Error ? err.message : 'Could not compact the conversation.',
          FAILURE_TOAST_MS,
        )
      }
    }
    refreshSessionUsage()
  }

  async function handleSend() {
    const content = draft.trim()
    if (!content) return
    pendingSendRef.current = content
    setMessages((previous) => [...previous, { id: crypto.randomUUID(), role: 'user', content }])
    setDraft('')
    setError(null)
    setIsStreaming(true)
    // The textarea's grown height is set directly on the DOM node (see
    // handleDraftChange), so clearing the draft alone wouldn't shrink it
    // back — 'auto' lets the CSS min-height take back over.
    if (textareaRef.current) textareaRef.current.style.height = 'auto'
    try {
      await sendStudyMessage(sessionId, sessionTopic, content, sourceMode)
    } catch (err) {
      setIsStreaming(false)
      setError(err instanceof Error ? err.message : 'Failed to send the message.')
    }
  }

  // Grows the textarea to fit its content, ChatGPT/Claude-style, instead of
  // staying a fixed height with an internal scrollbar — the icon buttons
  // docked at its bottom corners (see the JSX below) track along for free,
  // since they're positioned relative to this same element's box.
  function handleDraftChange(event: ChangeEvent<HTMLTextAreaElement>) {
    setDraft(event.target.value)
    const el = event.target
    el.style.height = 'auto'
    el.style.height = `${el.scrollHeight}px`
  }

  // Re-arms (or releases) the auto-scroll above whenever the user moves the
  // transcript by hand. Programmatic scrollTop writes fire this too, which is
  // harmless: they land at the bottom, so the flag just stays on.
  function handleTranscriptScroll(event: UIEvent<HTMLDivElement>) {
    const el = event.currentTarget
    const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
    followTranscriptRef.current = distanceFromBottom <= STICKY_BOTTOM_THRESHOLD_PX
  }

  function handleDraftKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== 'Enter' || event.shiftKey) return
    event.preventDefault()
    if (!draft.trim() || isStreaming) return
    void handleSend()
  }

  const sourceModeSelect = (
    <SourceModeSelect value={sourceMode} onValueChange={setSourceMode} disabled={isStreaming} />
  )

  return (
    <div className="flex h-full w-full flex-col gap-3">
      <ToastViewport toasts={toasts} onDismiss={dismissToast} />
      <SessionUsageBar context={{ state: contextState, ...contextTokens }} usage={sessionUsage} />
      <div
        ref={transcriptRef}
        onScroll={handleTranscriptScroll}
        role="log"
        aria-label="Conversation"
        className="thin-scroll flex min-h-0 flex-1 transform-gpu flex-col gap-3 overflow-y-auto pr-2 [overflow-anchor:none]"
      >
        {hasEarlierMessages && (
          <div
            ref={sentinelRef}
            aria-live="polite"
            className="flex min-h-px shrink-0 justify-center text-xs text-muted-foreground"
          >
            {earlierStatus === 'loading' && <span>Loading earlier messages…</span>}
            {earlierStatus === 'error' && (
              <div className="flex items-center gap-2">
                <span>{earlierError}</span>
                <Button variant="outline" size="sm" onClick={() => void loadEarlierMessages()}>
                  Retry
                </Button>
              </div>
            )}
          </div>
        )}
        {messages.map((message) => (
          <MessageRow key={message.id} message={message} onOpenCitation={onOpenCitation} />
        ))}
        {isStreaming && (
          <StreamingTurn
            sources={streamingSources}
            registerPush={registerStreamPush}
            onContentChange={scrollToBottomIfFollowing}
            compacting={compacting}
          />
        )}
      </div>
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {contextState === 'warning' && (
        <Alert>
          <AlertDescription className="text-orange-600 dark:text-orange-400">
            This session is approaching the model&apos;s context limit. Older messages will be
            summarized automatically when you send your next message.
          </AlertDescription>
        </Alert>
      )}
      {contextState === 'blocked' && (
        <Alert variant="destructive">
          <AlertDescription className="flex items-center justify-between gap-3">
            <span>
              {compactionFailure
                ? `Could not compact the conversation (${compactionFailure}). Try again.`
                : 'Could not compact the conversation. Try again.'}
            </span>
            <span className="flex shrink-0 gap-2">
              <Button
                size="sm"
                disabled={compacting || isStreaming}
                onClick={() => void handleRetryCompaction()}
              >
                {compacting ? 'Compacting…' : 'Try to compact again'}
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={startingNewSession}
                onClick={() => void onStartNewSession()}
              >
                Start new session
              </Button>
            </span>
          </AlertDescription>
        </Alert>
      )}
      {contextUnavailable && (
        <Alert>
          <AlertDescription className="flex items-center justify-between gap-3">
            <span>{contextUnavailable}</span>
            <Button size="sm" variant="ghost" onClick={() => setContextUnavailable(null)}>
              Dismiss
            </Button>
          </AlertDescription>
        </Alert>
      )}
      {/* Prefers AppShell's topbar slot (see its sourceModeSlot state) so the
          selector sits at the top of the page instead of floating over the
          composer; falls back to the composer when no slot is given (e.g.
          this screen rendered standalone, as in its own tests). */}
      {sourceModeSlot && createPortal(sourceModeSelect, sourceModeSlot)}
      <div className="relative">
        <Textarea
          ref={textareaRef}
          value={draft}
          onChange={handleDraftChange}
          onKeyDown={handleDraftKeyDown}
          placeholder="Type your answer..."
          className="min-h-24 max-h-[200px] resize-none overflow-y-auto pb-11"
        />
        {!sourceModeSlot && <div className="absolute bottom-2 left-2">{sourceModeSelect}</div>}
        <div className="absolute right-2 bottom-2 flex items-center gap-2">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                size="icon-sm"
                aria-label="Send"
                disabled={!draft.trim() || isStreaming}
                onClick={() => void handleSend()}
              >
                <ArrowUp className="size-4" aria-hidden="true" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Send message</TooltipContent>
          </Tooltip>
        </div>
      </div>
    </div>
  )
}

export default StudyChatScreen
