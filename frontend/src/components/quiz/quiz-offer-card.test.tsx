import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { setLabLanguage } from '@/i18n'
import { acceptQuizOffer, declineQuizOffer, type QuizSummary } from '@/lib/quiz'
import { QuizOfferCard } from './quiz-offer-card'

vi.mock('@/lib/quiz', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/quiz')>()),
  acceptQuizOffer: vi.fn(),
  declineQuizOffer: vi.fn(),
}))

const QUIZ: QuizSummary = {
  id: 'quiz-1',
  sessionId: 's1',
  sessionTitle: 'Distributed Caching',
  difficulty: 'HARD',
  questionCount: 8,
  createdAt: '2026-11-16T09:30:00Z',
  attemptCount: 0,
  latestScore: null,
  messageId: 'm1',
}

function setup(props: { language?: string; defaultDifficulty?: 'EASY' | 'MEDIUM' | 'HARD' } = {}) {
  const onAccepted = vi.fn()
  const onDeclined = vi.fn()
  // The card speaks the quiz language, which follows the profile.
  setLabLanguage(props.language)
  render(
    <QuizOfferCard
      offerId="o1"
      defaultDifficulty={props.defaultDifficulty ?? 'MEDIUM'}
      onAccepted={onAccepted}
      onDeclined={onDeclined}
    />,
  )
  return { onAccepted, onDeclined, user: userEvent.setup() }
}

describe('QuizOfferCard', () => {
  beforeEach(() => vi.clearAllMocks())

  it('offers Accept and Decline in English', () => {
    setup()

    expect(screen.getByText('Take a test about this session')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeEnabled()
  })

  it('offers Aceitar and Recusar in Portuguese', () => {
    setup({ language: 'pt' })

    expect(screen.getByRole('button', { name: 'Aceitar' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Recusar' })).toBeInTheDocument()
  })

  it('starts with ten questions and the difficulty of the profile, and accepts them', async () => {
    vi.mocked(acceptQuizOffer).mockResolvedValueOnce(QUIZ)
    const { user, onAccepted } = setup({ defaultDifficulty: 'EASY' })

    expect(screen.getByLabelText('Questions')).toHaveValue(10)
    expect(screen.getByRole('combobox', { name: 'Difficulty' })).toHaveTextContent('Easy')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(acceptQuizOffer).toHaveBeenCalledWith('o1', 10, 'EASY')
    await waitFor(() => expect(onAccepted).toHaveBeenCalledWith(QUIZ))
  })

  it('accepts the count and the difficulty the user picked', async () => {
    vi.mocked(acceptQuizOffer).mockResolvedValueOnce(QUIZ)
    const { user } = setup()

    await user.clear(screen.getByLabelText('Questions'))
    await user.type(screen.getByLabelText('Questions'), '5')
    await user.click(screen.getByRole('combobox', { name: 'Difficulty' }))
    await user.click(await screen.findByRole('option', { name: 'Hard' }))
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(acceptQuizOffer).toHaveBeenCalledWith('o1', 5, 'HARD')
  })

  it.each(['2', '21', '', 'abc'])('does not accept %j questions', async (text) => {
    const { user } = setup()

    await user.clear(screen.getByLabelText('Questions'))
    if (text) await user.type(screen.getByLabelText('Questions'), text)

    expect(screen.getByRole('button', { name: 'Accept' })).toBeDisabled()
  })

  it('shows progress and locks everything while the test is written', async () => {
    let finish: (quiz: QuizSummary) => void = () => {}
    vi.mocked(acceptQuizOffer).mockReturnValueOnce(
      new Promise<QuizSummary>((resolve) => {
        finish = resolve
      }),
    )
    const { user, onAccepted } = setup()

    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(screen.getByText('Writing your test... this can take a moment.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeDisabled()
    expect(screen.getByLabelText('Questions')).toBeDisabled()
    expect(screen.getByRole('combobox', { name: 'Difficulty' })).toBeDisabled()
    finish(QUIZ)
    await waitFor(() => expect(onAccepted).toHaveBeenCalledWith(QUIZ))
  })

  it('keeps the offer with the translated error when the generation fails', async () => {
    vi.mocked(acceptQuizOffer).mockRejectedValueOnce(
      JSON.stringify({ code: 'invalid_generation', message: 'x' }),
    )
    const { user, onAccepted } = setup()

    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(
      await screen.findByText('The generated test was not valid. Try again.'),
    ).toBeInTheDocument()
    expect(onAccepted).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeEnabled()
    expect(
      screen.queryByText('Writing your test... this can take a moment.'),
    ).not.toBeInTheDocument()
  })

  it('clears the error when it tries again', async () => {
    vi.mocked(acceptQuizOffer).mockRejectedValueOnce('boom').mockResolvedValueOnce(QUIZ)
    const { user } = setup()

    await user.click(screen.getByRole('button', { name: 'Accept' }))
    await screen.findByText('Something went wrong. Please try again.')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    await waitFor(() =>
      expect(screen.queryByText('Something went wrong. Please try again.')).not.toBeInTheDocument(),
    )
  })

  it('declines the offer', async () => {
    vi.mocked(declineQuizOffer).mockResolvedValueOnce()
    const { user, onDeclined } = setup()

    await user.click(screen.getByRole('button', { name: 'Decline' }))

    expect(declineQuizOffer).toHaveBeenCalledWith('o1')
    await waitFor(() => expect(onDeclined).toHaveBeenCalledOnce())
  })

  it('keeps the offer with an error when declining fails', async () => {
    vi.mocked(declineQuizOffer).mockRejectedValueOnce(
      JSON.stringify({ code: 'offer_not_found', message: 'x' }),
    )
    const { user, onDeclined } = setup()

    await user.click(screen.getByRole('button', { name: 'Decline' }))

    expect(await screen.findByText('This test offer is no longer available.')).toBeInTheDocument()
    expect(onDeclined).not.toHaveBeenCalled()
  })
})
