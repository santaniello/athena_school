import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { setLabLanguage } from '@/i18n'
import { QuizResultView } from './quiz-result-view'
import { RESULT } from '@/test/quiz-fixtures'

function questionItem(number: number) {
  return screen.getByRole('listitem', { name: `Question ${number}` })
}

describe('QuizResultView', () => {
  it('shows the grade, the count, the session and the difficulty', () => {
    render(<QuizResultView result={RESULT} />)

    expect(screen.getByText('3.3 / 10')).toBeInTheDocument()
    expect(screen.getByText('1 of 3 correct')).toBeInTheDocument()
    expect(screen.getByText('Distributed Caching')).toBeInTheDocument()
    expect(screen.getByText('Hard')).toBeInTheDocument()
  })

  it('shows a whole score without decimals', () => {
    render(<QuizResultView result={{ ...RESULT, attempt: { ...RESULT.attempt, score: 10 } }} />)

    expect(screen.getByText('10 / 10')).toBeInTheDocument()
  })

  it('colors the grade by band without changing what it says', () => {
    const { rerender } = render(<QuizResultView result={RESULT} />)
    expect(screen.getByText('3.3 / 10')).toHaveAttribute('data-band', 'low')

    rerender(<QuizResultView result={{ ...RESULT, attempt: { ...RESULT.attempt, score: 6 } }} />)
    expect(screen.getByText('6 / 10')).toHaveAttribute('data-band', 'mid')

    rerender(<QuizResultView result={{ ...RESULT, attempt: { ...RESULT.attempt, score: 9 } }} />)
    expect(screen.getByText('9 / 10')).toHaveAttribute('data-band', 'high')
  })

  it('lists every question in order with its prompt, status and explanation', () => {
    render(<QuizResultView result={RESULT} />)

    const items = screen.getAllByRole('listitem', { name: /^Question \d$/ })
    expect(items).toHaveLength(3)
    expect(
      within(questionItem(1)).getByText('Which policy evicts the oldest entry?'),
    ).toBeInTheDocument()
    expect(within(questionItem(1)).getByText('Correct')).toBeInTheDocument()
    expect(within(questionItem(2)).getByText('Wrong')).toBeInTheDocument()
    expect(within(questionItem(3)).getByText('Blank')).toBeInTheDocument()
    expect(within(questionItem(2)).getByText('A cold cache adds a hop.')).toBeInTheDocument()
  })

  it('marks the correct option and the one the user chose', () => {
    render(<QuizResultView result={RESULT} />)

    // Question 2: the user chose "True", the answer was "False".
    const wrong = questionItem(2)
    expect(
      within(within(wrong).getByText('True').closest('li')!).getByText('Your answer'),
    ).toBeInTheDocument()
    expect(
      within(within(wrong).getByText('False').closest('li')!).getByText('Correct answer'),
    ).toBeInTheDocument()
    // Question 1: the chosen option is also the correct one, with both marks.
    const right = within(within(questionItem(1)).getByText('FIFO').closest('li')!)
    expect(right.getByText('Your answer')).toBeInTheDocument()
    expect(right.getByText('Correct answer')).toBeInTheDocument()
    // An option that is neither has neither.
    expect(
      within(within(questionItem(1)).getByText('LRU').closest('li')!).queryByText(/answer/),
    ).not.toBeInTheDocument()
  })

  it('says a blank question was not answered', () => {
    render(<QuizResultView result={RESULT} />)

    const blank = questionItem(3)
    expect(within(blank).getByText('Not answered')).toBeInTheDocument()
    expect(within(blank).queryByText('Your answer')).not.toBeInTheDocument()
    expect(
      within(within(blank).getByText('A').closest('li')!).getByText('Correct answer'),
    ).toBeInTheDocument()
  })

  it('shows only the missed questions when asked, blank ones included, and back again', async () => {
    const user = userEvent.setup()
    render(<QuizResultView result={RESULT} />)

    await user.click(screen.getByRole('checkbox', { name: 'Show only the questions I missed' }))

    expect(screen.queryByRole('listitem', { name: 'Question 1' })).not.toBeInTheDocument()
    expect(questionItem(2)).toBeInTheDocument()
    expect(questionItem(3)).toBeInTheDocument()

    await user.click(screen.getByRole('checkbox', { name: 'Show only the questions I missed' }))
    expect(questionItem(1)).toBeInTheDocument()
  })

  it('congratulates a perfect run when the filter has nothing to show', async () => {
    const user = userEvent.setup()
    const perfect = {
      ...RESULT,
      questions: RESULT.questions.map((question) => ({ ...question, correct: true })),
    }
    render(<QuizResultView result={perfect} />)

    await user.click(screen.getByRole('checkbox', { name: 'Show only the questions I missed' }))

    expect(screen.getByText('You did not miss any question.')).toBeInTheDocument()
    expect(screen.queryAllByRole('listitem', { name: /^Question/ })).toHaveLength(0)
  })

  it('offers Take again and Back only when given the handlers', async () => {
    const user = userEvent.setup()
    const onTakeAgain = vi.fn()
    const onBack = vi.fn()
    const { rerender } = render(<QuizResultView result={RESULT} />)
    expect(screen.queryByRole('button', { name: 'Take again' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Back to tests' })).not.toBeInTheDocument()

    rerender(<QuizResultView result={RESULT} onTakeAgain={onTakeAgain} onBack={onBack} />)
    await user.click(screen.getByRole('button', { name: 'Take again' }))
    await user.click(screen.getByRole('button', { name: 'Back to tests' }))

    expect(onTakeAgain).toHaveBeenCalledOnce()
    expect(onBack).toHaveBeenCalledOnce()
  })

  it('speaks the profile language', () => {
    setLabLanguage('pt')
    render(<QuizResultView result={RESULT} />)

    expect(screen.getByText('1 de 3 corretas')).toBeInTheDocument()
    expect(screen.getByText('Difícil')).toBeInTheDocument()
    expect(screen.getByRole('listitem', { name: 'Questão 2' })).toBeInTheDocument()
  })
})
