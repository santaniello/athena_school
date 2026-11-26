import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  generateProgressFolderInsight,
  getProgressFolderInsight,
  type ProgressInsightState,
} from '@/lib/progress'
import { setLabLanguage } from '@/i18n'
import { ProgressFolderInsightCard } from './progress-folder-insight-card'

vi.mock('@/lib/progress', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/progress')>()),
  getProgressFolderInsight: vi.fn(),
  generateProgressFolderInsight: vi.fn(),
}))

const FRESH: ProgressInsightState = {
  insight: {
    analysis: [
      {
        text: 'Cache is solid.',
        citations: [{ kind: 'TOPIC', id: 's1', label: 'Cache' }],
      },
      { text: 'Queues need work.', citations: [] },
    ],
    recommendations: [
      {
        text: 'Study ordering next.',
        citations: [{ kind: 'TOPIC', id: 's2', label: 'Queues' }],
      },
    ],
    model: 'anthropic/x',
    language: 'en',
    createdAt: '2026-12-01T09:00:00Z',
  },
  stale: false,
  canGenerate: true,
}

const NONE: ProgressInsightState = { insight: null, stale: false, canGenerate: true }

function renderCard(onOpenTopic = vi.fn(), topicIds = ['s1', 's2']) {
  render(<ProgressFolderInsightCard folderId="f1" topicIds={topicIds} onOpenTopic={onOpenTopic} />)
  return onOpenTopic
}

describe('ProgressFolderInsightCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLabLanguage('en')
    vi.mocked(getProgressFolderInsight).mockResolvedValue(FRESH)
  })

  it('reads the stored insight of the folder without calling the model', async () => {
    renderCard()

    expect(await screen.findByText(/Cache is solid\./)).toBeInTheDocument()
    expect(getProgressFolderInsight).toHaveBeenCalledWith('f1')
    expect(generateProgressFolderInsight).not.toHaveBeenCalled()
  })

  it('shows only the first sentence until expanded', async () => {
    renderCard()

    expect(await screen.findByText(/Cache is solid\./)).toBeInTheDocument()
    expect(screen.queryByText(/Queues need work\./)).not.toBeInTheDocument()
    expect(screen.queryByText('Recommendations')).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Show more' }))

    expect(screen.getByText(/Queues need work\./)).toBeInTheDocument()
    expect(screen.getByText('Recommendations')).toBeInTheDocument()
    expect(screen.getByText(/Study ordering next\./)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Show less' }))

    expect(screen.queryByText(/Queues need work\./)).not.toBeInTheDocument()
  })

  it('opens the topic a recommendation cites', async () => {
    const onOpenTopic = renderCard()
    await userEvent.click(await screen.findByRole('button', { name: 'Show more' }))

    await userEvent.click(screen.getByRole('button', { name: 'Queues' }))

    expect(onOpenTopic).toHaveBeenCalledWith('s2')
  })

  it('shows a citation as plain text when its topic is no longer in the folder', async () => {
    renderCard(vi.fn(), ['s1'])
    await userEvent.click(await screen.findByRole('button', { name: 'Show more' }))

    expect(screen.getByText('Queues')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Queues' })).not.toBeInTheDocument()
  })

  it('offers to generate the first insight', async () => {
    vi.mocked(getProgressFolderInsight).mockResolvedValue(NONE)
    renderCard()

    expect(await screen.findByRole('button', { name: 'Generate insights' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Show more' })).not.toBeInTheDocument()
  })

  it('flags a stale insight and offers to update it', async () => {
    vi.mocked(getProgressFolderInsight).mockResolvedValue({ ...FRESH, stale: true })
    renderCard()

    expect(await screen.findByText(/New activity since this analysis/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Update insights' })).toBeInTheDocument()
  })

  it('has no button, only guidance, when no topic has evidence', async () => {
    vi.mocked(getProgressFolderInsight).mockResolvedValue({
      insight: null,
      stale: false,
      canGenerate: false,
    })
    renderCard()

    expect(await screen.findByText(/no topic with evidence/i)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /insights/i })).not.toBeInTheDocument()
  })

  it('generates for its folder and replaces the card with the fresh insight', async () => {
    vi.mocked(getProgressFolderInsight).mockResolvedValue(NONE)
    vi.mocked(generateProgressFolderInsight).mockResolvedValue(FRESH)
    renderCard()

    await userEvent.click(await screen.findByRole('button', { name: 'Generate insights' }))

    expect(generateProgressFolderInsight).toHaveBeenCalledWith('f1')
    expect(await screen.findByText(/Cache is solid\./)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Update insights' })).toBeInTheDocument()
  })

  it('keeps the stored insight when the folder lost its evidence meanwhile', async () => {
    vi.mocked(getProgressFolderInsight).mockResolvedValue({ ...FRESH, stale: true })
    vi.mocked(generateProgressFolderInsight).mockResolvedValue({
      insight: null,
      stale: false,
      canGenerate: false,
    })
    renderCard()

    await userEvent.click(await screen.findByRole('button', { name: 'Update insights' }))

    expect(await screen.findByText(/Cache is solid\./)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Update insights' })).not.toBeInTheDocument()
  })

  it('disables the button while generating', async () => {
    vi.mocked(getProgressFolderInsight).mockResolvedValue(NONE)
    let resolve: (state: ProgressInsightState) => void = () => {}
    vi.mocked(generateProgressFolderInsight).mockReturnValue(
      new Promise((done) => {
        resolve = done
      }),
    )
    renderCard()

    await userEvent.click(await screen.findByRole('button', { name: 'Generate insights' }))

    expect(screen.getByRole('button', { name: 'Generating...' })).toBeDisabled()
    resolve(FRESH)
    expect(await screen.findByText(/Cache is solid\./)).toBeInTheDocument()
  })

  it('shows the error, keeps the previous insight and retries', async () => {
    vi.mocked(getProgressFolderInsight).mockResolvedValue({ ...FRESH, stale: true })
    vi.mocked(generateProgressFolderInsight).mockRejectedValueOnce(new Error('provider down'))
    vi.mocked(generateProgressFolderInsight).mockResolvedValueOnce({
      ...FRESH,
      insight: { ...FRESH.insight!, analysis: [{ text: 'A new analysis.', citations: [] }] },
    })
    renderCard()

    await userEvent.click(await screen.findByRole('button', { name: 'Update insights' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not generate the insights')
    expect(screen.getByText(/Cache is solid\./)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByText(/A new analysis\./)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('does not claim a previous analysis was kept when there was none', async () => {
    vi.mocked(getProgressFolderInsight).mockResolvedValue(NONE)
    vi.mocked(generateProgressFolderInsight).mockRejectedValueOnce(new Error('provider down'))
    renderCard()

    await userEvent.click(await screen.findByRole('button', { name: 'Generate insights' }))

    const alert = await screen.findByRole('alert')
    expect(alert).not.toHaveTextContent('previous analysis')
  })

  it('shows a failure to read the stored insight with a retry, never blocking', async () => {
    vi.mocked(getProgressFolderInsight).mockRejectedValueOnce(new Error('boom'))
    renderCard()

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the insights')

    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByText(/Cache is solid\./)).toBeInTheDocument()
  })

  it('follows the assistant language', async () => {
    setLabLanguage('pt')
    vi.mocked(getProgressFolderInsight).mockResolvedValue(NONE)
    renderCard()

    expect(await screen.findByRole('button', { name: 'Gerar insights' })).toBeInTheDocument()
  })
})
