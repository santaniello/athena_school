import {
  GenerateProgressFolderInsight,
  GenerateProgressInsight,
  GetProgressDashboard,
  GetProgressFolderInsight,
  GetProgressTopic,
} from '../../wailsjs/go/desktop/App'

// The Progress dashboard. The backend derives every score and status on read;
// this module only wraps its binding. See
// specs/phases/phase-05-progress/01-progress-dashboard.md.

export type TopicStatus = 'INSUFFICIENT_EVIDENCE' | 'NEEDS_REINFORCEMENT' | 'IMPROVING' | 'STABLE'

// A topic is a study session. A null score means there is nothing to score yet.
export interface ProgressTopic {
  sessionId: string
  name: string
  quizScore: number | null
  labScore: number | null
  status: TopicStatus
}

// A folder that has at least one topic, the most recently active first.
export interface ProgressFolder {
  id: string
  name: string
  topicCount: number
  topics: ProgressTopic[]
}

export interface ProgressDashboard {
  counters: { folders: number; topics: number; topicsToReview: number }
  folders: ProgressFolder[]
}

export async function getProgressDashboard(): Promise<ProgressDashboard> {
  const result = await GetProgressDashboard()
  return {
    counters: {
      folders: result.counters.folders,
      topics: result.counters.topics,
      topicsToReview: result.counters.topicsToReview,
    },
    folders: result.folders.map((folder) => ({
      id: folder.id,
      name: folder.name,
      topicCount: folder.topicCount,
      topics: folder.topics.map((topic) => ({
        sessionId: topic.sessionId,
        name: topic.name,
        quizScore: topic.quizScore ?? null,
        labScore: topic.labScore ?? null,
        status: topic.status as TopicStatus,
      })),
    })),
  }
}

export type LabResult = 'PASSED' | 'FAILED' | 'PENDING'

// One proof the tutor recorded for a concept.
export interface ProgressProof {
  assessment: string
  description: string
  assisted: boolean
  createdAt: string
}

// A concept of the topic with its proofs, the newest first.
export interface ProgressConcept {
  name: string
  evidenceCount: number
  proofs: ProgressProof[]
}

// A difficulty comes from the tutor (a concept) or from an Arch Lab rubric
// criterion; challengeTitle names the challenge for the latter.
export interface ProgressDifficulty extends ProgressConcept {
  source: 'TUTOR' | 'ARCH_LAB'
  challengeTitle: string
  toReview: boolean
}

// The latest result of a quiz; null percent means it was never taken.
export interface ProgressQuizResult {
  quizId: string
  number: number
  difficulty: string
  percent: number | null
  attempts: number
}

// The latest evaluated attempt of a challenge; a null score is still pending.
export interface ProgressChallengeResult {
  challengeId: string
  title: string
  difficulty: string
  timeLimitMinutes: number
  diagramScore: number | null
  defenseScore: number | null
  result: LabResult
  attempts: number
}

// One attempt in the evolution chart. order is its position within its series;
// itemNumber is the number of its quiz or challenge in the topic; title names a
// challenge (empty for quizzes).
export interface ProgressEvolutionPoint {
  order: number
  itemNumber: number
  title: string
  score: number
  at: string
}

// The two series of the chart; enough is false when there is too little to draw.
export interface ProgressEvolution {
  quiz: ProgressEvolutionPoint[]
  lab: ProgressEvolutionPoint[]
  passScore: number
  enough: boolean
}

// An activity that supports a sentence of an insight, or a topic for a folder
// insight (id is then the session); label is what to show.
export interface ProgressCitation {
  kind: 'QUIZ' | 'CHALLENGE' | 'CONCEPT' | 'TOPIC'
  id: string
  label: string
}

// One sentence of the analysis, or one recommendation, with its support.
export interface ProgressStatement {
  text: string
  citations: ProgressCitation[]
}

// The Athena Insights the model wrote from the stored evidence of a topic.
export interface ProgressInsight {
  analysis: ProgressStatement[]
  recommendations: ProgressStatement[]
  model: string
  language: string
  createdAt: string
}

// The insight card of a topic. insight is null until one is generated; stale
// means there is new activity (or a new language) since it was written;
// canGenerate is false while the topic has no evidence to analyse.
export interface ProgressInsightState {
  insight: ProgressInsight | null
  stale: boolean
  canGenerate: boolean
}

export interface ProgressTopicDetail {
  sessionId: string
  name: string
  folderName: string
  quizCount: number
  challengeCount: number
  conceptCount: number
  quizzes: ProgressQuizResult[]
  challenges: ProgressChallengeResult[]
  understood: ProgressConcept[]
  difficulties: ProgressDifficulty[]
  explainedOnly: ProgressConcept[]
  evolution: ProgressEvolution
  insight: ProgressInsightState
}

type InsightResult = Awaited<ReturnType<typeof GenerateProgressInsight>>
type StatementResult = NonNullable<InsightResult['insight']>['analysis'][number]

function toStatement(statement: StatementResult): ProgressStatement {
  return {
    text: statement.text,
    citations: statement.citations.map((citation) => ({
      kind: citation.kind as ProgressCitation['kind'],
      id: citation.id,
      label: citation.label,
    })),
  }
}

function toInsightState(result: InsightResult): ProgressInsightState {
  const insight = result.insight
  return {
    insight: insight
      ? {
          analysis: insight.analysis.map(toStatement),
          recommendations: insight.recommendations.map(toStatement),
          model: insight.model,
          language: insight.language,
          createdAt: insight.createdAt,
        }
      : null,
    stale: result.stale,
    canGenerate: result.canGenerate,
  }
}

// Asks the model for the insight of the topic. It is the only Progress call
// that uses the LLM, and only when the user clicks Generate.
export async function generateProgressInsight(sessionId: string): Promise<ProgressInsightState> {
  return toInsightState(await GenerateProgressInsight(sessionId))
}

// The insight card of a folder: what is stored, with no LLM call (spec 5.5).
export async function getProgressFolderInsight(folderId: string): Promise<ProgressInsightState> {
  return toInsightState(await GetProgressFolderInsight(folderId))
}

// Asks the model for the insight of the folder, only when the user clicks Generate.
export async function generateProgressFolderInsight(
  folderId: string,
): Promise<ProgressInsightState> {
  return toInsightState(await GenerateProgressFolderInsight(folderId))
}

export async function getProgressTopic(sessionId: string): Promise<ProgressTopicDetail> {
  const result = await GetProgressTopic(sessionId)
  const proofs = (list: ProgressProof[]) => list.map((proof) => ({ ...proof }))
  const concept = (item: ProgressConcept): ProgressConcept => ({
    name: item.name,
    evidenceCount: item.evidenceCount,
    proofs: proofs(item.proofs),
  })
  return {
    sessionId: result.sessionId,
    name: result.name,
    folderName: result.folderName,
    quizCount: result.quizCount,
    challengeCount: result.challengeCount,
    conceptCount: result.conceptCount,
    quizzes: result.quizzes.map((quiz) => ({
      quizId: quiz.quizId,
      number: quiz.number,
      difficulty: quiz.difficulty,
      percent: quiz.percent ?? null,
      attempts: quiz.attempts,
    })),
    challenges: result.challenges.map((challenge) => ({
      challengeId: challenge.challengeId,
      title: challenge.title,
      difficulty: challenge.difficulty,
      timeLimitMinutes: challenge.timeLimitMinutes,
      diagramScore: challenge.diagramScore ?? null,
      defenseScore: challenge.defenseScore ?? null,
      result: challenge.result as LabResult,
      attempts: challenge.attempts,
    })),
    understood: result.understood.map(concept),
    difficulties: result.difficulties.map((difficulty) => ({
      ...concept(difficulty),
      source: difficulty.source as ProgressDifficulty['source'],
      challengeTitle: difficulty.challengeTitle,
      toReview: difficulty.toReview,
    })),
    explainedOnly: result.explainedOnly.map(concept),
    evolution: {
      quiz: result.evolution.quiz.map((point) => ({ ...point })),
      lab: result.evolution.lab.map((point) => ({ ...point })),
      passScore: result.evolution.passScore,
      enough: result.evolution.enough,
    },
    insight: toInsightState(result.insight),
  }
}
