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
  const handlers: { chunk?: (chunk: string) => void; done?: () => void; error?: (message: string) => void } = {}
  vi.mocked(onStudyChunk).mockImplementation((handler) => {
    handlers.chunk = handler
    return vi.fn()
  })
  vi.mocked(onStudyDone).mockImplementation((handler) => {
    handlers.done = handler
    return vi.fn()
  })
  vi.mocked(onStudyError).mockImplementation((handler) => {
    handlers.error = handler
    return vi.fn()
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

  it('starts a session and switches to the chat view', async () => {
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

    // Then it called startStudySession with the topic and moved to the chat view
    expect(startStudySession).toHaveBeenCalledWith('Distributed systems')
    expect(await screen.findByRole('button', { name: 'End session' })).toBeInTheDocument()
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

    // Then the full message is still shown as a settled assistant message
    expect(await screen.findByText('Hello there!')).toBeInTheDocument()
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

    // Then the user message appears immediately and sendStudyMessage was called
    expect(await screen.findByText('What is CAP theorem?')).toBeInTheDocument()
    expect(sendStudyMessage).toHaveBeenCalledWith('session-1', 'Distributed systems', 'What is CAP theorem?')
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

    // Then it is shown inline
    expect(await screen.findByText('upstream failure')).toBeInTheDocument()
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
})
