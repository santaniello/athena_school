import { setLabLanguage } from '@/i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { acceptOffer, declineOffer, type ChallengeDifficulty } from '@/lib/exercise'
import { ExerciseOfferCard } from './exercise-offer-card'

vi.mock('@/lib/exercise', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/exercise')>()),
  acceptOffer: vi.fn(),
  declineOffer: vi.fn(),
}))

function setup(
  props: { language?: string; defaultDifficulty?: ChallengeDifficulty; topic?: string } = {},
) {
  const onAccepted = vi.fn()
  const onDeclined = vi.fn()
  const onGone = vi.fn()
  // The card speaks the Lab's language, which follows the profile.
  setLabLanguage(props.language)
  render(
    <ExerciseOfferCard
      offerId="o1"
      defaultDifficulty={props.defaultDifficulty ?? 'INTERMEDIATE'}
      topic={props.topic}
      onAccepted={onAccepted}
      onDeclined={onDeclined}
      onGone={onGone}
    />,
  )
  return { onAccepted, onDeclined, onGone, user: userEvent.setup() }
}

describe('ExerciseOfferCard', () => {
  beforeEach(() => vi.clearAllMocks())

  it('offers Accept, which waits for the goal, and Decline in English', () => {
    setup()

    expect(screen.getByRole('button', { name: 'Accept' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeEnabled()
  })

  it('offers Aceitar and Recusar for a Portuguese assistant', () => {
    setup({ language: 'pt' })

    expect(screen.getByRole('button', { name: 'Aceitar' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Recusar' })).toBeInTheDocument()
  })

  it('starts with the difficulty of the profile and accepts it with the goal', async () => {
    vi.mocked(acceptOffer).mockResolvedValueOnce()
    const { user } = setup({ defaultDifficulty: 'BEGINNER' })

    expect(screen.getByRole('combobox', { name: 'Difficulty' })).toHaveTextContent('Beginner')
    await user.type(screen.getByLabelText('What do you want to practice?'), 'cache aside')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(acceptOffer).toHaveBeenCalledWith('o1', 'BEGINNER', 'cache aside')
  })

  it('accepts the difficulty the user picked instead of the default', async () => {
    vi.mocked(acceptOffer).mockResolvedValueOnce()
    const { user } = setup({ defaultDifficulty: 'BEGINNER' })

    await user.click(screen.getByRole('combobox', { name: 'Difficulty' }))
    await user.click(await screen.findByRole('option', { name: 'Advanced' }))
    await user.type(screen.getByLabelText('What do you want to practice?'), 'cache aside')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(acceptOffer).toHaveBeenCalledWith('o1', 'ADVANCED', 'cache aside')
  })

  it('has no kind choice: the offer is always a practice', () => {
    setup()

    expect(screen.queryByRole('combobox', { name: 'Type' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('What do you want to practice?')).toBeInTheDocument()
  })

  it('asks what to practice, and Accept waits for it', async () => {
    const { user } = setup()

    expect(screen.getByRole('button', { name: 'Accept' })).toBeDisabled()
    await user.type(screen.getByLabelText('What do you want to practice?'), '   ')
    expect(screen.getByRole('button', { name: 'Accept' })).toBeDisabled()
  })

  it('accepts with the trimmed goal', async () => {
    vi.mocked(acceptOffer).mockResolvedValueOnce()
    const { user } = setup({ defaultDifficulty: 'BEGINNER' })

    await user.type(screen.getByLabelText('What do you want to practice?'), '  cache aside ')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(acceptOffer).toHaveBeenCalledWith('o1', 'BEGINNER', 'cache aside')
  })

  it('fills the goal with the session topic from a suggestion chip', async () => {
    const { user } = setup({ topic: 'Distributed caching' })

    await user.click(screen.getByRole('button', { name: 'Distributed caching' }))

    expect(screen.getByLabelText('What do you want to practice?')).toHaveValue(
      'Distributed caching',
    )
    expect(screen.getByRole('button', { name: 'Accept' })).toBeEnabled()
  })

  it('speaks Portuguese in the goal question', () => {
    setup({ language: 'pt' })

    expect(screen.getByLabelText('O que você quer praticar?')).toBeInTheDocument()
  })

  it('locks the goal while the exercise is being prepared', async () => {
    vi.mocked(acceptOffer).mockReturnValueOnce(new Promise(() => {}))
    const { user } = setup()

    await user.type(screen.getByLabelText('What do you want to practice?'), 'cache aside')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(screen.getByLabelText('What do you want to practice?')).toBeDisabled()
  })

  it('speaks Portuguese in the difficulty choice for a Portuguese assistant', async () => {
    const { user } = setup({ language: 'pt', defaultDifficulty: 'ADVANCED' })

    expect(screen.getByRole('combobox', { name: 'Dificuldade' })).toHaveTextContent('Avançado')
    await user.click(screen.getByRole('combobox', { name: 'Dificuldade' }))
    expect(await screen.findByRole('option', { name: 'Iniciante' })).toBeInTheDocument()
  })

  it('locks the difficulty while the exercise is being prepared', async () => {
    vi.mocked(acceptOffer).mockReturnValueOnce(new Promise(() => {}))
    const { user } = setup()

    await user.type(screen.getByLabelText('What do you want to practice?'), 'cache aside')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(screen.getByRole('combobox', { name: 'Difficulty' })).toBeDisabled()
  })

  it('accepts: shows progress with both buttons disabled, then tells the screen', async () => {
    let resolve: () => void = () => {}
    vi.mocked(acceptOffer).mockReturnValueOnce(new Promise((r) => (resolve = r)))
    const { user, onAccepted } = setup()

    await user.type(screen.getByLabelText('What do you want to practice?'), 'cache aside')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(acceptOffer).toHaveBeenCalledWith('o1', 'INTERMEDIATE', 'cache aside')
    expect(screen.getByText('Preparing your exercise…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeDisabled()

    resolve()
    await waitFor(() => expect(onAccepted).toHaveBeenCalledOnce())
  })

  it('keeps the offer with the error and Accept available again when generation fails', async () => {
    vi.mocked(acceptOffer).mockRejectedValueOnce(
      new Error('{"code":"invalid_generation","message":"x"}'),
    )
    const { user, onAccepted } = setup()

    await user.type(screen.getByLabelText('What do you want to practice?'), 'cache aside')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The generated challenge was not valid. Try again.',
    )
    expect(onAccepted).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeEnabled()
  })

  it('clears the previous error when accepting again', async () => {
    vi.mocked(acceptOffer).mockRejectedValueOnce(new Error('boom'))
    vi.mocked(acceptOffer).mockResolvedValueOnce()
    const { user } = setup()

    await user.type(screen.getByLabelText('What do you want to practice?'), 'cache aside')
    await user.click(screen.getByRole('button', { name: 'Accept' }))
    await screen.findByRole('alert')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('declines and tells the screen', async () => {
    vi.mocked(declineOffer).mockResolvedValueOnce()
    const { user, onDeclined } = setup()

    await user.click(screen.getByRole('button', { name: 'Decline' }))

    expect(declineOffer).toHaveBeenCalledWith('o1')
    await waitFor(() => expect(onDeclined).toHaveBeenCalled())
  })

  it('keeps the offer with the error when declining fails', async () => {
    vi.mocked(declineOffer).mockRejectedValueOnce(new Error('boom'))
    const { user, onDeclined } = setup()

    await user.click(screen.getByRole('button', { name: 'Decline' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Something went wrong. Please try again.',
    )
    expect(onDeclined).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeEnabled()
  })

  it.each(['accept', 'decline'] as const)(
    'hands a vanished offer back to the chat when %s finds it gone',
    async (action) => {
      const gone = new Error(JSON.stringify({ code: 'offer_not_found' }))
      vi.mocked(acceptOffer).mockRejectedValue(gone)
      vi.mocked(declineOffer).mockRejectedValue(gone)
      const { onGone, user } = setup()

      await user.type(screen.getByLabelText('What do you want to practice?'), 'cache aside')
      await user.click(
        screen.getByRole('button', { name: action === 'accept' ? 'Accept' : 'Decline' }),
      )

      expect(onGone).toHaveBeenCalledOnce()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    },
  )
})
