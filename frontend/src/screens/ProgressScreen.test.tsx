import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  getProgressDashboard,
  getProgressTopic,
  type ProgressDashboard,
  type ProgressTopicDetail,
} from '@/lib/progress'
import { setLabLanguage } from '@/i18n'
import ProgressScreen from './ProgressScreen'

vi.mock('@/lib/progress', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/progress')>()),
  getProgressDashboard: vi.fn(),
  getProgressTopic: vi.fn(),
}))

vi.mock('@/lib/passScore', () => ({ getPassScore: vi.fn().mockResolvedValue(80) }))

const DASHBOARD: ProgressDashboard = {
  counters: { folders: 2, topics: 3, topicsToReview: 1 },
  folders: [
    {
      id: 'f1',
      name: 'System Design',
      topicCount: 2,
      topics: [
        {
          sessionId: 's1',
          name: 'Cache',
          quizScore: 85,
          labScore: 63,
          status: 'NEEDS_REINFORCEMENT',
        },
        {
          sessionId: 's2',
          name: 'Queues',
          quizScore: null,
          labScore: null,
          status: 'INSUFFICIENT_EVIDENCE',
        },
      ],
    },
    {
      id: 'f2',
      name: 'Java',
      topicCount: 1,
      topics: [
        { sessionId: 's3', name: 'Collections', quizScore: 90, labScore: null, status: 'STABLE' },
      ],
    },
  ],
}

const DETAIL: ProgressTopicDetail = {
  sessionId: 's1',
  name: 'Cache',
  folderName: 'System Design',
  quizCount: 1,
  challengeCount: 1,
  conceptCount: 0,
  quizzes: [{ quizId: 'q1', number: 1, difficulty: 'EASY', percent: 90, attempts: 1 }],
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
  understood: [],
  difficulties: [],
  explainedOnly: [],
  evolution: { quiz: [], lab: [], passScore: 80, enough: false },
  insight: { insight: null, stale: false, canGenerate: true },
}

function renderScreen() {
  const onStartQuiz = vi.fn()
  const onStartExam = vi.fn()
  render(<ProgressScreen onStartQuiz={onStartQuiz} onStartExam={onStartExam} />)
  return { onStartQuiz, onStartExam }
}

describe('ProgressScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLabLanguage('en')
    vi.mocked(getProgressDashboard).mockResolvedValue(DASHBOARD)
    vi.mocked(getProgressTopic).mockResolvedValue(DETAIL)
  })

  it('shows the title, the subtitle and the three counters', async () => {
    renderScreen()

    expect(screen.getByText('Loading your progress...')).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Progress' })).toBeInTheDocument()
    expect(
      screen.getByText('Track how you are doing in each subject you study.'),
    ).toBeInTheDocument()
    expect(screen.getByText('Folders').previousElementSibling).toHaveTextContent('2')
    expect(screen.getByText('Topics to review').previousElementSibling).toHaveTextContent('1')
    expect(screen.getByText('Topics').previousElementSibling).toHaveTextContent('3')
  })

  it('lists the folders with their topic count and selects the first one', async () => {
    renderScreen()

    const first = await screen.findByRole('button', { name: /System Design/ })
    expect(first).toHaveAttribute('aria-pressed', 'true')
    expect(within(first).getByText('2 topics')).toBeInTheDocument()
    const second = screen.getByRole('button', { name: /Java/ })
    expect(second).toHaveAttribute('aria-pressed', 'false')
    expect(within(second).getByText('1 topic')).toBeInTheDocument()
  })

  it('lists the topics of the selected folder with their scores and status', async () => {
    renderScreen()

    const cache = await screen.findByRole('button', { name: /Cache/ })
    expect(within(cache).getByText('85%')).toBeInTheDocument()
    expect(within(cache).getByText('63%')).toBeInTheDocument()
    expect(within(cache).getByText('Needs reinforcement')).toBeInTheDocument()
    const queues = screen.getByRole('button', { name: /Queues/ })
    expect(within(queues).getAllByText('—')).toHaveLength(2)
    expect(within(queues).getByText('Insufficient evidence')).toBeInTheDocument()
    expect(screen.queryByText('Collections')).not.toBeInTheDocument()
  })

  it('shows the topics of the folder that is picked', async () => {
    const user = userEvent.setup()
    renderScreen()

    await user.click(await screen.findByRole('button', { name: /Java/ }))

    expect(screen.getByRole('button', { name: /Java/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /System Design/ })).toHaveAttribute(
      'aria-pressed',
      'false',
    )
    const collections = screen.getByRole('button', { name: /Collections/ })
    expect(within(collections).getByText('Stable')).toBeInTheDocument()
    expect(screen.queryByText('Cache')).not.toBeInTheDocument()
  })

  it('highlights the selected topic', async () => {
    const user = userEvent.setup()
    renderScreen()
    const cache = await screen.findByRole('button', { name: /Cache/ })
    expect(cache).toHaveAttribute('aria-pressed', 'false')

    await user.click(cache)

    expect(screen.getByRole('button', { name: /Cache/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /Queues/ })).toHaveAttribute('aria-pressed', 'false')
  })

  it('clears the highlighted topic when another folder is picked', async () => {
    const user = userEvent.setup()
    renderScreen()
    await user.click(await screen.findByRole('button', { name: /Cache/ }))

    await user.click(screen.getByRole('button', { name: /Java/ }))
    await user.click(screen.getByRole('button', { name: /System Design/ }))

    expect(screen.getByRole('button', { name: /Cache/ })).toHaveAttribute('aria-pressed', 'false')
  })

  it('points to the Study tab when no folder has a session', async () => {
    vi.mocked(getProgressDashboard).mockResolvedValue({
      counters: { folders: 0, topics: 0, topicsToReview: 0 },
      folders: [],
    })

    renderScreen()

    expect(await screen.findByText('Nothing to show yet')).toBeInTheDocument()
    expect(screen.getByText(/Go to the Study tab/)).toBeInTheDocument()
    expect(screen.queryByText('Topics to review')).not.toBeInTheDocument()
  })

  it('offers a retry when the dashboard cannot be loaded', async () => {
    const user = userEvent.setup()
    vi.mocked(getProgressDashboard).mockRejectedValueOnce(new Error('boom'))
    renderScreen()

    expect(await screen.findByText('Could not load your progress.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByRole('button', { name: /System Design/ })).toBeInTheDocument()
    expect(getProgressDashboard).toHaveBeenCalledTimes(2)
  })

  it('asks to select a topic until one is selected', async () => {
    renderScreen()

    expect(
      await screen.findByText('Select a topic to see what supports its status.'),
    ).toBeInTheDocument()
    expect(getProgressTopic).not.toHaveBeenCalled()
  })

  it('shows the detail of the selected topic on the right', async () => {
    const user = userEvent.setup()
    renderScreen()

    await user.click(await screen.findByRole('button', { name: /Cache/ }))

    expect(getProgressTopic).toHaveBeenCalledWith('s1')
    expect(await screen.findByRole('heading', { name: 'Cache', level: 3 })).toBeInTheDocument()
    expect(await screen.findByRole('region', { name: 'Results' })).toBeInTheDocument()
    expect(
      screen.queryByText('Select a topic to see what supports its status.'),
    ).not.toBeInTheDocument()
  })

  it('hides the detail when another folder is picked', async () => {
    const user = userEvent.setup()
    renderScreen()
    await user.click(await screen.findByRole('button', { name: /Cache/ }))
    await screen.findByRole('region', { name: 'Results' })

    await user.click(screen.getByRole('button', { name: /Java/ }))

    expect(screen.queryByRole('region', { name: 'Results' })).not.toBeInTheDocument()
  })

  it('opens a quiz of the topic', async () => {
    const user = userEvent.setup()
    const { onStartQuiz } = renderScreen()
    await user.click(await screen.findByRole('button', { name: /Cache/ }))

    await user.click(await screen.findByRole('button', { name: /Quiz 1 · Easy/ }))

    expect(onStartQuiz).toHaveBeenCalledWith('q1')
  })

  it('shows the exam rules of a challenge and starts it once confirmed', async () => {
    const user = userEvent.setup()
    const { onStartExam } = renderScreen()
    await user.click(await screen.findByRole('button', { name: /Cache/ }))

    await user.click(await screen.findByRole('button', { name: /Design a cache/ }))

    const dialog = await screen.findByRole('dialog')
    expect(onStartExam).not.toHaveBeenCalled()
    await user.click(within(dialog).getByRole('button', { name: /Start/ }))
    expect(onStartExam).toHaveBeenCalledWith({
      id: 'c1',
      title: 'Design a cache',
      difficulty: 'ADVANCED',
      timeLimitMinutes: 60,
    })
  })

  it('closes the exam rules without starting the exam', async () => {
    const user = userEvent.setup()
    const { onStartExam } = renderScreen()
    await user.click(await screen.findByRole('button', { name: /Cache/ }))
    await user.click(await screen.findByRole('button', { name: /Design a cache/ }))
    const dialog = await screen.findByRole('dialog')

    await user.click(within(dialog).getByRole('button', { name: /Cancel/ }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(onStartExam).not.toHaveBeenCalled()
  })

  it('follows the profile language', async () => {
    setLabLanguage('pt')

    renderScreen()

    expect(
      await screen.findByText('Acompanhe como você está em cada assunto que estuda.'),
    ).toBeInTheDocument()
    expect(screen.getByText('Precisa reforçar')).toBeInTheDocument()
    expect(screen.getByText('2 assuntos')).toBeInTheDocument()
  })
})
