import React from 'react'
import { describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  onStudyChunk,
  onStudyDone,
  onStudyError,
  requestOpeningTurn,
  resumeStudySession,
  sendStudyMessage,
} from '@/lib/study'
import { extractKnowledge } from '@/lib/knowledge'
import StudyChatScreen from './StudyChatScreen'

vi.mock('@/lib/study', () => ({
  requestOpeningTurn: vi.fn(),
  resumeStudySession: vi.fn(),
  sendStudyMessage: vi.fn(),
  onStudyChunk: vi.fn(),
  onStudyDone: vi.fn(),
  onStudyError: vi.fn(),
}))

vi.mock('@/lib/knowledge', () => ({
  extractKnowledge: vi.fn(),
  saveExtractedKnowledge: vi.fn(),
}))

function setupSubscriptions() {
  const unsubscribe = { chunk: vi.fn(), done: vi.fn(), error: vi.fn() }
  const handlers: {
    chunk?: (chunk: string) => void
    done?: () => void
    error?: (message: string) => void
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
  return handlers
}

function renderNewSession() {
  return render(
    <StudyChatScreen sessionId="session-1" initialTopic="Distributed systems" mode="new" />,
  )
}

async function renderStartedSession() {
  const handlers = setupSubscriptions()
  vi.mocked(requestOpeningTurn).mockReturnValueOnce(new Promise(() => {}))
  renderNewSession()
  await screen.findByRole('status', { name: /thinking/i })
  return handlers
}

async function renderSettledSession() {
  const handlers = setupSubscriptions()
  vi.mocked(requestOpeningTurn).mockResolvedValueOnce()
  renderNewSession()
  await screen.findByRole('status', { name: /thinking/i })
  act(() => {
    handlers.chunk?.('Welcome!')
    handlers.done?.()
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
      handlers.chunk?.('Welcome!')
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
        <StudyChatScreen sessionId="session-1" initialTopic="Distributed systems" mode="new" />
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
      handlers.chunk?.('Hello ')
      handlers.chunk?.('there!')
    })

    // Then the partial text is visible before the stream finishes
    expect(await screen.findByText('Hello there!')).toBeInTheDocument()

    // When the stream finishes
    act(() => {
      handlers.done?.()
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
        startedAt: '2026-08-16T10:00:00Z',
      },
      messages: [
        { role: 'user', content: 'Hi', createdAt: '2026-08-16T10:00:00Z' },
        { role: 'assistant', content: 'Hello!', createdAt: '2026-08-16T10:00:01Z' },
      ],
    })

    // When the chat screen mounts in "resume" mode
    const onTopicResolved = vi.fn()
    render(
      <StudyChatScreen
        sessionId="session-1"
        initialTopic=""
        mode="resume"
        onTopicResolved={onTopicResolved}
      />,
    )

    // Then its history is loaded and shown, the real topic is reported to
    // the caller, and no opening turn is requested
    expect(await screen.findByText('Hi')).toBeInTheDocument()
    expect(screen.getByText('Hello!')).toBeInTheDocument()
    expect(onTopicResolved).toHaveBeenCalledWith('Cache invalidation')
    expect(requestOpeningTurn).not.toHaveBeenCalled()
  })

  it('shows an inline error when resuming fails', async () => {
    // Given a resume call that fails
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockRejectedValueOnce(new Error('session not found'))

    // When the chat screen mounts in "resume" mode
    render(<StudyChatScreen sessionId="session-1" initialTopic="" mode="resume" />)

    // Then the error is shown
    expect(await screen.findByText('session not found')).toBeInTheDocument()
  })

  it('falls back to a generic message when resuming fails with a non-Error value', async () => {
    // Given a resume call that rejects with something other than an Error
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockRejectedValueOnce('network down')

    // When the chat screen mounts in "resume" mode
    render(<StudyChatScreen sessionId="session-1" initialTopic="" mode="resume" />)

    // Then the generic fallback message is shown
    expect(await screen.findByText('Failed to load the session.')).toBeInTheDocument()
  })

  it('does not show the thinking indicator while resuming, unlike starting a new session', async () => {
    // Given a resume call that hasn't settled yet
    setupSubscriptions()
    vi.mocked(resumeStudySession).mockReturnValueOnce(new Promise(() => {}))

    // When the chat screen mounts in "resume" mode
    render(<StudyChatScreen sessionId="session-1" initialTopic="" mode="resume" />)

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
        startedAt: '2026-08-16T10:00:00Z',
      },
      messages: [{ role: 'user', content: 'Hi', createdAt: '2026-08-16T10:00:00Z' }],
    })

    // When the chat screen mounts in "resume" mode
    render(<StudyChatScreen sessionId="session-1" initialTopic="" mode="resume" />)

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
        startedAt: '2026-08-16T10:00:00Z',
      },
      messages: [{ role: 'user', content: 'Hi', createdAt: '2026-08-16T10:00:00Z' }],
    })
    const { rerender } = render(
      <StudyChatScreen sessionId="session-1" initialTopic="" mode="resume" />,
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
        startedAt: '2026-08-16T11:00:00Z',
      },
      messages: [{ role: 'user', content: 'Hello again', createdAt: '2026-08-16T11:00:00Z' }],
    })
    rerender(<StudyChatScreen sessionId="session-2" initialTopic="" mode="resume" />)

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
      handlers.chunk?.('Fresh reply')
    })
    expect(await screen.findByText('Fresh reply')).toBeInTheDocument()
  })

  it('starts the next streamed reply from empty text, not leftover from a failed one', async () => {
    // Given a stream that partially arrived, then failed
    const handlers = await renderStartedSession()
    act(() => {
      handlers.chunk?.('Partial before failure')
      handlers.error?.('upstream failure')
    })
    await screen.findByText('upstream failure')
    vi.mocked(sendStudyMessage).mockResolvedValueOnce()
    const user = userEvent.setup()

    // When retrying with a new message
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Retry question')
    await user.click(screen.getByRole('button', { name: 'Send' }))
    act(() => {
      handlers.chunk?.('Fresh reply')
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
      handlers.error?.('upstream failure')
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
  })
})

describe('StudyChatScreen — knowledge extraction', () => {
  it('enables the labeled extraction button only after a message exists and streaming ends', async () => {
    // Given a new session still waiting for its first message
    const handlers = await renderStartedSession()

    // Then extraction is disabled during streaming
    const extractionButton = screen.getByRole('button', { name: 'Extract knowledge' })
    expect(extractionButton).toBeDisabled()
    expect(extractionButton).toHaveAttribute('aria-label', 'Extract knowledge')
    expect(extractionButton).toHaveTextContent('Extract knowledge')

    // When the first assistant message settles
    act(() => {
      handlers.chunk?.('Welcome!')
      handlers.done?.()
    })

    // Then extraction becomes available
    expect(await screen.findByRole('button', { name: 'Extract knowledge' })).toBeEnabled()
  })

  it('shows candidates and blocks duplicate extraction calls while one is in flight', async () => {
    // Given a settled session and an extraction request that is still pending
    await renderSettledSession()
    let resolveExtraction!: (value: Awaited<ReturnType<typeof extractKnowledge>>) => void
    vi.mocked(extractKnowledge).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveExtraction = resolve
      }),
    )
    const user = userEvent.setup()

    // When extracting knowledge
    await user.click(screen.getByRole('button', { name: 'Extract knowledge' }))

    // Then the local loading state prevents another call
    const extractionButton = screen.getByRole('button', { name: 'Extracting knowledge' })
    expect(extractionButton).toBeDisabled()
    expect(extractionButton).toHaveAttribute('aria-label', 'Extracting knowledge')
    expect(extractionButton).toHaveTextContent('Extracting...')
    expect(extractKnowledge).toHaveBeenCalledTimes(1)

    // When the candidates arrive
    resolveExtraction({
      truncated: false,
      items: [
        {
          id: 'candidate-1',
          topic: 'Distributed systems',
          concept: 'CAP theorem',
          definition: 'A distributed-systems trade-off.',
          properties: [],
          tradeOffs: [],
          relatedConcepts: [],
          source: 'athena',
          status: 'draft',
          createdAt: '2026-08-18T10:00:00Z',
          updatedAt: '2026-08-18T10:00:00Z',
        },
      ],
    })

    // Then the review dialog opens
    expect(await screen.findByText('New knowledge found')).toBeInTheDocument()
    expect(screen.getByText('CAP theorem')).toBeInTheDocument()
  })

  it('asks before retrying a truncated transcript and proceeds only after confirmation', async () => {
    // Given a settled long session whose first extraction asks for confirmation
    await renderSettledSession()
    vi.mocked(extractKnowledge)
      .mockResolvedValueOnce({ items: [], truncated: true })
      .mockResolvedValueOnce({ items: [], truncated: true })
    const user = userEvent.setup()

    // When starting extraction
    await user.click(screen.getByRole('button', { name: 'Extract knowledge' }))

    // Then a plain confirmation appears before the second call
    expect(await screen.findByText(/this session is long/i)).toBeInTheDocument()
    expect(extractKnowledge).toHaveBeenCalledTimes(1)

    // When confirming
    await user.click(screen.getByRole('button', { name: 'Yes' }))

    // Then the warning closes, extraction is re-invoked with confirmation, and review opens
    await waitFor(() => expect(screen.queryByText(/this session is long/i)).not.toBeInTheDocument())
    await waitFor(() => expect(extractKnowledge).toHaveBeenLastCalledWith('session-1', true))
    expect(await screen.findByText('No new knowledge found')).toBeInTheDocument()
  })

  it('stops after the user declines truncated transcript processing', async () => {
    // Given a settled long session whose extraction needs confirmation
    await renderSettledSession()
    vi.mocked(extractKnowledge).mockResolvedValueOnce({ items: [], truncated: true })
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Extract knowledge' }))
    expect(await screen.findByText(/this session is long/i)).toBeInTheDocument()

    // When declining truncated processing
    await user.click(screen.getByRole('button', { name: 'No' }))

    // Then the warning closes and no confirmed extraction is sent
    await waitFor(() => expect(screen.queryByText(/this session is long/i)).not.toBeInTheDocument())
    expect(extractKnowledge).toHaveBeenCalledOnce()
  })

  it('closes extracted candidate review when ignored', async () => {
    // Given an extraction review with no new candidates
    await renderSettledSession()
    vi.mocked(extractKnowledge).mockResolvedValueOnce({ items: [], truncated: false })
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Extract knowledge' }))
    expect(await screen.findByText('No new knowledge found')).toBeInTheDocument()

    // When ignoring the result
    await user.click(screen.getByRole('button', { name: 'Dismiss' }))

    // Then review closes
    await waitFor(() =>
      expect(screen.queryByText('No new knowledge found')).not.toBeInTheDocument(),
    )
  })

  it('shows a genuine extraction failure as an inline error', async () => {
    // Given a settled session and a failed extraction call
    await renderSettledSession()
    vi.mocked(extractKnowledge).mockRejectedValueOnce(new Error('openrouter api key is missing'))
    const user = userEvent.setup()

    // When extracting knowledge
    await user.click(screen.getByRole('button', { name: 'Extract knowledge' }))

    // Then the failure is shown outside the empty-result modal
    expect(await screen.findByText('openrouter api key is missing')).toBeInTheDocument()
    expect(screen.queryByText('No new knowledge found')).not.toBeInTheDocument()
  })

  it('clears an extraction failure when retrying successfully', async () => {
    // Given an extraction that fails once and then succeeds
    await renderSettledSession()
    vi.mocked(extractKnowledge)
      .mockRejectedValueOnce(new Error('temporary failure'))
      .mockResolvedValueOnce({ items: [], truncated: false })
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Extract knowledge' }))
    expect(await screen.findByText('temporary failure')).toBeInTheDocument()

    // When retrying extraction
    await user.click(screen.getByRole('button', { name: 'Extract knowledge' }))

    // Then the stale failure is cleared and review opens
    expect(await screen.findByText('No new knowledge found')).toBeInTheDocument()
    expect(screen.queryByText('temporary failure')).not.toBeInTheDocument()
  })

  it('shows a safe extraction error for an unexpected rejection value', async () => {
    // Given an extraction binding that rejects without an Error
    await renderSettledSession()
    vi.mocked(extractKnowledge).mockRejectedValueOnce('unavailable')
    const user = userEvent.setup()

    // When extracting knowledge
    await user.click(screen.getByRole('button', { name: 'Extract knowledge' }))

    // Then a user-safe fallback is shown
    expect(await screen.findByText('Failed to extract knowledge.')).toBeInTheDocument()
  })

  it('explains when no complete transcript message fits the extraction limit', async () => {
    // Given a session whose newest complete message is too large for extraction
    await renderSettledSession()
    vi.mocked(extractKnowledge).mockRejectedValueOnce(
      new Error('no complete transcript message fits within the extraction limit'),
    )
    const user = userEvent.setup()

    // When extracting knowledge
    await user.click(screen.getByRole('button', { name: 'Extract knowledge' }))

    // Then the internal error is translated into actionable Portuguese UI text
    expect(
      await screen.findByText(
        'The most recent message is too large to process in full.',
      ),
    ).toBeInTheDocument()
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
      handlers.chunk?.('More of the answer')
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
      handlers.chunk?.('More of the answer')
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
      handlers.chunk?.('More of the answer')
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
      handlers.chunk?.('More of the answer')
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
