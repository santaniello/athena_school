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
import StudyChatScreen from './StudyChatScreen'

vi.mock('@/lib/study', () => ({
  requestOpeningTurn: vi.fn(),
  resumeStudySession: vi.fn(),
  sendStudyMessage: vi.fn(),
  onStudyChunk: vi.fn(),
  onStudyDone: vi.fn(),
  onStudyError: vi.fn(),
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

    // Then the error is shown
    expect(await screen.findByText('invalid OpenRouter key')).toBeInTheDocument()
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
