import {
  EndStudySession,
  RequestOpeningTurn,
  SendStudyMessage,
  StartStudySession,
} from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export interface StudySession {
  id: string
  topic: string
  startedAt: string
}

export async function startStudySession(topic: string): Promise<StudySession> {
  const result = await StartStudySession(topic)
  return { id: result.id, topic: result.topic, startedAt: result.startedAt }
}

// requestOpeningTurn streams the assistant's opening turn for a session
// already created via startStudySession. Kept as a separate call so the
// chat view can switch in immediately after startStudySession resolves,
// instead of waiting for the whole opening response before showing anything.
export async function requestOpeningTurn(sessionId: string, topic: string): Promise<void> {
  await RequestOpeningTurn(sessionId, topic)
}

export async function sendStudyMessage(sessionId: string, topic: string, content: string): Promise<void> {
  await SendStudyMessage(sessionId, topic, content)
}

export async function endStudySession(sessionId: string): Promise<void> {
  await EndStudySession(sessionId)
}

// EventsOn returns its own unsubscribe function. Callers must invoke it on
// unmount/session change to avoid leaking a listener across study sessions
// (the first use of Wails runtime events in this codebase).
export function onStudyChunk(handler: (chunk: string) => void): () => void {
  return EventsOn('study:chunk', (chunk: string) => handler(chunk))
}

export function onStudyDone(handler: () => void): () => void {
  return EventsOn('study:done', () => handler())
}

export function onStudyError(handler: (message: string) => void): () => void {
  return EventsOn('study:error', (message: string) => handler(message))
}
