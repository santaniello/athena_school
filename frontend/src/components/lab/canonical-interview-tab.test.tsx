import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  generateCanonicalChallenge,
  listCanonicalProblems,
  type CanonicalProblem,
} from '@/lib/exercise'
import { CanonicalInterviewTab } from './canonical-interview-tab'

vi.mock('@/lib/exercise', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/exercise')>()
  return {
    ...original,
    listCanonicalProblems: vi.fn(),
    generateCanonicalChallenge: vi.fn(),
  }
})

function problem(id: string, name: string, history = false): CanonicalProblem {
  return {
    id,
    name,
    summary: `${name} summary`,
    attempts: history ? 1 : 0,
    hasHistory: history,
    difficulties: [
      { difficulty: 'BEGINNER', generated: false, challengeId: '', status: '' },
      { difficulty: 'INTERMEDIATE', generated: false, challengeId: '', status: '' },
      { difficulty: 'ADVANCED', generated: false, challengeId: '', status: '' },
    ],
  }
}

function renderTab(overrides: Partial<Parameters<typeof CanonicalInterviewTab>[0]> = {}) {
  const props = { onReady: vi.fn(), onHistory: vi.fn(), ...overrides }
  render(<CanonicalInterviewTab {...props} />)
  return props
}

describe('CanonicalInterviewTab', () => {
  beforeEach(() => vi.clearAllMocks())

  it('lists the problems in the order the backend sends, with no generation', async () => {
    vi.mocked(listCanonicalProblems).mockResolvedValueOnce([
      problem('url-shortener', 'URL shortener'),
      problem('rate-limiter', 'Rate limiter'),
    ])

    renderTab()

    const rows = await screen.findAllByRole('listitem')
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent('URL shortener')
    expect(rows[1]).toHaveTextContent('Rate limiter')
    expect(generateCanonicalChallenge).not.toHaveBeenCalled()
  })

  it('says it is loading until the list arrives', () => {
    vi.mocked(listCanonicalProblems).mockReturnValueOnce(new Promise(() => {}))

    renderTab()

    expect(screen.getByText('Loading interviews…')).toBeInTheDocument()
  })

  it('shows the failure and reads the list again on retry', async () => {
    vi.mocked(listCanonicalProblems)
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce([problem('url-shortener', 'URL shortener')])

    renderTab()
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the interviews.')
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByText('URL shortener')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('starts a problem: asks the difficulty, then hands the challenge to the screen and refreshes', async () => {
    vi.mocked(listCanonicalProblems)
      .mockResolvedValueOnce([problem('url-shortener', 'URL shortener')])
      .mockResolvedValueOnce([problem('url-shortener', 'URL shortener', true)])
    vi.mocked(generateCanonicalChallenge).mockResolvedValueOnce({
      id: 'c1',
      kind: 'INTERVIEW',
      title: 'URL shortener',
      topic: 'url shortener',
      difficulty: 'BEGINNER',
      timeLimitMinutes: 30,
    })
    const { onReady } = renderTab({ experienceLevel: 'beginner' })

    await userEvent.click(await screen.findByRole('button', { name: 'Start' }))
    await userEvent.click(screen.getByRole('button', { name: 'Start' }))

    await waitFor(() =>
      expect(onReady).toHaveBeenCalledWith({
        id: 'c1',
        title: 'URL shortener',
        difficulty: 'BEGINNER',
        timeLimitMinutes: 30,
      }),
    )
    expect(generateCanonicalChallenge).toHaveBeenCalledWith('url-shortener', 'BEGINNER')
    await waitFor(() => expect(listCanonicalProblems).toHaveBeenCalledTimes(2))
    expect(await screen.findByRole('button', { name: 'History' })).toBeInTheDocument()
  })

  it('opens the history of a problem', async () => {
    const withHistory = problem('url-shortener', 'URL shortener', true)
    vi.mocked(listCanonicalProblems).mockResolvedValueOnce([withHistory])
    const { onHistory } = renderTab()

    await userEvent.click(await screen.findByRole('button', { name: 'History' }))

    expect(onHistory).toHaveBeenCalledWith(withHistory)
  })
})
