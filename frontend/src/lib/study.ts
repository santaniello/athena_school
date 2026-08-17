import {
  DeleteStudySession,
  EndStudySession,
  ListStudySessionsByFolder,
  MoveStudySession,
  RequestOpeningTurn,
  ResumeStudySession,
  SendStudyMessage,
  StartStudySession,
} from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export interface StudySession {
  id: string
  topic: string
  folderId: string
  startedAt: string
  endedAt: string // empty when the session is still open
}

export interface StudyMessage {
  role: string
  content: string
  createdAt: string
}

export interface StudySessionHistory {
  session: StudySession
  messages: StudyMessage[]
}

function toStudySession(result: {
  id: string
  topic: string
  folderId: string
  startedAt: string
  endedAt: string
}): StudySession {
  return {
    id: result.id,
    topic: result.topic,
    folderId: result.folderId,
    startedAt: result.startedAt,
    endedAt: result.endedAt,
  }
}

// folderId defaults to an empty string, which the backend falls back to
// the default folder for.
export async function startStudySession(topic: string, folderId = ''): Promise<StudySession> {
  const result = await StartStudySession(topic, folderId)
  return toStudySession(result)
}

// requestOpeningTurn streams the assistant's opening turn for a session
// already created via startStudySession. Kept as a separate call so the
// chat view can switch in immediately after startStudySession resolves,
// instead of waiting for the whole opening response before showing anything.
export async function requestOpeningTurn(sessionId: string, topic: string): Promise<void> {
  await RequestOpeningTurn(sessionId, topic)
}

export async function sendStudyMessage(
  sessionId: string,
  topic: string,
  content: string,
): Promise<void> {
  await SendStudyMessage(sessionId, topic, content)
}

export async function endStudySession(sessionId: string): Promise<void> {
  await EndStudySession(sessionId)
}

// deleteStudySession permanently deletes sessionId and every message in
// it. Unlike endStudySession, this cannot be undone.
export async function deleteStudySession(sessionId: string): Promise<void> {
  await DeleteStudySession(sessionId)
}

// resumeStudySession reopens sessionId if it had been ended and returns its
// full message history, so the chat view can hydrate from it and let the
// user keep chatting.
export async function resumeStudySession(sessionId: string): Promise<StudySessionHistory> {
  const result = await ResumeStudySession(sessionId)
  return {
    session: toStudySession(result.session),
    messages: result.messages.map((message) => ({
      role: message.role,
      content: message.content,
      createdAt: message.createdAt,
    })),
  }
}

export async function moveStudySession(sessionId: string, folderId: string): Promise<void> {
  await MoveStudySession(sessionId, folderId)
}

export async function listStudySessionsByFolder(folderId: string): Promise<StudySession[]> {
  const results = await ListStudySessionsByFolder(folderId)
  return results.map(toStudySession)
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
