import { useEffect, useRef, useState } from 'react'
import type { ChangeEvent, KeyboardEvent, UIEvent } from 'react'
import { ArrowUp } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { MessageBubble } from '@/components/message-bubble'
import { ThinkingIndicator } from '@/components/thinking-indicator'
import { KnowledgeExtractionDialog } from '@/components/knowledge-extraction-dialog'
import { TranscriptTruncationDialog } from '@/components/transcript-truncation-dialog'
import { SourceModeSelect } from '@/components/source-mode-select'
import { LocalSourcesStrip } from '@/components/local-sources-strip'

const TRANSCRIPT_TOO_LARGE_ERROR = 'no complete transcript message fits within the extraction limit'

function extractionErrorMessage(error: unknown): string {
  if (!(error instanceof Error)) return 'Failed to extract knowledge.'
  if (error.message.includes(TRANSCRIPT_TOO_LARGE_ERROR)) {
    return 'The most recent message is too large to process in full.'
  }
  return error.message
}
import { extractKnowledge, type KnowledgeItem } from '@/lib/knowledge'
import {
  onStudyChunk,
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
  // Fired after saving extracted candidates as drafts, so AppShell can
  // refresh the sidebar/Review-tab badge without a reload. See
  // specs/phases/phase-02-knowledge-engine/07-knowledge-review.md.
  onKnowledgeChanged?: () => void
}

// ContextState mirrors the persisted study.ContextState the backend tracks
// per session (see StudyContextUsage in lib/study.ts).
type ContextState = 'normal' | 'warning' | 'blocked'

// Error codes carried by StudyErrorEvent that mean the failure happened
// before the user message was persisted — see StudyErrorEvent.code in
// internal/interfaces/desktop/study.go.
const PRE_PERSISTENCE_ERROR_CODES = new Set(['context_limit_reached', 'turn_in_progress'])

interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  // Only ever set on a completed assistant message; undefined for the
  // opening turn (no retrieval) and for user messages.
  sources?: StudySource[]
}

// How close to the bottom of the transcript the user has to be for it to
// count as "following along", and keep auto-scrolling with the stream.
const STICKY_BOTTOM_THRESHOLD_PX = 80

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
  onKnowledgeChanged,
}: StudyChatScreenProps) {
  const [sessionTopic, setSessionTopic] = useState(initialTopic)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [streamingText, setStreamingText] = useState('')
  const [isStreaming, setIsStreaming] = useState(mode === 'new')
  const [sourceMode, setSourceMode] = useState<SourceMode>('notes')
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [extractionError, setExtractionError] = useState<string | null>(null)
  const [isExtracting, setIsExtracting] = useState(false)
  const [extractedItems, setExtractedItems] = useState<KnowledgeItem[]>([])
  const [showExtractionDialog, setShowExtractionDialog] = useState(false)
  const [showTruncationDialog, setShowTruncationDialog] = useState(false)
  // Persistent context-limit state, restored on resume and updated live by
  // the study:context-* events below — unlike `error`, never cleared by the
  // next handleSend. See
  // specs/phases/phase-02-knowledge-engine/06-study-context-limits.md.
  const [contextState, setContextState] = useState<ContextState>('normal')
  const [contextUnavailable, setContextUnavailable] = useState<string | null>(null)
  const streamingTextRef = useRef('')
  // Holds the sources emitted by "study:sources" for the turn currently in
  // flight, until "study:done" attaches them to the completed assistant
  // message and clears this back to empty.
  const pendingSourcesRef = useRef<StudySource[]>([])
  // Holds the draft text of the currently in-flight optimistic send, so a
  // study:error carrying a pre-persistence code (context_limit_reached,
  // turn_in_progress) can pop the optimistic bubble and restore it.
  const pendingSendRef = useRef<string | null>(null)
  // The unavailable notice shows at most once per mounted screen instance.
  const contextUnavailableShownRef = useRef(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const transcriptRef = useRef<HTMLDivElement>(null)
  // Starts pinned so a resumed session opens on its most recent message.
  const followTranscriptRef = useRef(true)
  // Tracks the sessionId already sent to requestOpeningTurn, so React 18
  // StrictMode's dev-only double-invoke of this effect (mount → cleanup →
  // mount) doesn't fire two independent LLM generations for one session.
  const openingTurnRequestedRef = useRef<string | null>(null)

  function clearContextUnavailable() {
    contextUnavailableShownRef.current = false
    setContextUnavailable(null)
  }

  // Keeps the newest content in view as messages arrive and the answer
  // streams in — but only while the user is still parked at the bottom, so
  // scrolling up to re-read an earlier explanation isn't yanked back down by
  // the next chunk.
  useEffect(() => {
    const el = transcriptRef.current
    if (!el || !followTranscriptRef.current) return
    el.scrollTop = el.scrollHeight
  }, [messages, streamingText])

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
    resumeStudySession(sessionId)
      .then((history) => {
        setSessionTopic(history.session.topic)
        onTopicResolved?.(history.session.topic)
        setContextState(history.session.context.state)
        setMessages(
          history.messages.map((message) => ({
            role: message.role as ChatMessage['role'],
            content: message.content,
          })),
        )
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : 'Failed to load the session.')
      })
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
      setStreamingText(streamingTextRef.current)
    })
    const offDone = onStudyDone((event) => {
      if (event.sessionId !== sessionId) return
      const content = streamingTextRef.current
      streamingTextRef.current = ''
      const sources = pendingSourcesRef.current
      pendingSourcesRef.current = []
      pendingSendRef.current = null
      setMessages((previous) => [...previous, { role: 'assistant', content, sources }])
      setStreamingText('')
      setIsStreaming(false)
    })
    const offError = onStudyError((event) => {
      if (event.sessionId !== sessionId) return
      streamingTextRef.current = ''
      setStreamingText('')
      setIsStreaming(false)
      // A pre-persistence error (the turn was rejected before the user
      // message was ever saved) only applies to a send actually in flight —
      // an opening turn never sets pendingSendRef, so it falls through to
      // the normal error path below with nothing to reconcile.
      if (PRE_PERSISTENCE_ERROR_CODES.has(event.code) && pendingSendRef.current !== null) {
        const restored = pendingSendRef.current
        pendingSendRef.current = null
        setMessages((previous) => previous.slice(0, -1))
        setDraft(restored)
        return
      }
      pendingSendRef.current = null
      setError(event.message)
    })
    const offSources = onStudySources((event) => {
      if (event.sessionId !== sessionId) return
      pendingSourcesRef.current = event.sources
    })
    const offContextNormal = onStudyContextNormal((event) => {
      if (event.sessionId !== sessionId) return
      setContextState('normal')
      clearContextUnavailable()
    })
    const offContextWarning = onStudyContextWarning((event) => {
      if (event.sessionId !== sessionId) return
      setContextState('warning')
      clearContextUnavailable()
    })
    const offContextLimitReached = onStudyContextLimitReached((event) => {
      if (event.sessionId !== sessionId) return
      setContextState('blocked')
      clearContextUnavailable()
    })
    const offContextLimitUnavailable = onStudyContextLimitUnavailable((event) => {
      if (event.sessionId !== sessionId) return
      if (contextUnavailableShownRef.current) return
      contextUnavailableShownRef.current = true
      setContextUnavailable(event.message)
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
    }
  }, [sessionId])

  async function handleSend() {
    const content = draft.trim()
    if (!content) return
    pendingSendRef.current = content
    setMessages((previous) => [...previous, { role: 'user', content }])
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

  async function handleExtractKnowledge(confirmedTruncation: boolean) {
    if (isExtracting || messages.length === 0 || isStreaming) return
    setIsExtracting(true)
    setExtractionError(null)
    try {
      const result = await extractKnowledge(sessionId, confirmedTruncation)
      if (result.truncated && !confirmedTruncation) {
        setShowTruncationDialog(true)
        return
      }
      setExtractedItems(result.items)
      setShowExtractionDialog(true)
    } catch (err) {
      setExtractionError(extractionErrorMessage(err))
    } finally {
      setIsExtracting(false)
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
    if (!draft.trim() || isStreaming || contextState === 'blocked') return
    void handleSend()
  }

  return (
    <div className="flex h-full w-full flex-col gap-3">
      <div
        ref={transcriptRef}
        onScroll={handleTranscriptScroll}
        role="log"
        aria-label="Conversation"
        className="thin-scroll flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto pr-2"
      >
        {messages.map((message, index) =>
          message.role === 'assistant' && message.sources && message.sources.length > 0 ? (
            <div key={index} className="flex flex-col items-start gap-1">
              <MessageBubble role={message.role} content={message.content} />
              <LocalSourcesStrip sources={message.sources} />
            </div>
          ) : (
            <MessageBubble key={index} role={message.role} content={message.content} />
          ),
        )}
        {isStreaming && !streamingText && <ThinkingIndicator />}
        {isStreaming && streamingText && (
          <MessageBubble role="assistant" content={streamingText} isStreaming />
        )}
      </div>
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {extractionError && (
        <Alert variant="destructive">
          <AlertDescription>{extractionError}</AlertDescription>
        </Alert>
      )}
      {contextState === 'warning' && (
        <Alert>
          <AlertDescription className="flex items-center justify-between gap-3">
            <span>
              This session is approaching the model&apos;s context limit. Start a new session on the
              same topic to keep responses reliable.
            </span>
            <Button
              size="sm"
              variant="outline"
              disabled={startingNewSession}
              onClick={() => void onStartNewSession()}
            >
              Start new session
            </Button>
          </AlertDescription>
        </Alert>
      )}
      {contextState === 'blocked' && (
        <Alert variant="destructive">
          <AlertDescription className="flex items-center justify-between gap-3">
            <span>
              This session has reached its context limit. Start a new session on the same topic to
              continue.
            </span>
            <Button
              size="sm"
              variant="outline"
              disabled={startingNewSession}
              onClick={() => void onStartNewSession()}
            >
              Start new session
            </Button>
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
      <div className="relative">
        <Textarea
          ref={textareaRef}
          value={draft}
          onChange={handleDraftChange}
          onKeyDown={handleDraftKeyDown}
          readOnly={contextState === 'blocked'}
          placeholder="Type your answer..."
          className="min-h-24 max-h-[200px] resize-none overflow-y-auto pb-11"
        />
        <div className="absolute bottom-2 left-2">
          <SourceModeSelect
            value={sourceMode}
            onValueChange={setSourceMode}
            disabled={isStreaming || contextState === 'blocked'}
          />
        </div>
        <div className="absolute right-2 bottom-2 flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            aria-label={isExtracting ? 'Extracting knowledge' : 'Extract knowledge'}
            disabled={messages.length === 0 || isStreaming || isExtracting}
            onClick={() => void handleExtractKnowledge(false)}
          >
            {isExtracting ? 'Extracting...' : 'Extract knowledge'}
          </Button>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                size="icon-sm"
                aria-label="Send"
                disabled={!draft.trim() || isStreaming || contextState === 'blocked'}
                onClick={() => void handleSend()}
              >
                <ArrowUp className="size-4" aria-hidden="true" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Send message</TooltipContent>
          </Tooltip>
        </div>
      </div>
      <TranscriptTruncationDialog
        open={showTruncationDialog}
        onDecline={() => setShowTruncationDialog(false)}
        onConfirm={() => {
          setShowTruncationDialog(false)
          void handleExtractKnowledge(true)
        }}
      />
      {showExtractionDialog && (
        <KnowledgeExtractionDialog
          open
          items={extractedItems}
          onClose={() => setShowExtractionDialog(false)}
          onKnowledgeChanged={onKnowledgeChanged}
        />
      )}
    </div>
  )
}

export default StudyChatScreen
