import { setLabLanguage } from '@/i18n'
import type { ComponentProps } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { getOfferProfile, getPendingOffer, listSessionExercises } from '@/lib/exercise'
import { getPendingQuizOffer, getQuizOfferDifficulty, listSessionQuizzes } from '@/lib/quiz'
import {
  acceptMindMapOffer,
  declineMindMapOffer,
  getPendingMindMapOffer,
  listSessionMindMaps,
  onMindMapOfferResolved,
  type MindMapOffer,
  type MindMapOfferResolvedEvent,
  type MindMapSummary,
} from '@/lib/mindmap'
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
  onExerciseOfferResolved: () => () => {},
  getPendingOffer: vi.fn(),
  listSessionExercises: vi.fn(),
  getOfferProfile: vi.fn(),
}))

vi.mock('@/lib/quiz', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/quiz')>()),
  onQuizOfferResolved: () => () => {},
  getPendingQuizOffer: vi.fn(),
  getQuizOfferDifficulty: vi.fn(),
  listSessionQuizzes: vi.fn(),
}))

vi.mock('@/lib/mindmap', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/mindmap')>()),
  getPendingMindMapOffer: vi.fn(),
  acceptMindMapOffer: vi.fn(),
  declineMindMapOffer: vi.fn(),
  listSessionMindMaps: vi.fn(),
  onMindMapOfferResolved: vi.fn(),
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

const OFFER: MindMapOffer = { id: 'offer-1', sessionId: 'session-1', messageId: 'm2' }
const PENDING_TEXT = 'Accept or decline the offer to keep chatting.'

const CARD: MindMapSummary = {
  id: 'map-1',
  sessionId: 'session-1',
  sessionTitle: 'Caching',
  title: 'Caching map',
  createdAt: '2026-11-17T09:30:00Z',
  updatedAt: '2026-11-17T10:30:00Z',
  messageId: 'm2',
}

interface Handlers {
  done?: (event: StudyDoneEvent) => void
  offerResolved?: (event: MindMapOfferResolvedEvent) => void
}

function subscribe(): Handlers {
  const handlers: Handlers = {}
  vi.mocked(onStudyDone).mockImplementation((handler) => {
    handlers.done = handler
    return vi.fn()
  })
  vi.mocked(onMindMapOfferResolved).mockImplementation((handler) => {
    handlers.offerResolved = handler
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

async function renderResumed(
  props: Partial<ComponentProps<typeof StudyChatScreen>> = {},
  withLaterMessage = false,
) {
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
      { id: 'm1', role: 'user', content: 'draw me a mind map', createdAt: 't', sources: [] },
      { id: 'm2', role: 'assistant', content: 'Sure, a mind map.', createdAt: 't', sources: [] },
      ...(withLaterMessage
        ? [{ id: 'm3', role: 'user', content: 'thanks', createdAt: 't', sources: [] }]
        : []),
    ],
    hasMore: false,
    nextCursor: '',
    summaryCoversUntilMessageId: '',
  } as Awaited<ReturnType<typeof resumeStudySession>>)
  const onOpenMindMap = vi.fn()
  const { unmount } = render(
    <StudyChatScreen
      sessionId="session-1"
      initialTopic=""
      mode="resume"
      onStartNewSession={vi.fn()}
      startingNewSession={false}
      onStartExam={vi.fn()}
      onStartQuiz={vi.fn()}
      onOpenMindMap={onOpenMindMap}
      {...props}
    />,
  )
  await screen.findByText('Sure, a mind map.')
  return { onOpenMindMap, unmount }
}

describe('StudyChatScreen — mind map offer', () => {
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
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(null)
    vi.mocked(listSessionExercises).mockResolvedValue([])
    vi.mocked(listSessionQuizzes).mockResolvedValue([])
    vi.mocked(listSessionMindMaps).mockResolvedValue([])
  })

  it('shows the pending mind map offer under its message, with the input disabled and the reason', async () => {
    subscribe()
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)

    await renderResumed({}, true)

    expect(await screen.findByText('Draw a mind map of this session')).toBeInTheDocument()
    expect(getPendingMindMapOffer).toHaveBeenCalledWith('session-1')
    expect(screen.getByPlaceholderText(PENDING_TEXT)).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    const bubbles = document.querySelectorAll('[data-slot="message-bubble"]')
    const card = screen.getByText('Draw a mind map of this session').closest('div.rounded-lg')!
    expect(bubbles[1].compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(bubbles[2].compareDocumentPosition(card) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy()
  })

  it('shows the quiz offer, not a mind map one, when both are pending', async () => {
    subscribe()
    vi.mocked(getPendingQuizOffer).mockResolvedValue({
      id: 'offer-9',
      sessionId: 'session-1',
      messageId: 'm2',
    })
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)

    await renderResumed()

    expect(await screen.findByText('Take a test about this session')).toBeInTheDocument()
    expect(screen.queryByText('Draw a mind map of this session')).not.toBeInTheDocument()
  })

  it('speaks the profile language', async () => {
    subscribe()
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)
    setLabLanguage('pt')

    await renderResumed()

    expect(await screen.findByRole('button', { name: 'Aceitar' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Recusar' })).toBeInTheDocument()
  })

  it('looks for an offer when a reply finishes and shows it', async () => {
    const handlers = subscribe()
    await renderResumed()
    vi.mocked(sendStudyMessage).mockResolvedValue()
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText('Type your answer...'), 'a mind map again')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    act(() => handlers.done?.({ sessionId: 'session-1' }))

    expect(await screen.findByRole('button', { name: 'Accept' })).toBeInTheDocument()
    expect(screen.getByPlaceholderText(PENDING_TEXT)).toBeDisabled()
  })

  it('accepting replaces the offer with the mind map card under the same message and re-enables the input', async () => {
    subscribe()
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)
    vi.mocked(acceptMindMapOffer).mockResolvedValue(CARD)
    await renderResumed({}, true)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Accept' }))

    expect(acceptMindMapOffer).toHaveBeenCalledWith('offer-1')
    expect(await screen.findByRole('button', { name: 'Open mind map' })).toBeInTheDocument()
    expect(screen.getByText('Caching map')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Accept' })).not.toBeInTheDocument()
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
    const bubbles = document.querySelectorAll('[data-slot="message-bubble"]')
    const card = screen.getByRole('button', { name: 'Open mind map' }).closest('div.rounded-lg')!
    expect(bubbles[1].compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(bubbles[2].compareDocumentPosition(card) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy()
  })

  it('keeps the input disabled and the offer in place when accepting fails', async () => {
    subscribe()
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)
    vi.mocked(acceptMindMapOffer).mockRejectedValue(
      JSON.stringify({ code: 'invalid_generation', message: 'x' }),
    )
    await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Accept' }))

    expect(
      await screen.findByText('The generated mind map was not valid. Try again.'),
    ).toBeInTheDocument()
    expect(screen.getByPlaceholderText(PENDING_TEXT)).toBeDisabled()
  })

  it('declining removes the card, keeps the tutor text and re-enables the input', async () => {
    subscribe()
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)
    vi.mocked(declineMindMapOffer).mockResolvedValue()
    await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Decline' }))

    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Decline' })).not.toBeInTheDocument(),
    )
    expect(screen.getByText('Sure, a mind map.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Open mind map' })).not.toBeInTheDocument()
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
  })

  it('shows the stored mind map card where it hangs when the session is opened', async () => {
    subscribe()
    vi.mocked(listSessionMindMaps).mockResolvedValue([CARD])

    await renderResumed()

    expect(await screen.findByRole('button', { name: 'Open mind map' })).toBeInTheDocument()
    expect(listSessionMindMaps).toHaveBeenCalledWith('session-1')
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
  })

  it('shows no card for a mind map that hangs under a message that is not in the conversation', async () => {
    subscribe()
    vi.mocked(listSessionMindMaps).mockResolvedValue([{ ...CARD, messageId: 'elsewhere' }])

    await renderResumed()

    await waitFor(() => expect(listSessionMindMaps).toHaveBeenCalled())
    expect(screen.queryByRole('button', { name: 'Open mind map' })).not.toBeInTheDocument()
  })

  it('opens the mind map of a card, with the session it came from', async () => {
    subscribe()
    vi.mocked(listSessionMindMaps).mockResolvedValue([CARD])
    const { onOpenMindMap } = await renderResumed()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Open mind map' }))

    expect(onOpenMindMap).toHaveBeenCalledWith('map-1', 'session-1')
  })

  it('keeps the chat working when the mind map offer or the cards cannot be read', async () => {
    subscribe()
    vi.mocked(getPendingMindMapOffer).mockRejectedValue(new Error('down'))
    vi.mocked(listSessionMindMaps).mockRejectedValue(new Error('down'))

    await renderResumed()

    await waitFor(() => expect(listSessionMindMaps).toHaveBeenCalled())
    await waitFor(() => expect(getPendingMindMapOffer).toHaveBeenCalled())
    vi.mocked(sendStudyMessage).mockResolvedValue()
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText('Type your answer...'), 'still here')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    expect(sendStudyMessage).toHaveBeenCalledWith(
      'session-1',
      'Caching',
      'still here',
      expect.anything(),
    )
  })

  it('does not bring back an offer that was declined while an older read was still pending', async () => {
    const handlers = subscribe()
    let answerOldRead: (offer: MindMapOffer) => void = () => {}
    vi.mocked(getPendingMindMapOffer).mockReturnValueOnce(
      new Promise((resolve) => {
        answerOldRead = resolve
      }),
    )
    await renderResumed()
    vi.mocked(sendStudyMessage).mockResolvedValue()
    vi.mocked(declineMindMapOffer).mockResolvedValue()
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)
    const user = userEvent.setup()
    await user.type(screen.getByPlaceholderText('Type your answer...'), 'a mind map')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    act(() => handlers.done?.({ sessionId: 'session-1' }))
    await user.click(await screen.findByRole('button', { name: 'Decline' }))
    expect(screen.queryByRole('button', { name: 'Decline' })).not.toBeInTheDocument()

    await act(async () => answerOldRead(OFFER))

    expect(screen.queryByRole('button', { name: 'Accept' })).not.toBeInTheDocument()
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
  })

  it('keeps the accepted card when a read that started before the accept answers late', async () => {
    subscribe()
    let answerOldRead: (cards: MindMapSummary[]) => void = () => {}
    vi.mocked(listSessionMindMaps).mockReturnValueOnce(
      new Promise((resolve) => {
        answerOldRead = resolve
      }),
    )
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)
    vi.mocked(acceptMindMapOffer).mockResolvedValue(CARD)
    await renderResumed()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Accept' }))
    expect(await screen.findByRole('button', { name: 'Open mind map' })).toBeInTheDocument()

    await act(async () => answerOldRead([]))

    expect(screen.getByRole('button', { name: 'Open mind map' })).toBeInTheDocument()
  })

  it('shows the card and frees the input, without a click, when the offer was accepted while the chat was unmounted', async () => {
    // Given an offer accepted in a chat that was left before the map was drawn
    const handlers = subscribe()
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)
    const first = await renderResumed({}, true)
    first.unmount()
    // and the same session opened again while the offer is still pending
    await renderResumed({}, true)
    expect(await screen.findByRole('button', { name: 'Accept' })).toBeInTheDocument()

    // When the backend finishes: the map is saved and the offer is gone
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(null)
    vi.mocked(listSessionMindMaps).mockResolvedValue([CARD])
    act(() => handlers.offerResolved?.({ sessionId: 'session-1', offerId: 'offer-1' }))

    // Then the mounted chat shows the card and the input again
    expect(await screen.findByRole('button', { name: 'Open mind map' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Accept' })).not.toBeInTheDocument()
    expect(screen.getByPlaceholderText('Type your answer...')).toBeEnabled()
  })

  it('ignores the resolution of an offer of another session', async () => {
    const handlers = subscribe()
    vi.mocked(getPendingMindMapOffer).mockResolvedValue(OFFER)
    await renderResumed()
    await screen.findByRole('button', { name: 'Accept' })
    vi.mocked(getPendingMindMapOffer).mockClear()

    act(() => handlers.offerResolved?.({ sessionId: 'session-2', offerId: 'offer-9' }))

    expect(getPendingMindMapOffer).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeInTheDocument()
  })
})
