import React from 'react'
import { describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  compactStudySession,
  getStudySessionSummary,
  getStudySessionUsage,
  listMessagesBefore,
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
  requestOpeningTurn,
  resumeStudySession,
  sendStudyMessage,
  type StudyChunkEvent,
  type StudyCompactionFailedEvent,
  type StudyContextCompactedEvent,
  type StudyContextCompactingEvent,
  type StudyContextEvent,
  type StudyContextUnavailableEvent,
  type StudyContextUsage,
  type StudyDoneEvent,
  type StudyErrorEvent,
  type StudyMessagePage,
  type StudySourcesEvent,
} from '@/lib/study'
import { setSentinelVisible, setSentinelVisibleOnObserve } from '@/test/intersection-observer'
import StudyChatScreen from './StudyChatScreen'

// importOriginal keeps pure helpers (e.g. sourceLabel, used by
// LocalSourcesStrip) real.
vi.mock('@/lib/study', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/study')>()
  return {
    ...original,
    compactStudySession: vi.fn(),
    getStudySessionSummary: vi.fn(),
    getStudySessionUsage: vi.fn(),
    listMessagesBefore: vi.fn(),
    requestOpeningTurn: vi.fn(),
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
  }
})

const CONTEXT_NORMAL: StudyContextUsage = {
  state: 'normal',
  model: '',
  usedTokens: 0,
  contextLength: 0,
  estimated: false,
}

function studyErrorEvent(sessionId: string, message: string, code = ''): StudyErrorEvent {
  return { sessionId, message, code }
}

function setupSubscriptions() {
  const unsubscribe = {
    chunk: vi.fn(),
    done: vi.fn(),
    error: vi.fn(),
    sources: vi.fn(),
    contextNormal: vi.fn(),
    contextWarning: vi.fn(),
    contextLimitReached: vi.fn(),
    contextLimitUnavailable: vi.fn(),
    compacting: vi.fn(),
    compacted: vi.fn(),
    compactionFailed: vi.fn(),
  }
  const handlers: {
    chunk?: (event: StudyChunkEvent) => void
    done?: (event: StudyDoneEvent) => void
    error?: (event: StudyErrorEvent) => void
    sources?: (event: StudySourcesEvent) => void
    contextNormal?: (event: StudyContextEvent) => void
    contextWarning?: (event: StudyContextEvent) => void
    contextLimitReached?: (event: StudyContextEvent) => void
    contextLimitUnavailable?: (event: StudyContextUnavailableEvent) => void
    compacting?: (event: StudyContextCompactingEvent) => void
    compacted?: (event: StudyContextCompactedEvent) => void
    compactionFailed?: (event: StudyCompactionFailedEvent) => void
    unsubscribe: typeof unsubscribe
  } = { unsubscribe }
  vi.mocked(onStudyChunk).mockImplementation((handler) => {
    handlers.chunk = handler
    return unsubscribe.chunk
  })
  vi.mocked(onStudyDone).mockImplementation((handler) => {
    handlers.done = handler
    return unsubscribe.done
  })
  vi.mocked(onStudyError).mockImplementation((handler) => {
    handlers.error = handler
    return unsubscribe.error
  })
  vi.mocked(onStudySources).mockImplementation((handler) => {
    handlers.sources = handler
    return unsubscribe.sources
  })
  vi.mocked(onStudyContextNormal).mockImplementation((handler) => {
    handlers.contextNormal = handler
    return unsubscribe.contextNormal
  })
  vi.mocked(onStudyContextWarning).mockImplementation((handler) => {
    handlers.contextWarning = handler
    return unsubscribe.contextWarning
  })
  vi.mocked(onStudyContextLimitReached).mockImplementation((handler) => {
    handlers.contextLimitReached = handler
    return unsubscribe.contextLimitReached
  })
  vi.mocked(onStudyContextLimitUnavailable).mockImplementation((handler) => {
    handlers.contextLimitUnavailable = handler
    return unsubscribe.contextLimitUnavailable
  })
  vi.mocked(onStudyContextCompacting).mockImplementation((handler) => {
    handlers.compacting = handler
    return unsubscribe.compacting
  })
  vi.mocked(onStudyContextCompacted).mockImplementation((handler) => {
    handlers.compacted = handler
    return unsubscribe.compacted
  })
  vi.mocked(onStudyCompactionFailed).mockImplementation((handler) => {
    handlers.compactionFailed = handler
    return unsubscribe.compactionFailed
  })
  return handlers
}

function renderNewSession(props: Partial<React.ComponentProps<typeof StudyChatScreen>> = {}) {
  return render(
    <StudyChatScreen
      sessionId="session-1"
      initialTopic="Distributed systems"
      mode="new"
      onStartNewSession={vi.fn()}
      startingNewSession={false}
      {...props}
    />,
  )
}

// Spread into every direct <StudyChatScreen> render in this file so the two
// new required context-limit props don't need repeating at every call site.
function newSessionActionProps() {
  return { onStartNewSession: vi.fn(), startingNewSession: false }
}

async function renderStartedSession(
  props: Partial<React.ComponentProps<typeof StudyChatScreen>> = {},
) {
  const handlers = setupSubscriptions()
  vi.mocked(requestOpeningTurn).mockReturnValueOnce(new Promise(() => {}))
  renderNewSession(props)
  await screen.findByRole('status', { name: /thinking/i })
  return handlers
}

async function renderSettledSession(
  props: Partial<React.ComponentProps<typeof StudyChatScreen>> = {},
) {
  const handlers = setupSubscriptions()
  vi.mocked(requestOpeningTurn).mockResolvedValueOnce()
  renderNewSession(props)
  await screen.findByRole('status', { name: /thinking/i })
  act(() => {
    handlers.chunk?.({ sessionId: 'session-1', content: 'Welcome!' })
    handlers.done?.({ sessionId: 'session-1' })
  })
  await screen.findByText('Welcome!')
  return handlers
}

describe('StudyChatScreen — starting a new session', () => {
  it('requests the opening turn for the given session and topic', async () => {
    // Given a session that was just created elsewhere (the sidebar tree)
    setupSubscriptions()
    vi.mocked(requestOpeningTurn).mockReturnValueOnce(new Promise(() => {}))

    // When the chat screen mounts in "new" mode
    renderNewSession()

    // Then it requests the opening turn immediately
    await waitFor(() =>
      expect(requestOpeningTurn).toHaveBeenCalledWith('session-1', 'Distributed systems'),
    )
  })

  it('shows the thinking indicator while waiting for the opening turn to start streaming', async () => {
    await renderStartedSession()

    expect(screen.getByRole('status', { name: /thinking/i })).toBeInTheDocument()
  })

  it('replaces the thinking indicator with the streamed reply once the first chunk arrives', async () => {
    // Given a new session showing the thinking indicator
    const handlers = await renderStartedSession()

    // When the first chunk of the reply arrives
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'Welcome!' })
    })

    // Then the thinking indicator is gone and the streamed text is shown instead
    expect(screen.queryByRole('status', { name: /thinking/i })).not.toBeInTheDocument()
    expect(await screen.findByText('Welcome!')).toBeInTheDocument()
  })

  it('focuses the reply textarea as soon as the chat view mounts', async () => {
    await renderStartedSession()

    await waitFor(() => expect(screen.getByPlaceholderText(/type your answer/i)).toHaveFocus())
  })

  it('shows an inline error when requesting the opening turn fails', async () => {
    // Given a request that fails
    setupSubscriptions()
    vi.mocked(requestOpeningTurn).mockRejectedValueOnce(new Error('invalid OpenRouter key'))

    // When the chat screen mounts
    renderNewSession()

    // Then the error is shown, and streaming stops (the thinking indicator goes away)
    expect(await screen.findByText('invalid OpenRouter key')).toBeInTheDocument()
    expect(screen.queryByRole('status', { name: /thinking/i })).not.toBeInTheDocument()
  })

  it('falls back to a generic message when the opening turn fails with a non-Error value', async () => {
    // Given a request that rejects with something other than an Error
    setupSubscriptions()
    vi.mocked(requestOpeningTurn).mockRejectedValueOnce('network down')

    // When the chat screen mounts
    renderNewSession()

    // Then the generic fallback message is shown
    expect(await screen.findByText('Failed to start the session.')).toBeInTheDocument()
  })

  it('shows nothing but the thinking indicator before the first chunk arrives — no empty streamed bubble', async () => {
    // Given a new session that hasn't streamed anything yet
    await renderStartedSession()

    // Then only the thinking indicator is shown, no message bubble at all
    expect(screen.getByRole('status', { name: /thinking/i })).toBeInTheDocument()
    expect(document.querySelector('[data-slot="message-bubble"]')).not.toBeInTheDocument()
  })

  it('requests the opening turn only once even if effects run twice, like under StrictMode', async () => {
    // Given a session that was just created elsewhere
    setupSubscriptions()
    vi.mocked(requestOpeningTurn).mockReturnValueOnce(new Promise(() => {}))

    // When the chat screen mounts under StrictMode, which double-invokes
    // effects in dev (mount → cleanup → mount) to surface missing cleanup
    render(
      <React.StrictMode>
        <StudyChatScreen
          sessionId="session-1"
          initialTopic="Distributed systems"
          mode="new"
          {...newSessionActionProps()}
        />
      </React.StrictMode>,
    )

    // Then the opening turn is only requested once for this session
    await waitFor(() => expect(requestOpeningTurn).toHaveBeenCalledTimes(1))
  })

  it('accumulates streamed chunks and appends the assistant message once done', async () => {
    // Given a new session
    const handlers = await renderStartedSession()

    // When chunks stream in incrementally
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'Hello ' })
      handlers.chunk?.({ sessionId: 'session-1', content: 'there!' })
    })

    // Then the partial text is visible before the stream finishes
    expect(await screen.findByText('Hello there!')).toBeInTheDocument()

    // When the stream finishes
    act(() => {
      handlers.done?.({ sessionId: 'session-1' })
    })

    // Then the full message is shown as a settled assistant message
    const text = await screen.findByText('Hello there!')
    const bubble = text.closest('[data-slot="message-bubble"]')
    expect(bubble).toHaveAttribute('data-role', 'assistant')
    expect(bubble).toHaveClass('self-start')
  })
})

describe('StudyChatScreen — resuming a session', () => {
  it('loads the full history instead of requesting an opening turn', async () => {
    // Given an ended session with prior history
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-1',
        topic: 'Cache invalidation',
        folderId: 'folder-1',
        goal: 'Ace the SQL interview',
        startedAt: '2026-08-16T10:00:00Z',
        context: CONTEXT_NORMAL,
      },
      messages: [
        { id: 'm1', role: 'user', content: 'Hi', createdAt: '2026-08-16T10:00:00Z', sources: [] },
        {
          id: 'm2',
          role: 'assistant',
          content: 'Hello!',
          createdAt: '2026-08-16T10:00:01Z',
          sources: [],
        },
      ],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })

    // When the chat screen mounts in "resume" mode
    const onTopicResolved = vi.fn()
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        onTopicResolved={onTopicResolved}
        {...newSessionActionProps()}
      />,
    )

    // Then its history is loaded and shown, the real topic is reported to
    // the caller, and no opening turn is requested
    expect(await screen.findByText('Hi')).toBeInTheDocument()
    expect(screen.getByText('Hello!')).toBeInTheDocument()
    expect(onTopicResolved).toHaveBeenCalledWith('Cache invalidation')
    expect(requestOpeningTurn).not.toHaveBeenCalled()
  })

  it('shows the Local sources strip for a resumed message with persisted sources', async () => {
    // Given a resumed session whose assistant message already has sources
    // persisted from when it was first answered
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-1',
        topic: 'Cache invalidation',
        folderId: 'folder-1',
        goal: 'Ace the SQL interview',
        startedAt: '2026-08-16T10:00:00Z',
        context: CONTEXT_NORMAL,
      },
      messages: [
        {
          id: 'm3',
          role: 'user',
          content: 'What is CAP theorem?',
          createdAt: '2026-08-16T10:00:00Z',
          sources: [],
        },
        {
          id: 'm4',
          role: 'assistant',
          content: 'It stands for...',
          createdAt: '2026-08-16T10:00:01Z',
          sources: [
            {
              chunkId: 'chunk-1',
              itemId: 'item-1',
              sourceType: 'athena',
              filePath: '',
              heading: '',
              concept: 'CAP theorem',
              score: 0.9,
              excerpt: '',
            },
          ],
        },
      ],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })

    // When the chat screen mounts in "resume" mode
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )

    // Then the strip shows up without any live stream ever happening
    expect(await screen.findByText('It stands for...')).toBeInTheDocument()
    expect(screen.getByText('Local sources (1)')).toBeInTheDocument()
  })

  it('shows no Local sources strip for a resumed message with no persisted sources', async () => {
    // Given a resumed conversation created before sources were persisted
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-1',
        topic: 'Cache invalidation',
        folderId: 'folder-1',
        goal: 'Ace the SQL interview',
        startedAt: '2026-08-16T10:00:00Z',
        context: CONTEXT_NORMAL,
      },
      messages: [
        {
          id: 'm5',
          role: 'assistant',
          content: 'Hello!',
          createdAt: '2026-08-16T10:00:01Z',
          sources: [],
        },
      ],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })

    // When the chat screen mounts in "resume" mode
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )

    // Then the message shows with no strip, and no error
    expect(await screen.findByText('Hello!')).toBeInTheDocument()
    expect(screen.queryByText(/Local sources/)).not.toBeInTheDocument()
  })

  it('shows an inline error when resuming fails', async () => {
    // Given a resume call that fails
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockRejectedValueOnce(new Error('session not found'))

    // When the chat screen mounts in "resume" mode
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )

    // Then the error is shown
    expect(await screen.findByText('session not found')).toBeInTheDocument()
  })

  it('falls back to a generic message when resuming fails with a non-Error value', async () => {
    // Given a resume call that rejects with something other than an Error
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockRejectedValueOnce('network down')

    // When the chat screen mounts in "resume" mode
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )

    // Then the generic fallback message is shown
    expect(await screen.findByText('Failed to load the session.')).toBeInTheDocument()
  })

  it('does not show the thinking indicator while resuming, unlike starting a new session', async () => {
    // Given a resume call that hasn't settled yet
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockReturnValueOnce(new Promise(() => {}))

    // When the chat screen mounts in "resume" mode
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )

    // Then no thinking indicator shows — resuming isn't "streaming a new turn"
    // — and no message is rendered yet either, since the history hasn't
    // arrived
    expect(screen.queryByRole('status', { name: /thinking/i })).not.toBeInTheDocument()
    expect(document.querySelector('[data-slot="message-bubble"]')).not.toBeInTheDocument()
  })

  it('does not crash when resuming without an onTopicResolved callback', async () => {
    // Given a resume call that succeeds, and no onTopicResolved prop passed
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-1',
        topic: 'Cache invalidation',
        folderId: 'folder-1',
        goal: 'Ace the SQL interview',
        startedAt: '2026-08-16T10:00:00Z',
        context: CONTEXT_NORMAL,
      },
      messages: [
        { id: 'm6', role: 'user', content: 'Hi', createdAt: '2026-08-16T10:00:00Z', sources: [] },
      ],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })

    // When the chat screen mounts in "resume" mode
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )

    // Then it resolves normally instead of throwing
    expect(await screen.findByText('Hi')).toBeInTheDocument()
  })

  it('refetches the session history when the sessionId changes', async () => {
    // Given a resumed session already loaded
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-1',
        topic: 'Cache invalidation',
        folderId: 'folder-1',
        goal: 'Ace the SQL interview',
        startedAt: '2026-08-16T10:00:00Z',
        context: CONTEXT_NORMAL,
      },
      messages: [
        { id: 'm7', role: 'user', content: 'Hi', createdAt: '2026-08-16T10:00:00Z', sources: [] },
      ],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })
    const { rerender } = render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )
    await screen.findByText('Hi')

    // When a different session is resumed in its place (a fresh mounted
    // instance would be the normal case via AppShell's `key`, but the
    // effect's own sessionId dependency must still cover an in-place swap)
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-2',
        topic: 'Load balancing',
        folderId: 'folder-1',
        goal: 'Ace the SQL interview',
        startedAt: '2026-08-16T11:00:00Z',
        context: CONTEXT_NORMAL,
      },
      messages: [
        {
          id: 'm8',
          role: 'user',
          content: 'Hello again',
          createdAt: '2026-08-16T11:00:00Z',
          sources: [],
        },
      ],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })
    rerender(
      <StudyChatScreen
        sessionId="session-2"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )

    // Then the new session's history is fetched and shown
    expect(await screen.findByText('Hello again')).toBeInTheDocument()
    expect(resumeStudySession).toHaveBeenCalledWith('session-2')
  })
})

describe('StudyChatScreen — composing and sending', () => {
  it('grows the textarea to fit multi-line content, same as ChatGPT/Claude', async () => {
    await renderSettledSession()
    const user = userEvent.setup()
    const textarea = screen.getByPlaceholderText(/type your answer/i) as HTMLTextAreaElement
    // jsdom never computes real layout, so scrollHeight is always 0 — stub
    // it to the "content wrapped onto more lines" case being tested.
    Object.defineProperty(textarea, 'scrollHeight', { value: 140, configurable: true })

    // When typing a reply that spans multiple lines
    await user.type(textarea, 'First line{Shift>}{Enter}{/Shift}Second line')

    // Then the textarea grows to fit it
    expect(textarea.style.height).toBe('140px')
  })

  it('resets the textarea height back to its minimum after sending a reply', async () => {
    await renderSettledSession()
    vi.mocked(sendStudyMessage).mockResolvedValueOnce()
    const user = userEvent.setup()
    const textarea = screen.getByPlaceholderText(/type your answer/i) as HTMLTextAreaElement
    Object.defineProperty(textarea, 'scrollHeight', { value: 140, configurable: true })
    await user.type(textarea, 'First line{Shift>}{Enter}{/Shift}Second line')
    expect(textarea.style.height).toBe('140px')

    // When sending the reply
    await user.click(screen.getByRole('button', { name: 'Send' }))

    // Then the textarea collapses back to its default (CSS-driven) height
    expect(textarea.style.height).toBe('auto')
  })

  it('keeps the send button disabled for an empty or whitespace-only draft', async () => {
    await renderSettledSession()
    const user = userEvent.setup()

    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()

    await user.type(screen.getByPlaceholderText(/type your answer/i), '   ')
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
  })

  it('keeps the send button disabled while the opening turn is still streaming', async () => {
    await renderStartedSession()
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')

    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
  })

  it('sends a typed message and appends it immediately', async () => {
    await renderSettledSession()
    vi.mocked(sendStudyMessage).mockResolvedValueOnce()
    const user = userEvent.setup()

    // When typing and sending a reply
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    // Then the user message appears immediately, styled as the user's own
    // bubble, and sendStudyMessage was called
    const text = await screen.findByText('What is CAP theorem?')
    const bubble = text.closest('[data-slot="message-bubble"]')
    expect(bubble).toHaveAttribute('data-role', 'user')
    expect(bubble).toHaveClass('self-end')
    expect(sendStudyMessage).toHaveBeenCalledWith(
      'session-1',
      'Distributed systems',
      'What is CAP theorem?',
      'notes',
    )
  })

  it('sends the message on Enter, without inserting a newline', async () => {
    await renderSettledSession()
    vi.mocked(sendStudyMessage).mockResolvedValueOnce()
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?{Enter}')

    await screen.findByText('What is CAP theorem?')
    expect(sendStudyMessage).toHaveBeenCalledWith(
      'session-1',
      'Distributed systems',
      'What is CAP theorem?',
      'notes',
    )
    expect(screen.getByPlaceholderText(/type your answer/i)).toHaveValue('')
  })

  it('inserts a newline on Shift+Enter, without sending', async () => {
    await renderSettledSession()
    const user = userEvent.setup()
    const textarea = screen.getByPlaceholderText(/type your answer/i)

    await user.type(textarea, 'First line{Shift>}{Enter}{/Shift}Second line')

    expect(textarea).toHaveValue('First line\nSecond line')
    expect(sendStudyMessage).not.toHaveBeenCalled()
  })

  it('trims leading and trailing whitespace from the draft before sending', async () => {
    await renderSettledSession()
    vi.mocked(sendStudyMessage).mockResolvedValueOnce()
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText(/type your answer/i), '  What is CAP theorem?  ')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    expect(sendStudyMessage).toHaveBeenCalledWith(
      'session-1',
      'Distributed systems',
      'What is CAP theorem?',
      'notes',
    )
  })

  it('shows an inline error and re-enables the send button when sending fails', async () => {
    await renderSettledSession()
    vi.mocked(sendStudyMessage).mockRejectedValueOnce(new Error('upstream failure'))
    const user = userEvent.setup()

    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    expect(await screen.findByText('upstream failure')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Retry')
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
  })

  it('shows the thinking indicator immediately after sending a message', async () => {
    // Given a settled session and a reply that hasn't resolved yet
    await renderSettledSession()
    vi.mocked(sendStudyMessage).mockReturnValueOnce(new Promise(() => {}))
    const user = userEvent.setup()

    // When sending a message
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    // Then the thinking indicator shows while waiting for the reply
    expect(await screen.findByRole('status', { name: /thinking/i })).toBeInTheDocument()
  })

  it('does not send on Enter when the draft is empty', async () => {
    // Given a settled session with an empty draft
    await renderSettledSession()
    const user = userEvent.setup()

    // When pressing Enter with nothing typed
    await user.click(screen.getByPlaceholderText(/type your answer/i))
    await user.keyboard('{Enter}')

    // Then nothing is sent
    expect(sendStudyMessage).not.toHaveBeenCalled()
  })

  it('does not send on Enter when the draft is only whitespace', async () => {
    // Given a settled session
    await renderSettledSession()
    const user = userEvent.setup()

    // When pressing Enter after typing only spaces
    await user.type(screen.getByPlaceholderText(/type your answer/i), '   {Enter}')

    // Then nothing is sent
    expect(sendStudyMessage).not.toHaveBeenCalled()
  })

  it('does not send on Enter while a reply is already streaming', async () => {
    // Given a new session still streaming its opening turn
    await renderStartedSession()
    const user = userEvent.setup()

    // When typing a reply and pressing Enter before the stream settles
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.keyboard('{Enter}')

    // Then nothing is sent
    expect(sendStudyMessage).not.toHaveBeenCalled()
  })

  it('starts the next streamed reply from empty text, not leftover from a completed one', async () => {
    // Given a completed exchange
    const handlers = await renderSettledSession()
    vi.mocked(sendStudyMessage).mockResolvedValueOnce()
    const user = userEvent.setup()

    // When sending a follow-up, before any new chunk has arrived
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Another question')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    // Then no stale streamed content leaks in — just the thinking indicator
    // (if streamingText hadn't been reset to empty, it would render as a
    // stray message bubble instead of the thinking indicator)
    expect(screen.getByRole('status', { name: /thinking/i })).toBeInTheDocument()

    // And the new reply starts clean once chunks arrive
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'Fresh reply' })
    })
    expect(await screen.findByText('Fresh reply')).toBeInTheDocument()
  })

  it('starts the next streamed reply from empty text, not leftover from a failed one', async () => {
    // Given a stream that partially arrived, then failed
    const handlers = await renderStartedSession()
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'Partial before failure' })
      handlers.error?.(studyErrorEvent('session-1', 'upstream failure'))
    })
    await screen.findByText('upstream failure')
    vi.mocked(sendStudyMessage).mockResolvedValueOnce()
    const user = userEvent.setup()

    // When retrying with a new message
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Retry question')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'Fresh reply' })
    })

    // Then the new reply doesn't carry over the earlier partial text
    expect(await screen.findByText('Fresh reply')).toBeInTheDocument()
    expect(screen.queryByText(/Partial before failure/)).not.toBeInTheDocument()
  })

  it('does not show an error alert when there is no error', async () => {
    // Given a normal, settled session with no failures
    await renderSettledSession()

    // Then no error alert is rendered
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows an inline error when the stream fails', async () => {
    const handlers = await renderStartedSession()

    act(() => {
      handlers.error?.(studyErrorEvent('session-1', 'upstream failure'))
    })

    expect(await screen.findByText('upstream failure')).toBeInTheDocument()
    const user = userEvent.setup()
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Try again')
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
  })

  it('unsubscribes from every study event on unmount', async () => {
    const handlers = setupSubscriptions()
    vi.mocked(requestOpeningTurn).mockReturnValueOnce(new Promise(() => {}))
    const { unmount } = renderNewSession()
    await screen.findByRole('status', { name: /thinking/i })

    unmount()

    expect(handlers.unsubscribe.chunk).toHaveBeenCalledOnce()
    expect(handlers.unsubscribe.done).toHaveBeenCalledOnce()
    expect(handlers.unsubscribe.error).toHaveBeenCalledOnce()
    expect(handlers.unsubscribe.sources).toHaveBeenCalledOnce()
  })
})

describe('StudyChatScreen — composer actions', () => {
  it('offers no knowledge extraction action, since documents are the only source of knowledge', async () => {
    // Given a settled session with messages
    await renderSettledSession()

    // Then the composer only offers sending a message
    expect(screen.queryByRole('button', { name: /extract knowledge/i })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Send' })).toBeInTheDocument()
  })
})

// jsdom has no layout engine: scrollHeight/clientHeight are always 0 and
// scrollTop is a read-only 0. Backing all three with plain values is what
// makes the "is the user at the bottom?" arithmetic observable in a test.
function stubScrollMetrics(element: HTMLElement, scrollHeight: number, clientHeight: number) {
  let scrollTop = 0
  Object.defineProperty(element, 'scrollHeight', { value: scrollHeight, configurable: true })
  Object.defineProperty(element, 'clientHeight', { value: clientHeight, configurable: true })
  Object.defineProperty(element, 'scrollTop', {
    configurable: true,
    get: () => scrollTop,
    set: (value: number) => {
      scrollTop = value
    },
  })
}

describe('StudyChatScreen — transcript scrolling', () => {
  it('follows the streaming answer while the user is parked at the bottom', async () => {
    // Given a transcript taller than its viewport, scrolled to the bottom
    const handlers = await renderStartedSession()
    const transcript = screen.getByRole('log', { name: 'Conversation' })
    stubScrollMetrics(transcript, 1000, 400)

    // When the next chunk of the answer streams in
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'More of the answer' })
    })

    // Then the transcript scrolls to keep it in view
    await waitFor(() => expect(transcript.scrollTop).toBe(1000))
  })

  it('keeps following the stream when scrolled within the sticky-bottom threshold', async () => {
    // Given a transcript scrolled to 50px from the bottom (under the 80px threshold)
    const handlers = await renderStartedSession()
    const transcript = screen.getByRole('log', { name: 'Conversation' })
    stubScrollMetrics(transcript, 1000, 400)
    transcript.scrollTop = 550
    fireEvent.scroll(transcript)

    // When the next chunk of the answer streams in
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'More of the answer' })
    })

    // Then the transcript still auto-scrolls to keep it in view
    await waitFor(() => expect(transcript.scrollTop).toBe(1000))
  })

  it('follows the stream when scrolled exactly at the sticky-bottom threshold', async () => {
    // Given a transcript scrolled to exactly 80px from the bottom
    const handlers = await renderStartedSession()
    const transcript = screen.getByRole('log', { name: 'Conversation' })
    stubScrollMetrics(transcript, 1080, 400)
    transcript.scrollTop = 600
    fireEvent.scroll(transcript)

    // When the next chunk of the answer streams in
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'More of the answer' })
    })

    // Then the boundary counts as "still following" and it auto-scrolls
    await waitFor(() => expect(transcript.scrollTop).toBe(1080))
  })

  it('leaves the scroll position alone once the user scrolls up to re-read', async () => {
    // Given a user who scrolled back up to an earlier explanation
    const handlers = await renderStartedSession()
    const transcript = screen.getByRole('log', { name: 'Conversation' })
    stubScrollMetrics(transcript, 1000, 400)
    transcript.scrollTop = 0
    fireEvent.scroll(transcript)

    // When the next chunk of the answer streams in
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'More of the answer' })
    })

    // Then the transcript stays where they left it
    await screen.findByText('More of the answer')
    expect(transcript.scrollTop).toBe(0)
  })
})

describe('StudyChatScreen — navigation', () => {
  it('shows a tooltip explaining the send icon button', async () => {
    await renderSettledSession()
    const user = userEvent.setup()

    await user.hover(screen.getByRole('button', { name: 'Send' }))
    expect(await screen.findByText('Send message')).toBeInTheDocument()
  })
})

describe('StudyChatScreen — source modes and local sources', () => {
  it('renders the source-mode selector defaulted to Notes', async () => {
    await renderSettledSession()

    expect(screen.getByRole('combobox', { name: 'Source mode' })).toHaveTextContent('Notes')
  })

  it('resets to Notes when a different session mounts', async () => {
    // Given a settled session with Strict notes picked
    await renderSettledSession()
    const user = userEvent.setup()
    await user.click(screen.getByRole('combobox', { name: 'Source mode' }))
    await user.click(within(screen.getByRole('listbox')).getByText('Strict notes'))
    expect(screen.getByRole('combobox', { name: 'Source mode' })).toHaveTextContent('Strict notes')

    // When a different session mounts as a fresh instance (AppShell keys
    // StudyChatScreen by session id, so every session gets its own mount)
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-2',
        topic: 'Load balancing',
        folderId: 'folder-1',
        goal: 'Ace the SQL interview',
        startedAt: '2026-08-16T11:00:00Z',
        context: CONTEXT_NORMAL,
      },
      messages: [],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })
    render(
      <StudyChatScreen
        sessionId="session-2"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )

    // Then the new instance defaults back to Notes
    const selects = screen.getAllByRole('combobox', { name: 'Source mode' })
    expect(selects[selects.length - 1]).toHaveTextContent('Notes')
  })

  it('disables the mode selector while a response is streaming', async () => {
    await renderStartedSession()

    expect(screen.getByRole('combobox', { name: 'Source mode' })).toBeDisabled()
  })

  it('enables the mode selector once streaming settles', async () => {
    await renderSettledSession()

    expect(screen.getByRole('combobox', { name: 'Source mode' })).toBeEnabled()
  })

  it('sends the chosen source mode', async () => {
    await renderSettledSession()
    vi.mocked(sendStudyMessage).mockResolvedValueOnce()
    const user = userEvent.setup()
    await user.click(screen.getByRole('combobox', { name: 'Source mode' }))
    await user.click(within(screen.getByRole('listbox')).getByText('Strict notes'))

    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    expect(sendStudyMessage).toHaveBeenCalledWith(
      'session-1',
      'Distributed systems',
      'What is CAP theorem?',
      'strict-notes',
    )
  })

  it('attaches post-cap sources to the completed assistant message only, not mid-stream', async () => {
    // Given a new session about to stream a reply
    const handlers = await renderStartedSession()
    const sources = [
      {
        chunkId: 'chunk-1',
        itemId: 'item-1',
        sourceType: 'imported_doc',
        filePath: 'notes/a.md',
        heading: 'CAP theorem',
        concept: '',
        score: 0.68,
        excerpt: '',
      },
    ]

    // When sources arrive, then chunks stream, before the turn completes
    act(() => {
      handlers.sources?.({ sessionId: 'session-1', sources })
      handlers.chunk?.({ sessionId: 'session-1', content: 'It stands for...' })
    })

    // Then no strip is shown yet, while the reply is still streaming
    await screen.findByText('It stands for...')
    expect(screen.queryByText(/Local sources/)).not.toBeInTheDocument()

    // When the turn completes
    act(() => {
      handlers.done?.({ sessionId: 'session-1' })
    })

    // Then the strip appears under the now-completed assistant message
    expect(await screen.findByText('Local sources (1)')).toBeInTheDocument()
  })

  it('renders no strip when a completed message has no sources', async () => {
    const handlers = await renderStartedSession()

    act(() => {
      handlers.sources?.({ sessionId: 'session-1', sources: [] })
      handlers.chunk?.({ sessionId: 'session-1', content: 'Hello!' })
      handlers.done?.({ sessionId: 'session-1' })
    })

    await screen.findByText('Hello!')
    expect(screen.queryByText(/Local sources/)).not.toBeInTheDocument()
  })

  it('expands the collapsed strip to show each source', async () => {
    const handlers = await renderStartedSession()
    const sources = [
      {
        chunkId: 'chunk-1',
        itemId: 'item-1',
        sourceType: 'imported_doc',
        filePath: 'notes/a.md',
        heading: 'CAP theorem',
        concept: '',
        score: 0.68,
        excerpt: '',
      },
    ]

    act(() => {
      handlers.sources?.({ sessionId: 'session-1', sources })
      handlers.chunk?.({ sessionId: 'session-1', content: 'It stands for...' })
      handlers.done?.({ sessionId: 'session-1' })
    })
    const strip = await screen.findByText('Local sources (1)')
    const user = userEvent.setup()

    await user.click(strip)

    expect(screen.getByText(/notes\/a\.md/)).toBeInTheDocument()
  })

  it('ignores study:chunk/done/error/sources events whose sessionId does not match the displayed session', async () => {
    const handlers = await renderStartedSession()

    act(() => {
      handlers.sources?.({
        sessionId: 'another-session',
        sources: [
          {
            chunkId: 'chunk-1',
            itemId: 'item-1',
            sourceType: 'imported_doc',
            filePath: 'x.md',
            heading: 'H',
            concept: '',
            score: 0.9,
            excerpt: '',
          },
        ],
      })
      handlers.chunk?.({ sessionId: 'another-session', content: 'Should not appear' })
      handlers.done?.({ sessionId: 'another-session' })
      handlers.error?.(studyErrorEvent('another-session', 'Should not appear either'))
    })

    // Then nothing from the foreign session affected this screen's state —
    // still just the thinking indicator, no stray message or error
    expect(screen.getByRole('status', { name: /thinking/i })).toBeInTheDocument()
    expect(screen.queryByText('Should not appear')).not.toBeInTheDocument()
    expect(screen.queryByText('Should not appear either')).not.toBeInTheDocument()
  })

  it('never renders a sources strip for the opening turn', async () => {
    const handlers = await renderStartedSession()

    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'Welcome!' })
      handlers.done?.({ sessionId: 'session-1' })
    })

    await screen.findByText('Welcome!')
    expect(screen.queryByText(/Local sources/)).not.toBeInTheDocument()
  })

  it('does not resolve a citation chip during the opening turn, which never announces sources', async () => {
    // Given a new session streaming its opening turn — which never emits
    // study:sources at all
    const handlers = await renderStartedSession()

    // When a chunk citing [1] streams in, with no sources ever announced
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'Welcome [1]!' })
    })

    // Then the marker stays plain text — there is no source at index 0 to
    // resolve it against
    await screen.findByText(/\[1\]/)
    expect(screen.queryByRole('button', { name: '1' })).not.toBeInTheDocument()
  })

  it("clears the prior turn's sources so a later turn's citation does not resolve before its own sources are announced", async () => {
    // Given a completed turn that cited its one source
    const handlers = await renderStartedSession()
    const user = userEvent.setup()
    const source = {
      chunkId: 'chunk-1',
      itemId: 'item-1',
      sourceType: 'imported_doc',
      filePath: 'notes/a.md',
      heading: 'CAP theorem',
      concept: '',
      score: 0.68,
      excerpt: '',
    }
    act(() => {
      handlers.sources?.({ sessionId: 'session-1', sources: [source] })
      handlers.chunk?.({ sessionId: 'session-1', content: 'It stands for [1].' })
      handlers.done?.({ sessionId: 'session-1' })
    })
    await screen.findByRole('button', { name: '1' })

    // When a follow-up message starts streaming, before its own sources
    // (if any) are announced
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Another question')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'Partial answer [1]' })
    })

    // Then the new streaming bubble's [1] stays plain text — the previous
    // turn's sources were cleared, not carried over
    const streamingBubble = await screen.findByText(/Partial answer/)
    expect(
      within(streamingBubble.closest('[data-slot="message-bubble"]')!).getByText(/\[1\]/),
    ).toBeInTheDocument()
    expect(
      within(streamingBubble.closest('[data-slot="message-bubble"]')!).queryByRole('button', {
        name: '1',
      }),
    ).not.toBeInTheDocument()
  })

  it('resolves a citation chip mid-stream, once its source is announced', async () => {
    // Given a new session whose sources arrived before the turn completed
    const handlers = await renderStartedSession()
    const sources = [
      {
        chunkId: 'chunk-1',
        itemId: 'item-1',
        sourceType: 'imported_doc',
        filePath: 'notes/a.md',
        heading: 'CAP theorem',
        concept: '',
        score: 0.68,
        excerpt: '',
      },
    ]

    // When sources arrive, then a chunk citing [1] streams in, before
    // study:done
    act(() => {
      handlers.sources?.({ sessionId: 'session-1', sources })
      handlers.chunk?.({ sessionId: 'session-1', content: 'It stands for [1].' })
    })

    // Then the chip already resolves against the announced source
    expect(await screen.findByRole('button', { name: '1' })).toHaveAttribute(
      'data-slot',
      'citation-chip',
    )
  })

  it('calls onOpenCitation when a citation chip in a completed message is clicked', async () => {
    // Given a completed assistant message citing its one source
    const onOpenCitation = vi.fn()
    const handlers = await renderStartedSession({ onOpenCitation })
    const source = {
      chunkId: 'chunk-1',
      itemId: 'item-1',
      sourceType: 'imported_doc',
      filePath: 'notes/a.md',
      heading: 'CAP theorem',
      concept: '',
      score: 0.68,
      excerpt: '',
    }
    act(() => {
      handlers.sources?.({ sessionId: 'session-1', sources: [source] })
      handlers.chunk?.({ sessionId: 'session-1', content: 'It stands for [1].' })
      handlers.done?.({ sessionId: 'session-1' })
    })
    const user = userEvent.setup()

    // When clicking the chip
    await user.click(await screen.findByRole('button', { name: '1' }))

    // Then onOpenCitation is called with that exact source
    expect(onOpenCitation).toHaveBeenCalledWith(source)
  })

  it('calls onOpenCitation when a Local-sources entry is clicked', async () => {
    // Given a completed assistant message with one local source
    const onOpenCitation = vi.fn()
    const handlers = await renderStartedSession({ onOpenCitation })
    const source = {
      chunkId: 'chunk-1',
      itemId: 'item-1',
      sourceType: 'imported_doc',
      filePath: 'notes/a.md',
      heading: 'CAP theorem',
      concept: '',
      score: 0.68,
      excerpt: '',
    }
    act(() => {
      handlers.sources?.({ sessionId: 'session-1', sources: [source] })
      handlers.chunk?.({ sessionId: 'session-1', content: 'It stands for...' })
      handlers.done?.({ sessionId: 'session-1' })
    })
    const strip = await screen.findByText('Local sources (1)')
    const user = userEvent.setup()
    await user.click(strip)

    // When clicking the entry
    await user.click(screen.getByText(/notes\/a\.md/))

    // Then onOpenCitation is called with that exact source
    expect(onOpenCitation).toHaveBeenCalledWith(source)
  })
})

describe('StudyChatScreen — context limits', () => {
  it('hydrates the persistent state from a resumed session already at warning', async () => {
    // Given a resumed session already measured into the warning state
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-1',
        topic: 'Distributed systems',
        folderId: 'folder-1',
        goal: 'Ace the SQL interview',
        startedAt: '2026-08-16T10:00:00Z',
        context: {
          state: 'warning',
          model: 'anthropic/claude',
          usedTokens: 8000,
          contextLength: 10000,
          estimated: false,
        },
      },
      messages: [],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })

    // When the chat screen mounts in "resume" mode
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )

    // Then the persistent warning banner renders immediately, with no event
    // needed
    expect(await screen.findByText(/approaching the model's context limit/)).toBeInTheDocument()
  })

  it('shows the persistent warning banner and enables sending on study:context-warning', async () => {
    const handlers = await renderSettledSession()

    act(() => {
      handlers.contextWarning?.({
        sessionId: 'session-1',
        usedTokens: 8000,
        contextLength: 10000,
        estimated: false,
      })
    })

    expect(await screen.findByText(/approaching the model's context limit/)).toBeInTheDocument()
    // The button stays disabled only because the draft is empty, not
    // because of the warning state — typing re-enables it.
    const user = userEvent.setup()
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'x')
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
  })

  it('keeps the composer usable on study:context-limit-reached and offers to retry the compaction', async () => {
    const handlers = await renderSettledSession()
    const user = userEvent.setup()

    act(() => {
      handlers.contextLimitReached?.({
        sessionId: 'session-1',
        usedTokens: 9500,
        contextLength: 10000,
        estimated: false,
      })
    })

    // The alert asks to retry; it no longer says the session is over
    expect(
      await screen.findByText('Could not compact the conversation. Try again.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Try to compact again' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Start new session' })).toBeEnabled()

    // Sending is the recovery trigger, so the composer stays fully usable
    const textarea = screen.getByPlaceholderText(/type your answer/i)
    expect(textarea).not.toHaveAttribute('readonly')
    expect(screen.getByRole('combobox', { name: 'Source mode' })).toBeEnabled()
    vi.mocked(sendStudyMessage).mockReturnValueOnce(new Promise(() => {}))
    await user.type(textarea, 'What is CAP theorem?')
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
    await user.keyboard('{Enter}')
    expect(sendStudyMessage).toHaveBeenCalledWith(
      'session-1',
      'Distributed systems',
      'What is CAP theorem?',
      'notes',
    )
  })

  it('shows the warning banner in orange without a "Start new session" button', async () => {
    const handlers = await renderSettledSession()

    act(() => {
      handlers.contextWarning?.({
        sessionId: 'session-1',
        usedTokens: 8000,
        contextLength: 10000,
        estimated: false,
      })
    })

    const message = await screen.findByText(/approaching the model's context limit/)
    expect(message).toHaveClass('text-orange-600')
    expect(screen.queryByRole('button', { name: 'Start new session' })).not.toBeInTheDocument()
  })

  it('disables the "Start new session" button while startingNewSession is true', async () => {
    const handlers = setupSubscriptions()
    vi.mocked(requestOpeningTurn).mockResolvedValueOnce()
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic="Distributed systems"
        mode="new"
        onStartNewSession={vi.fn()}
        startingNewSession
      />,
    )
    await screen.findByRole('status', { name: /thinking/i })
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'Welcome!' })
      handlers.done?.({ sessionId: 'session-1' })
      handlers.contextLimitReached?.({
        sessionId: 'session-1',
        usedTokens: 9500,
        contextLength: 10000,
        estimated: false,
      })
    })

    expect(await screen.findByRole('button', { name: 'Start new session' })).toBeDisabled()
  })

  it('shows the unavailable notice at most once per mount, and clears it on a resolved state event', async () => {
    const handlers = await renderSettledSession()

    act(() => {
      handlers.contextLimitUnavailable?.({
        sessionId: 'session-1',
        message: "Unable to determine this session's context limit.",
      })
    })
    expect(
      await screen.findByText("Unable to determine this session's context limit."),
    ).toBeInTheDocument()

    // A second unavailable event while still shown does not duplicate it
    act(() => {
      handlers.contextLimitUnavailable?.({
        sessionId: 'session-1',
        message: "Unable to determine this session's context limit.",
      })
    })
    expect(screen.getAllByText("Unable to determine this session's context limit.")).toHaveLength(1)

    // A later resolved state event clears it
    act(() => {
      handlers.contextNormal?.({
        sessionId: 'session-1',
        usedTokens: 100,
        contextLength: 10000,
        estimated: false,
      })
    })
    await waitFor(() =>
      expect(
        screen.queryByText("Unable to determine this session's context limit."),
      ).not.toBeInTheDocument(),
    )
  })

  it('dismisses the unavailable notice via its close button', async () => {
    const handlers = await renderSettledSession()
    act(() => {
      handlers.contextLimitUnavailable?.({
        sessionId: 'session-1',
        message: "Unable to determine this session's context limit.",
      })
    })
    await screen.findByText("Unable to determine this session's context limit.")
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: 'Dismiss' }))

    expect(
      screen.queryByText("Unable to determine this session's context limit."),
    ).not.toBeInTheDocument()
  })

  it('removes the optimistic user message and restores the draft on a context_limit_reached error', async () => {
    const handlers = await renderSettledSession()
    vi.mocked(sendStudyMessage).mockReturnValueOnce(new Promise(() => {}))
    const user = userEvent.setup()
    const transcript = screen.getByRole('log', { name: 'Conversation' })

    // When sending, then the backend rejects it as blocked before persisting
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    expect(await within(transcript).findByText('What is CAP theorem?')).toBeInTheDocument()

    act(() => {
      handlers.error?.(studyErrorEvent('session-1', 'blocked', 'context_limit_reached'))
    })

    // Then the optimistic bubble is gone from the transcript and the draft
    // is restored into the textarea (checking the transcript specifically,
    // not the whole document — the restored draft's own text also lives in
    // the textarea, which a page-wide query would otherwise match too)
    await waitFor(() =>
      expect(within(transcript).queryByText('What is CAP theorem?')).not.toBeInTheDocument(),
    )
    expect(screen.getByPlaceholderText(/type your answer/i)).toHaveValue('What is CAP theorem?')
  })

  it('removes the optimistic user message on a turn_in_progress error', async () => {
    const handlers = await renderSettledSession()
    vi.mocked(sendStudyMessage).mockReturnValueOnce(new Promise(() => {}))
    const user = userEvent.setup()
    const transcript = screen.getByRole('log', { name: 'Conversation' })

    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Another question')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    expect(await within(transcript).findByText('Another question')).toBeInTheDocument()

    act(() => {
      handlers.error?.(studyErrorEvent('session-1', 'in progress', 'turn_in_progress'))
    })

    await waitFor(() =>
      expect(within(transcript).queryByText('Another question')).not.toBeInTheDocument(),
    )
    expect(screen.getByPlaceholderText(/type your answer/i)).toHaveValue('Another question')
  })

  it('keeps the optimistic message for a plain (non-coded) error, unlike a pre-persistence one', async () => {
    const handlers = await renderSettledSession()
    vi.mocked(sendStudyMessage).mockReturnValueOnce(new Promise(() => {}))
    const user = userEvent.setup()
    const transcript = screen.getByRole('log', { name: 'Conversation' })

    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    expect(await within(transcript).findByText('What is CAP theorem?')).toBeInTheDocument()

    act(() => {
      handlers.error?.(studyErrorEvent('session-1', 'retrieval failed'))
    })

    // Then the bubble stays — the user message was already persisted before
    // this kind of failure, so it's not reconciled away
    expect(await screen.findByText('retrieval failed')).toBeInTheDocument()
    expect(within(transcript).getByText('What is CAP theorem?')).toBeInTheDocument()
  })
})

describe('StudyChatScreen — session usage visibility', () => {
  const resumedHistory = {
    session: {
      id: 'session-1',
      topic: 'Distributed systems',
      folderId: 'folder-1',
      goal: 'Ace the SQL interview',
      startedAt: '2026-08-16T10:00:00Z',
      context: {
        state: 'normal' as const,
        model: 'anthropic/claude',
        usedTokens: 5000,
        contextLength: 100000,
        estimated: true,
      },
    },
    messages: [],
    hasMore: false,
    nextCursor: '',
    summaryCoversUntilMessageId: '',
  }
  const totals = {
    inputTokens: 8000,
    outputTokens: 2000,
    cost: 0.5,
    unpricedCalls: 0,
    calls: 2,
    context: resumedHistory.session.context,
  }

  function renderResumed() {
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )
  }

  it('shows the persisted context and the accumulated usage on resume', async () => {
    // Given a resumed session with persisted context and recorded usage
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce(resumedHistory)
    vi.mocked(getStudySessionUsage).mockResolvedValueOnce(totals)

    // When the chat screen mounts in "resume" mode
    renderResumed()

    // Then both metrics render, kept apart
    expect(await screen.findByText('~5k / 100k tokens · 5%')).toBeInTheDocument()
    expect(await screen.findByText('10k tokens · US$ 0.50')).toBeInTheDocument()
    expect(getStudySessionUsage).toHaveBeenCalledWith('session-1')
  })

  it('refreshes the meter and the totals after a completed turn without reopening', async () => {
    // Given a resumed session
    const handlers = setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce(resumedHistory)
    vi.mocked(getStudySessionUsage).mockResolvedValueOnce(totals)
    renderResumed()
    await screen.findByText('10k tokens · US$ 0.50')

    // When a turn completes with a new context measurement and new totals
    // (the state stays "normal", so the backend emits no study:context-*
    // event — the meter must refresh from the fetched snapshot alone)
    vi.mocked(getStudySessionUsage).mockResolvedValueOnce({
      ...totals,
      inputTokens: 18000,
      cost: 0.75,
      calls: 4,
      context: { ...totals.context, usedTokens: 20000, estimated: false },
    })
    act(() => {
      handlers.done?.({ sessionId: 'session-1' })
    })

    // Then the meter and the totals both update
    expect(await screen.findByText('20k / 100k tokens · 20%')).toBeInTheDocument()
    expect(await screen.findByText('20k tokens · US$ 0.75')).toBeInTheDocument()
  })

  it('refreshes the totals after a failed turn, which may have recorded embedding calls', async () => {
    // Given a resumed session
    const handlers = setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce(resumedHistory)
    vi.mocked(getStudySessionUsage).mockResolvedValueOnce(totals)
    renderResumed()
    await screen.findByText('10k tokens · US$ 0.50')

    // When a turn fails after its embedding call was recorded
    vi.mocked(getStudySessionUsage).mockResolvedValueOnce({ ...totals, cost: 0.51, calls: 3 })
    act(() => {
      handlers.error?.(studyErrorEvent('session-1', 'stream failed'))
    })

    // Then the totals are fetched again
    expect(await screen.findByText('10k tokens · US$ 0.51')).toBeInTheDocument()
    expect(getStudySessionUsage).toHaveBeenCalledTimes(2)
  })

  it('does not let a stale usage snapshot overwrite a newer context event', async () => {
    // Given a resumed session whose post-turn usage fetch is still pending
    const handlers = setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce(resumedHistory)
    vi.mocked(getStudySessionUsage).mockResolvedValueOnce(totals)
    renderResumed()
    await screen.findByText('10k tokens · US$ 0.50')
    let resolveStale: (value: typeof totals) => void = () => {}
    vi.mocked(getStudySessionUsage).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveStale = resolve
      }),
    )
    act(() => {
      handlers.done?.({ sessionId: 'session-1' })
    })

    // When a context event arrives, then the older snapshot resolves
    act(() => {
      handlers.contextLimitReached?.({
        sessionId: 'session-1',
        usedTokens: 9500,
        contextLength: 10000,
        estimated: false,
      })
    })
    await act(async () => {
      resolveStale({ ...totals, cost: 0.9 })
    })

    // Then the blocked state stays, while the fresher totals are still applied
    expect(screen.getByText(/Could not compact the conversation/)).toBeInTheDocument()
    expect(screen.getByText('10k tokens · US$ 0.90')).toBeInTheDocument()
  })

  it('keeps the chat working and shows a dash when the usage fetch fails', async () => {
    // Given a usage fetch that fails
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce(resumedHistory)
    vi.mocked(getStudySessionUsage).mockRejectedValueOnce(new Error('db down'))

    // When the chat screen mounts in "resume" mode
    renderResumed()

    // Then the context meter still renders and no error banner appears
    expect(await screen.findByText('~5k / 100k tokens · 5%')).toBeInTheDocument()
    expect(screen.queryByText('db down')).not.toBeInTheDocument()
  })

  it('shows text only, with no bar, for a resumed session whose limit is unresolved', async () => {
    // Given a resumed session with tokens but no context length
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      ...resumedHistory,
      session: {
        ...resumedHistory.session,
        context: { ...resumedHistory.session.context, contextLength: 0 },
      },
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })
    vi.mocked(getStudySessionUsage).mockResolvedValueOnce({
      ...totals,
      calls: 0,
      context: { ...totals.context, contextLength: 0 },
    })

    // When the chat screen mounts in "resume" mode
    renderResumed()

    // Then no percentage or bar is shown
    expect(await screen.findByText('~5k tokens · limit unknown')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })
})

describe('StudyChatScreen — context compaction', () => {
  const blocked = {
    sessionId: 'session-1',
    usedTokens: 9500,
    contextLength: 10000,
    estimated: false,
  }

  it('retries the compaction from the blocked alert and reflects progress while it runs', async () => {
    // Given a blocked session and a compaction that stays in flight
    const handlers = await renderSettledSession()
    let finish: () => void = () => {}
    vi.mocked(compactStudySession).mockReturnValueOnce(
      new Promise<void>((resolve) => {
        finish = resolve
      }),
    )
    act(() => handlers.contextLimitReached?.(blocked))
    const user = userEvent.setup()

    // When retrying
    await user.click(await screen.findByRole('button', { name: 'Try to compact again' }))
    expect(compactStudySession).toHaveBeenCalledWith('session-1')
    act(() => handlers.compacting?.({ sessionId: 'session-1' }))

    // Then the action is disabled and says it is working
    expect(await screen.findByRole('button', { name: 'Compacting…' })).toBeDisabled()

    // And when it succeeds the blocked alert goes away and the notice appears
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)
    act(() => {
      handlers.compacted?.({ sessionId: 'session-1', usedTokens: 4000, contextLength: 10000 })
      finish()
    })
    expect(screen.queryByText(/Could not compact the conversation/)).not.toBeInTheDocument()
    expect(
      await screen.findByText('Older messages were summarized to keep the conversation going'),
    ).toBeInTheDocument()
  })

  it('clears the warning alert when the conversation is compacted', async () => {
    // Given a session in the warning state
    const handlers = await renderSettledSession()
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)
    act(() =>
      handlers.contextWarning?.({
        sessionId: 'session-1',
        usedTokens: 8000,
        contextLength: 10000,
        estimated: false,
      }),
    )
    expect(await screen.findByText(/approaching the model's context limit/)).toBeInTheDocument()

    // When it is compacted
    act(() =>
      handlers.compacted?.({ sessionId: 'session-1', usedTokens: 4000, contextLength: 10000 }),
    )

    // Then the warning is gone and the success notice is a status, not an alert
    expect(screen.queryByText(/approaching the model's context limit/)).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Older messages were summarized')
  })

  it('shows a top-center warning toast with the reason when a compaction fails below the limit', async () => {
    // Given a normal session
    const handlers = await renderSettledSession()

    // When a compaction fails
    act(() => handlers.compactionFailed?.({ sessionId: 'session-1', reason: 'invalid format' }))

    // Then an alert toast explains it and says the turn continues with the full history
    const toast = await screen.findByRole('alert')
    expect(toast).toHaveTextContent(
      'Could not summarize the conversation (invalid format). Continuing with the full history.',
    )
    expect(screen.getByRole('region', { name: /notifications/i })).toHaveClass('top-4')
  })

  it('does not promise to continue when the failure leaves the session blocked', async () => {
    // Given a blocked session
    const handlers = await renderSettledSession()
    act(() => handlers.contextLimitReached?.(blocked))
    await screen.findByText('Could not compact the conversation. Try again.')

    // When the compaction fails
    act(() =>
      handlers.compactionFailed?.({ sessionId: 'session-1', reason: 'provider unavailable' }),
    )

    // Then the toast has no "continuing" promise and the alert names the reason
    const toast = await screen.findByText(
      'Could not summarize the conversation (provider unavailable).',
    )
    expect(toast).not.toHaveTextContent('Continuing')
    expect(
      screen.getByText('Could not compact the conversation (provider unavailable). Try again.'),
    ).toBeInTheDocument()
  })

  it('never shows the raw error text from a failure event', async () => {
    // Given a failure event
    const handlers = await renderSettledSession()

    // When it arrives
    act(() => handlers.compactionFailed?.({ sessionId: 'session-1', reason: 'summary too large' }))

    // Then only the short reason is visible
    expect(await screen.findByRole('alert')).toHaveTextContent('(summary too large)')
  })

  it('shows "Summarizing earlier messages…" instead of the thinking indicator while compacting a send', async () => {
    // Given a send in flight
    const handlers = await renderSettledSession()
    vi.mocked(sendStudyMessage).mockReturnValueOnce(new Promise(() => {}))
    const user = userEvent.setup()
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    expect(await screen.findByRole('status', { name: /thinking/i })).toBeInTheDocument()

    // When the backend starts compacting
    act(() => handlers.compacting?.({ sessionId: 'session-1' }))

    // Then the indicator is captioned with the compaction step
    expect(
      await screen.findByRole('status', { name: 'Summarizing earlier messages…' }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('status', { name: /thinking/i })).not.toBeInTheDocument()
  })

  it('ignores compaction events of another session', async () => {
    // Given a normal session
    const handlers = await renderSettledSession()

    // When events for a different session arrive
    act(() => {
      handlers.compacting?.({ sessionId: 'other' })
      handlers.compacted?.({ sessionId: 'other', usedTokens: 1, contextLength: 2 })
      handlers.compactionFailed?.({ sessionId: 'other', reason: 'invalid format' })
    })

    // Then nothing is shown
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByText(/Older messages were summarized/)).not.toBeInTheDocument()
  })

  it('toasts the rejection when a retry fails without a failure event', async () => {
    // Given a blocked session whose retry is rejected as "nothing to compact"
    const handlers = await renderSettledSession()
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)
    vi.mocked(compactStudySession).mockRejectedValueOnce(new Error('study: nothing to compact'))
    act(() => handlers.contextLimitReached?.(blocked))
    const user = userEvent.setup()

    // When retrying
    await user.click(await screen.findByRole('button', { name: 'Try to compact again' }))

    // Then the reason reaches the user
    expect(await screen.findByText('study: nothing to compact')).toBeInTheDocument()
  })

  it('does not add a second toast when the retry already reported its failure', async () => {
    // Given a retry whose failure event arrives before the rejection
    const handlers = await renderSettledSession()
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)
    act(() => handlers.contextLimitReached?.(blocked))
    vi.mocked(compactStudySession).mockImplementationOnce(async () => {
      handlers.compactionFailed?.({ sessionId: 'session-1', reason: 'invalid format' })
      throw new Error('study: conversation summary does not have the required sections')
    })
    const user = userEvent.setup()

    // When retrying
    await user.click(await screen.findByRole('button', { name: 'Try to compact again' }))

    // Then exactly one toast is shown, with the short reason only
    await waitFor(() => expect(screen.getAllByRole('alert').length).toBeGreaterThan(0))
    expect(screen.queryByText(/required sections/)).not.toBeInTheDocument()
    expect(screen.getAllByText(/Could not summarize the conversation/)).toHaveLength(1)
  })

  it.each([
    ['message_too_large', /too long for the model's context window/],
    ['budget_impossible', /no longer fits the model's context window/],
  ])('restores the draft and explains a %s rejection', async (code, text) => {
    // Given a send rejected before the message was persisted
    const handlers = await renderSettledSession()
    vi.mocked(sendStudyMessage).mockReturnValueOnce(new Promise(() => {}))
    const user = userEvent.setup()
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'A very long question')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    // When the backend rejects it
    act(() => handlers.error?.(studyErrorEvent('session-1', 'raw backend text', code)))

    // Then the draft is back and the user is told why, in plain language
    await waitFor(() =>
      expect(screen.getByPlaceholderText(/type your answer/i)).toHaveValue('A very long question'),
    )
    expect(screen.getByText(text)).toBeInTheDocument()
    expect(screen.queryByText('raw backend text')).not.toBeInTheDocument()
  })

  it('unsubscribes from the compaction events on unmount', async () => {
    // Given a mounted session
    const handlers = setupSubscriptions()
    vi.mocked(requestOpeningTurn).mockReturnValueOnce(new Promise(() => {}))
    const { unmount } = renderNewSession()

    // When the screen unmounts
    unmount()

    // Then every compaction listener is released
    expect(handlers.unsubscribe.compacting).toHaveBeenCalledTimes(1)
    expect(handlers.unsubscribe.compacted).toHaveBeenCalledTimes(1)
    expect(handlers.unsubscribe.compactionFailed).toHaveBeenCalledTimes(1)
  })
})

describe('StudyChatScreen — earlier messages', () => {
  const SESSION = {
    id: 'session-1',
    topic: 'Cache invalidation',
    folderId: 'folder-1',
    goal: 'Ace the SQL interview',
    startedAt: '2026-08-16T10:00:00Z',
    context: CONTEXT_NORMAL,
  }

  function msg(id: string, role: 'user' | 'assistant' = 'user') {
    return { id, role, content: id, createdAt: '2026-08-16T10:00:00Z', sources: [] }
  }

  function olderPage(ids: string[], hasMore: boolean, nextCursor = ''): StudyMessagePage {
    return { messages: ids.map((id) => msg(id)), hasMore, nextCursor }
  }

  function deferred<T>() {
    let resolve!: (value: T) => void
    let reject!: (reason: unknown) => void
    const promise = new Promise<T>((res, rej) => {
      resolve = res
      reject = rej
    })
    return { promise, resolve, reject }
  }

  // A resumed session whose latest page is m51-m52, with older ones behind m51.
  async function renderResumedWithHistory(hasMore = true) {
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: SESSION,
      messages: [msg('m51'), msg('m52', 'assistant')],
      hasMore,
      nextCursor: hasMore ? 'm51' : '',
      summaryCoversUntilMessageId: '',
    })
    const view = render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )
    await screen.findByText('m52')
    return view
  }

  function transcriptTexts() {
    const transcript = screen.getByRole('log', { name: 'Conversation' })
    return within(transcript)
      .queryAllByText(/^[a-z]\d+$/)
      .map((element) => element.textContent)
  }

  // Each rendered message is 100px tall, so the transcript's scrollHeight
  // follows the DOM the way a real layout would when a page is prepended.
  function stubGrowingScrollMetrics(transcript: HTMLElement, clientHeight: number) {
    let scrollTop = 0
    Object.defineProperty(transcript, 'scrollHeight', {
      configurable: true,
      get: () => within(transcript).queryAllByText(/^[a-z]\d+$/).length * 100,
    })
    Object.defineProperty(transcript, 'clientHeight', { value: clientHeight, configurable: true })
    Object.defineProperty(transcript, 'scrollTop', {
      configurable: true,
      get: () => scrollTop,
      set: (value: number) => {
        scrollTop = value
      },
    })
  }

  it('loads the page before the cursor and shows it above the existing messages', async () => {
    // Given a resumed session with older messages behind m51
    await renderResumedWithHistory()
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(olderPage(['m49', 'm50'], false))

    // When the top of the transcript comes into view
    setSentinelVisible(true)

    // Then the older page is requested at that cursor and rendered first
    expect(await screen.findByText('m49')).toBeInTheDocument()
    expect(listMessagesBefore).toHaveBeenCalledWith('session-1', 'm51', 50)
    expect(transcriptTexts()).toEqual(['m49', 'm50', 'm51', 'm52'])
  })

  it('shows a loading row while the older page is being fetched', async () => {
    // Given an older page that has not resolved yet
    await renderResumedWithHistory()
    const inFlight = deferred<StudyMessagePage>()
    vi.mocked(listMessagesBefore).mockReturnValueOnce(inFlight.promise)

    // When the top of the transcript comes into view
    setSentinelVisible(true)

    // Then the loading row is shown, then removed once the page lands
    expect(await screen.findByText('Loading earlier messages…')).toBeInTheDocument()
    await act(async () => {
      inFlight.resolve(olderPage(['m50'], false))
    })
    await screen.findByText('m50')
    expect(screen.queryByText('Loading earlier messages…')).not.toBeInTheDocument()
  })

  it('requests only once while the top stays in view during a load', async () => {
    // Given an older page that has not resolved yet
    await renderResumedWithHistory()
    const inFlight = deferred<StudyMessagePage>()
    vi.mocked(listMessagesBefore).mockReturnValueOnce(inFlight.promise)

    // When the sentinel reports visible several times meanwhile
    setSentinelVisible(true)
    setSentinelVisible(true)
    setSentinelVisible(true)

    // Then a single request was made
    await screen.findByText('Loading earlier messages…')
    expect(listMessagesBefore).toHaveBeenCalledTimes(1)
    await act(async () => {
      inFlight.resolve(olderPage(['m50'], false))
    })
    await screen.findByText('m50')
  })

  it('keeps loading while a short page leaves the top in view, then stops', async () => {
    // Given the top stays in view after every page (the content never fills the viewport)
    await renderResumedWithHistory()
    vi.mocked(listMessagesBefore)
      .mockResolvedValueOnce(olderPage(['m50'], true, 'm50'))
      .mockResolvedValueOnce(olderPage(['m49'], false))

    // When the top comes into view
    setSentinelVisible(true)

    // Then the next page is requested from the new cursor without any scroll
    await screen.findByText('m49')
    expect(listMessagesBefore).toHaveBeenNthCalledWith(1, 'session-1', 'm51', 50)
    expect(listMessagesBefore).toHaveBeenNthCalledWith(2, 'session-1', 'm50', 50)

    // And once hasMore is false no further request is made
    await act(async () => {})
    expect(listMessagesBefore).toHaveBeenCalledTimes(2)
  })

  it('starts loading when the top is already in view as the history appears', async () => {
    // Given a viewport that already shows the top when the screen mounts
    setSentinelVisibleOnObserve(true)
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(olderPage(['m50'], false))

    // When the latest page arrives with older messages behind it
    await renderResumedWithHistory()

    // Then the older page loads on its own
    expect(await screen.findByText('m50')).toBeInTheDocument()
  })

  it('makes no request for a session that has no older messages', async () => {
    // Given a resumed session whose whole history fit in the first page
    await renderResumedWithHistory(false)

    // When the top comes into view
    setSentinelVisible(true)

    // Then nothing is requested and no loading row appears
    await act(async () => {})
    expect(listMessagesBefore).not.toHaveBeenCalled()
    expect(screen.queryByText('Loading earlier messages…')).not.toBeInTheDocument()
  })

  it('makes no request for a brand new session', async () => {
    // Given a new session (no history to page through)
    await renderStartedSession()

    // When the top comes into view
    setSentinelVisible(true)

    // Then nothing is requested
    await act(async () => {})
    expect(listMessagesBefore).not.toHaveBeenCalled()
  })

  it('shows an inline retry on failure, keeps the messages and does not retry by itself', async () => {
    // Given an older page that fails to load
    await renderResumedWithHistory()
    vi.mocked(listMessagesBefore).mockRejectedValueOnce(new Error('network down'))

    // When the top comes into view
    setSentinelVisible(true)

    // Then the failure is shown inline with a retry, and the messages stay
    expect(await screen.findByText('network down')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
    expect(transcriptTexts()).toEqual(['m51', 'm52'])

    // And the top still being in view does not fire another request
    setSentinelVisible(true)
    await act(async () => {})
    expect(listMessagesBefore).toHaveBeenCalledTimes(1)
  })

  it('requests the same page again when retry is clicked', async () => {
    // Given a failed older-page load
    await renderResumedWithHistory()
    vi.mocked(listMessagesBefore).mockRejectedValueOnce(new Error('network down'))
    setSentinelVisible(true)
    await screen.findByText('network down')

    // When the user clicks Retry and the request now succeeds
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(olderPage(['m50'], false))
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))

    // Then the same cursor was requested twice and the page is shown
    expect(await screen.findByText('m50')).toBeInTheDocument()
    expect(listMessagesBefore).toHaveBeenNthCalledWith(1, 'session-1', 'm51', 50)
    expect(listMessagesBefore).toHaveBeenNthCalledWith(2, 'session-1', 'm51', 50)
    expect(screen.queryByText('network down')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument()
  })

  it('does not mix a page failure into the global error alert', async () => {
    // Given a failed older-page load
    await renderResumedWithHistory()
    vi.mocked(listMessagesBefore).mockRejectedValueOnce(new Error('network down'))

    // When the top comes into view
    setSentinelVisible(true)
    await screen.findByText('network down')

    // Then the screen-level alert is not used for it
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('keeps the message under the viewport in place when an older page is prepended', async () => {
    // Given a user reading near the top, with the transcript following by default
    await renderResumedWithHistory()
    const transcript = screen.getByRole('log', { name: 'Conversation' })
    stubGrowingScrollMetrics(transcript, 150)
    transcript.scrollTop = 10
    fireEvent.scroll(transcript)
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(olderPage(['m49', 'm50'], false))

    // When two older messages (200px) are inserted above
    setSentinelVisible(true)
    await screen.findByText('m49')

    // Then the scroll offset grows by exactly that height, not jumping to the bottom
    await waitFor(() => expect(transcript.scrollTop).toBe(210))
  })

  it('does not scroll to the bottom on a prepend even when the user is within the sticky threshold', async () => {
    // Given a short transcript that is both at the top and within 80px of the bottom
    await renderResumedWithHistory()
    const transcript = screen.getByRole('log', { name: 'Conversation' })
    stubGrowingScrollMetrics(transcript, 150)
    transcript.scrollTop = 0
    fireEvent.scroll(transcript)
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(olderPage(['m49', 'm50'], false))

    // When older messages are prepended
    setSentinelVisible(true)
    await screen.findByText('m49')

    // Then the viewport stays on the same message instead of following the bottom
    await waitFor(() => expect(transcript.scrollTop).toBe(200))
    expect(transcript.scrollTop).not.toBe(transcript.scrollHeight)
  })

  it('keeps following the bottom for a message appended after older pages were loaded', async () => {
    // Given older messages were prepended and the user is back at the bottom
    const handlers = setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: SESSION,
      messages: [msg('m51'), msg('m52', 'assistant')],
      hasMore: true,
      nextCursor: 'm51',
      summaryCoversUntilMessageId: '',
    })
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )
    await screen.findByText('m52')
    const transcript = screen.getByRole('log', { name: 'Conversation' })
    stubGrowingScrollMetrics(transcript, 150)
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(olderPage(['m50'], false))
    setSentinelVisible(true)
    await screen.findByText('m50')
    transcript.scrollTop = transcript.scrollHeight
    fireEvent.scroll(transcript)

    // When the assistant streams a new answer
    act(() => {
      handlers.chunk?.({ sessionId: 'session-1', content: 'Streaming' })
    })

    // Then the transcript still follows it to the bottom
    await waitFor(() => expect(transcript.scrollTop).toBe(transcript.scrollHeight))
  })

  it('does not insert a message that is already in the transcript twice', async () => {
    // Given an older page that repeats m51, which is already shown
    await renderResumedWithHistory()
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(olderPage(['m50', 'm51'], false))

    // When it loads
    setSentinelVisible(true)
    await screen.findByText('m50')

    // Then m51 appears once
    expect(transcriptTexts()).toEqual(['m50', 'm51', 'm52'])
  })

  it('does not disturb the scroll position when a page adds nothing new', async () => {
    // Given an older page made only of messages already shown
    await renderResumedWithHistory()
    const transcript = screen.getByRole('log', { name: 'Conversation' })
    stubGrowingScrollMetrics(transcript, 150)
    transcript.scrollTop = 40
    fireEvent.scroll(transcript)
    vi.mocked(listMessagesBefore).mockResolvedValueOnce(olderPage(['m51'], false))

    // When it loads
    setSentinelVisible(true)
    await waitFor(() => expect(listMessagesBefore).toHaveBeenCalledTimes(1))
    await act(async () => {})

    // Then the position is untouched
    expect(transcript.scrollTop).toBe(40)
  })

  it('ignores an older page that resolves after switching sessions', async () => {
    // Given an older page in flight for session-1
    const view = await renderResumedWithHistory()
    const inFlight = deferred<StudyMessagePage>()
    vi.mocked(listMessagesBefore).mockReturnValueOnce(inFlight.promise)
    setSentinelVisible(true)
    await screen.findByText('Loading earlier messages…')

    // When the user opens session-2 and the old page then resolves
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: { ...SESSION, id: 'session-2' },
      messages: [msg('n1')],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })
    view.rerender(
      <StudyChatScreen
        sessionId="session-2"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )
    await screen.findByText('n1')
    await act(async () => {
      inFlight.resolve(olderPage(['m50'], false))
    })

    // Then session-1's page never shows up in session-2
    expect(screen.queryByText('m50')).not.toBeInTheDocument()
    expect(screen.queryByText('Loading earlier messages…')).not.toBeInTheDocument()
  })

  it('ignores a resume that resolves after switching sessions', async () => {
    // Given session-1's resume still pending
    setupSubscriptions()
    const slow = deferred<Awaited<ReturnType<typeof resumeStudySession>>>()
    vi.mocked(resumeStudySession).mockReturnValueOnce(slow.promise)
    const view = render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )

    // When the user opens session-2 and session-1's history then arrives
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: { ...SESSION, id: 'session-2' },
      messages: [msg('n1')],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: '',
    })
    view.rerender(
      <StudyChatScreen
        sessionId="session-2"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )
    await screen.findByText('n1')
    await act(async () => {
      slow.resolve({
        session: SESSION,
        messages: [msg('m51')],
        hasMore: true,
        nextCursor: 'm51',
        summaryCoversUntilMessageId: '',
      })
    })

    // Then only session-2's messages are shown and no older page is offered
    expect(transcriptTexts()).toEqual(['n1'])
    setSentinelVisible(true)
    await act(async () => {})
    expect(listMessagesBefore).not.toHaveBeenCalled()
  })
})

describe('StudyChatScreen — summary marker', () => {
  const SESSION = {
    id: 'session-1',
    topic: 'Cache invalidation',
    folderId: 'folder-1',
    goal: 'Ace the SQL interview',
    startedAt: '2026-08-16T10:00:00Z',
    context: CONTEXT_NORMAL,
  }
  const MARKER_TEXT = 'Earlier messages were summarized'

  function msg(id: string) {
    return { id, role: 'user', content: id, createdAt: '2026-08-16T10:00:00Z', sources: [] }
  }

  async function renderResumed(
    messageIds: string[],
    summaryCoversUntilMessageId: string,
    hasMore = false,
  ) {
    const handlers = setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: SESSION,
      messages: messageIds.map(msg),
      hasMore,
      nextCursor: hasMore ? messageIds[0] : '',
      summaryCoversUntilMessageId,
    })
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )
    await screen.findByText(messageIds[messageIds.length - 1])
    return handlers
  }

  function precedes(first: HTMLElement, second: HTMLElement) {
    return Boolean(first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING)
  }

  it('places the marker right after the last summarized message', async () => {
    // Given a resumed session summarized up to its second message
    await renderResumed(['m1', 'm2', 'm3'], 'm2')

    // Then the marker sits between m2 and m3
    const marker = await screen.findByText(MARKER_TEXT)
    expect(precedes(screen.getByText('m2'), marker)).toBe(true)
    expect(precedes(marker, screen.getByText('m3'))).toBe(true)
  })

  it('places the marker above the loaded messages when the summarized one is older', async () => {
    // Given a resumed session whose summary covers a message outside the loaded page
    await renderResumed(['m5', 'm6'], 'm2', true)

    // Then the marker sits above the first loaded message
    const marker = await screen.findByText(MARKER_TEXT)
    expect(precedes(marker, screen.getByText('m5'))).toBe(true)
  })

  it('shows no marker for a session without a summary', async () => {
    // Given a resumed session that was never compacted
    await renderResumed(['m1', 'm2'], '')

    // Then there is no marker
    expect(screen.queryByText(MARKER_TEXT)).not.toBeInTheDocument()
  })

  it('shows the marker as soon as a compaction finishes', async () => {
    // Given a resumed session without a summary
    const handlers = await renderResumed(['m1', 'm2', 'm3'], '')
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)
    vi.mocked(getStudySessionSummary).mockResolvedValueOnce({
      found: true,
      content: '## Progress',
      coversUntilMessageId: 'm1',
      updatedAt: '2026-10-21T12:00:00Z',
    })

    // When a compaction of this session finishes
    act(() =>
      handlers.compacted?.({ sessionId: 'session-1', usedTokens: 4000, contextLength: 10000 }),
    )

    // Then the marker appears right after the newly summarized message
    const marker = await screen.findByText(MARKER_TEXT)
    expect(getStudySessionSummary).toHaveBeenCalledWith('session-1')
    expect(precedes(screen.getByText('m1'), marker)).toBe(true)
    expect(precedes(marker, screen.getByText('m2'))).toBe(true)
  })

  it('shows no marker when the summary cannot be read after a compaction', async () => {
    // Given a resumed session without a summary
    const handlers = await renderResumed(['m1', 'm2'], '')
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)
    vi.mocked(getStudySessionSummary).mockRejectedValueOnce(new Error('database unavailable'))

    // When a compaction finishes but reading the summary fails
    act(() =>
      handlers.compacted?.({ sessionId: 'session-1', usedTokens: 4000, contextLength: 10000 }),
    )
    await screen.findByText('Older messages were summarized to keep the conversation going')

    // Then the chat keeps working without a marker
    await waitFor(() => expect(getStudySessionSummary).toHaveBeenCalled())
    expect(screen.queryByText(MARKER_TEXT)).not.toBeInTheDocument()
  })

  it('keeps the previous marker when a compaction reports no summary', async () => {
    // Given a resumed session summarized up to m1
    const handlers = await renderResumed(['m1', 'm2'], 'm1')
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)
    vi.mocked(getStudySessionSummary).mockResolvedValueOnce({
      found: false,
      content: '',
      coversUntilMessageId: '',
      updatedAt: '',
    })

    // When a compaction finishes and the read finds no summary
    act(() =>
      handlers.compacted?.({ sessionId: 'session-1', usedTokens: 4000, contextLength: 10000 }),
    )
    await waitFor(() => expect(getStudySessionSummary).toHaveBeenCalled())

    // Then the marker stays where it was
    expect(screen.getByText(MARKER_TEXT)).toBeInTheDocument()
  })

  it('ignores a compaction of another session', async () => {
    // Given a resumed session without a summary
    const handlers = await renderResumed(['m1', 'm2'], '')
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)

    // When a compaction of another session finishes
    act(() =>
      handlers.compacted?.({ sessionId: 'session-2', usedTokens: 4000, contextLength: 10000 }),
    )

    // Then nothing is read and no marker appears
    expect(getStudySessionSummary).not.toHaveBeenCalled()
    expect(screen.queryByText(MARKER_TEXT)).not.toBeInTheDocument()
  })

  it('keeps View summary disabled for a session without a summary', async () => {
    // Given a resumed session that was never compacted
    await renderResumed(['m1', 'm2'], '')

    // Then the header shows the button, but it cannot be used yet
    expect(screen.getByRole('button', { name: 'View summary' })).toBeDisabled()
  })

  it('enables View summary for a session that has one', async () => {
    // Given a resumed session with a summary
    await renderResumed(['m1', 'm2'], 'm1')

    // Then the button is usable
    expect(screen.getByRole('button', { name: 'View summary' })).toBeEnabled()
  })

  it('offers View summary next to the source mode selector', async () => {
    // Given a resumed session with a summary, rendered with the header slot
    const slot = document.createElement('div')
    document.body.appendChild(slot)
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: SESSION,
      messages: [msg('m1'), msg('m2')],
      hasMore: false,
      nextCursor: '',
      summaryCoversUntilMessageId: 'm1',
    })
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        sourceModeSlot={slot}
        {...newSessionActionProps()}
      />,
    )
    await screen.findByText('m2')

    // Then the button sits in the header slot, before the selector
    const button = within(slot).getByRole('button', { name: 'View summary' })
    expect(precedes(button, within(slot).getByRole('combobox'))).toBe(true)
    slot.remove()
  })

  it('enables the button as soon as a compaction creates the summary', async () => {
    // Given a resumed session without a summary
    const handlers = await renderResumed(['m1', 'm2'], '')
    vi.mocked(getStudySessionUsage).mockResolvedValue(null as never)
    vi.mocked(getStudySessionSummary).mockResolvedValueOnce({
      found: true,
      content: '## Progress',
      coversUntilMessageId: 'm1',
      updatedAt: '2026-10-21T12:00:00Z',
    })
    expect(screen.getByRole('button', { name: 'View summary' })).toBeDisabled()

    // When a compaction finishes
    act(() =>
      handlers.compacted?.({ sessionId: 'session-1', usedTokens: 4000, contextLength: 10000 }),
    )

    // Then the button is unlocked
    await waitFor(() => expect(screen.getByRole('button', { name: 'View summary' })).toBeEnabled())
  })

  it('opens the read-only summary from the header button', async () => {
    // Given a resumed session with a summary
    const user = userEvent.setup()
    await renderResumed(['m1', 'm2'], 'm1')
    vi.mocked(getStudySessionSummary).mockResolvedValueOnce({
      found: true,
      content: '## Progress\n- covered B-trees',
      coversUntilMessageId: 'm1',
      updatedAt: '2026-10-21T12:00:00Z',
    })

    // When clicking View summary
    await user.click(screen.getByRole('button', { name: 'View summary' }))

    // Then the summary is shown in a dialog
    expect(await screen.findByRole('heading', { name: 'Progress' })).toBeInTheDocument()
    expect(screen.getByText('covered B-trees')).toBeInTheDocument()

    // And closing it returns to the chat
    await user.click(screen.getAllByRole('button', { name: 'Close' })[0])
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })
})

describe('StudyChatScreen — loading a resumed session', () => {
  const SESSION = {
    id: 'session-1',
    topic: 'Cache invalidation',
    folderId: 'folder-1',
    goal: 'Ace the SQL interview',
    startedAt: '2026-08-16T10:00:00Z',
    context: CONTEXT_NORMAL,
  }

  function renderResume() {
    return render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        {...newSessionActionProps()}
      />,
    )
  }

  it('shows an animated loading indicator until the history arrives', async () => {
    // Given a resume that has not answered yet
    setupSubscriptions()
    let finish!: (history: Awaited<ReturnType<typeof resumeStudySession>>) => void
    vi.mocked(resumeStudySession).mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve
      }),
    )

    // When the screen mounts
    renderResume()

    // Then the loading indicator is on screen
    expect(screen.getByRole('status', { name: 'Loading conversation…' })).toBeInTheDocument()

    // And once the history arrives it is replaced by the messages
    await act(async () => {
      finish({
        session: SESSION,
        messages: [
          { id: 'm1', role: 'user', content: 'Hi', createdAt: '2026-08-16T10:00:00Z', sources: [] },
        ],
        hasMore: false,
        nextCursor: '',
        summaryCoversUntilMessageId: '',
      })
    })
    expect(await screen.findByText('Hi')).toBeInTheDocument()
    expect(screen.queryByRole('status', { name: 'Loading conversation…' })).not.toBeInTheDocument()
  })

  it('stops the indicator when the session fails to load', async () => {
    // Given a resume that fails
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockRejectedValueOnce(new Error('session not found'))

    // When the screen mounts
    renderResume()

    // Then the error is shown and the indicator is gone
    expect(await screen.findByText('session not found')).toBeInTheDocument()
    expect(screen.queryByRole('status', { name: 'Loading conversation…' })).not.toBeInTheDocument()
  })

  it('shows no loading indicator for a new session', () => {
    // Given a new session
    setupSubscriptions()
    vi.mocked(requestOpeningTurn).mockReturnValueOnce(new Promise(() => {}))

    // When the screen mounts
    renderNewSession()

    // Then only the thinking indicator is shown
    expect(screen.queryByRole('status', { name: 'Loading conversation…' })).not.toBeInTheDocument()
  })
})
