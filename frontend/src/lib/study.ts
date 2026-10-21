import {
  CompactStudySession,
  DeleteStudySession,
  GetStudySessionUsage,
  ListStudyMessagesBefore,
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
  goal: string
  startedAt: string
  context: StudyContextUsage
}

export interface StudyMessage {
  id: string
  role: string
  content: string
  createdAt: string
  sources: StudySource[]
}

// StudyMessagePage is one chronological page of a session's history.
// nextCursor is opaque: hand it back to listMessagesBefore to get the page
// of messages older than this one (only meaningful while hasMore is true).
export interface StudyMessagePage {
  messages: StudyMessage[]
  hasMore: boolean
  nextCursor: string
}

export interface StudySessionHistory extends StudyMessagePage {
  session: StudySession
}

// SourceMode controls whether and how a study turn consults local
// knowledge before answering. It is transient — passed per call, never
// stored or inferred from prior messages.
export type SourceMode = 'notes' | 'strict-notes'

export interface StudySource {
  chunkId: string
  itemId: string
  sourceType: string
  filePath: string
  heading: string
  concept: string
  score: number
  excerpt: string
}

// truncateBreadcrumb caps a " > "-joined heading breadcrumb to maxLength,
// keeping the last (most specific) segment intact and collapsing earlier
// segments behind a leading "… > " instead of cutting mid-segment — a
// breadcrumb like "Arquitetura > Monolito > Tradeoffs > Vantagens" stays
// readable even when the full chain would otherwise overflow the citation
// chip/strip it is shown in. When the leaf segment alone already fills the
// budget, it is hard-truncated with a trailing "…" instead — collapsing
// ancestors can't help there, and exceeding maxLength just to keep one
// long segment whole would defeat the point of capping it at all.
export function truncateBreadcrumb(breadcrumb: string, maxLength = 60): string {
  if (breadcrumb.length <= maxLength) {
    return breadcrumb
  }
  const ellipsisPrefix = '… > '
  const segments = breadcrumb.split(' > ')
  const lastSegment = segments[segments.length - 1]
  if (lastSegment.length >= maxLength) {
    return `${lastSegment.slice(0, Math.max(0, maxLength - 1))}…`
  }

  let result = lastSegment
  for (let i = segments.length - 2; i >= 0; i--) {
    const candidate = `${segments[i]} > ${result}`
    if (candidate.length + ellipsisPrefix.length > maxLength) {
      break
    }
    result = candidate
  }
  return result === breadcrumb ? result : `${ellipsisPrefix}${result}`
}

// Maps a Source to its display title/subtitle per
// specs/phases/phase-02-knowledge-engine/05-rag-integration.md's three
// source-type label variants. Used by LocalSourcesStrip, the per-message
// list of what backed a reply — the Sources panel instead lists a
// session's imported documents directly (see lib/sources.ts's
// SessionSource), which needs no such mapping.
export function sourceLabel(source: StudySource): { title: string; subtitle: string } {
  switch (source.sourceType) {
    case 'user_note':
      return { title: 'User note', subtitle: source.concept }
    case 'athena':
      return { title: 'Athena Knowledge', subtitle: source.concept }
    default:
      return { title: source.filePath, subtitle: truncateBreadcrumb(source.heading) }
  }
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

// StudyContextCompactedEvent is study:context-compacted's payload: the
// context usage after older messages were folded into the summary.
export interface StudyContextCompactedEvent {
  sessionId: string
  usedTokens: number
  contextLength: number
}

// StudyContextCompactingEvent is study:context-compacting's payload.
export interface StudyContextCompactingEvent {
  sessionId: string
}

// StudyCompactionFailedEvent is study:compaction-failed's payload. reason is
// a short user-facing kind ("invalid format", "provider unavailable", …),
// never a raw error.
export interface StudyCompactionFailedEvent {
  sessionId: string
  reason: string
}

// The generated StudyContextResult types `state` as a plain string (Wails
// has no way to carry the Go ContextState enum's exact values into the
// binding); the backend only ever sends 'normal' | 'warning' | 'blocked'.
function toStudySession(result: {
  id: string
  topic: string
  folderId: string
  goal: string
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
    goal: result.goal,
    startedAt: result.startedAt,
    context: { ...result.context, state: result.context.state as StudyContextUsage['state'] },
  }
}

// StudySessionUsage is the tokens and cost accumulated by every recorded LLM
// call of a session (chat, HyDE, embeddings) — never the context window
// occupancy. cost is a lower bound when unpricedCalls > 0; calls === 0 means
// nothing was recorded. Interrupted calls leave no record, so it only covers
// completed ones. See specs/phases/phase-02-knowledge-engine/22-usage-visibility.md.
export interface StudySessionUsage {
  inputTokens: number
  outputTokens: number
  cost: number
  unpricedCalls: number
  calls: number
  // The session's persisted context occupancy — a separate metric from the
  // totals above, returned alongside because study:context-* events only
  // fire on state transitions, not after every turn.
  context: StudyContextUsage
}

export async function getStudySessionUsage(sessionId: string): Promise<StudySessionUsage> {
  const result = await GetStudySessionUsage(sessionId)
  return {
    ...result,
    context: { ...result.context, state: result.context.state as StudyContextUsage['state'] },
  }
}

// folderId defaults to an empty string, which the backend falls back to
// the default folder for. goal is required by the backend.
export async function startStudySession(
  topic: string,
  goal: string,
  folderId = '',
): Promise<StudySession> {
  const result = await StartStudySession(topic, folderId, goal)
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

// compactStudySession folds the session's oldest messages into its summary
// (the blocked alert's retry action). Progress and the outcome arrive as
// study:context-compacting / -compacted / compaction-failed events; the
// promise rejects when nothing was compacted.
export async function compactStudySession(sessionId: string): Promise<void> {
  await CompactStudySession(sessionId)
}

function toStudyMessages(
  messages: {
    id: string
    role: string
    content: string
    createdAt: string
    sources: StudySource[]
  }[],
): StudyMessage[] {
  return messages.map((message) => ({
    id: message.id,
    role: message.role,
    content: message.content,
    createdAt: message.createdAt,
    sources: message.sources,
  }))
}

// resumeStudySession returns sessionId's latest page of messages, so the chat
// view can hydrate from it and let the user keep chatting; older messages come
// from listMessagesBefore via the returned nextCursor.
export async function resumeStudySession(sessionId: string): Promise<StudySessionHistory> {
  const result = await ResumeStudySession(sessionId)
  return {
    session: toStudySession(result.session),
    messages: toStudyMessages(result.messages),
    hasMore: result.hasMore,
    nextCursor: result.nextCursor,
  }
}

// listMessagesBefore returns the page of up to limit messages older than the
// message identified by cursor, in chronological order.
export async function listMessagesBefore(
  sessionId: string,
  cursor: string,
  limit: number,
): Promise<StudyMessagePage> {
  const result = await ListStudyMessagesBefore(sessionId, cursor, limit)
  return {
    messages: toStudyMessages(result.messages),
    hasMore: result.hasMore,
    nextCursor: result.nextCursor,
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

export function onStudyContextCompacting(
  handler: (event: StudyContextCompactingEvent) => void,
): () => void {
  return EventsOn('study:context-compacting', (event: StudyContextCompactingEvent) =>
    handler(event),
  )
}

export function onStudyContextCompacted(
  handler: (event: StudyContextCompactedEvent) => void,
): () => void {
  return EventsOn('study:context-compacted', (event: StudyContextCompactedEvent) => handler(event))
}

export function onStudyCompactionFailed(
  handler: (event: StudyCompactionFailedEvent) => void,
): () => void {
  return EventsOn('study:compaction-failed', (event: StudyCompactionFailedEvent) => handler(event))
}
