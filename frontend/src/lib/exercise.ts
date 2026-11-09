import {
  AcceptOffer,
  AnswerDefenseQuestion,
  CancelChallengeGeneration,
  DeclineOffer,
  DiscardAttempt,
  EvaluateAttempt,
  EvaluateDefense,
  GenerateChallenge,
  GetAttempt,
  GetAttemptResult,
  GetEvaluation,
  GetPendingOffer,
  GetProfile,
  ListAttemptHistory,
  ListChallengeCatalog,
  ListChallengeTopics,
  ListHints,
  ListSessionsForGeneration,
  NextDefenseQuestion,
  RequestHint,
  StartAttempt,
  SubmitAttempt,
} from '../../wailsjs/go/desktop/App'
import type { desktop } from '../../wailsjs/go/models'
import type { LabCriterion, LabHint, LabStatement } from './lab'

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

export const ATTEMPT_IN_PROGRESS = 'attempt_in_progress'
export const SUBMISSION_TOO_LATE = 'submission_too_late'
export const SUBMISSION_TOO_LARGE = 'submission_too_large'

const GENERIC_MESSAGE = 'Something went wrong. Please try again.'

const ERROR_MESSAGES: Record<string, string> = {
  invalid_generation: 'The generated challenge was not valid. Try again.',
  offer_not_found: 'This exercise offer is no longer available.',
  source_session_not_found: 'That study session no longer exists. Pick another one.',
  generation_in_progress: 'A challenge is already being generated.',
  [SUBMISSION_TOO_LARGE]: 'The diagram is too large to submit. Remove some elements.',
  [SUBMISSION_TOO_LATE]: 'The time was over and the exam could not be submitted.',
}

export function exerciseErrorMessage(err: unknown): string {
  return ERROR_MESSAGES[exerciseErrorCode(err)] ?? GENERIC_MESSAGE
}

// Preselects the generation dialog's difficulty from the profile's experience
// level (beginner, intermediate or advanced).
export function difficultyForExperience(level: string | undefined): ChallengeDifficulty {
  const match = (Object.keys(DIFFICULTY_LABELS) as ChallengeDifficulty[]).find(
    (difficulty) => difficulty.toLowerCase() === level,
  )
  return match ?? 'INTERMEDIATE'
}

export function formatSessionDate(startedAt: string): string {
  return new Date(startedAt).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

// The exam attempt, as the exam screen reads it. See
// specs/phases/phase-03-architecture-lab/10-diagram-workspace.md.
export interface Attempt {
  id: string
  challengeId: string
  title: string
  topic: string
  // ISO timestamp; the backend owns the clock.
  deadline: string
  statement: LabStatement
  criteria: LabCriterion[]
}

// The content type a submitted diagram carries.
export const DIAGRAM_CONTENT_TYPE = 'drawio-xml'

function toAttempt(dto: desktop.AttemptDTO): Attempt {
  return {
    id: dto.id,
    challengeId: dto.challengeId,
    title: dto.challengeTitle,
    topic: dto.challengeTopic,
    deadline: dto.deadline,
    statement: {
      scenario: dto.statement.scenario,
      functionalRequirements: dto.statement.functionalRequirements,
      nonFunctionalRequirements: dto.statement.nonFunctionalRequirements,
      constraints: dto.statement.constraints,
      objectives: dto.statement.objectives,
    },
    criteria: dto.criteria.map((criterion) => ({
      id: criterion.key,
      name: criterion.name,
      description: criterion.description,
      weight: criterion.weight,
      mandatory: criterion.mandatory,
    })),
  }
}

// Starts an exam, standalone or, with a session id, linked to that study
// session. When another exam is running it rejects with ATTEMPT_IN_PROGRESS
// unless discardInProgress is true.
export async function startAttempt(
  challengeId: string,
  discardInProgress: boolean,
  sessionId = '',
): Promise<Attempt> {
  return toAttempt(await StartAttempt(challengeId, sessionId, discardInProgress))
}

export async function submitAttempt(attemptId: string, xml: string): Promise<void> {
  await SubmitAttempt(attemptId, DIAGRAM_CONTENT_TYPE, xml)
}

export function discardAttempt(attemptId: string): Promise<void> {
  return DiscardAttempt(attemptId)
}

export async function getAttempt(attemptId: string): Promise<Attempt> {
  return toAttempt(await GetAttempt(attemptId))
}

// The tutor's hints during the exam. The backend counts them and refuses the
// fourth; a failed request spends nothing. See
// specs/phases/phase-03-architecture-lab/10-01-tutor-hints.md.
function toHint(dto: desktop.HintDTO): LabHint {
  return { id: dto.id, text: dto.content }
}

// Asks the tutor for a hint, given the board as it is now.
export async function requestHint(attemptId: string, boardXml: string): Promise<LabHint> {
  return toHint(await RequestHint(attemptId, boardXml))
}

// The hints already given for the attempt, in the order given.
export async function listHints(attemptId: string): Promise<LabHint[]> {
  return (await ListHints(attemptId)).map(toHint)
}

// The diagram evaluation of a submitted attempt: the score is computed by the
// backend, the model only picks levels. See
// specs/phases/phase-03-architecture-lab/11-architecture-evaluator.md.
export interface EvaluationResult {
  key: string
  name: string
  // The criterion's weight in the rubric, in percentage points.
  weight: number
  // 0..4; meaningless when insufficientEvidence is set.
  level: number
  justification: string
  evidence: string
  // The diagram did not show enough to judge the criterion.
  insufficientEvidence: boolean
}

export interface Evaluation {
  score: number
  summary: string
  strengths: string[]
  gaps: string[]
  recommendations: string[]
  results: EvaluationResult[]
}

function toEvaluation(dto: desktop.EvaluationDTO): Evaluation {
  return {
    score: dto.score,
    summary: dto.summary,
    strengths: dto.strengths,
    gaps: dto.gaps,
    recommendations: dto.recommendations,
    results: dto.results.map((result) => ({
      key: result.criterionKey,
      name: result.criterionName,
      weight: result.weight,
      level: result.level,
      justification: result.justification,
      evidence: result.evidence,
      insufficientEvidence: result.insufficientEvidence,
    })),
  }
}

// Evaluates a submitted attempt. A failure can be retried by calling it again.
export async function evaluateAttempt(attemptId: string): Promise<Evaluation> {
  return toEvaluation(await EvaluateAttempt(attemptId))
}

export async function getEvaluation(attemptId: string): Promise<Evaluation> {
  return toEvaluation(await GetEvaluation(attemptId))
}

const EVALUATION_FAILED_MESSAGE = 'The evaluator could not complete the evaluation.'

const EVALUATION_ERROR_MESSAGES: Record<string, string> = {
  invalid_diagram: 'The diagram could not be read.',
  empty_diagram: 'The diagram is empty.',
  invalid_evaluation: 'The evaluator returned an invalid evaluation.',
}

export function evaluationErrorMessage(err: unknown): string {
  return EVALUATION_ERROR_MESSAGES[exerciseErrorCode(err)] ?? EVALUATION_FAILED_MESSAGE
}

// The tutor's defense: questions one at a time, then the outcome. See
// specs/phases/phase-03-architecture-lab/12-architecture-defense.md.

// The text the backend recognises as no knowledge of the criterion.
export const DONT_KNOW_ANSWER = "I don't know"
export const MAX_ANSWER_LENGTH = 4000

export interface DefenseQuestion {
  position: number
  text: string
  total: number
}

export interface DefenseProgress {
  total: number
  // The last answer is in: the defense can be evaluated.
  complete: boolean
}

export type OutcomeResult = 'PASSED' | 'FAILED'

export interface OutcomeReason {
  kind: 'DIAGRAM_SCORE' | 'UNDERSTANDING_SCORE' | 'MANDATORY_CRITERION'
  evaluation: 'DIAGRAM' | 'UNDERSTANDING'
  // Set for MANDATORY_CRITERION, '' otherwise.
  criterionName: string
}

export interface Outcome {
  result: OutcomeResult
  passScore: number
  diagramScore: number
  understandingScore: number
  reasons: OutcomeReason[]
}

// The question the user must answer now. The first call starts the defense.
export async function nextDefenseQuestion(attemptId: string): Promise<DefenseQuestion> {
  const dto = await NextDefenseQuestion(attemptId)
  return { position: dto.position, text: dto.question, total: dto.total }
}

export async function answerDefenseQuestion(
  attemptId: string,
  answer: string,
): Promise<DefenseProgress> {
  const dto = await AnswerDefenseQuestion(attemptId, answer)
  return { total: dto.total, complete: dto.complete }
}

// Judges the answers and finishes the attempt. A failure can be retried by
// calling it again.
export async function evaluateDefense(attemptId: string): Promise<Outcome> {
  return toOutcome(await EvaluateDefense(attemptId))
}

function toOutcome(dto: desktop.OutcomeDTO): Outcome {
  return {
    result: dto.result as OutcomeResult,
    passScore: dto.passScore,
    diagramScore: dto.diagramScore,
    understandingScore: dto.understandingScore,
    reasons: dto.reasons.map((reason) => ({
      kind: reason.kind as OutcomeReason['kind'],
      evaluation: reason.evaluation as OutcomeReason['evaluation'],
      criterionName: reason.criterionName,
    })),
  }
}

const DEFENSE_FAILED_MESSAGE = 'The tutor could not continue the defense.'

const DEFENSE_ERROR_MESSAGES: Record<string, string> = {
  invalid_answer: 'The answer must be between 1 and 4,000 characters.',
  invalid_evaluation: 'The tutor returned an invalid evaluation.',
  invalid_diagram: 'The diagram could not be read.',
}

export function defenseErrorMessage(err: unknown): string {
  return DEFENSE_ERROR_MESSAGES[exerciseErrorCode(err)] ?? DEFENSE_FAILED_MESSAGE
}

// The history and the result of finished attempts, read-only. The backend
// decides the outcome and its reasons; nothing is derived here. See
// specs/phases/phase-03-architecture-lab/13-evaluation-and-history-screen.md.

export interface AttemptHistoryItem {
  attemptId: string
  // ISO timestamp.
  finishedAt: string
  diagramScore: number
  understandingScore: number
  outcome: OutcomeResult
}

export interface DefenseTurn {
  position: number
  question: string
  answer: string
}

export type SubmitReason = 'USER' | 'TIME_EXPIRED'

export interface AttemptResult {
  challenge: {
    id: string
    title: string
    topic: string
    difficulty: string
    statement: LabStatement
  }
  attemptId: string
  // ISO timestamps.
  startedAt: string
  finishedAt: string
  timeSpentSeconds: number
  submitReason: SubmitReason
  hintsUsed: number
  maxHints: number
  outcome: Outcome
  diagram: Evaluation
  understanding: Evaluation
  defense: DefenseTurn[]
  // The submitted diagram, draw.io XML.
  submission: string
}

// The finished attempts of a challenge, newest first.
export async function listAttemptHistory(challengeId: string): Promise<AttemptHistoryItem[]> {
  const items = await ListAttemptHistory(challengeId)
  return items.map((item) => ({
    attemptId: item.attemptId,
    finishedAt: item.finishedAt,
    diagramScore: item.diagramScore,
    understandingScore: item.understandingScore,
    outcome: item.outcome as OutcomeResult,
  }))
}

// The full result of a finished attempt; rejects with attempt_not_finished
// for any other.
export async function getAttemptResult(attemptId: string): Promise<AttemptResult> {
  const dto = await GetAttemptResult(attemptId)
  return {
    challenge: {
      id: dto.challenge.id,
      title: dto.challenge.title,
      topic: dto.challenge.topic,
      difficulty: dto.challenge.difficulty,
      statement: {
        scenario: dto.challenge.statement.scenario,
        functionalRequirements: dto.challenge.statement.functionalRequirements,
        nonFunctionalRequirements: dto.challenge.statement.nonFunctionalRequirements,
        constraints: dto.challenge.statement.constraints,
        objectives: dto.challenge.statement.objectives,
      },
    },
    attemptId: dto.attemptId,
    startedAt: dto.startedAt,
    finishedAt: dto.finishedAt,
    timeSpentSeconds: dto.timeSpentSeconds,
    submitReason: dto.submitReason as SubmitReason,
    hintsUsed: dto.hintsUsed,
    maxHints: dto.maxHints,
    outcome: toOutcome(dto.outcome),
    diagram: toEvaluation(dto.diagram),
    understanding: toEvaluation(dto.understanding),
    defense: dto.defense.map((turn) => ({
      position: turn.position,
      question: turn.question,
      answer: turn.answer,
    })),
    submission: dto.submission,
  }
}

// The exercise offer of the study chat: the tutor's answer to the user's
// request for an exercise, accepted or declined. See
// specs/phases/phase-03-architecture-lab/14-ai-challenge-generation.md.
export interface ExerciseOffer {
  id: string
  sessionId: string
  messageId: string
}

export interface SessionExercise {
  challengeId: string
  title: string
  topic: string
  difficulty: string
  timeLimitMinutes: number
}

// What the rules dialog and the exam screen need to start the exam of a
// challenge.
export interface ExamToStart {
  id: string
  title: string
  difficulty: string
  timeLimitMinutes: number
}

// The exam of a challenge linked to a study session.
export function toExamToStart(exercise: SessionExercise): ExamToStart {
  return {
    id: exercise.challengeId,
    title: exercise.title,
    difficulty: exercise.difficulty,
    timeLimitMinutes: exercise.timeLimitMinutes,
  }
}

export const OFFER_PENDING_MESSAGE = 'Accept or decline the exercise offer to keep chatting.'

export async function getPendingOffer(sessionId: string): Promise<ExerciseOffer | null> {
  const offer = await GetPendingOffer(sessionId)
  if (!offer) return null
  return { id: offer.id, sessionId: offer.sessionId, messageId: offer.messageId }
}

export async function acceptOffer(offerId: string): Promise<SessionExercise> {
  const exercise = await AcceptOffer(offerId)
  return {
    challengeId: exercise.challengeId,
    title: exercise.title,
    topic: exercise.topic,
    difficulty: exercise.difficulty,
    timeLimitMinutes: exercise.timeLimitMinutes,
  }
}

export function declineOffer(offerId: string): Promise<void> {
  return DeclineOffer(offerId)
}

export interface OfferLabels {
  title: string
  accept: string
  decline: string
  preparing: string
}

// The offer speaks the assistant language; the rest of the interface stays in
// English.
export function offerLabels(language: string | undefined): OfferLabels {
  if (language === 'pt') {
    return {
      title: 'Praticar com um exercício de arquitetura',
      accept: 'Aceitar',
      decline: 'Recusar',
      preparing: 'Preparando seu exercício…',
    }
  }
  return {
    title: 'Practice with an architecture exercise',
    accept: 'Accept',
    decline: 'Decline',
    preparing: 'Preparing your exercise…',
  }
}

// The profile's assistant language ('pt' or 'en'); English when it cannot be read.
export async function getAssistantLanguage(): Promise<string> {
  try {
    return (await GetProfile()).assistantLanguage || 'en'
  } catch {
    return 'en'
  }
}
