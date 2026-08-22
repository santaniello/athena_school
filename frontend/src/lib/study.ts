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

// StudyContextUsage is a session's last-measured occupancy of its model's
// context window. See specs/phases/phase-02-knowledge-engine/06-study-context-limits.md.
export interface StudyContextUsage {
  state: 'normal' | 'warning' | 'blocked'
  model: string
  usedTokens: number
  contextLength: number
  estimated: boolean
}

export interface StudySession {
  id: string
  topic: string
  folderId: string
  startedAt: string
  context: StudyContextUsage
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
  code: string
}

export interface StudySourcesEvent {
  sessionId: string
  sources: StudySource[]
}

// StudyContextEvent is the payload of study:context-normal/warning/
// limit-reached.
export interface StudyContextEvent {
  sessionId: string
  usedTokens: number
  contextLength: number
  estimated: boolean
}

// StudyContextUnavailableEvent is study:context-limit-unavailable's
// payload: transient technical feedback, not persisted context state.
export interface StudyContextUnavailableEvent {
  sessionId: string
  message: string
}

// The generated StudyContextResult types `state` as a plain string (Wails
// has no way to carry the Go ContextState enum's exact values into the
// binding); the backend only ever sends 'normal' | 'warning' | 'blocked'.
function toStudySession(result: {
  id: string
  topic: string
  folderId: string
  startedAt: string
  context: {
    state: string
    model: string
    usedTokens: number
    contextLength: number
    estimated: boolean
  }
}): StudySession {
  return {
    id: result.id,
    topic: result.topic,
    folderId: result.folderId,
    startedAt: result.startedAt,
    context: { ...result.context, state: result.context.state as StudyContextUsage['state'] },
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

export function onStudyContextNormal(handler: (event: StudyContextEvent) => void): () => void {
  return EventsOn('study:context-normal', (event: StudyContextEvent) => handler(event))
}

export function onStudyContextWarning(handler: (event: StudyContextEvent) => void): () => void {
  return EventsOn('study:context-warning', (event: StudyContextEvent) => handler(event))
}

export function onStudyContextLimitReached(
  handler: (event: StudyContextEvent) => void,
): () => void {
  return EventsOn('study:context-limit-reached', (event: StudyContextEvent) => handler(event))
}

export function onStudyContextLimitUnavailable(
  handler: (event: StudyContextUnavailableEvent) => void,
): () => void {
  return EventsOn('study:context-limit-unavailable', (event: StudyContextUnavailableEvent) =>
    handler(event),
  )
}
