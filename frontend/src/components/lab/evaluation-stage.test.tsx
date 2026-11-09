import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { EvaluationStage } from './evaluation-stage'

function setup(props: Partial<React.ComponentProps<typeof EvaluationStage>> = {}) {
  const onRetry = vi.fn()
  const onBack = vi.fn()
  render(
    <EvaluationStage
      phase="evaluating"
      message={null}
      backLabel="Back to catalog"
      onRetry={onRetry}
      onBack={onBack}
      {...props}
    />,
  )
  return { onRetry, onBack }
}

describe('EvaluationStage', () => {
  it('shows progress while the diagram is evaluated, with nothing to click', () => {
    setup({ phase: 'evaluating' })

    expect(screen.getByRole('status')).toHaveTextContent('Evaluating your diagram...')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('shows the reason and offers to try again or go back when the evaluation failed', async () => {
    const user = userEvent.setup()
    const { onRetry, onBack } = setup({
      phase: 'evaluationFailed',
      message: 'The diagram is empty.',
    })

    expect(screen.getByRole('alert')).toHaveTextContent('The diagram is empty.')
    await user.click(screen.getByRole('button', { name: 'Try evaluating again' }))
    expect(onRetry).toHaveBeenCalledOnce()
    await user.click(screen.getByRole('button', { name: 'Back to catalog' }))
    expect(onBack).toHaveBeenCalledOnce()
  })

  it('names the way back as it is told to', async () => {
    setup({ phase: 'evaluationFailed', message: 'x', backLabel: 'Back to study session' })

    expect(screen.getByRole('button', { name: 'Back to study session' })).toBeInTheDocument()
  })
})
