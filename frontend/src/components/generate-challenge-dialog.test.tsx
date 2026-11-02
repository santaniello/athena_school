import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  cancelChallengeGeneration,
  generateChallenge,
  listSessionsForGeneration,
  type GeneratedChallenge,
  type SessionOption,
} from '@/lib/exercise'
import { GenerateChallengeDialog } from './generate-challenge-dialog'

vi.mock('@/lib/exercise', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/exercise')>()
  return {
    ...original,
    listSessionsForGeneration: vi.fn(),
    generateChallenge: vi.fn(),
    cancelChallengeGeneration: vi.fn(),
  }
})

const caching: SessionOption = {
  id: 's-cache',
  topic: 'Caching',
  folderName: 'Backend',
  startedAt: '2026-11-02T12:00:00Z',
}
const queues: SessionOption = {
  id: 's-queue',
  topic: 'Message queues',
  folderName: 'Infra',
  startedAt: '2026-10-20T12:00:00Z',
}
const created: GeneratedChallenge = {
  id: 'c9',
  title: 'Design a cache',
  topic: 'caching',
  difficulty: 'ADVANCED',
  timeLimitMinutes: 60,
}

function renderDialog(overrides: Partial<Parameters<typeof GenerateChallengeDialog>[0]> = {}) {
  const props = {
    open: true,
    defaultDifficulty: 'INTERMEDIATE' as const,
    onClose: vi.fn(),
    onGenerated: vi.fn(),
    ...overrides,
  }
  render(<GenerateChallengeDialog {...props} />)
  return props
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

describe('GenerateChallengeDialog', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(listSessionsForGeneration).mockResolvedValue([caching, queues])
  })

  it('renders nothing while closed', () => {
    renderDialog({ open: false })

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(listSessionsForGeneration).not.toHaveBeenCalled()
  })

  it('lists the study sessions with their topic, folder and date, in the order received', async () => {
    renderDialog()

    const options = await screen.findAllByRole('radio')
    expect(options).toHaveLength(2)
    expect(options[0]).toHaveTextContent('Caching')
    expect(options[0]).toHaveTextContent('Backend')
    expect(options[0]).toHaveTextContent('Nov 2, 2026')
    expect(options[1]).toHaveTextContent('Message queues')
    expect(options[1]).toHaveTextContent('Infra')
  })

  it('filters the sessions by the search text over topic and folder name', async () => {
    const user = userEvent.setup()
    renderDialog()
    await screen.findAllByRole('radio')

    await user.type(screen.getByRole('searchbox', { name: 'Search study sessions' }), 'INFRA')

    const options = screen.getAllByRole('radio')
    expect(options).toHaveLength(1)
    expect(options[0]).toHaveTextContent('Message queues')
  })

  it('says so when the search matches no session', async () => {
    const user = userEvent.setup()
    renderDialog()
    await screen.findAllByRole('radio')

    await user.type(screen.getByRole('searchbox', { name: 'Search study sessions' }), 'kubernetes')

    expect(screen.queryByRole('radio')).not.toBeInTheDocument()
    expect(screen.getByText('No study session matches your search.')).toBeInTheDocument()
  })

  it('keeps Generate disabled until a session is chosen', async () => {
    const user = userEvent.setup()
    renderDialog()
    await screen.findAllByRole('radio')
    const generate = screen.getByRole('button', { name: 'Generate' })
    expect(generate).toBeDisabled()

    await user.click(screen.getByRole('radio', { name: /Caching/ }))

    expect(generate).toBeEnabled()
    expect(screen.getByRole('radio', { name: /Caching/ })).toBeChecked()
  })

  it('preselects the difficulty it is given', async () => {
    renderDialog({ defaultDifficulty: 'BEGINNER' })
    await screen.findAllByRole('radio')

    expect(screen.getByRole('combobox', { name: 'Difficulty' })).toHaveTextContent('Beginner')
  })

  it('generates for the chosen session and difficulty, then reports the new challenge', async () => {
    const user = userEvent.setup()
    vi.mocked(generateChallenge).mockResolvedValueOnce(created)
    const props = renderDialog()
    await user.click(await screen.findByRole('radio', { name: /Message queues/ }))
    await user.click(screen.getByRole('combobox', { name: 'Difficulty' }))
    await user.click(await screen.findByRole('option', { name: 'Advanced' }))

    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(generateChallenge).toHaveBeenCalledWith('s-queue', 'ADVANCED')
    await waitFor(() => expect(props.onGenerated).toHaveBeenCalledExactlyOnceWith(created))
  })

  it('shows progress while generating, and Cancel stops the call and returns to the choice', async () => {
    const user = userEvent.setup()
    const pending = deferred<GeneratedChallenge>()
    vi.mocked(generateChallenge).mockReturnValueOnce(pending.promise)
    vi.mocked(cancelChallengeGeneration).mockResolvedValueOnce()
    const props = renderDialog()
    await user.click(await screen.findByRole('radio', { name: /Caching/ }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(screen.getByRole('status')).toHaveTextContent('Generating the challenge')
    expect(screen.queryByRole('button', { name: 'Generate' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    pending.reject(new Error(JSON.stringify({ code: 'cancelled', message: 'cancelled' })))

    expect(cancelChallengeGeneration).toHaveBeenCalledOnce()
    expect(await screen.findByRole('button', { name: 'Generate' })).toBeEnabled()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(props.onGenerated).not.toHaveBeenCalled()
    expect(props.onClose).not.toHaveBeenCalled()
  })

  it('cancels the running generation when the dialog is dismissed', async () => {
    const user = userEvent.setup()
    vi.mocked(generateChallenge).mockReturnValueOnce(deferred<GeneratedChallenge>().promise)
    vi.mocked(cancelChallengeGeneration).mockResolvedValueOnce()
    const props = renderDialog()
    await user.click(await screen.findByRole('radio', { name: /Caching/ }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    await user.keyboard('{Escape}')

    expect(cancelChallengeGeneration).toHaveBeenCalledOnce()
    expect(props.onClose).toHaveBeenCalledOnce()
  })

  it('stays open on a failure, shows the error and offers a retry that works', async () => {
    const user = userEvent.setup()
    vi.mocked(generateChallenge)
      .mockRejectedValueOnce(new Error(JSON.stringify({ code: 'invalid_generation' })))
      .mockResolvedValueOnce(created)
    const props = renderDialog()
    await user.click(await screen.findByRole('radio', { name: /Caching/ }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The generated challenge was not valid. Try again.',
    )
    expect(props.onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('radio', { name: /Caching/ })).toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(generateChallenge).toHaveBeenCalledTimes(2)
    await waitFor(() => expect(props.onGenerated).toHaveBeenCalledExactlyOnceWith(created))
  })

  it('clears an earlier error as soon as a new generation starts', async () => {
    const user = userEvent.setup()
    vi.mocked(generateChallenge)
      .mockRejectedValueOnce(new Error('{"code":"internal"}'))
      .mockReturnValueOnce(deferred<GeneratedChallenge>().promise)
    renderDialog()
    await user.click(await screen.findByRole('radio', { name: /Caching/ }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))
    await screen.findByRole('alert')

    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('explains that a study session is needed first, with no Generate button', async () => {
    vi.mocked(listSessionsForGeneration).mockResolvedValue([])
    renderDialog()

    expect(
      await screen.findByText(
        'Study a topic first. A challenge is generated from a session you have already studied.',
      ),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Generate' })).not.toBeInTheDocument()
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
  })

  it('shows an error with a retry when the sessions cannot be loaded', async () => {
    const user = userEvent.setup()
    vi.mocked(listSessionsForGeneration).mockRejectedValueOnce(new Error('boom'))
    renderDialog()

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Could not load your study sessions.')
    await user.click(within(alert).getByRole('button', { name: 'Retry' }))

    expect(await screen.findAllByRole('radio')).toHaveLength(2)
  })

  it('closes with Cancel when nothing is running', async () => {
    const user = userEvent.setup()
    const props = renderDialog()
    await screen.findAllByRole('radio')

    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(props.onClose).toHaveBeenCalledOnce()
    expect(cancelChallengeGeneration).not.toHaveBeenCalled()
  })
})
