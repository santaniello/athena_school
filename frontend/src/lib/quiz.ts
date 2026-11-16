import {
  AcceptQuizOffer,
  CancelQuizGeneration,
  DeclineQuizOffer,
  DeleteQuiz,
  GenerateQuiz,
  GetPendingQuizOffer,
  GetQuiz,
  GetQuizAttemptResult,
  ListQuizAttempts,
  ListQuizzes,
  ListSessionQuizzes,
  SubmitQuizAttempt,
} from '../../wailsjs/go/desktop/App'
import i18n, { QUIZ_NAMESPACE } from '@/i18n'

// The Interactive Quiz. The backend grades and stores everything; this module
// only wraps its bindings and holds the few pure helpers the screens share. See
// specs/phases/phase-03-architecture-lab/22-interactive-quiz-screen.md.

export type QuizDifficulty = 'EASY' | 'MEDIUM' | 'HARD'
export const QUIZ_DIFFICULTIES: QuizDifficulty[] = ['EASY', 'MEDIUM', 'HARD']

// The number of questions a quiz can have, as the backend enforces it.
export const MIN_QUESTIONS = 3
export const MAX_QUESTIONS = 20
export const DEFAULT_QUESTIONS = 10

// A count field is text while it is typed; only a whole number inside the
// backend's range is a valid count, and anything else is null.
export function parseQuestionCount(text: string): number | null {
  if (!/^\d+$/.test(text)) return null
  const count = Number(text)
  return count >= MIN_QUESTIONS && count <= MAX_QUESTIONS ? count : null
}

// A quiz as the catalog lists it. latestScore is null until it has been taken.
export interface QuizSummary {
  id: string
  sessionId: string
  sessionTitle: string
  difficulty: string
  questionCount: number
  // ISO timestamp.
  createdAt: string
  attemptCount: number
  latestScore: number | null
  // The study-chat message whose offer produced the quiz; empty for one made
  // from the menu.
  messageId: string
}

export interface QuizQuestion {
  id: string
  position: number
  kind: string
  prompt: string
  options: string[]
}

// A quiz to take: no answer key, which only comes with a result.
export interface Quiz {
  id: string
  sessionTitle: string
  difficulty: string
  questions: QuizQuestion[]
}

// What the user chose for one question. null is a blank answer.
export interface QuizSelection {
  questionId: string
  selectedIndex: number | null
}

export interface QuizAttemptSummary {
  id: string
  quizId: string
  correctCount: number
  total: number
  // 0 to 10.
  score: number
  // ISO timestamp.
  finishedAt: string
}

export interface QuizResultQuestion extends QuizQuestion {
  correctIndex: number
  explanation: string
  selectedIndex: number | null
  correct: boolean
}

export interface QuizAttemptResult {
  attempt: QuizAttemptSummary
  sessionTitle: string
  difficulty: string
  questions: QuizResultQuestion[]
}

function toSummary(dto: Awaited<ReturnType<typeof ListQuizzes>>[number]): QuizSummary {
  return { ...dto, latestScore: dto.latestScore ?? null }
}

export async function listQuizzes(): Promise<QuizSummary[]> {
  return (await ListQuizzes()).map(toSummary)
}

export function getQuiz(id: string): Promise<Quiz> {
  return GetQuiz(id)
}

// Asks for a quiz about a study session. It resolves with the new quiz's
// summary and rejects with a coded error (see quizErrorCode).
export async function generateQuiz(
  sessionId: string,
  count: number,
  difficulty: QuizDifficulty,
): Promise<QuizSummary> {
  return toSummary(await GenerateQuiz(sessionId, count, difficulty))
}

// A pending quiz offer of the study chat: it hangs under the assistant message
// messageId. See
// specs/phases/phase-03-architecture-lab/23-interactive-quiz-chat-offer.md.
export interface QuizOffer {
  id: string
  sessionId: string
  messageId: string
}

export async function getPendingQuizOffer(sessionId: string): Promise<QuizOffer | null> {
  const offer = await GetPendingQuizOffer(sessionId)
  if (!offer) return null
  return { id: offer.id, sessionId: offer.sessionId, messageId: offer.messageId }
}

// Accepting generates the quiz, so it takes as long as generateQuiz and rejects
// with a coded error the same way. The offer stays pending on a failure.
export async function acceptQuizOffer(
  offerId: string,
  count: number,
  difficulty: QuizDifficulty,
): Promise<QuizSummary> {
  return toSummary(await AcceptQuizOffer(offerId, count, difficulty))
}

export function declineQuizOffer(offerId: string): Promise<void> {
  return DeclineQuizOffer(offerId)
}

// The quizzes the session's offers produced: the cards under the chat messages.
export async function listSessionQuizzes(sessionId: string): Promise<QuizSummary[]> {
  return (await ListSessionQuizzes(sessionId)).map(toSummary)
}

export function cancelQuizGeneration(): Promise<void> {
  return CancelQuizGeneration()
}

export function submitQuizAttempt(
  quizId: string,
  selections: QuizSelection[],
): Promise<QuizAttemptSummary> {
  // The generated binding types a blank as undefined, which the JSON leaves
  // out and Go reads as nil.
  return SubmitQuizAttempt(
    quizId,
    selections.map(({ questionId, selectedIndex }) => ({
      questionId,
      selectedIndex: selectedIndex ?? undefined,
    })),
  )
}

export function listQuizAttempts(quizId: string): Promise<QuizAttemptSummary[]> {
  return ListQuizAttempts(quizId)
}

export async function getQuizAttemptResult(attemptId: string): Promise<QuizAttemptResult> {
  const result = await GetQuizAttemptResult(attemptId)
  return {
    ...result,
    questions: result.questions.map((question) => ({
      ...question,
      selectedIndex: question.selectedIndex ?? null,
    })),
  }
}

export function deleteQuiz(id: string): Promise<void> {
  return DeleteQuiz(id)
}

// Wails rejects with the Go error's text, as a string or wrapped in an Error;
// for the quiz bindings it is {"code": "...", "message": "..."}.
export function quizErrorCode(err: unknown): string {
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

// The codes that have their own message; any other one reads as generic.
const ERROR_CODES = [
  'invalid_input',
  'source_session_not_found',
  'quiz_not_found',
  'attempt_not_found',
  'invalid_generation',
  'generation_in_progress',
  'selection_invalid',
  'offer_not_found',
]

export function quizErrorMessage(err: unknown): string {
  const code = quizErrorCode(err)
  return i18n.t(ERROR_CODES.includes(code) ? `errors.${code}` : 'errors.generic', {
    ns: QUIZ_NAMESPACE,
  })
}

// Preselects the generation dialog's difficulty from the profile's experience
// level (beginner, intermediate or advanced).
export function difficultyForExperience(level: string | undefined): QuizDifficulty {
  if (level === 'beginner') return 'EASY'
  if (level === 'advanced') return 'HARD'
  return 'MEDIUM'
}

// The label follows the profile's assistant language. A value the backend sends
// that is not known here is shown as it came.
export function quizDifficultyLabel(difficulty: string): string {
  return QUIZ_DIFFICULTIES.includes(difficulty as QuizDifficulty)
    ? i18n.t(`difficulty.${difficulty}`, { ns: QUIZ_NAMESPACE })
    : difficulty
}

// A score is on a 0-10 scale with at most one decimal: "7.5", "10", "0".
export function formatScore(score: number): string {
  return Number.isInteger(score) ? String(score) : score.toFixed(1)
}

export type ScoreBand = 'low' | 'mid' | 'high'

// Only a reading aid for the color of a score: below 5, 5 to 7, above 7. The
// backend attaches no meaning to it (there is no pass or fail).
export function scoreBand(score: number): ScoreBand {
  if (score < 5) return 'low'
  return score > 7 ? 'high' : 'mid'
}

export function countBlank(selections: QuizSelection[]): number {
  return selections.filter((selection) => selection.selectedIndex === null).length
}

// The day and time an attempt finished, in the profile's assistant language.
export function formatQuizDate(iso: string): string {
  const locale = i18n.language === 'pt' ? 'pt-BR' : 'en-US'
  return new Date(iso).toLocaleString(locale, { dateStyle: 'medium', timeStyle: 'short' })
}
