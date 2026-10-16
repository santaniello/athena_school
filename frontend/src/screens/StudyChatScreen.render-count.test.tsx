import { describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen } from '@testing-library/react'
import {
  getStudySessionUsage,
  onStudyChunk,
  onStudyContextLimitReached,
  onStudyContextLimitUnavailable,
  onStudyContextNormal,
  onStudyContextWarning,
  onStudyDone,
  onStudyError,
  onStudySources,
  resumeStudySession,
  sendStudyMessage,
  type StudyChunkEvent,
} from '@/lib/study'
import StudyChatScreen from './StudyChatScreen'

// Counts how many times each message's markdown is (re-)rendered, keyed by
// content. jsdom has no layout, so render counts are the measurable proxy
// for the work a streamed chunk causes (spec 2.23, decision 6).
const markdownRenders = vi.hoisted(() => new Map<string, number>())

vi.mock('react-markdown', () => ({
  default: ({ children }: { children: string }) => {
    markdownRenders.set(children, (markdownRenders.get(children) ?? 0) + 1)
    return <p>{children}</p>
  },
}))

vi.mock('@/lib/study', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/study')>()
  return {
    ...original,
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
  }
})

describe('StudyChatScreen — render cost while streaming', () => {
  it('does not re-render finished messages while a reply streams', async () => {
    // Given a resumed session with finished messages
    let chunk: ((event: StudyChunkEvent) => void) | undefined
    vi.mocked(onStudyChunk).mockImplementation((handler) => {
      chunk = handler
      return vi.fn()
    })
    for (const subscribe of [
      onStudyDone,
      onStudyError,
      onStudySources,
      onStudyContextNormal,
      onStudyContextWarning,
      onStudyContextLimitReached,
      onStudyContextLimitUnavailable,
    ]) {
      vi.mocked(subscribe as () => () => void).mockReturnValue(vi.fn())
    }
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-1',
        topic: 'Caches',
        folderId: 'folder-1',
        goal: '',
        startedAt: '2026-08-16T10:00:00Z',
        context: { state: 'normal', model: '', usedTokens: 0, contextLength: 0, estimated: false },
      },
      messages: [
        { id: 'm1', role: 'user', content: 'first question', createdAt: '', sources: [] },
        { id: 'm2', role: 'assistant', content: 'first answer', createdAt: '', sources: [] },
      ],
    })
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        onStartNewSession={vi.fn()}
        startingNewSession={false}
      />,
    )
    await screen.findByText('first answer')

    // When the user sends a message and the reply streams in over several chunks
    vi.mocked(sendStudyMessage).mockReturnValue(new Promise(() => {}))
    fireEvent.change(screen.getByPlaceholderText('Type your answer...'), {
      target: { value: 'next question' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    for (const content of ['a', 'b', 'c', 'd', 'e']) {
      act(() => chunk?.({ sessionId: 'session-1', content }))
    }

    // Then the streamed text is shown, and each finished message was
    // rendered exactly once
    expect(screen.getByText('abcde')).toBeInTheDocument()
    expect(markdownRenders.get('first question')).toBe(1)
    expect(markdownRenders.get('first answer')).toBe(1)
  })

  it('shows a chunk emitted while sendStudyMessage is still being called', async () => {
    // Given a resumed session whose send emits a chunk synchronously, before
    // the streaming turn has mounted and registered itself
    let chunk: ((event: StudyChunkEvent) => void) | undefined
    vi.mocked(onStudyChunk).mockImplementation((handler) => {
      chunk = handler
      return vi.fn()
    })
    for (const subscribe of [
      onStudyDone,
      onStudyError,
      onStudySources,
      onStudyContextNormal,
      onStudyContextWarning,
      onStudyContextLimitReached,
      onStudyContextLimitUnavailable,
    ]) {
      vi.mocked(subscribe as () => () => void).mockReturnValue(vi.fn())
    }
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-1',
        topic: 'Caches',
        folderId: 'folder-1',
        goal: '',
        startedAt: '2026-08-16T10:00:00Z',
        context: { state: 'normal', model: '', usedTokens: 0, contextLength: 0, estimated: false },
      },
      messages: [{ id: 'm1', role: 'user', content: 'earlier', createdAt: '', sources: [] }],
    })
    vi.mocked(sendStudyMessage).mockImplementation(() => {
      chunk?.({ sessionId: 'session-1', content: 'early chunk' })
      return new Promise(() => {})
    })
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        onStartNewSession={vi.fn()}
        startingNewSession={false}
      />,
    )
    await screen.findByText('earlier')

    // When the user sends a message
    fireEvent.change(screen.getByPlaceholderText('Type your answer...'), {
      target: { value: 'question' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Send' }))

    // Then the early chunk is shown without waiting for another one
    expect(await screen.findByText('early chunk')).toBeInTheDocument()
  })
})
