import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { generateProgressInsight, type ProgressInsightState } from '@/lib/progress'
import { setLabLanguage } from '@/i18n'
import { ProgressInsightCard } from './progress-insight-card'

vi.mock('@/lib/progress', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/progress')>()),
  generateProgressInsight: vi.fn(),
}))

const FRESH: ProgressInsightState = {
  insight: {
    analysis: [
      { text: 'Quizzes improved.', citations: [{ kind: 'QUIZ', id: 'q1', label: 'Quiz 1' }] },
      { text: 'TTL is weak.', citations: [] },
    ],
    recommendations: [
      {
        text: 'Review eviction policies.',
        citations: [
          { kind: 'CHALLENGE', id: 'c1', label: 'Design a cache' },
          { kind: 'CONCEPT', id: 'TTL', label: 'TTL' },
        ],
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

describe('ProgressInsightCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLabLanguage('en')
  })

  it('shows the stored insight at once, without calling the model', () => {
    render(<ProgressInsightCard sessionId="s1" initial={FRESH} />)

    expect(screen.getByRole('heading', { name: 'Athena Insights' })).toBeInTheDocument()
    expect(screen.getByText(/Quizzes improved\./)).toBeInTheDocument()
    expect(screen.getByText(/TTL is weak\./)).toBeInTheDocument()
    expect(screen.getByText('Recommendations')).toBeInTheDocument()
    expect(screen.getByText(/Review eviction policies\./)).toBeInTheDocument()
    expect(generateProgressInsight).not.toHaveBeenCalled()
  })

  it('names the activities that support a sentence and a recommendation', () => {
    render(<ProgressInsightCard sessionId="s1" initial={FRESH} />)

    expect(screen.getByText('based on: Quiz 1')).toBeInTheDocument()
    expect(screen.getByText('based on: Design a cache, TTL')).toBeInTheDocument()
  })

  it('shows when it was generated and by which model', () => {
    render(<ProgressInsightCard sessionId="s1" initial={FRESH} />)

    expect(screen.getByText(/Generated on .* · anthropic\/x/)).toBeInTheDocument()
  })

  it('offers to generate the first insight and explains what it uses', () => {
    render(<ProgressInsightCard sessionId="s1" initial={NONE} />)

    expect(screen.getByRole('button', { name: 'Generate insights' })).toBeInTheDocument()
    expect(screen.getByText(/never the whole conversation/)).toBeInTheDocument()
  })

  it('offers to update a stored insight and flags it when stale', () => {
    render(<ProgressInsightCard sessionId="s1" initial={{ ...FRESH, stale: true }} />)

    expect(screen.getByRole('button', { name: 'Update insights' })).toBeInTheDocument()
    expect(screen.getByText(/New activity since this analysis/)).toBeInTheDocument()
  })

  it('does not flag a fresh insight as stale', () => {
    render(<ProgressInsightCard sessionId="s1" initial={FRESH} />)

    expect(screen.queryByText(/New activity since this analysis/)).not.toBeInTheDocument()
  })

  it('has no button, only guidance, when the topic has no evidence', () => {
    render(
      <ProgressInsightCard
        sessionId="s1"
        initial={{ insight: null, stale: false, canGenerate: false }}
      />,
    )

    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getByText(/Study with the tutor, take a quiz or a lab/)).toBeInTheDocument()
  })

  it('generates for its session, shows it and clears the stale flag', async () => {
    vi.mocked(generateProgressInsight).mockResolvedValue(FRESH)
    render(<ProgressInsightCard sessionId="s1" initial={{ ...NONE }} />)

    await userEvent.click(screen.getByRole('button', { name: 'Generate insights' }))

    expect(generateProgressInsight).toHaveBeenCalledWith('s1')
    expect(await screen.findByText(/Quizzes improved\./)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Update insights' })).toBeInTheDocument()
  })

  it('disables the button while generating', async () => {
    let resolve: (state: ProgressInsightState) => void = () => {}
    vi.mocked(generateProgressInsight).mockReturnValue(
      new Promise((done) => {
        resolve = done
      }),
    )
    render(<ProgressInsightCard sessionId="s1" initial={NONE} />)

    await userEvent.click(screen.getByRole('button', { name: 'Generate insights' }))

    expect(screen.getByRole('button', { name: 'Generating...' })).toBeDisabled()
    resolve(FRESH)
    expect(await screen.findByText(/Quizzes improved\./)).toBeInTheDocument()
  })

  it('shows the error with Try again, keeps the previous insight and retries', async () => {
    vi.mocked(generateProgressInsight).mockRejectedValueOnce(new Error('provider down'))
    vi.mocked(generateProgressInsight).mockResolvedValueOnce({
      ...FRESH,
      insight: { ...FRESH.insight!, analysis: [{ text: 'A new analysis.', citations: [] }] },
    })
    render(<ProgressInsightCard sessionId="s1" initial={{ ...FRESH, stale: true }} />)

    await userEvent.click(screen.getByRole('button', { name: 'Update insights' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not generate the insights')
    expect(screen.getByText(/Quizzes improved\./)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByText(/A new analysis\./)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('follows the assistant language', () => {
    setLabLanguage('pt')
    render(<ProgressInsightCard sessionId="s1" initial={NONE} />)

    expect(screen.getByRole('button', { name: 'Gerar insights' })).toBeInTheDocument()
  })
})
