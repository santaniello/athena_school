import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { listSessionsForGeneration } from '@/lib/exercise'
import { cancelQuizGeneration, generateQuiz, type QuizSummary } from '@/lib/quiz'
import { setLabLanguage } from '@/i18n'
import { GenerateQuizDialog } from './generate-quiz-dialog'

vi.mock('@/lib/exercise', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/exercise')>()),
  listSessionsForGeneration: vi.fn(),
}))
vi.mock('@/lib/quiz', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/quiz')>()),
  generateQuiz: vi.fn(),
  cancelQuizGeneration: vi.fn(),
}))

const SESSIONS = [
  {
    id: 's1',
    topic: 'Distributed Caching',
    folderName: 'Backend',
    startedAt: '2026-11-10T10:00:00Z',
  },
  { id: 's2', topic: 'Go Concurrency', folderName: 'Languages', startedAt: '2026-11-12T10:00:00Z' },
]
const QUIZ: QuizSummary = {
  id: 'quiz-1',
  sessionId: 's1',
  sessionTitle: 'Distributed Caching',
  difficulty: 'MEDIUM',
  questionCount: 10,
  createdAt: '2026-11-16T09:30:00Z',
  attemptCount: 0,
  latestScore: null,
  messageId: '',
}

function renderDialog(overrides: Partial<React.ComponentProps<typeof GenerateQuizDialog>> = {}) {
  const props = {
    open: true,
    defaultDifficulty: 'MEDIUM' as const,
    onClose: vi.fn(),
    onGenerated: vi.fn(),
    onGeneratedAfterClose: vi.fn(),
    ...overrides,
  }
  const view = render(<GenerateQuizDialog {...props} />)
  return { ...props, ...view }
}

async function pickFirstSession(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('radio', { name: /Distributed Caching/ }))
}

describe('GenerateQuizDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(listSessionsForGeneration).mockResolvedValue(SESSIONS)
  })

  it('renders nothing while closed and loads no sessions', () => {
    const { container } = renderDialog({ open: false })

    expect(container).toBeEmptyDOMElement()
    expect(listSessionsForGeneration).not.toHaveBeenCalled()
  })

  it('lists the study sessions to choose from', async () => {
    renderDialog()

    expect(screen.getByText('Loading your study sessions...')).toBeInTheDocument()
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(await screen.findByRole('radio', { name: /Distributed Caching/ })).toBeInTheDocument()
    expect(screen.queryByText('Loading your study sessions...')).not.toBeInTheDocument()
    expect(screen.queryByText('No session matches your search.')).not.toBeInTheDocument()
    expect(screen.queryByText(/You have no study sessions/)).not.toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /Go Concurrency/ })).toBeInTheDocument()
    expect(screen.getByText(/Backend/)).toBeInTheDocument()
  })

  it('shows loading again while a retry runs', async () => {
    const user = userEvent.setup()
    vi.mocked(listSessionsForGeneration).mockRejectedValueOnce(new Error('nope'))
    renderDialog()
    await screen.findByRole('alert')

    vi.mocked(listSessionsForGeneration).mockReturnValueOnce(new Promise(() => {}))
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(screen.getByText('Loading your study sessions...')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('searches by topic or folder', async () => {
    const user = userEvent.setup()
    renderDialog()
    await screen.findByRole('radio', { name: /Go Concurrency/ })

    await user.type(screen.getByRole('searchbox', { name: 'Search sessions' }), 'backend')
    expect(screen.queryByRole('radio', { name: /Go Concurrency/ })).not.toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /Distributed Caching/ })).toBeInTheDocument()

    await user.clear(screen.getByRole('searchbox', { name: 'Search sessions' }))
    await user.type(screen.getByRole('searchbox', { name: 'Search sessions' }), 'CONCURRENCY')
    expect(screen.getByRole('radio', { name: /Go Concurrency/ })).toBeInTheDocument()
    expect(screen.queryByRole('radio', { name: /Distributed Caching/ })).not.toBeInTheDocument()

    await user.clear(screen.getByRole('searchbox', { name: 'Search sessions' }))
    await user.type(screen.getByRole('searchbox', { name: 'Search sessions' }), 'zzz')
    expect(screen.getByText('No session matches your search.')).toBeInTheDocument()
  })

  it('says there are no sessions, and offers nothing to generate', async () => {
    vi.mocked(listSessionsForGeneration).mockResolvedValue([])
    renderDialog()

    expect(
      await screen.findByText(
        'You have no study sessions yet. Study something first, then come back.',
      ),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Generate' })).not.toBeInTheDocument()
  })

  it('says the sessions could not load and retries', async () => {
    const user = userEvent.setup()
    vi.mocked(listSessionsForGeneration).mockRejectedValueOnce(new Error('nope'))
    renderDialog()

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Could not load your study sessions.',
    )
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
    expect(screen.queryByText(/You have no study sessions/)).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByRole('radio', { name: /Distributed Caching/ })).toBeInTheDocument()
  })

  it('generates the default 10 questions of the preselected difficulty for the chosen session', async () => {
    const user = userEvent.setup()
    vi.mocked(generateQuiz).mockResolvedValue(QUIZ)
    const { onGenerated } = renderDialog({ defaultDifficulty: 'HARD' })
    expect(screen.queryByRole('button', { name: 'Generate' })).not.toBeInTheDocument()
    await screen.findByRole('radio', { name: /Distributed Caching/ })
    expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled()

    await pickFirstSession(user)
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(generateQuiz).toHaveBeenCalledWith('s1', 10, 'HARD')
    await waitFor(() => expect(onGenerated).toHaveBeenCalledWith(QUIZ))
  })

  it('sends the chosen count and difficulty', async () => {
    const user = userEvent.setup()
    vi.mocked(generateQuiz).mockResolvedValue(QUIZ)
    renderDialog()
    await pickFirstSession(user)

    const count = screen.getByRole('spinbutton', { name: 'Questions' })
    await user.clear(count)
    await user.type(count, '5')
    await user.click(screen.getByRole('combobox', { name: 'Difficulty' }))
    await user.click(await screen.findByRole('option', { name: 'Easy' }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(generateQuiz).toHaveBeenCalledWith('s1', 5, 'EASY')
  })

  it.each([['2'], ['21'], [''], ['5.5'], ['-5']])(
    'does not generate with %j questions',
    async (value) => {
      const user = userEvent.setup()
      renderDialog()
      await pickFirstSession(user)
      const count = screen.getByRole('spinbutton', { name: 'Questions' })
      await user.clear(count)
      if (value) await user.type(count, value)

      expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled()
    },
  )

  it.each([['3'], ['20']])('accepts %s questions, the ends of the range', async (value) => {
    const user = userEvent.setup()
    renderDialog()
    await pickFirstSession(user)
    const count = screen.getByRole('spinbutton', { name: 'Questions' })
    await user.clear(count)
    await user.type(count, value)

    expect(screen.getByRole('button', { name: 'Generate' })).toBeEnabled()
  })

  it('shows progress while generating and cancels on request, quietly', async () => {
    const user = userEvent.setup()
    let reject: (reason: unknown) => void = () => {}
    vi.mocked(generateQuiz).mockReturnValue(new Promise((_, fail) => (reject = fail)))
    vi.mocked(cancelQuizGeneration).mockResolvedValue(undefined)
    renderDialog()
    await pickFirstSession(user)
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(screen.getByRole('status')).toHaveTextContent('Writing your test...')
    expect(screen.queryByRole('button', { name: 'Generate' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(cancelQuizGeneration).toHaveBeenCalledOnce()

    reject(new Error(JSON.stringify({ code: 'cancelled' })))
    expect(await screen.findByRole('button', { name: 'Generate' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows the error and lets the user try again', async () => {
    const user = userEvent.setup()
    vi.mocked(generateQuiz)
      .mockRejectedValueOnce(new Error(JSON.stringify({ code: 'invalid_generation' })))
      .mockResolvedValueOnce(QUIZ)
    const { onGenerated } = renderDialog()
    await pickFirstSession(user)
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The generated test was not valid. Try again.',
    )
    vi.mocked(generateQuiz).mockReset()
    vi.mocked(generateQuiz).mockReturnValueOnce(new Promise(() => {}))
    await user.click(screen.getByRole('button', { name: 'Try again' }))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(onGenerated).not.toHaveBeenCalled()
  })

  it('delivers the quiz when trying again works', async () => {
    const user = userEvent.setup()
    vi.mocked(generateQuiz)
      .mockRejectedValueOnce(new Error(JSON.stringify({ code: 'invalid_generation' })))
      .mockResolvedValueOnce(QUIZ)
    const { onGenerated } = renderDialog()
    await pickFirstSession(user)
    await user.click(screen.getByRole('button', { name: 'Generate' }))
    await user.click(await screen.findByRole('button', { name: 'Try again' }))

    await waitFor(() => expect(onGenerated).toHaveBeenCalledWith(QUIZ))
  })

  it('marks only the chosen session', async () => {
    const user = userEvent.setup()
    renderDialog()
    await screen.findByRole('radio', { name: /Go Concurrency/ })
    expect(screen.getByRole('radio', { name: /Go Concurrency/ })).not.toBeChecked()

    await user.click(screen.getByRole('radio', { name: /Distributed Caching/ }))

    expect(screen.getByRole('radio', { name: /Distributed Caching/ })).toBeChecked()
    expect(screen.getByRole('radio', { name: /Go Concurrency/ })).not.toBeChecked()
  })

  it('closes with the X without stopping anything when nothing is generating', async () => {
    const user = userEvent.setup()
    const { onClose } = renderDialog()
    await screen.findByRole('radio', { name: /Distributed Caching/ })

    await user.click(screen.getByRole('button', { name: 'Close' }))

    expect(onClose).toHaveBeenCalledOnce()
    expect(cancelQuizGeneration).not.toHaveBeenCalled()
  })

  it('closes with Cancel when nothing is generating', async () => {
    const user = userEvent.setup()
    const { onClose } = renderDialog()
    await screen.findByRole('radio', { name: /Distributed Caching/ })

    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onClose).toHaveBeenCalledOnce()
    expect(cancelQuizGeneration).not.toHaveBeenCalled()
  })

  it('stops a running generation when dismissed, and tells the caller when the quiz arrives late', async () => {
    const user = userEvent.setup()
    let resolve: (quiz: QuizSummary) => void = () => {}
    vi.mocked(generateQuiz).mockReturnValue(new Promise((done) => (resolve = done)))
    vi.mocked(cancelQuizGeneration).mockResolvedValue(undefined)
    const props = renderDialog()
    await pickFirstSession(user)
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    await user.click(screen.getByRole('button', { name: 'Close' }))
    expect(cancelQuizGeneration).toHaveBeenCalledOnce()
    expect(props.onClose).toHaveBeenCalledOnce()

    props.rerender(<GenerateQuizDialog {...props} open={false} />)
    resolve(QUIZ)
    await waitFor(() => expect(props.onGeneratedAfterClose).toHaveBeenCalledWith(QUIZ))
    expect(props.onGenerated).not.toHaveBeenCalled()
  })

  it('speaks the profile language', async () => {
    setLabLanguage('pt')
    renderDialog()

    expect(await screen.findByText('Gerar um teste')).toBeInTheDocument()
    expect(screen.getByRole('spinbutton', { name: 'Questões' })).toBeInTheDocument()
  })
})
