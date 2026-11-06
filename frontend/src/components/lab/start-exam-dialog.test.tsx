import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { getPassScore } from '@/lib/passScore'
import { StartExamDialog } from './start-exam-dialog'

vi.mock('@/lib/passScore', () => ({ getPassScore: vi.fn() }))

const challenge = {
  id: 'c1',
  title: 'Cache and invalidation',
  difficulty: 'INTERMEDIATE',
  timeLimitMinutes: 45,
}

function renderDialog(overrides: Partial<Parameters<typeof StartExamDialog>[0]> = {}) {
  const props = { challenge, onCancel: vi.fn(), onConfirm: vi.fn(), ...overrides }
  render(<StartExamDialog {...props} />)
  return props
}

describe('StartExamDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(getPassScore).mockResolvedValue(70)
  })

  it('renders nothing without a challenge', () => {
    renderDialog({ challenge: null })

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(getPassScore).not.toHaveBeenCalled()
  })

  it('shows the title, time limit and rules of the exam', async () => {
    renderDialog()

    expect(await screen.findByText('Cache and invalidation')).toBeInTheDocument()
    expect(screen.getByText('Time limit: 45 minutes')).toBeInTheDocument()
    expect(screen.getByText('There is no pause and you cannot resume.')).toBeInTheDocument()
    expect(
      screen.getByText('Leaving discards the exam. It will not be graded.'),
    ).toBeInTheDocument()
    expect(screen.getByText('You can ask the tutor for up to 3 hints.')).toBeInTheDocument()
    expect(
      screen.getByText('After you submit, the tutor will ask you questions about your choices.'),
    ).toBeInTheDocument()
  })

  it('shows the current minimum passing score', async () => {
    renderDialog()

    expect(await screen.findByText('Minimum passing score: 70')).toBeInTheDocument()
  })

  it('still lets the user start when the passing score cannot be read', async () => {
    vi.mocked(getPassScore).mockRejectedValue(new Error('boom'))
    renderDialog()

    expect(await screen.findByRole('button', { name: 'Start exam' })).toBeEnabled()
    expect(screen.queryByText(/Minimum passing score/)).not.toBeInTheDocument()
  })

  it('uses the singular for a one minute exam', async () => {
    renderDialog({ challenge: { ...challenge, timeLimitMinutes: 1 } })

    expect(await screen.findByText('Time limit: 1 minute')).toBeInTheDocument()
  })

  it('cancels without starting', async () => {
    const user = userEvent.setup()
    const props = renderDialog()

    await user.click(await screen.findByRole('button', { name: 'Cancel' }))

    expect(props.onCancel).toHaveBeenCalledTimes(1)
    expect(props.onConfirm).not.toHaveBeenCalled()
  })

  it('confirms the start with the challenge', async () => {
    const user = userEvent.setup()
    const props = renderDialog()

    await user.click(await screen.findByRole('button', { name: 'Start exam' }))

    expect(props.onConfirm).toHaveBeenCalledExactlyOnceWith(challenge)
  })
})
