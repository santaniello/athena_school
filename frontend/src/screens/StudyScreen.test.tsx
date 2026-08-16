import { describe, expect, it, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  endStudySession,
  onStudyChunk,
  onStudyDone,
  onStudyError,
  sendStudyMessage,
  startStudySession,
} from '@/lib/study'
import StudyScreen from './StudyScreen'

vi.mock('@/lib/study', () => ({
  startStudySession: vi.fn(),
  sendStudyMessage: vi.fn(),
  endStudySession: vi.fn(),
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

describe('StudyScreen', () => {
  it('disables the start button until a topic is entered', async () => {
    // Given the screen is on the topic-selection step
    setupSubscriptions()
    const user = userEvent.setup()
    render(<StudyScreen />)

    // Then the start button is disabled
    expect(screen.getByRole('button', { name: 'Start session' })).toBeDisabled()

    // When typing a topic
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')

    // Then the button becomes enabled
    expect(screen.getByRole('button', { name: 'Start session' })).toBeEnabled()
  })

  it('shows no error alert on the topic step before anything has failed', () => {
    // Given the screen just mounted, with no error yet
    setupSubscriptions()
    render(<StudyScreen />)

    // Then no alert is shown
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('keeps the start button disabled for a whitespace-only topic', async () => {
    // Given the screen is on the topic-selection step
    setupSubscriptions()
    const user = userEvent.setup()
    render(<StudyScreen />)

    // When typing only whitespace
    await user.type(screen.getByLabelText(/study today/i), '   ')

    // Then the start button stays disabled
    expect(screen.getByRole('button', { name: 'Start session' })).toBeDisabled()
  })

  it('starts a session and switches to the chat view, with no messages or alert yet', async () => {
    // Given a topic and a StartStudySession call that succeeds
    setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')

    // When starting the session
    await user.click(screen.getByRole('button', { name: 'Start session' }))

    // Then it called startStudySession with the topic and moved to the chat
    // view, with no leftover message bubbles or error alert
    expect(startStudySession).toHaveBeenCalledWith('Distributed systems')
    expect(await screen.findByRole('button', { name: 'End session' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(document.querySelectorAll('.self-start, .self-end')).toHaveLength(0)
  })

  it('trims leading and trailing whitespace from the topic before starting', async () => {
    // Given a topic typed with surrounding whitespace
    setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), '  Distributed systems  ')

    // When starting the session
    await user.click(screen.getByRole('button', { name: 'Start session' }))

    // Then the surrounding whitespace was trimmed before the call
    expect(startStudySession).toHaveBeenCalledWith('Distributed systems')
  })

  it('shows an inline error and re-enables the start button when starting fails', async () => {
    // Given a StartStudySession call that fails
    setupSubscriptions()
    vi.mocked(startStudySession).mockRejectedValueOnce(new Error('invalid OpenRouter key'))
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')

    // When starting the session
    await user.click(screen.getByRole('button', { name: 'Start session' }))

    // Then the error is shown and the topic step is still active, with the
    // start button usable again (not stuck disabled from a stale streaming
    // state)
    expect(await screen.findByText('invalid OpenRouter key')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Start session' })).toBeEnabled()
  })

  it('shows a fallback error message when starting fails with a non-Error rejection', async () => {
    // Given a StartStudySession call that rejects with something other than an Error
    setupSubscriptions()
    vi.mocked(startStudySession).mockRejectedValueOnce('not an Error instance')
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')

    // When starting the session
    await user.click(screen.getByRole('button', { name: 'Start session' }))

    // Then the fallback message is shown
    expect(await screen.findByText('Failed to start the session.')).toBeInTheDocument()
  })

  it('accumulates streamed chunks and appends the assistant message once done', async () => {
    // Given a started session
    const handlers = setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })

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

    // Then the full message is still shown as a settled assistant message,
    // rendered with the assistant's own styling (not the user's)
    const bubble = await screen.findByText('Hello there!')
    expect(bubble).toBeInTheDocument()
    expect(bubble).toHaveClass('self-start')
    expect(bubble).not.toHaveClass('self-end')
  })

  it('keeps the send button disabled for an empty or whitespace-only draft, once streaming has settled', async () => {
    // Given a started session whose opening turn has already finished
    // streaming (so isStreaming is false and only the draft controls the
    // button's disabled state)
    const handlers = setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })
    act(() => {
      handlers.chunk?.('Welcome!')
      handlers.done?.()
    })
    await screen.findByText('Welcome!')

    // Then the send button starts disabled (empty draft)
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()

    // When typing only whitespace
    await user.type(screen.getByPlaceholderText(/type your answer/i), '   ')

    // Then it is still disabled (trimmed content is empty)
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
  })

  it('keeps the send button disabled while a reply is still streaming', async () => {
    // Given a started session, whose opening turn is still streaming
    let resolveStart!: (value: { id: string; topic: string; startedAt: string }) => void
    setupSubscriptions()
    vi.mocked(startStudySession).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveStart = resolve
      }),
    )
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await act(async () => {
      resolveStart({ id: 'session-1', topic: 'Distributed systems', startedAt: '2026-08-16T10:00:00Z' })
    })
    await screen.findByRole('button', { name: 'End session' })

    // When typing a reply before the opening turn has finished streaming
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')

    // Then the send button stays disabled
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
  })

  it('sends a typed message and appends it immediately', async () => {
    // Given a started session
    const handlers = setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    vi.mocked(sendStudyMessage).mockResolvedValueOnce()
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })
    act(() => {
      handlers.chunk?.('Welcome!')
      handlers.done?.()
    })
    await screen.findByText('Welcome!')

    // When typing and sending a reply
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    // Then the user message appears immediately, styled as the user's own
    // bubble, and sendStudyMessage was called
    const bubble = await screen.findByText('What is CAP theorem?')
    expect(bubble).toHaveClass('self-end')
    expect(bubble).not.toHaveClass('self-start')
    expect(sendStudyMessage).toHaveBeenCalledWith('session-1', 'Distributed systems', 'What is CAP theorem?')
  })

  it('trims leading and trailing whitespace from the draft before sending', async () => {
    // Given a started session with a settled opening turn
    const handlers = setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    vi.mocked(sendStudyMessage).mockResolvedValueOnce()
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })
    act(() => {
      handlers.chunk?.('Welcome!')
      handlers.done?.()
    })
    await screen.findByText('Welcome!')

    // When sending a reply typed with surrounding whitespace
    await user.type(screen.getByPlaceholderText(/type your answer/i), '  What is CAP theorem?  ')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    // Then the surrounding whitespace was trimmed before the call
    expect(sendStudyMessage).toHaveBeenCalledWith('session-1', 'Distributed systems', 'What is CAP theorem?')
  })

  it('keeps the send button disabled while its own reply is streaming', async () => {
    // Given a started session with a settled opening turn
    const handlers = setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    let resolveSend!: () => void
    vi.mocked(sendStudyMessage).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveSend = resolve
      }),
    )
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })
    act(() => {
      handlers.chunk?.('Welcome!')
      handlers.done?.()
    })
    await screen.findByText('Welcome!')

    // When sending a reply whose stream has not resolved yet
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    // Then the send button is disabled while the reply is in flight, even
    // if new content is typed in the meantime (proving it's isStreaming
    // holding it disabled, not merely the draft being empty)
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Another question')
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()

    // When the reply resolves and its "study:done" event fires (the two are
    // distinct in production: the bound call blocks until the stream ends,
    // which is signaled by the event, not by the resolved promise alone)
    await act(async () => {
      resolveSend()
    })
    act(() => {
      handlers.done?.()
    })

    // Then it stays disabled only because the draft is empty again, not
    // because of a stuck streaming flag — proven by typing new content
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Follow-up')
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
  })

  it('shows an inline error and re-enables the send button when sending fails', async () => {
    // Given a started session and a SendStudyMessage call that fails
    const handlers = setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    vi.mocked(sendStudyMessage).mockRejectedValueOnce(new Error('upstream failure'))
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })
    act(() => {
      handlers.chunk?.('Welcome!')
      handlers.done?.()
    })
    await screen.findByText('Welcome!')

    // When sending a reply that fails
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    // Then the error is shown, and typing again re-enables the button —
    // proving isStreaming was reset, not just that the draft is empty
    expect(await screen.findByText('upstream failure')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Retry')
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
  })

  it('shows a fallback error message when sending fails with a non-Error rejection', async () => {
    // Given a started session and a SendStudyMessage call rejecting with something other than an Error
    const handlers = setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    vi.mocked(sendStudyMessage).mockRejectedValueOnce('not an Error instance')
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })
    act(() => {
      handlers.chunk?.('Welcome!')
      handlers.done?.()
    })
    await screen.findByText('Welcome!')

    // When sending a reply that fails with a non-Error rejection
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'What is CAP theorem?')
    await user.click(screen.getByRole('button', { name: 'Send' }))

    // Then the fallback message is shown
    expect(await screen.findByText('Failed to send the message.')).toBeInTheDocument()
  })

  it('shows an inline error when the stream fails', async () => {
    // Given a started session
    const handlers = setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })

    // When the stream errors
    act(() => {
      handlers.error?.('upstream failure')
    })

    // Then it is shown inline, and the send button is usable again — proving
    // isStreaming/streamingText were reset, not left stuck from the aborted
    // opening turn
    expect(await screen.findByText('upstream failure')).toBeInTheDocument()
    await user.type(screen.getByPlaceholderText(/type your answer/i), 'Try again')
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
  })

  it('unsubscribes from every study event on unmount', async () => {
    // Given a started session
    const handlers = setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    const user = userEvent.setup()
    const { unmount } = render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })

    // When the screen unmounts
    unmount()

    // Then every subscription was cleaned up
    expect(handlers.unsubscribe.chunk).toHaveBeenCalledOnce()
    expect(handlers.unsubscribe.done).toHaveBeenCalledOnce()
    expect(handlers.unsubscribe.error).toHaveBeenCalledOnce()
  })

  it('ends the session and notifies the caller', async () => {
    // Given a started session
    setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    vi.mocked(endStudySession).mockResolvedValueOnce()
    const onEndSession = vi.fn()
    const user = userEvent.setup()
    render(<StudyScreen onEndSession={onEndSession} />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })

    // When ending the session
    await user.click(screen.getByRole('button', { name: 'End session' }))

    // Then it closed the session and notified the caller
    expect(endStudySession).toHaveBeenCalledWith('session-1')
    expect(onEndSession).toHaveBeenCalledOnce()
  })

  it('ends the session without a callback prop, without crashing', async () => {
    // Given a started session and no onEndSession prop at all
    setupSubscriptions()
    vi.mocked(startStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    })
    vi.mocked(endStudySession).mockResolvedValueOnce()
    const user = userEvent.setup()
    render(<StudyScreen />)
    await user.type(screen.getByLabelText(/study today/i), 'Distributed systems')
    await user.click(screen.getByRole('button', { name: 'Start session' }))
    await screen.findByRole('button', { name: 'End session' })

    // When ending the session
    await user.click(screen.getByRole('button', { name: 'End session' }))

    // Then it closes the session without throwing
    expect(endStudySession).toHaveBeenCalledWith('session-1')
  })
})
