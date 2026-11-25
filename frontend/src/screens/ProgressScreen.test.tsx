import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getProgressDashboard, type ProgressDashboard } from '@/lib/progress'
import { setLabLanguage } from '@/i18n'
import ProgressScreen from './ProgressScreen'

vi.mock('@/lib/progress', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/progress')>()),
  getProgressDashboard: vi.fn(),
}))

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

describe('ProgressScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLabLanguage('en')
    vi.mocked(getProgressDashboard).mockResolvedValue(DASHBOARD)
  })

  it('shows the title, the subtitle and the three counters', async () => {
    render(<ProgressScreen />)

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
    render(<ProgressScreen />)

    const first = await screen.findByRole('button', { name: /System Design/ })
    expect(first).toHaveAttribute('aria-pressed', 'true')
    expect(within(first).getByText('2 topics')).toBeInTheDocument()
    const second = screen.getByRole('button', { name: /Java/ })
    expect(second).toHaveAttribute('aria-pressed', 'false')
    expect(within(second).getByText('1 topic')).toBeInTheDocument()
  })

  it('lists the topics of the selected folder with their scores and status', async () => {
    render(<ProgressScreen />)

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
    render(<ProgressScreen />)

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

  it('only highlights a topic when it is selected', async () => {
    const user = userEvent.setup()
    render(<ProgressScreen />)
    const cache = await screen.findByRole('button', { name: /Cache/ })
    expect(cache).toHaveAttribute('aria-pressed', 'false')

    await user.click(cache)

    expect(screen.getByRole('button', { name: /Cache/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /Queues/ })).toHaveAttribute('aria-pressed', 'false')
  })

  it('clears the highlighted topic when another folder is picked', async () => {
    const user = userEvent.setup()
    render(<ProgressScreen />)
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

    render(<ProgressScreen />)

    expect(await screen.findByText('Nothing to show yet')).toBeInTheDocument()
    expect(screen.getByText(/Go to the Study tab/)).toBeInTheDocument()
    expect(screen.queryByText('Topics to review')).not.toBeInTheDocument()
  })

  it('offers a retry when the dashboard cannot be loaded', async () => {
    const user = userEvent.setup()
    vi.mocked(getProgressDashboard).mockRejectedValueOnce(new Error('boom'))
    render(<ProgressScreen />)

    expect(await screen.findByText('Could not load your progress.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByRole('button', { name: /System Design/ })).toBeInTheDocument()
    expect(getProgressDashboard).toHaveBeenCalledTimes(2)
  })

  it('follows the profile language', async () => {
    setLabLanguage('pt')

    render(<ProgressScreen />)

    expect(
      await screen.findByText('Acompanhe como você está em cada assunto que estuda.'),
    ).toBeInTheDocument()
    expect(screen.getByText('Precisa reforçar')).toBeInTheDocument()
    expect(screen.getByText('2 assuntos')).toBeInTheDocument()
  })
})
