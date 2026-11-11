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

function setup(props: { language?: string; defaultDifficulty?: ChallengeDifficulty } = {}) {
  const onAccepted = vi.fn()
  const onDeclined = vi.fn()
  render(
    <ExerciseOfferCard
      offerId="o1"
      language={props.language ?? 'en'}
      defaultDifficulty={props.defaultDifficulty ?? 'INTERMEDIATE'}
      onAccepted={onAccepted}
      onDeclined={onDeclined}
    />,
  )
  return { onAccepted, onDeclined, user: userEvent.setup() }
}

describe('ExerciseOfferCard', () => {
  beforeEach(() => vi.clearAllMocks())

  it('offers Accept and Decline in English', () => {
    setup()

    expect(screen.getByRole('button', { name: 'Accept' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeEnabled()
  })

  it('offers Aceitar and Recusar for a Portuguese assistant', () => {
    setup({ language: 'pt' })

    expect(screen.getByRole('button', { name: 'Aceitar' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Recusar' })).toBeInTheDocument()
  })

  it('starts with the difficulty of the profile and accepts it', async () => {
    vi.mocked(acceptOffer).mockResolvedValueOnce()
    const { user } = setup({ defaultDifficulty: 'BEGINNER' })

    expect(screen.getByRole('combobox', { name: 'Difficulty' })).toHaveTextContent('Beginner')
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(acceptOffer).toHaveBeenCalledWith('o1', 'BEGINNER')
  })

  it('accepts the difficulty the user picked instead of the default', async () => {
    vi.mocked(acceptOffer).mockResolvedValueOnce()
    const { user } = setup({ defaultDifficulty: 'BEGINNER' })

    await user.click(screen.getByRole('combobox', { name: 'Difficulty' }))
    await user.click(await screen.findByRole('option', { name: 'Advanced' }))
    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(acceptOffer).toHaveBeenCalledWith('o1', 'ADVANCED')
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

    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(screen.getByRole('combobox', { name: 'Difficulty' })).toBeDisabled()
  })

  it('accepts: shows progress with both buttons disabled, then tells the screen', async () => {
    let resolve: () => void = () => {}
    vi.mocked(acceptOffer).mockReturnValueOnce(new Promise((r) => (resolve = r)))
    const { user, onAccepted } = setup()

    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(acceptOffer).toHaveBeenCalledWith('o1', 'INTERMEDIATE')
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
})
