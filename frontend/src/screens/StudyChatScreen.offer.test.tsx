import type { ComponentProps } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  acceptOffer,
  declineOffer,
  getAssistantLanguage,
  getPendingOffer,
  type ExerciseOffer,
  type SessionExercise,
} from '@/lib/exercise'
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
  type StudyErrorEvent,
} from '@/lib/study'
import { getPassScore } from '@/lib/passScore'
import StudyChatScreen from './StudyChatScreen'

vi.mock('@/lib/passScore', () => ({ getPassScore: vi.fn() }))

vi.mock('@/lib/exercise', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/exercise')>()),
  getPendingOffer: vi.fn(),
  getAssistantLanguage: vi.fn(),
  acceptOffer: vi.fn(),
  declineOffer: vi.fn(),
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

const OFFER: ExerciseOffer = { id: 'offer-1', sessionId: 'session-1', messageId: 'm2' }
const PENDING_TEXT = 'Accept or decline the exercise offer to keep chatting.'

interface Handlers {
  done?: (event: StudyDoneEvent) => void
  error?: (event: StudyErrorEvent) => void
}

function subscribe(): Handlers {
  const handlers: Handlers = {}
  vi.mocked(onStudyDone).mockImplementation((handler) => {
    handlers.done = handler
    return vi.fn()
  })
  vi.mocked(onStudyError).mockImplementation((handler) => {
    handlers.error = handler
    return vi.fn()
  })
  for (const subscription of [
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

const EXERCISE: SessionExercise = {
  challengeId: 'c1',
  title: 'Design a cache',
  topic: 'caching',
  difficulty: 'ADVANCED',
  timeLimitMinutes: 45,
}

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
      { id: 'm1', role: 'user', content: 'give me an exercise', createdAt: 't', sources: [] },
      {
        id: 'm2',
        role: 'assistant',
        content: 'Sure, let us practice.',
        createdAt: 't',
        sources: [],
      },
    ],
    hasMore: false,
    nextCursor: '',
    summaryCoversUntilMessageId: '',
  } as Awaited<ReturnType<typeof resumeStudySession>>)
  render(
    <StudyChatScreen
      sessionId="session-1"
      initialTopic=""
      mode="resume"
      onStartNewSession={vi.fn()}
      startingNewSession={false}
      {...props}
    />,
  )
  await screen.findByText('Sure, let us practice.')
}

describe('StudyChatScreen — exercise offer', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(getPassScore).mockResolvedValue(70)
    vi.mocked(getStudySessionSummary).mockResolvedValue({
      found: false,
      coversUntilMessageId: '',
    } as Awaited<ReturnType<typeof getStudySessionSummary>>)
    vi.mocked(getStudySessionUsage).mockRejectedValue(new Error('no usage'))
    vi.mocked(getAssistantLanguage).mockResolvedValue('en')
    vi.mocked(getPendingOffer).mockResolvedValue(null)
  })

  it('shows no card and keeps the input enabled when no offer is pending', async () => {
    subscribe()
    await renderResumed()

    expect(screen.queryByRole('button', { name: 'Accept' })).not.toBeInTheDocument()
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
  })

  it('shows the pending offer when the session is reopened, with the input disabled and the reason', async () => {
    subscribe()
    vi.mocked(getPendingOffer).mockResolvedValue(OFFER)

    await renderResumed()

    expect(await screen.findByRole('button', { name: 'Accept' })).toBeInTheDocument()
    expect(getPendingOffer).toHaveBeenCalledWith('session-1')
    const input = screen.getByPlaceholderText(PENDING_TEXT)
    expect(input).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
  })

  it('uses the assistant language for the buttons', async () => {
    subscribe()
    vi.mocked(getPendingOffer).mockResolvedValue(OFFER)
    vi.mocked(getAssistantLanguage).mockResolvedValue('pt')

    await renderResumed()

    expect(await screen.findByRole('button', { name: 'Aceitar' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Recusar' })).toBeInTheDocument()
  })

  it('looks for an offer when a reply finishes and shows it', async () => {
    const handlers = subscribe()
    await renderResumed()
    vi.mocked(sendStudyMessage).mockResolvedValue()
    vi.mocked(getPendingOffer).mockResolvedValue(OFFER)
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText('Type your answer...'), 'I want to practice')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    act(() => handlers.done?.({ sessionId: 'session-1' }))

    expect(await screen.findByRole('button', { name: 'Accept' })).toBeInTheDocument()
    expect(screen.getByPlaceholderText(PENDING_TEXT)).toBeDisabled()
  })

  it('declining removes the card, keeps the tutor text and re-enables the input', async () => {
    subscribe()
    vi.mocked(getPendingOffer).mockResolvedValue(OFFER)
    vi.mocked(declineOffer).mockResolvedValue()
    await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Decline' }))

    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Decline' })).not.toBeInTheDocument(),
    )
    expect(screen.getByText('Sure, let us practice.')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
  })

  it('accepting shows the ready exercise and re-enables the input', async () => {
    subscribe()
    vi.mocked(getPendingOffer).mockResolvedValue(OFFER)
    vi.mocked(acceptOffer).mockResolvedValue({
      challengeId: 'c1',
      title: 'Design a cache',
      topic: 'caching',
      difficulty: 'ADVANCED',
      timeLimitMinutes: 45,
    })
    await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Accept' }))

    expect(await screen.findByText('Exercise ready: Design a cache')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
  })

  it('opens the exam rules from the ready card and starts the exam with the session', async () => {
    subscribe()
    vi.mocked(getPendingOffer).mockResolvedValue(OFFER)
    vi.mocked(acceptOffer).mockResolvedValue(EXERCISE)
    const onStartExam = vi.fn()
    await renderResumed({ onStartExam })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Accept' }))

    await user.click(await screen.findByRole('button', { name: 'Open in Architecture Lab' }))
    expect(onStartExam).not.toHaveBeenCalled()
    expect(screen.getByText('Time limit: 45 minutes')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Start exam' }))

    expect(onStartExam).toHaveBeenCalledExactlyOnceWith(EXERCISE, 'session-1')
  })

  it('does not start the exam when the rules are cancelled', async () => {
    subscribe()
    vi.mocked(getPendingOffer).mockResolvedValue(null)
    const onStartExam = vi.fn()
    await renderResumed({ onStartExam, initialExercise: EXERCISE })
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: 'Open in Architecture Lab' }))
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onStartExam).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: 'Start exam' })).not.toBeInTheDocument()
  })

  it('shows the ready exercise again when the chat comes back from the lab', async () => {
    subscribe()
    vi.mocked(getPendingOffer).mockResolvedValue(null)

    await renderResumed({ initialExercise: EXERCISE })

    expect(screen.getByText('Exercise ready: Design a cache')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Open in Architecture Lab' })).toBeInTheDocument()
  })

  it('keeps the input disabled while accepting fails and the offer stays', async () => {
    subscribe()
    vi.mocked(getPendingOffer).mockResolvedValue(OFFER)
    vi.mocked(acceptOffer).mockRejectedValue(new Error('boom'))
    await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Accept' }))

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByPlaceholderText(PENDING_TEXT)).toBeDisabled()
  })

  it('shows the offer when a send is refused because one is pending', async () => {
    const handlers = subscribe()
    await renderResumed()
    vi.mocked(sendStudyMessage).mockResolvedValue()
    vi.mocked(getPendingOffer).mockResolvedValue(OFFER)
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText('Type your answer...'), 'hello')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    act(() =>
      handlers.error?.({ sessionId: 'session-1', message: 'pending', code: 'offer_pending' }),
    )

    expect(await screen.findByRole('button', { name: 'Accept' })).toBeInTheDocument()
    // the optimistic bubble is gone and the draft is back in the composer
    expect(document.querySelectorAll('[data-slot="message-bubble"]')).toHaveLength(2)
    expect(screen.getByPlaceholderText(PENDING_TEXT)).toHaveValue('hello')
  })

  it('clears the ready exercise card when the next message is sent', async () => {
    subscribe()
    vi.mocked(getPendingOffer).mockResolvedValue(OFFER)
    vi.mocked(acceptOffer).mockResolvedValue({
      challengeId: 'c1',
      title: 'Design a cache',
      topic: 'caching',
      difficulty: 'ADVANCED',
      timeLimitMinutes: 45,
    })
    vi.mocked(sendStudyMessage).mockResolvedValue()
    await renderResumed()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Accept' }))
    await screen.findByText('Exercise ready: Design a cache')

    await user.type(screen.getByPlaceholderText('Type your answer...'), 'thanks')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    expect(screen.queryByText('Exercise ready: Design a cache')).not.toBeInTheDocument()
  })

  it('explains the refusal when a send is rejected because an offer is pending', async () => {
    const handlers = subscribe()
    await renderResumed()
    vi.mocked(sendStudyMessage).mockResolvedValue()
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText('Type your answer...'), 'hello')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    act(() =>
      handlers.error?.({ sessionId: 'session-1', message: 'pending', code: 'offer_pending' }),
    )

    expect(
      await screen.findByText(
        'Accept or decline the exercise offer before sending another message.',
      ),
    ).toBeInTheDocument()
  })
})
