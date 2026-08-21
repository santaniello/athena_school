import {
  DeleteStudySession,
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

// SourceMode controls whether and how a study turn consults local
// knowledge before answering. It is transient — passed per call, never
// stored or inferred from prior messages.
export type SourceMode = 'notes' | 'strict-notes' | 'web'

export interface StudySource {
  sourceType: string
  filePath: string
  heading: string
  concept: string
  score: number
}

export interface StudyChunkEvent {
  sessionId: string
  content: string
}

export interface StudyDoneEvent {
  sessionId: string
}

export interface StudyErrorEvent {
  sessionId: string
  message: string
}

export interface StudySourcesEvent {
  sessionId: string
  sources: StudySource[]
}

function toStudySession(result: {
  id: string
  topic: string
  folderId: string
  startedAt: string
}): StudySession {
  return {
    id: result.id,
    topic: result.topic,
    folderId: result.folderId,
    startedAt: result.startedAt,
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
  sourceMode: SourceMode,
): Promise<void> {
  await SendStudyMessage(sessionId, topic, content, sourceMode)
}

// deleteStudySession permanently deletes sessionId and every message in it.
// This cannot be undone.
export async function deleteStudySession(sessionId: string): Promise<void> {
  await DeleteStudySession(sessionId)
}

// resumeStudySession returns sessionId's full message history, so the chat
// view can hydrate from it and let the user keep chatting.
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
// (the first use of Wails runtime events in this codebase). Every study
// event is session-scoped; filtering by the currently displayed session is
// the caller's responsibility (see StudyChatScreen), not this thin adapter.
export function onStudyChunk(handler: (event: StudyChunkEvent) => void): () => void {
  return EventsOn('study:chunk', (event: StudyChunkEvent) => handler(event))
}

export function onStudyDone(handler: (event: StudyDoneEvent) => void): () => void {
  return EventsOn('study:done', (event: StudyDoneEvent) => handler(event))
}

export function onStudyError(handler: (event: StudyErrorEvent) => void): () => void {
  return EventsOn('study:error', (event: StudyErrorEvent) => handler(event))
}

export function onStudySources(handler: (event: StudySourcesEvent) => void): () => void {
  return EventsOn('study:sources', (event: StudySourcesEvent) => handler(event))
}
