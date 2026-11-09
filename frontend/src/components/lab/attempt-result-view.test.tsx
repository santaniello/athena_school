import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { getAttemptResult, type AttemptResult } from '@/lib/exercise'
import { AttemptResultView } from './attempt-result-view'

vi.mock('@/lib/exercise', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/exercise')>()
  return { ...original, getAttemptResult: vi.fn() }
})

vi.mock('@/components/lab/diagram-editor', () => ({
  DiagramEditor: (props: { readOnly?: boolean; xml?: string }) => (
    <div data-testid="editor" data-read-only={String(props.readOnly)} data-xml={props.xml} />
  ),
}))

const result: AttemptResult = {
  challenge: {
    id: 'c1',
    title: 'Cache and invalidation',
    topic: 'caching',
    difficulty: 'INTERMEDIATE',
    statement: {
      scenario: 'A catalog service.',
      functionalRequirements: ['Serve products'],
      nonFunctionalRequirements: ['Fast'],
      constraints: ['No new DB'],
      objectives: ['Cut reads'],
    },
  },
  attemptId: 'a1',
  startedAt: '2026-11-07T09:30:00Z',
  finishedAt: '2026-11-07T11:00:00Z',
  timeSpentSeconds: 1800,
  submitReason: 'USER',
  hintsUsed: 1,
  maxHints: 3,
  outcome: {
    result: 'FAILED',
    passScore: 80,
    diagramScore: 85,
    understandingScore: 60,
    reasons: [{ kind: 'UNDERSTANDING_SCORE', evaluation: 'UNDERSTANDING', criterionName: '' }],
  },
  diagram: {
    score: 85,
    summary: 'Good diagram.',
    strengths: [],
    gaps: [],
    recommendations: [],
    results: [
      {
        key: 'k1',
        name: 'Consistency',
        weight: 50,
        level: 3,
        justification: 'Drawn.',
        evidence: '',
        insufficientEvidence: false,
      },
      {
        key: 'k2',
        name: 'Resilience',
        weight: 50,
        level: 0,
        justification: '',
        evidence: '',
        insufficientEvidence: true,
      },
    ],
  },
  understanding: {
    score: 60,
    summary: 'Weak answers.',
    strengths: [],
    gaps: [],
    recommendations: [],
    results: [
      {
        key: 'k1',
        name: 'Consistency',
        weight: 50,
        level: 2,
        justification: 'Fair.',
        evidence: '',
        insufficientEvidence: false,
      },
      {
        key: 'k2',
        name: 'Resilience',
        weight: 50,
        level: 0,
        justification: '',
        evidence: '',
        insufficientEvidence: true,
      },
    ],
  },
  defense: [{ position: 1, question: 'Why a cache?', answer: 'To cut reads.' }],
  submission: '<mxfile/>',
}

function setup() {
  const onBack = vi.fn()
  const view = render(
    <AttemptResultView attemptId="a1" backLabel="Back to catalog" onBack={onBack} />,
  )
  return { onBack, ...view }
}

describe('AttemptResultView', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows a loading message while the result is read', () => {
    vi.mocked(getAttemptResult).mockReturnValue(new Promise(() => {}))

    setup()

    expect(screen.getByRole('status')).toHaveTextContent('Loading the result...')
    expect(getAttemptResult).toHaveBeenCalledExactlyOnceWith('a1')
  })

  it('shows every section of the result, the statement above the outcome', async () => {
    vi.mocked(getAttemptResult).mockResolvedValue(result)

    setup()

    expect(
      await screen.findByRole('heading', { name: 'Cache and invalidation' }),
    ).toBeInTheDocument()
    const headings = screen.getAllByRole('heading').map((heading) => heading.textContent)
    const order = [
      'Cache and invalidation',
      'Scenario',
      'Failed',
      'Diagram evaluation',
      'Understanding evaluation',
      'Tutor questions',
      'Submitted diagram',
    ].map((title) => headings.indexOf(title))
    expect(order).not.toContain(-1)
    expect([...order].sort((a, b) => a - b)).toEqual(order)
    expect(screen.getByText('A catalog service.')).toBeInTheDocument()
    expect(screen.getByText('Minimum to pass: 80')).toBeInTheDocument()
    expect(
      screen.getByText('The understanding score is below the minimum of 80.'),
    ).toBeInTheDocument()
    expect(screen.getByText('To cut reads.')).toBeInTheDocument()
    expect(screen.getByTestId('editor')).toHaveAttribute('data-xml', '<mxfile/>')
  })

  it('labels the not judged criteria differently in each evaluation', async () => {
    vi.mocked(getAttemptResult).mockResolvedValue(result)

    setup()

    const diagram = await screen.findByRole('region', { name: 'Diagram evaluation' })
    const understanding = screen.getByRole('region', { name: 'Understanding evaluation' })
    expect(within(diagram).getByText('Not judged from the diagram')).toBeInTheDocument()
    expect(within(understanding).getByText('Not asked')).toBeInTheDocument()
    expect(within(understanding).queryByText('Not judged from the diagram')).not.toBeInTheDocument()
  })

  it('goes back with the label it was given', async () => {
    vi.mocked(getAttemptResult).mockResolvedValue(result)
    const { onBack } = setup()
    await screen.findByRole('heading', { name: 'Cache and invalidation' })

    await userEvent.click(screen.getByRole('button', { name: 'Back to catalog' }))

    expect(onBack).toHaveBeenCalledOnce()
  })

  it('says the result could not be loaded and reads it again on request', async () => {
    vi.mocked(getAttemptResult).mockRejectedValueOnce(new Error('boom'))
    const { onBack } = setup()
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the result.')

    vi.mocked(getAttemptResult).mockResolvedValue(result)
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))

    expect(
      await screen.findByRole('heading', { name: 'Cache and invalidation' }),
    ).toBeInTheDocument()
    expect(getAttemptResult).toHaveBeenCalledTimes(2)
    expect(onBack).not.toHaveBeenCalled()
  })

  it('can leave from the failure', async () => {
    vi.mocked(getAttemptResult).mockRejectedValue(new Error('boom'))
    const { onBack } = setup()
    await screen.findByRole('alert')

    await userEvent.click(screen.getByRole('button', { name: 'Back to catalog' }))

    expect(onBack).toHaveBeenCalledOnce()
  })
})
