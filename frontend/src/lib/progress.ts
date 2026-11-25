import { GetProgressDashboard } from '../../wailsjs/go/desktop/App'

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
