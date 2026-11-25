import { beforeEach, describe, expect, it, vi } from 'vitest'
import { GetProgressDashboard, GetProgressTopic } from '../../wailsjs/go/desktop/App'
import { getProgressDashboard, getProgressTopic } from './progress'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  GetProgressDashboard: vi.fn(),
  GetProgressTopic: vi.fn(),
}))

describe('getProgressDashboard', () => {
  beforeEach(() => vi.clearAllMocks())

  it('maps the counters, the folders and their topics', async () => {
    vi.mocked(GetProgressDashboard).mockResolvedValue({
      counters: { folders: 1, topics: 1, topicsToReview: 1 },
      folders: [
        {
          id: 'f1',
          name: 'System Design',
          topicCount: 1,
          topics: [
            {
              sessionId: 's1',
              name: 'Cache',
              quizScore: 85,
              labScore: 63,
              status: 'NEEDS_REINFORCEMENT',
            },
          ],
        },
      ],
    } as never)

    const dashboard = await getProgressDashboard()

    expect(dashboard).toEqual({
      counters: { folders: 1, topics: 1, topicsToReview: 1 },
      folders: [
        {
          id: 'f1',
          name: 'System Design',
          topicCount: 1,
          topics: [
            {
              sessionId: 's1',
              name: 'Cache',
              quizScore: 85,
              labScore: 63,
              status: 'NEEDS_REINFORCEMENT',
            },
          ],
        },
      ],
    })
  })

  it('keeps a missing score as null', async () => {
    vi.mocked(GetProgressDashboard).mockResolvedValue({
      counters: { folders: 1, topics: 1, topicsToReview: 0 },
      folders: [
        {
          id: 'f1',
          name: 'F',
          topicCount: 1,
          topics: [
            {
              sessionId: 's1',
              name: 'Cache',
              quizScore: null,
              labScore: undefined,
              status: 'INSUFFICIENT_EVIDENCE',
            },
          ],
        },
      ],
    } as never)

    const dashboard = await getProgressDashboard()

    expect(dashboard.folders[0].topics[0].quizScore).toBeNull()
    expect(dashboard.folders[0].topics[0].labScore).toBeNull()
  })
})

describe('getProgressTopic', () => {
  beforeEach(() => vi.clearAllMocks())

  it('asks for the session and maps the whole detail, nulls included', async () => {
    const proof = { assessment: 'CORRECT', description: 'ok', assisted: true, createdAt: 'now' }
    vi.mocked(GetProgressTopic).mockResolvedValue({
      sessionId: 's1',
      name: 'Cache',
      folderName: 'System Design',
      quizCount: 1,
      challengeCount: 1,
      conceptCount: 2,
      quizzes: [{ quizId: 'q1', number: 1, difficulty: 'BEGINNER', percent: null, attempts: 0 }],
      challenges: [
        {
          challengeId: 'c1',
          title: 'Design a cache',
          difficulty: 'ADVANCED',
          timeLimitMinutes: 60,
          diagramScore: 85,
          defenseScore: null,
          result: 'PENDING',
          attempts: 1,
        },
      ],
      understood: [{ name: 'Cache-aside', evidenceCount: 1, proofs: [proof] }],
      difficulties: [
        {
          name: 'ttl',
          source: 'TUTOR',
          challengeTitle: '',
          toReview: true,
          evidenceCount: 2,
          proofs: [],
        },
      ],
      explainedOnly: [{ name: 'lru', evidenceCount: 0, proofs: [] }],
      evolution: {
        quiz: [{ order: 1, itemNumber: 1, title: '', score: 60, at: '2026-11-02T10:00:00Z' }],
        lab: [],
        passScore: 80,
        enough: false,
      },
    } as never)

    const detail = await getProgressTopic('s1')

    expect(GetProgressTopic).toHaveBeenCalledWith('s1')
    expect(detail.name).toBe('Cache')
    expect(detail.folderName).toBe('System Design')
    expect(detail.quizzes).toEqual([
      { quizId: 'q1', number: 1, difficulty: 'BEGINNER', percent: null, attempts: 0 },
    ])
    expect(detail.challenges[0]).toMatchObject({
      diagramScore: 85,
      defenseScore: null,
      result: 'PENDING',
      timeLimitMinutes: 60,
    })
    expect(detail.understood[0].proofs).toEqual([proof])
    expect(detail.difficulties[0]).toMatchObject({ source: 'TUTOR', toReview: true })
    expect(detail.explainedOnly[0].name).toBe('lru')
    expect(detail.evolution).toEqual({
      quiz: [{ order: 1, itemNumber: 1, title: '', score: 60, at: '2026-11-02T10:00:00Z' }],
      lab: [],
      passScore: 80,
      enough: false,
    })
    expect(detail.quizCount + detail.challengeCount + detail.conceptCount).toBe(4)
  })

  it('turns an undefined score into null', async () => {
    vi.mocked(GetProgressTopic).mockResolvedValue({
      quizzes: [{ quizId: 'q1', number: 1, difficulty: 'BEGINNER', attempts: 0 }],
      challenges: [
        {
          challengeId: 'c1',
          title: 'T',
          difficulty: 'BEGINNER',
          timeLimitMinutes: 45,
          result: 'PENDING',
          attempts: 0,
        },
      ],
      understood: [],
      difficulties: [],
      explainedOnly: [],
      evolution: { quiz: [], lab: [], passScore: 80, enough: false },
    } as never)

    const detail = await getProgressTopic('s1')

    expect(detail.quizzes[0].percent).toBeNull()
    expect(detail.challenges[0].diagramScore).toBeNull()
    expect(detail.challenges[0].defenseScore).toBeNull()
  })
})
