import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Evaluation } from '@/lib/exercise'
import { EvaluationStage } from './evaluation-stage'

const evaluation: Evaluation = {
  score: 72,
  summary: 'A solid start.',
  strengths: ['Clear API'],
  gaps: ['No cache failover'],
  recommendations: ['Add replicas'],
  results: [
    {
      key: 'scalability',
      name: 'Scalability',
      level: 3,
      justification: 'The cache is shown.',
      evidence: 'API -> Cache',
      insufficientEvidence: false,
    },
    {
      key: 'resilience',
      name: 'Resilience',
      level: 0,
      justification: 'Nothing about failures was drawn.',
      evidence: '',
      insufficientEvidence: true,
    },
  ],
}

function setup(props: Partial<React.ComponentProps<typeof EvaluationStage>> = {}) {
  const onRetry = vi.fn()
  const onBack = vi.fn()
  render(
    <EvaluationStage
      phase="evaluated"
      message={null}
      evaluation={evaluation}
      onRetry={onRetry}
      onBack={onBack}
      {...props}
    />,
  )
  return { onRetry, onBack }
}

describe('EvaluationStage', () => {
  it('shows progress while the diagram is evaluated, with nothing to click', () => {
    setup({ phase: 'evaluating', evaluation: null })

    expect(screen.getByRole('status')).toHaveTextContent('Evaluating your diagram...')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('shows the reason and offers to try again or go back when the evaluation failed', async () => {
    const user = userEvent.setup()
    const { onRetry, onBack } = setup({
      phase: 'evaluationFailed',
      message: 'The diagram is empty.',
      evaluation: null,
    })

    expect(screen.getByRole('alert')).toHaveTextContent('The diagram is empty.')
    await user.click(screen.getByRole('button', { name: 'Try evaluating again' }))
    expect(onRetry).toHaveBeenCalledOnce()
    await user.click(screen.getByRole('button', { name: 'Back to catalog' }))
    expect(onBack).toHaveBeenCalledOnce()
  })

  it('shows the failure, not a result, when the evaluation failed after an earlier result was held', () => {
    setup({ phase: 'evaluationFailed', message: 'The diagram is empty.' })

    expect(screen.getByRole('alert')).toHaveTextContent('The diagram is empty.')
    expect(screen.queryByText('72')).not.toBeInTheDocument()
  })

  it('falls back to the failure state when evaluated without a result', () => {
    setup({ phase: 'evaluated', evaluation: null })

    expect(screen.getByRole('button', { name: 'Try evaluating again' })).toBeInTheDocument()
  })

  it('shows the score and the summary', () => {
    setup()

    expect(screen.getByText('72')).toBeInTheDocument()
    expect(screen.getByText('/ 100')).toBeInTheDocument()
    expect(screen.getByText('A solid start.')).toBeInTheDocument()
  })

  it('shows the feedback per criterion, flagging a criterion the diagram did not show', () => {
    setup()

    const scalability = screen.getByRole('listitem', { name: 'Scalability' })
    expect(within(scalability).getByText('Level 3 of 4')).toBeInTheDocument()
    expect(within(scalability).getByText('The cache is shown.')).toBeInTheDocument()
    expect(within(scalability).getByText('API -> Cache')).toBeInTheDocument()
    const resilience = screen.getByRole('listitem', { name: 'Resilience' })
    expect(within(resilience).getByText('Not enough evidence in the diagram')).toBeInTheDocument()
    expect(within(resilience).queryByText(/Level/)).not.toBeInTheDocument()
    expect(within(resilience).getByText('Nothing about failures was drawn.')).toBeInTheDocument()
    // No evidence quote means no empty paragraph for it.
    expect(resilience.querySelectorAll('p')).toHaveLength(1)
    expect(scalability.querySelectorAll('p')).toHaveLength(2)
  })

  it('lists strengths, gaps and recommendations, leaving out an empty list', () => {
    setup({ evaluation: { ...evaluation, gaps: [] } })

    expect(screen.getByRole('heading', { name: 'Strengths' })).toBeInTheDocument()
    expect(screen.getByText('Clear API')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Recommendations' })).toBeInTheDocument()
    expect(screen.getByText('Add replicas')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Gaps' })).not.toBeInTheDocument()
  })

  it('goes back to the catalog from the result', async () => {
    const user = userEvent.setup()
    const { onBack } = setup()

    await user.click(screen.getByRole('button', { name: 'Back to catalog' }))

    expect(onBack).toHaveBeenCalledOnce()
  })
})
