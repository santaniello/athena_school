import { describe, expect, it, vi } from 'vitest'
import { EndStudySession, SendStudyMessage, StartStudySession } from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { endStudySession, onStudyChunk, onStudyDone, onStudyError, sendStudyMessage, startStudySession } from './study'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  StartStudySession: vi.fn(),
  SendStudyMessage: vi.fn(),
  EndStudySession: vi.fn(),
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
}))

describe('startStudySession', () => {
  it('returns the created session', async () => {
    // Given a StartStudySession call that succeeds
    vi.mocked(StartStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      startedAt: '2026-08-16T10:00:00Z',
    } as never)

    // When starting a study session
    const session = await startStudySession('Distributed systems')

    // Then it returns the session and forwarded the topic
    expect(StartStudySession).toHaveBeenCalledWith('Distributed systems')
    expect(session).toEqual({ id: 'session-1', topic: 'Distributed systems', startedAt: '2026-08-16T10:00:00Z' })
  })
})

describe('sendStudyMessage', () => {
  it('forwards sessionId, topic and content', async () => {
    // Given a SendStudyMessage call that succeeds
    vi.mocked(SendStudyMessage).mockResolvedValueOnce()

    // When sending a message
    await sendStudyMessage('session-1', 'Distributed systems', 'What is CAP theorem?')

    // Then it forwarded every argument
    expect(SendStudyMessage).toHaveBeenCalledWith('session-1', 'Distributed systems', 'What is CAP theorem?')
  })
})

describe('endStudySession', () => {
  it('forwards the sessionId', async () => {
    // Given an EndStudySession call that succeeds
    vi.mocked(EndStudySession).mockResolvedValueOnce()

    // When ending a session
    await endStudySession('session-1')

    // Then it forwarded the sessionId
    expect(EndStudySession).toHaveBeenCalledWith('session-1')
  })
})

describe('onStudyChunk', () => {
  it('subscribes to the study:chunk event and forwards the chunk', () => {
    // Given a handler and a mocked EventsOn
    const unsubscribe = vi.fn()
    vi.mocked(EventsOn).mockReturnValueOnce(unsubscribe)
    const handler = vi.fn()

    // When subscribing
    const result = onStudyChunk(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback('Hello there!')

    // Then the handler received the chunk and the unsubscribe function is returned
    expect(EventsOn).toHaveBeenCalledWith('study:chunk', expect.any(Function))
    expect(handler).toHaveBeenCalledWith('Hello there!')
    expect(result).toBe(unsubscribe)
  })
})

describe('onStudyDone', () => {
  it('subscribes to the study:done event', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()

    // When subscribing
    onStudyDone(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback()

    // Then the handler fired
    expect(EventsOn).toHaveBeenCalledWith('study:done', expect.any(Function))
    expect(handler).toHaveBeenCalledOnce()
  })
})

describe('onStudyError', () => {
  it('subscribes to the study:error event and forwards the message', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()

    // When subscribing
    onStudyError(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback('upstream failure')

    // Then the handler received the error message
    expect(EventsOn).toHaveBeenCalledWith('study:error', expect.any(Function))
    expect(handler).toHaveBeenCalledWith('upstream failure')
  })
})
