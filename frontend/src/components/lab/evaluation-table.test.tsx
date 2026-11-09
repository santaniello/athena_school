import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import type { Evaluation } from '@/lib/exercise'
import { EvaluationTable } from './evaluation-table'

const evaluation: Evaluation = {
  score: 72,
  summary: 'A solid start.',
  strengths: ['Clear API'],
  gaps: ['No failover'],
  recommendations: ['Add replicas'],
  results: [
    {
      key: 'scalability',
      name: 'Scalability',
      weight: 60,
      level: 3,
      justification: 'Cache shown.',
      evidence: 'API -> Cache',
      insufficientEvidence: false,
    },
    {
      key: 'resilience',
      name: 'Resilience',
      weight: 40,
      level: 0,
      justification: '',
      evidence: '',
      insufficientEvidence: true,
    },
  ],
}

function setup(overrides: Partial<Evaluation> = {}) {
  render(
    <EvaluationTable
      title="Diagram evaluation"
      evaluation={{ ...evaluation, ...overrides }}
      notJudgedLabel="Not judged from the diagram"
    />,
  )
}

describe('EvaluationTable', () => {
  it('shows the title, the score and the summary', () => {
    setup()

    expect(screen.getByRole('heading', { name: 'Diagram evaluation' })).toBeInTheDocument()
    expect(screen.getByText('Score 72 / 100')).toBeInTheDocument()
    expect(screen.getByText('A solid start.')).toBeInTheDocument()
  })

  it('shows per criterion the name, weight, level, justification and evidence', () => {
    setup()

    const row = screen.getAllByRole('listitem')[0]
    expect(within(row).getByText(/Scalability/)).toBeInTheDocument()
    expect(within(row).getByText(/weight 60%/)).toBeInTheDocument()
    expect(within(row).getByText('Level 3 / 4')).toBeInTheDocument()
    expect(within(row).getByText('Cache shown.')).toBeInTheDocument()
    expect(within(row).getByText('Evidence: API -> Cache')).toBeInTheDocument()
  })

  it('says a criterion was not judged instead of showing level 0', () => {
    setup()

    const row = screen.getAllByRole('listitem')[1]
    expect(within(row).getByText('Not judged from the diagram')).toBeInTheDocument()
    expect(within(row).queryByText(/Level/)).not.toBeInTheDocument()
    expect(within(row).getByText(/weight 40%/)).toBeInTheDocument()
  })

  it('shows no justification or evidence line when there is none', () => {
    setup()

    const row = screen.getAllByRole('listitem')[1]
    expect(within(row).queryByText(/Evidence/)).not.toBeInTheDocument()
    expect(row.querySelectorAll('p')).toHaveLength(1)
  })

  it('shows the strengths, gaps and recommendations', () => {
    setup()

    expect(screen.getByRole('heading', { name: 'Strengths' })).toBeInTheDocument()
    expect(screen.getByText('Clear API')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Gaps' })).toBeInTheDocument()
    expect(screen.getByText('No failover')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Recommendations' })).toBeInTheDocument()
    expect(screen.getByText('Add replicas')).toBeInTheDocument()
  })

  it('leaves out the empty lists and the empty summary', () => {
    setup({ summary: '', strengths: [], gaps: [], recommendations: [] })

    expect(screen.queryByRole('heading', { name: 'Strengths' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Gaps' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Recommendations' })).not.toBeInTheDocument()
    expect(screen.queryByText('A solid start.')).not.toBeInTheDocument()
  })
})
