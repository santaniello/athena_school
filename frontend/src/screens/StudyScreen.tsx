import { useEffect, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { MessageBubble } from '@/components/message-bubble'
import {
  endStudySession,
  onStudyChunk,
  onStudyDone,
  onStudyError,
  requestOpeningTurn,
  sendStudyMessage,
  startStudySession,
} from '@/lib/study'

interface StudyScreenProps {
  onEndSession?: () => void
}

interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
}

// The chat-style Study Mode screen: pick a topic, then exchange messages
// with the LLM, whose reply streams in over Wails events (see lib/study.ts)
// rather than arriving all at once. See
// specs/phases/phase-01-desktop-mvp/06-study-mode.md.
function StudyScreen({ onEndSession }: StudyScreenProps) {
  const [topic, setTopic] = useState('')
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [sessionTopic, setSessionTopic] = useState('')
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [streamingText, setStreamingText] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | null>(null)
  const streamingTextRef = useRef('')

  // Subscribed once on mount, not gated on sessionId: the "study:chunk"/
  // "study:done" events for a session's opening turn are emitted by the Go
  // binding *before* StartStudySession's own promise resolves (it blocks
  // until the whole stream finishes, then returns). Subscribing only after
  // sessionId is set — which happens only once that promise resolves —
  // would always miss the opening turn's events entirely, leaving
  // isStreaming stuck true and the send button permanently disabled.
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

  async function handleStart() {
    const trimmedTopic = topic.trim()
    if (!trimmedTopic) return
    setError(null)
    setIsStreaming(true)
    try {
      const session = await startStudySession(trimmedTopic)
      // Switch to the chat view as soon as the (fast, non-streaming)
      // session is created — before requesting the opening turn — so the
      // streaming reply is actually visible instead of the whole response
      // appearing at once after a long wait.
      setSessionId(session.id)
      setSessionTopic(session.topic)
      await requestOpeningTurn(session.id, session.topic)
    } catch (err) {
      setIsStreaming(false)
      setError(err instanceof Error ? err.message : 'Failed to start the session.')
    }
  }

  async function handleSend() {
    const content = draft.trim()
    if (!sessionId || !content) return
    setMessages((previous) => [...previous, { role: 'user', content }])
    setDraft('')
    setError(null)
    setIsStreaming(true)
    try {
      await sendStudyMessage(sessionId, sessionTopic, content)
    } catch (err) {
      setIsStreaming(false)
      setError(err instanceof Error ? err.message : 'Failed to send the message.')
    }
  }

  function handleDraftKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== 'Enter' || event.shiftKey) return
    event.preventDefault()
    if (!draft.trim() || isStreaming) return
    void handleSend()
  }

  async function handleEnd() {
    if (sessionId) await endStudySession(sessionId)
    onEndSession?.()
  }

  if (!sessionId) {
    return (
      <div className="m-auto flex w-full max-w-md flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="study-topic">What do you want to study today?</Label>
          <Input
            id="study-topic"
            value={topic}
            onChange={(event) => setTopic(event.target.value)}
            placeholder="e.g. Distributed systems"
          />
        </div>
        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <Button disabled={!topic.trim() || isStreaming} onClick={() => void handleStart()}>
          Start session
        </Button>
      </div>
    )
  }

  return (
    <div className="flex h-full w-full flex-col gap-4">
      <div className="flex flex-1 flex-col gap-3 overflow-y-auto">
        {messages.map((message, index) => (
          <MessageBubble key={index} role={message.role} content={message.content} />
        ))}
        {isStreaming && streamingText && <MessageBubble role="assistant" content={streamingText} />}
      </div>
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <div className="flex gap-2">
        <Textarea
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={handleDraftKeyDown}
          placeholder="Type your answer..."
          className="flex-1"
        />
        <Button disabled={!draft.trim() || isStreaming} onClick={() => void handleSend()}>
          Send
        </Button>
      </div>
      <Button variant="outline" onClick={() => void handleEnd()}>
        End session
      </Button>
    </div>
  )
}

export default StudyScreen
