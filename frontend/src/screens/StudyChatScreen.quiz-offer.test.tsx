import { setLabLanguage } from '@/i18n'
import type { ComponentProps } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { getOfferProfile, getPendingOffer, listSessionExercises } from '@/lib/exercise'
import {
  acceptQuizOffer,
  declineQuizOffer,
  getPendingQuizOffer,
  getQuizOfferDifficulty,
  listQuizAttempts,
  listSessionQuizzes,
  type QuizOffer,
  type QuizSummary,
} from '@/lib/quiz'
import {
  getStudySessionSummary,
  getStudySessionUsage,
  onStudyChunk,
  onStudyCompactionFailed,
  onStudyContextCompacted,
  onStudyContextCompacting,
  onStudyContextLimitReached,
  onStudyContextLimitUnavailable,
  onStudyContextNormal,
  onStudyContextWarning,
  onStudyDone,
  onStudyError,
  onStudySources,
  resumeStudySession,
  sendStudyMessage,
  type StudyDoneEvent,
} from '@/lib/study'
import { getPassScore } from '@/lib/passScore'
import StudyChatScreen from './StudyChatScreen'

vi.mock('@/lib/passScore', () => ({ getPassScore: vi.fn() }))

vi.mock('@/lib/exercise', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/exercise')>()),
  getPendingOffer: vi.fn(),
  listSessionExercises: vi.fn(),
  getOfferProfile: vi.fn(),
}))

vi.mock('@/lib/quiz', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/quiz')>()),
  getPendingQuizOffer: vi.fn(),
  getQuizOfferDifficulty: vi.fn(),
  acceptQuizOffer: vi.fn(),
  declineQuizOffer: vi.fn(),
  listSessionQuizzes: vi.fn(),
  listQuizAttempts: vi.fn(),
}))

vi.mock('@/lib/study', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/study')>()),
  getStudySessionSummary: vi.fn(),
  getStudySessionUsage: vi.fn(),
  resumeStudySession: vi.fn(),
  sendStudyMessage: vi.fn(),
  onStudyChunk: vi.fn(),
  onStudyDone: vi.fn(),
  onStudyError: vi.fn(),
  onStudySources: vi.fn(),
  onStudyContextNormal: vi.fn(),
  onStudyContextWarning: vi.fn(),
  onStudyContextLimitReached: vi.fn(),
  onStudyContextLimitUnavailable: vi.fn(),
  onStudyContextCompacting: vi.fn(),
  onStudyContextCompacted: vi.fn(),
  onStudyCompactionFailed: vi.fn(),
}))

const OFFER: QuizOffer = { id: 'offer-1', sessionId: 'session-1', messageId: 'm2' }
const PENDING_TEXT = 'Accept or decline the offer to keep chatting.'

const CARD: QuizSummary = {
  id: 'quiz-1',
  sessionId: 'session-1',
  sessionTitle: 'Caching',
  difficulty: 'HARD',
  questionCount: 8,
  createdAt: '2026-11-16T09:30:00Z',
  attemptCount: 0,
  latestScore: null,
  messageId: 'm2',
}

interface Handlers {
  done?: (event: StudyDoneEvent) => void
}

function subscribe(): Handlers {
  const handlers: Handlers = {}
  vi.mocked(onStudyDone).mockImplementation((handler) => {
    handlers.done = handler
    return vi.fn()
  })
  for (const subscription of [
    onStudyError,
    onStudyChunk,
    onStudySources,
    onStudyContextNormal,
    onStudyContextWarning,
    onStudyContextLimitReached,
    onStudyContextLimitUnavailable,
    onStudyContextCompacting,
    onStudyContextCompacted,
    onStudyCompactionFailed,
  ]) {
    vi.mocked(subscription as (handler: () => void) => () => void).mockReturnValue(vi.fn())
  }
  return handlers
}

const CONTEXT = { state: 'normal', model: '', usedTokens: 0, contextLength: 0, estimated: false }

async function renderResumed(props: Partial<ComponentProps<typeof StudyChatScreen>> = {}) {
  vi.mocked(resumeStudySession).mockResolvedValue({
    session: {
      id: 'session-1',
      topic: 'Caching',
      folderId: 'f1',
      goal: 'g',
      startedAt: '2026-08-16T10:00:00Z',
      context: CONTEXT,
    },
    messages: [
      { id: 'm1', role: 'user', content: 'quiz me', createdAt: 't', sources: [] },
      { id: 'm2', role: 'assistant', content: 'Sure, a quiz.', createdAt: 't', sources: [] },
    ],
    hasMore: false,
    nextCursor: '',
    summaryCoversUntilMessageId: '',
  } as Awaited<ReturnType<typeof resumeStudySession>>)
  const onStartQuiz = vi.fn()
  render(
    <StudyChatScreen
      sessionId="session-1"
      initialTopic=""
      mode="resume"
      onStartNewSession={vi.fn()}
      startingNewSession={false}
      onStartExam={vi.fn()}
      onStartQuiz={onStartQuiz}
      onOpenMindMap={vi.fn()}
      {...props}
    />,
  )
  await screen.findByText('Sure, a quiz.')
  return { onStartQuiz }
}

describe('StudyChatScreen — quiz offer', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    setLabLanguage('en')
    vi.mocked(getPassScore).mockResolvedValue(70)
    vi.mocked(getStudySessionSummary).mockResolvedValue({
      found: false,
      coversUntilMessageId: '',
    } as Awaited<ReturnType<typeof getStudySessionSummary>>)
    vi.mocked(getStudySessionUsage).mockRejectedValue(new Error('no usage'))
    vi.mocked(getOfferProfile).mockResolvedValue({ difficulty: 'INTERMEDIATE' })
    vi.mocked(getQuizOfferDifficulty).mockResolvedValue('MEDIUM')
    vi.mocked(getPendingOffer).mockResolvedValue(null)
    vi.mocked(getPendingQuizOffer).mockResolvedValue(null)
    vi.mocked(listSessionExercises).mockResolvedValue([])
    vi.mocked(listSessionQuizzes).mockResolvedValue([])
  })

  it('shows the pending quiz offer under its message, with the input disabled and the reason', async () => {
    subscribe()
    vi.mocked(getPendingQuizOffer).mockResolvedValue(OFFER)

    await renderResumed()

    expect(await screen.findByText('Take a test about this session')).toBeInTheDocument()
    expect(getPendingQuizOffer).toHaveBeenCalledWith('session-1')
    expect(screen.getByPlaceholderText(PENDING_TEXT)).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    const bubbles = document.querySelectorAll('[data-slot="message-bubble"]')
    const card = screen.getByText('Take a test about this session').closest('div.rounded-lg')!
    expect(bubbles[1].compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('shows the exercise offer, and does not look for a quiz one, when an exercise offer is pending', async () => {
    subscribe()
    vi.mocked(getPendingOffer).mockResolvedValue({
      id: 'offer-9',
      sessionId: 'session-1',
      messageId: 'm2',
    })

    await renderResumed()

    expect(await screen.findByText('Practice with an architecture exercise')).toBeInTheDocument()
    expect(screen.queryByText('Take a test about this session')).not.toBeInTheDocument()
    expect(getPendingQuizOffer).not.toHaveBeenCalled()
  })

  it('starts the offer from the difficulty of the profile', async () => {
    subscribe()
    vi.mocked(getPendingQuizOffer).mockResolvedValue(OFFER)
    vi.mocked(getQuizOfferDifficulty).mockResolvedValue('HARD')

    await renderResumed()

    expect(await screen.findByRole('combobox', { name: 'Difficulty' })).toHaveTextContent('Hard')
  })

  it('speaks the profile language', async () => {
    subscribe()
    vi.mocked(getPendingQuizOffer).mockResolvedValue(OFFER)
    setLabLanguage('pt')

    await renderResumed()

    expect(await screen.findByRole('button', { name: 'Aceitar' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Recusar' })).toBeInTheDocument()
  })

  it('looks for an offer when a reply finishes and shows it', async () => {
    const handlers = subscribe()
    await renderResumed()
    vi.mocked(sendStudyMessage).mockResolvedValue()
    vi.mocked(getPendingQuizOffer).mockResolvedValue(OFFER)
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText('Type your answer...'), 'quiz me again')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    act(() => handlers.done?.({ sessionId: 'session-1' }))

    expect(await screen.findByRole('button', { name: 'Accept' })).toBeInTheDocument()
    expect(screen.getByPlaceholderText(PENDING_TEXT)).toBeDisabled()
  })

  it('accepting replaces the offer with the quiz card under the same message and re-enables the input', async () => {
    subscribe()
    vi.mocked(getPendingQuizOffer).mockResolvedValue(OFFER)
    vi.mocked(acceptQuizOffer).mockResolvedValue(CARD)
    await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Accept' }))

    expect(acceptQuizOffer).toHaveBeenCalledWith('offer-1', 10, 'MEDIUM')
    expect(await screen.findByRole('button', { name: 'Take test' })).toBeInTheDocument()
    expect(screen.getByText('8 questions')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Accept' })).not.toBeInTheDocument()
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
    const bubbles = document.querySelectorAll('[data-slot="message-bubble"]')
    const card = screen.getByRole('button', { name: 'Take test' }).closest('div.rounded-lg')!
    expect(bubbles[1].compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('keeps the input disabled and the offer in place when accepting fails', async () => {
    subscribe()
    vi.mocked(getPendingQuizOffer).mockResolvedValue(OFFER)
    vi.mocked(acceptQuizOffer).mockRejectedValue(
      JSON.stringify({ code: 'invalid_generation', message: 'x' }),
    )
    await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Accept' }))

    expect(
      await screen.findByText('The generated test was not valid. Try again.'),
    ).toBeInTheDocument()
    expect(screen.getByPlaceholderText(PENDING_TEXT)).toBeDisabled()
  })

  it('declining removes the card, keeps the tutor text and re-enables the input', async () => {
    subscribe()
    vi.mocked(getPendingQuizOffer).mockResolvedValue(OFFER)
    vi.mocked(declineQuizOffer).mockResolvedValue()
    await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Decline' }))

    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Decline' })).not.toBeInTheDocument(),
    )
    expect(screen.getByText('Sure, a quiz.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Take test' })).not.toBeInTheDocument()
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
  })

  it('shows the stored quiz card where it hangs when the session is opened', async () => {
    subscribe()
    vi.mocked(listSessionQuizzes).mockResolvedValue([CARD])

    await renderResumed()

    expect(await screen.findByRole('button', { name: 'Take test' })).toBeInTheDocument()
    expect(listSessionQuizzes).toHaveBeenCalledWith('session-1')
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
  })

  it('shows no card for a quiz that hangs under a message that is not in the conversation', async () => {
    subscribe()
    vi.mocked(listSessionQuizzes).mockResolvedValue([{ ...CARD, messageId: 'elsewhere' }])

    await renderResumed()

    await waitFor(() => expect(listSessionQuizzes).toHaveBeenCalled())
    expect(screen.queryByRole('button', { name: 'Take test' })).not.toBeInTheDocument()
  })

  it('starts the quiz of a card, with the session it came from', async () => {
    subscribe()
    vi.mocked(listSessionQuizzes).mockResolvedValue([CARD])
    const { onStartQuiz } = await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Take test' }))

    expect(onStartQuiz).toHaveBeenCalledWith('quiz-1', 'session-1')
  })

  it('opens the history of a quiz that was taken', async () => {
    subscribe()
    vi.mocked(listSessionQuizzes).mockResolvedValue([
      { ...CARD, attemptCount: 1, latestScore: 7.5 },
    ])
    vi.mocked(listQuizAttempts).mockResolvedValue([
      {
        id: 'attempt-1',
        quizId: 'quiz-1',
        correctCount: 6,
        total: 8,
        score: 7.5,
        finishedAt: '2026-11-16T10:00:00Z',
      },
    ])
    await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'History' }))

    const dialog = await screen.findByRole('dialog')
    expect(listQuizAttempts).toHaveBeenCalledWith('quiz-1')
    expect(await within(dialog).findByText('7.5 / 10 · 6 of 8')).toBeInTheDocument()
  })

  it('keeps the chat working when the quiz offer or the cards cannot be read', async () => {
    subscribe()
    vi.mocked(getPendingQuizOffer).mockRejectedValue(new Error('down'))
    vi.mocked(listSessionQuizzes).mockRejectedValue(new Error('down'))

    await renderResumed()

    await waitFor(() => expect(listSessionQuizzes).toHaveBeenCalled())
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
  })
})
