import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { GetPassScore, SetPassScore } from '../../wailsjs/go/desktop/App'
import { PassScoreSection } from './pass-score-section'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  GetPassScore: vi.fn(),
  SetPassScore: vi.fn(),
}))

const RANGE_MESSAGE = 'The minimum passing score must be between 50 and 100.'

async function renderLoaded(stored = 80) {
  vi.mocked(GetPassScore).mockResolvedValue(stored)
  render(<PassScoreSection />)
  return screen.findByLabelText('Minimum passing score')
}

async function typeScore(user: ReturnType<typeof userEvent.setup>, value: string) {
  const input = screen.getByLabelText('Minimum passing score')
  await user.clear(input)
  if (value) await user.type(input, value)
}

describe('PassScoreSection', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows the stored score and explains what it means', async () => {
    const input = await renderLoaded(65)

    expect(input).toHaveValue(65)
    expect(input).toHaveAttribute('type', 'number')
    expect(input).toHaveAttribute('min', '50')
    expect(input).toHaveAttribute('max', '100')
    expect(
      screen.getByText(
        'A challenge is passed when both the diagram and your explanation reach this score. Changing it does not affect attempts you already finished.',
      ),
    ).toBeInTheDocument()
  })

  it('saves a valid change and confirms it', async () => {
    const user = userEvent.setup()
    vi.mocked(SetPassScore).mockResolvedValue()
    await renderLoaded()

    await typeScore(user, '90')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(SetPassScore).toHaveBeenCalledWith(90)
    expect(await screen.findByRole('status')).toHaveTextContent('Saved.')
  })

  it('shows the validation message and stores nothing for an out-of-range value', async () => {
    const user = userEvent.setup()
    vi.mocked(SetPassScore).mockRejectedValue(
      new Error('minimum passing score must be between 50 and 100'),
    )
    await renderLoaded()

    await typeScore(user, '120')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(RANGE_MESSAGE)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it.each(['', '72.5'])(
    'does not call the backend for %j, which is not a whole number',
    async (value) => {
      const user = userEvent.setup()
      await renderLoaded()

      await typeScore(user, value)
      await user.click(screen.getByRole('button', { name: 'Save' }))

      expect(await screen.findByRole('alert')).toHaveTextContent(RANGE_MESSAGE)
      expect(SetPassScore).not.toHaveBeenCalled()
    },
  )

  it('clears an earlier message when the user edits the value again', async () => {
    const user = userEvent.setup()
    vi.mocked(SetPassScore).mockResolvedValue()
    await renderLoaded()
    await typeScore(user, '90')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await screen.findByRole('status')

    await typeScore(user, '95')

    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('disables Save while saving', async () => {
    const user = userEvent.setup()
    let finish: () => void = () => {}
    vi.mocked(SetPassScore).mockReturnValue(new Promise<void>((resolve) => (finish = resolve)))
    await renderLoaded()

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
    finish()
    expect(await screen.findByRole('status')).toBeInTheDocument()
  })

  it('offers a retry when the score cannot be loaded', async () => {
    const user = userEvent.setup()
    vi.mocked(GetPassScore).mockRejectedValueOnce(new Error('unreadable'))
    render(<PassScoreSection />)

    expect(await screen.findByText('Could not load the minimum passing score.')).toBeInTheDocument()
    vi.mocked(GetPassScore).mockResolvedValueOnce(70)
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByLabelText('Minimum passing score')).toHaveValue(70)
  })

  it('shows a loading state until the score arrives', async () => {
    vi.mocked(GetPassScore).mockReturnValue(new Promise(() => {}))
    render(<PassScoreSection />)

    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })
})
