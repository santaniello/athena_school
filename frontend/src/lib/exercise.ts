import {
  CancelChallengeGeneration,
  GenerateChallenge,
  ListChallengeCatalog,
  ListChallengeTopics,
  ListSessionsForGeneration,
} from '../../wailsjs/go/desktop/App'

// The Architecture Lab catalog. The frontend only renders what the backend
// returns: statuses and counts are derived there. See
// specs/phases/phase-03-architecture-lab/07-challenge-catalog-ui.md.

export type ChallengeStatus = 'NOT_STARTED' | 'IN_PROGRESS' | 'NEEDS_IMPROVEMENT' | 'COMPLETED'
export type ChallengeDifficulty = 'BEGINNER' | 'INTERMEDIATE' | 'ADVANCED'

export const STATUS_LABELS: Record<ChallengeStatus, string> = {
  NOT_STARTED: 'Not started',
  IN_PROGRESS: 'In progress',
  NEEDS_IMPROVEMENT: 'Needs improvement',
  COMPLETED: 'Completed',
}

export const DIFFICULTY_LABELS: Record<ChallengeDifficulty, string> = {
  BEGINNER: 'Beginner',
  INTERMEDIATE: 'Intermediate',
  ADVANCED: 'Advanced',
}

// An unset field does not filter.
export interface CatalogFilter {
  query?: string
  status?: ChallengeStatus | ''
  difficulty?: ChallengeDifficulty | ''
  topic?: string
}

// A row shows only what does not give the exam away.
export interface CatalogItem {
  id: string
  title: string
  topic: string
  difficulty: string
  timeLimitMinutes: number
  status: string
  attempts: number
  bestDiagramScore: number
}

export interface Catalog {
  total: number
  completed: number
  inProgress: number
  items: CatalogItem[]
}

export interface SessionOption {
  id: string
  topic: string
  folderName: string
  startedAt: string
}

export interface GeneratedChallenge {
  id: string
  title: string
  topic: string
  difficulty: string
  timeLimitMinutes: number
}

export async function listChallengeCatalog(filter: CatalogFilter): Promise<Catalog> {
  const catalog = await ListChallengeCatalog({
    query: filter.query ?? '',
    status: filter.status ?? '',
    difficulty: filter.difficulty ?? '',
    topic: filter.topic ?? '',
  })
  return {
    total: catalog.total,
    completed: catalog.completed,
    inProgress: catalog.inProgress,
    items: catalog.items.map((item) => ({ ...item })),
  }
}

export function listChallengeTopics(): Promise<string[]> {
  return ListChallengeTopics()
}

export async function listSessionsForGeneration(): Promise<SessionOption[]> {
  const sessions = await ListSessionsForGeneration()
  return sessions.map((session) => ({ ...session }))
}

export async function generateChallenge(
  sessionId: string,
  difficulty: ChallengeDifficulty,
): Promise<GeneratedChallenge> {
  const created = await GenerateChallenge(sessionId, difficulty)
  return {
    id: created.id,
    title: created.title,
    topic: created.topic,
    difficulty: created.difficulty,
    timeLimitMinutes: created.timeLimitMinutes,
  }
}

export function cancelChallengeGeneration(): Promise<void> {
  return CancelChallengeGeneration()
}

// Wails rejects with the Go error's text, which for the exercise bindings is
// {"code": "...", "message": "..."} (see ExerciseError). Returns the code, or
// '' when the rejection is not one.
export function exerciseErrorCode(err: unknown): string {
  const raw = err instanceof Error ? err.message : typeof err === 'string' ? err : ''
  try {
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed === 'object' && parsed !== null && 'code' in parsed) {
      return typeof parsed.code === 'string' ? parsed.code : ''
    }
  } catch {
    // not JSON: not a coded error
  }
  return ''
}

const GENERIC_MESSAGE = 'Something went wrong. Please try again.'

const ERROR_MESSAGES: Record<string, string> = {
  invalid_generation: 'The generated challenge was not valid. Try again.',
  source_session_not_found: 'That study session no longer exists. Pick another one.',
  generation_in_progress: 'A challenge is already being generated.',
}

export function exerciseErrorMessage(err: unknown): string {
  return ERROR_MESSAGES[exerciseErrorCode(err)] ?? GENERIC_MESSAGE
}
