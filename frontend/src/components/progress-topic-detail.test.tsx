import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getProgressTopic, type ProgressTopicDetail } from '@/lib/progress'
import { setLabLanguage } from '@/i18n'
import { ProgressTopicDetail as TopicDetailPanel } from './progress-topic-detail'

vi.mock('@/lib/progress', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/progress')>()),
  getProgressTopic: vi.fn(),
}))

const proof = (assessment: string, description: string, assisted = false) => ({
  assessment,
  description,
  assisted,
  createdAt: '2026-11-02T10:00:00Z',
})

const DETAIL: ProgressTopicDetail = {
  sessionId: 's1',
  name: 'Cache',
  folderName: 'System Design',
  quizCount: 2,
  challengeCount: 1,
  conceptCount: 4,
  quizzes: [
    { quizId: 'q1', number: 1, difficulty: 'EASY', percent: 90, attempts: 2 },
    { quizId: 'q2', number: 2, difficulty: 'HARD', percent: null, attempts: 0 },
  ],
  challenges: [
    {
      challengeId: 'c1',
      title: 'Design a cache',
      difficulty: 'ADVANCED',
      timeLimitMinutes: 60,
      diagramScore: 85,
      defenseScore: 40,
      result: 'FAILED',
      attempts: 3,
    },
  ],
  understood: [
    {
      name: 'Cache-aside',
      evidenceCount: 2,
      proofs: [
        proof('CORRECT', 'Explained invalidation', true),
        proof('PARTIALLY_CORRECT', 'Older'),
      ],
    },
  ],
  difficulties: [
    {
      name: 'TTL',
      source: 'TUTOR',
      challengeTitle: '',
      toReview: true,
      evidenceCount: 1,
      proofs: [],
    },
    {
      name: 'Consistency',
      source: 'ARCH_LAB',
      challengeTitle: 'Design a cache',
      toReview: false,
      evidenceCount: 0,
      proofs: [],
    },
  ],
  explainedOnly: [{ name: 'LRU', evidenceCount: 0, proofs: [] }],
}

function renderPanel(detail: ProgressTopicDetail = DETAIL) {
  vi.mocked(getProgressTopic).mockResolvedValue(detail)
  const onOpenQuiz = vi.fn()
  const onOpenChallenge = vi.fn()
  render(
    <TopicDetailPanel sessionId="s1" onOpenQuiz={onOpenQuiz} onOpenChallenge={onOpenChallenge} />,
  )
  return { onOpenQuiz, onOpenChallenge }
}

describe('ProgressTopicDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLabLanguage('en')
  })

  it('shows the topic, its folder and the activity counters', async () => {
    renderPanel()

    expect(await screen.findByRole('heading', { name: 'Cache' })).toBeInTheDocument()
    expect(screen.getByText('System Design')).toBeInTheDocument()
    const activities = screen.getByRole('region', { name: 'Related activities' })
    expect(within(activities).getByText('Quizzes').previousElementSibling).toHaveTextContent('2')
    expect(within(activities).getByText('Arch Lab').previousElementSibling).toHaveTextContent('1')
    expect(
      within(activities).getByText('Concepts worked on').previousElementSibling,
    ).toHaveTextContent('4')
  })

  it('reads the topic of the session it was given', async () => {
    renderPanel()

    await screen.findByRole('heading', { name: 'Cache' })

    expect(getProgressTopic).toHaveBeenCalledWith('s1')
  })

  it('lists a row per quiz with its latest percent and attempts, and opens it', async () => {
    const { onOpenQuiz } = renderPanel()

    const taken = await screen.findByRole('button', { name: /Quiz 1 · Easy/ })
    expect(taken).toHaveTextContent('90%')
    expect(taken).toHaveTextContent('2 attempts')
    const notTaken = screen.getByRole('button', { name: /Quiz 2 · Hard/ })
    expect(notTaken).toHaveTextContent('Not taken')

    await userEvent.click(taken)

    expect(onOpenQuiz).toHaveBeenCalledWith('q1')
  })

  it('shows the diagram and defense scores, the result and the attempts of a challenge, and opens it', async () => {
    const { onOpenChallenge } = renderPanel()

    const row = await screen.findByRole('button', { name: /Design a cache/ })
    expect(row).toHaveTextContent('Diagram 85%')
    expect(row).toHaveTextContent('Defense 40%')
    expect(row).toHaveTextContent('Failed')
    expect(row).toHaveTextContent('3 attempts')

    await userEvent.click(row)

    expect(onOpenChallenge).toHaveBeenCalledWith(DETAIL.challenges[0])
  })

  it('says the defense is pending and the result is pending when there is no defense yet', async () => {
    renderPanel({
      ...DETAIL,
      challenges: [{ ...DETAIL.challenges[0], defenseScore: null, result: 'PENDING', attempts: 1 }],
    })

    const row = await screen.findByRole('button', { name: /Design a cache/ })

    expect(row).toHaveTextContent('Defense pending')
    expect(row).toHaveTextContent('Pending')
    expect(row).toHaveTextContent('1 attempt')
    expect(row).not.toHaveTextContent('1 attempts')
  })

  it('says there is no assessment yet when the topic has no quiz or challenge', async () => {
    renderPanel({ ...DETAIL, quizzes: [], challenges: [], quizCount: 0, challengeCount: 0 })

    const results = await screen.findByRole('region', { name: 'Results' })

    expect(within(results).getAllByText('No assessment yet')).toHaveLength(2)
  })

  it('groups the concepts: understood, difficulties and explained only', async () => {
    renderPanel()

    const understood = await screen.findByRole('region', { name: 'Concepts understood' })
    expect(within(understood).getByText('Cache-aside')).toBeInTheDocument()
    const explained = screen.getByRole('region', { name: 'Explained, not yet assessed' })
    expect(within(explained).getByText('LRU')).toBeInTheDocument()
    expect(within(understood).queryByText('LRU')).not.toBeInTheDocument()
  })

  it('says where each difficulty comes from and marks the ones to review', async () => {
    renderPanel()

    const difficulties = await screen.findByRole('region', { name: 'Difficulties' })

    const ttl = within(difficulties).getByText('TTL').closest('li') as HTMLElement
    expect(ttl).toHaveTextContent('Tutor')
    expect(ttl).toHaveTextContent('to review')
    const consistency = within(difficulties).getByText('Consistency').closest('li') as HTMLElement
    expect(consistency).toHaveTextContent('Arch Lab: Design a cache')
    expect(consistency).not.toHaveTextContent('to review')
  })

  it('says so in every empty learning section', async () => {
    renderPanel({ ...DETAIL, understood: [], difficulties: [], explainedOnly: [] })

    const learning = await screen.findByRole('region', { name: 'Learning identified' })

    expect(within(learning).getByText('No assessment yet')).toBeInTheDocument()
    expect(within(learning).getByText('No difficulty identified')).toBeInTheDocument()
    expect(within(learning).getByText('Nothing was only explained')).toBeInTheDocument()
  })

  it('expands the proofs of a concept, newest first, and collapses them again', async () => {
    renderPanel()
    const understood = await screen.findByRole('region', { name: 'Concepts understood' })
    expect(within(understood).queryByText('Explained invalidation')).not.toBeInTheDocument()

    await userEvent.click(within(understood).getByRole('button', { name: /Show proofs/ }))

    const newest = within(understood)
      .getByText('Explained invalidation')
      .closest('li') as HTMLElement
    const oldest = within(understood).getByText('Older').closest('li') as HTMLElement
    expect(newest).toHaveTextContent('Correct')
    expect(newest).toHaveTextContent('With help')
    expect(oldest).toHaveTextContent('Partially correct')
    expect(newest.compareDocumentPosition(oldest) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

    await userEvent.click(within(understood).getByRole('button', { name: /Hide proofs/ }))

    expect(within(understood).queryByText('Explained invalidation')).not.toBeInTheDocument()
  })

  it('has no proofs button for a concept without proofs', async () => {
    renderPanel()

    const explained = await screen.findByRole('region', { name: 'Explained, not yet assessed' })

    expect(within(explained).queryByRole('button')).not.toBeInTheDocument()
  })

  it('shows an error when the topic cannot be loaded', async () => {
    vi.mocked(getProgressTopic).mockRejectedValue(new Error('boom'))

    render(<TopicDetailPanel sessionId="s1" onOpenQuiz={vi.fn()} onOpenChallenge={vi.fn()} />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load this topic.')
  })

  it('reloads when another topic is selected and ignores a late answer for the old one', async () => {
    let resolveFirst: (value: ProgressTopicDetail) => void = () => {}
    vi.mocked(getProgressTopic)
      .mockReturnValueOnce(new Promise((resolve) => (resolveFirst = resolve)))
      .mockResolvedValueOnce({ ...DETAIL, sessionId: 's2', name: 'Queues' })
    const { rerender } = render(
      <TopicDetailPanel sessionId="s1" onOpenQuiz={vi.fn()} onOpenChallenge={vi.fn()} />,
    )

    rerender(<TopicDetailPanel sessionId="s2" onOpenQuiz={vi.fn()} onOpenChallenge={vi.fn()} />)
    expect(await screen.findByRole('heading', { name: 'Queues' })).toBeInTheDocument()
    resolveFirst(DETAIL)

    await Promise.resolve()
    expect(screen.queryByRole('heading', { name: 'Cache' })).not.toBeInTheDocument()
  })

  it('follows the assistant language', async () => {
    setLabLanguage('pt')
    renderPanel()

    expect(await screen.findByRole('region', { name: 'Resultados' })).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Dificuldades' })).toBeInTheDocument()
  })
})
