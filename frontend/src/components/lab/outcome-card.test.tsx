import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { Outcome } from '@/lib/exercise'
import { OutcomeCard } from './outcome-card'

const passed: Outcome = {
  result: 'PASSED',
  passScore: 80,
  diagramScore: 85,
  understandingScore: 90,
  reasons: [],
}

const failed: Outcome = {
  result: 'FAILED',
  passScore: 80,
  diagramScore: 85,
  understandingScore: 60,
  reasons: [
    { kind: 'UNDERSTANDING_SCORE', evaluation: 'UNDERSTANDING', criterionName: '' },
    { kind: 'DIAGRAM_SCORE', evaluation: 'DIAGRAM', criterionName: '' },
    { kind: 'MANDATORY_CRITERION', evaluation: 'DIAGRAM', criterionName: 'Resilience' },
    { kind: 'MANDATORY_CRITERION', evaluation: 'UNDERSTANDING', criterionName: 'Scalability' },
  ],
}

function setup(outcome: Outcome) {
  render(<OutcomeCard outcome={outcome} />)
}

describe('OutcomeCard', () => {
  it('shows Passed with both scores and the minimum, and no reasons', () => {
    setup(passed)

    expect(screen.getByRole('heading', { name: 'Passed' })).toBeInTheDocument()
    expect(screen.getByText('Diagram')).toBeInTheDocument()
    expect(screen.getByText('85')).toBeInTheDocument()
    expect(screen.getByText('Understanding')).toBeInTheDocument()
    expect(screen.getByText('90')).toBeInTheDocument()
    expect(screen.getAllByText('/ 100')).toHaveLength(2)
    expect(screen.getByText('Minimum to pass: 80')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('shows Failed with every reason', () => {
    setup(failed)

    expect(screen.getByRole('heading', { name: 'Failed' })).toBeInTheDocument()
    expect(screen.getByText('85')).toBeInTheDocument()
    expect(screen.getByText('60')).toBeInTheDocument()
    const reasons = screen.getAllByRole('listitem').map((item) => item.textContent)
    expect(reasons).toEqual([
      'The understanding score is below the minimum of 80.',
      'The diagram score is below the minimum of 80.',
      'Mandatory criterion not met in the diagram: Resilience.',
      'Mandatory criterion not met in the defense: Scalability.',
    ])
  })
})
