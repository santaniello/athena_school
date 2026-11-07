import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { DefenseQuestion } from '@/lib/exercise'
import { DefenseStage } from './defense-stage'

const question: DefenseQuestion = { position: 2, text: 'What if the cache fails?', total: 5 }

function setup(props: Partial<React.ComponentProps<typeof DefenseStage>> = {}) {
  const onSend = vi.fn()
  const onRetry = vi.fn()
  const view = render(
    <DefenseStage
      phase="answering"
      question={question}
      message={null}
      onSend={onSend}
      onRetry={onRetry}
      {...props}
    />,
  )
  return { onSend, onRetry, ...view }
}

describe('DefenseStage', () => {
  it('shows the question with its position and nothing about score or correctness', () => {
    setup()

    expect(screen.getByRole('heading', { name: 'Question 2 of 5' })).toBeInTheDocument()
    expect(screen.getByText('What if the cache fails?')).toBeInTheDocument()
    expect(screen.queryByText(/score|level|correct|right|wrong/i)).not.toBeInTheDocument()
  })

  it('shows an empty answer box with a character counter, and nothing to send yet', () => {
    setup()

    expect(screen.getByRole('textbox', { name: 'Your answer' })).toHaveValue('')
    expect(screen.getByText('0 / 4000')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Send answer' })).toBeDisabled()
  })

  it('counts what is typed and limits the answer to 4,000 characters', async () => {
    const user = userEvent.setup()
    setup()
    const box = screen.getByRole('textbox', { name: 'Your answer' })

    await user.type(box, 'To cut reads.')

    expect(screen.getByText('13 / 4000')).toBeInTheDocument()
    expect(box).toHaveAttribute('maxLength', '4000')
  })

  it('does not send an answer that is only whitespace', async () => {
    const user = userEvent.setup()
    setup()

    await user.type(screen.getByRole('textbox', { name: 'Your answer' }), '   ')

    expect(screen.getByRole('button', { name: 'Send answer' })).toBeDisabled()
  })

  it('sends what the user wrote', async () => {
    const user = userEvent.setup()
    const { onSend } = setup()

    await user.type(screen.getByRole('textbox', { name: 'Your answer' }), 'We fall back.')
    await user.click(screen.getByRole('button', { name: 'Send answer' }))

    expect(onSend).toHaveBeenCalledExactlyOnceWith('We fall back.')
  })

  it('sends the fixed "I don\'t know" answer without needing any text', async () => {
    const user = userEvent.setup()
    const { onSend } = setup()

    await user.click(screen.getByRole('button', { name: "I don't know" }))

    expect(onSend).toHaveBeenCalledExactlyOnceWith("I don't know")
  })

  it('starts each question with an empty answer box', async () => {
    const user = userEvent.setup()
    const { rerender, onSend, onRetry } = setup()
    await user.type(screen.getByRole('textbox', { name: 'Your answer' }), 'draft')

    rerender(
      <DefenseStage
        phase="answering"
        question={{ position: 3, text: 'Next?', total: 5 }}
        message={null}
        onSend={onSend}
        onRetry={onRetry}
      />,
    )

    expect(screen.getByRole('textbox', { name: 'Your answer' })).toHaveValue('')
    expect(screen.getByText('0 / 4000')).toBeInTheDocument()
  })

  it('locks the answer while it is being sent', () => {
    setup({ phase: 'sendingAnswer' })

    expect(screen.getByRole('textbox', { name: 'Your answer' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Sending...' })).toBeDisabled()
    expect(screen.getByRole('button', { name: "I don't know" })).toBeDisabled()
  })

  it('says why an answer was refused and keeps the question and the text', async () => {
    const user = userEvent.setup()
    const { rerender, onSend, onRetry } = setup()
    await user.type(screen.getByRole('textbox', { name: 'Your answer' }), 'my answer')

    rerender(
      <DefenseStage
        phase="answering"
        question={question}
        message="The tutor could not continue the defense."
        onSend={onSend}
        onRetry={onRetry}
      />,
    )

    expect(screen.getByRole('alert')).toHaveTextContent('The tutor could not continue the defense.')
    expect(screen.getByRole('textbox', { name: 'Your answer' })).toHaveValue('my answer')
  })

  it('shows progress while the next question is written, with nothing to click', () => {
    setup({ phase: 'questioning', question: null })

    expect(screen.getByRole('status')).toHaveTextContent(
      'The tutor is preparing the next question...',
    )
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  it('shows progress while the answers are evaluated, with nothing to click', () => {
    setup({ phase: 'judging', question: null })

    expect(screen.getByRole('status')).toHaveTextContent('Evaluating your answers...')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it.each(['questionFailed', 'judgingFailed'] as const)(
    'shows the reason and offers to try again when %s',
    async (phase) => {
      const user = userEvent.setup()
      const { onRetry } = setup({
        phase,
        question: null,
        message: 'The tutor returned an invalid evaluation.',
      })

      expect(screen.getByRole('alert')).toHaveTextContent(
        'The tutor returned an invalid evaluation.',
      )
      await user.click(screen.getByRole('button', { name: 'Try again' }))

      expect(onRetry).toHaveBeenCalledOnce()
    },
  )
})
