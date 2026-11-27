import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  cancelChallengeGeneration,
  generateCanonicalChallenge,
  type CanonicalProblem,
  type GeneratedChallenge,
} from '@/lib/exercise'
import { StartCanonicalDialog } from './start-canonical-dialog'

vi.mock('@/lib/exercise', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/exercise')>()
  return {
    ...original,
    generateCanonicalChallenge: vi.fn(),
    cancelChallengeGeneration: vi.fn(),
  }
})

const problem: CanonicalProblem = {
  id: 'url-shortener',
  name: 'URL shortener',
  summary: 'Short links.',
  attempts: 1,
  hasHistory: false,
  difficulties: [
    { difficulty: 'BEGINNER', generated: true, challengeId: 'c1', status: 'COMPLETED' },
    { difficulty: 'INTERMEDIATE', generated: false, challengeId: '', status: '' },
    { difficulty: 'ADVANCED', generated: true, challengeId: 'c3', status: 'NOT_STARTED' },
  ],
}

const created: GeneratedChallenge = {
  id: 'c2',
  kind: 'INTERVIEW',
  title: 'URL shortener',
  topic: 'url shortener',
  difficulty: 'INTERMEDIATE',
  timeLimitMinutes: 45,
}

function renderDialog(overrides: Partial<Parameters<typeof StartCanonicalDialog>[0]> = {}) {
  const props = {
    problem,
    defaultDifficulty: 'INTERMEDIATE' as const,
    onClose: vi.fn(),
    onReady: vi.fn(),
    onGeneratedAfterClose: vi.fn(),
    ...overrides,
  }
  const view = render(<StartCanonicalDialog {...props} />)
  return { ...props, unmount: view.unmount }
}

// A promise the test settles by hand, like a slow model call.
function deferred<T>() {
  let resolve: (value: T) => void = () => {}
  let reject: (reason: unknown) => void = () => {}
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('StartCanonicalDialog', () => {
  beforeEach(() => vi.clearAllMocks())

  it('stays closed without a problem', () => {
    renderDialog({ problem: null })

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows each difficulty with its state and preselects the default one', () => {
    renderDialog()

    expect(screen.getByRole('heading', { name: 'Start URL shortener' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /Beginner\s*Completed/ })).not.toBeChecked()
    expect(screen.getByRole('radio', { name: /Intermediate\s*Not generated/ })).toBeChecked()
    expect(screen.getByRole('radio', { name: /Advanced\s*Not started/ })).not.toBeChecked()
  })

  it('gets the challenge of the chosen difficulty and hands it to the exam rules', async () => {
    vi.mocked(generateCanonicalChallenge).mockResolvedValueOnce({
      ...created,
      id: 'c1',
      difficulty: 'BEGINNER',
      timeLimitMinutes: 30,
    })
    const { onReady } = renderDialog()

    await userEvent.click(screen.getByRole('radio', { name: /Beginner/ }))
    await userEvent.click(screen.getByRole('button', { name: 'Start' }))

    expect(generateCanonicalChallenge).toHaveBeenCalledWith('url-shortener', 'BEGINNER')
    await waitFor(() =>
      expect(onReady).toHaveBeenCalledWith({
        id: 'c1',
        title: 'URL shortener',
        difficulty: 'BEGINNER',
        timeLimitMinutes: 30,
      }),
    )
  })

  it('shows progress while a missing challenge is generated and can cancel it', async () => {
    const pending = deferred<GeneratedChallenge>()
    vi.mocked(generateCanonicalChallenge).mockReturnValueOnce(pending.promise)
    vi.mocked(cancelChallengeGeneration).mockResolvedValueOnce()
    const { onClose } = renderDialog()

    await userEvent.click(screen.getByRole('button', { name: 'Start' }))

    expect(screen.getByRole('status')).toHaveTextContent(/Generating the interview/)
    expect(screen.queryByRole('button', { name: 'Start' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(cancelChallengeGeneration).toHaveBeenCalledTimes(1)
    expect(onClose).not.toHaveBeenCalled()
  })

  it('shows no generation progress when the challenge already exists', async () => {
    vi.mocked(generateCanonicalChallenge).mockReturnValueOnce(
      deferred<GeneratedChallenge>().promise,
    )
    renderDialog({ defaultDifficulty: 'BEGINNER' })

    await userEvent.click(screen.getByRole('button', { name: 'Start' }))

    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('returns quietly to the choice when the generation was cancelled', async () => {
    vi.mocked(generateCanonicalChallenge).mockRejectedValueOnce(
      new Error(JSON.stringify({ code: 'cancelled', message: 'cancelled' })),
    )
    renderDialog()

    await userEvent.click(screen.getByRole('button', { name: 'Start' }))

    expect(await screen.findByRole('button', { name: 'Start' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows the failure and offers a retry', async () => {
    vi.mocked(generateCanonicalChallenge)
      .mockRejectedValueOnce(
        new Error(JSON.stringify({ code: 'invalid_generation', message: 'bad' })),
      )
      .mockResolvedValueOnce(created)
    const { onReady } = renderDialog()

    await userEvent.click(screen.getByRole('button', { name: 'Start' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))

    await waitFor(() => expect(onReady).toHaveBeenCalledTimes(1))
    expect(generateCanonicalChallenge).toHaveBeenCalledTimes(2)
  })

  it('cancels a running generation when the dialog is dismissed', async () => {
    vi.mocked(generateCanonicalChallenge).mockReturnValueOnce(
      deferred<GeneratedChallenge>().promise,
    )
    vi.mocked(cancelChallengeGeneration).mockResolvedValueOnce()
    const { onClose } = renderDialog()

    await userEvent.click(screen.getByRole('button', { name: 'Start' }))
    await userEvent.keyboard('{Escape}')

    expect(cancelChallengeGeneration).toHaveBeenCalledTimes(1)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('closes with Cancel when nothing is running, without cancelling anything', async () => {
    const { onClose } = renderDialog()

    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onClose).toHaveBeenCalledTimes(1)
    expect(cancelChallengeGeneration).not.toHaveBeenCalled()
  })

  it('only refreshes the list when the challenge arrives after the dialog closed', async () => {
    const pending = deferred<GeneratedChallenge>()
    vi.mocked(generateCanonicalChallenge).mockReturnValueOnce(pending.promise)
    const { unmount, onReady, onGeneratedAfterClose } = renderDialog()

    await userEvent.click(screen.getByRole('button', { name: 'Start' }))
    unmount()
    pending.resolve(created)

    await waitFor(() => expect(onGeneratedAfterClose).toHaveBeenCalledTimes(1))
    expect(onReady).not.toHaveBeenCalled()
  })
})
