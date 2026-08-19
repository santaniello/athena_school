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
import { extractKnowledge, type KnowledgeItem } from '@/lib/knowledge'
import {
  onStudyChunk,
  onStudyDone,
  onStudyError,
  requestOpeningTurn,
  resumeStudySession,
  sendStudyMessage,
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
}

interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
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
function StudyChatScreen({ sessionId, initialTopic, mode, onTopicResolved }: StudyChatScreenProps) {
  const [sessionTopic, setSessionTopic] = useState(initialTopic)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [streamingText, setStreamingText] = useState('')
  const [isStreaming, setIsStreaming] = useState(mode === 'new')
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [extractionError, setExtractionError] = useState<string | null>(null)
  const [isExtracting, setIsExtracting] = useState(false)
  const [extractedItems, setExtractedItems] = useState<KnowledgeItem[]>([])
  const [showExtractionDialog, setShowExtractionDialog] = useState(false)
  const [showTruncationDialog, setShowTruncationDialog] = useState(false)
  const streamingTextRef = useRef('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const transcriptRef = useRef<HTMLDivElement>(null)
  // Starts pinned so a resumed session opens on its most recent message.
  const followTranscriptRef = useRef(true)
  // Tracks the sessionId already sent to requestOpeningTurn, so React 18
  // StrictMode's dev-only double-invoke of this effect (mount → cleanup →
  // mount) doesn't fire two independent LLM generations for one session.
  const openingTurnRequestedRef = useRef<string | null>(null)

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
  useEffect(() => {
    const offChunk = onStudyChunk((chunk) => {
      streamingTextRef.current += chunk
      setStreamingText(streamingTextRef.current)
    })
    const offDone = onStudyDone(() => {
      const content = streamingTextRef.current
      streamingTextRef.current = ''
      setMessages((previous) => [...previous, { role: 'assistant', content }])
      setStreamingText('')
      setIsStreaming(false)
    })
    const offError = onStudyError((message) => {
      streamingTextRef.current = ''
      setStreamingText('')
      setIsStreaming(false)
      setError(message)
    })

    // First use of Wails runtime events in this codebase: EventsOn returns
    // its own unsubscribe function, which must run on unmount to avoid
    // leaking a listener.
    return () => {
      offChunk()
      offDone()
      offError()
    }
  }, [])

  async function handleSend() {
    const content = draft.trim()
    if (!content) return
    setMessages((previous) => [...previous, { role: 'user', content }])
    setDraft('')
    setError(null)
    setIsStreaming(true)
    // The textarea's grown height is set directly on the DOM node (see
    // handleDraftChange), so clearing the draft alone wouldn't shrink it
    // back — 'auto' lets the CSS min-height take back over.
    if (textareaRef.current) textareaRef.current.style.height = 'auto'
    try {
      await sendStudyMessage(sessionId, sessionTopic, content)
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
      setExtractionError(err instanceof Error ? err.message : 'Falha ao extrair conhecimento.')
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
    if (!draft.trim() || isStreaming) return
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
        {messages.map((message, index) => (
          <MessageBubble key={index} role={message.role} content={message.content} />
        ))}
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
      <div className="relative">
        <Textarea
          ref={textareaRef}
          value={draft}
          onChange={handleDraftChange}
          onKeyDown={handleDraftKeyDown}
          placeholder="Type your answer..."
          className="min-h-24 max-h-[200px] resize-none overflow-y-auto pb-11"
        />
        <div className="absolute right-2 bottom-2 flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            aria-label={isExtracting ? 'Extraindo conhecimento' : 'Extrair conhecimento'}
            disabled={messages.length === 0 || isStreaming || isExtracting}
            onClick={() => void handleExtractKnowledge(false)}
          >
            {isExtracting ? 'Extraindo...' : 'Extrair conhecimento'}
          </Button>
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
        />
      )}
    </div>
  )
}

export default StudyChatScreen
