import { beforeEach, describe, expect, it, vi } from 'vitest'
import { GetProgressDashboard } from '../../wailsjs/go/desktop/App'
import { getProgressDashboard } from './progress'

vi.mock('../../wailsjs/go/desktop/App', () => ({ GetProgressDashboard: vi.fn() }))

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
