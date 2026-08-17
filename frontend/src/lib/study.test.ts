import { describe, expect, it, vi } from 'vitest'
import {
  DeleteStudySession,
  EndStudySession,
  ListStudySessionsByFolder,
  MoveStudySession,
  RequestOpeningTurn,
  ResumeStudySession,
  SendStudyMessage,
  StartStudySession,
} from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  deleteStudySession,
  endStudySession,
  listStudySessionsByFolder,
  moveStudySession,
  onStudyChunk,
  onStudyDone,
  onStudyError,
  requestOpeningTurn,
  resumeStudySession,
  sendStudyMessage,
  startStudySession,
} from './study'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  StartStudySession: vi.fn(),
  RequestOpeningTurn: vi.fn(),
  SendStudyMessage: vi.fn(),
  EndStudySession: vi.fn(),
  DeleteStudySession: vi.fn(),
  ResumeStudySession: vi.fn(),
  MoveStudySession: vi.fn(),
  ListStudySessionsByFolder: vi.fn(),
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
      folderId: 'folder-1',
      startedAt: '2026-08-16T10:00:00Z',
      endedAt: '',
    } as never)

    // When starting a study session in a folder
    const session = await startStudySession('Distributed systems', 'folder-1')

    // Then it forwarded the topic and folder id, and returned the session
    expect(StartStudySession).toHaveBeenCalledWith('Distributed systems', 'folder-1')
    expect(session).toEqual({
      id: 'session-1',
      topic: 'Distributed systems',
      folderId: 'folder-1',
      startedAt: '2026-08-16T10:00:00Z',
      endedAt: '',
    })
  })

  it('defaults folderId to an empty string when omitted', async () => {
    // Given a StartStudySession call that succeeds
    vi.mocked(StartStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      folderId: 'default',
      startedAt: '2026-08-16T10:00:00Z',
      endedAt: '',
    } as never)

    // When starting a session without specifying a folder
    await startStudySession('Distributed systems')

    // Then it forwarded an empty folder id, letting the backend fall back
    // to the default folder
    expect(StartStudySession).toHaveBeenCalledWith('Distributed systems', '')
  })
})

describe('resumeStudySession', () => {
  it('returns the reopened session and its full history', async () => {
    // Given a ResumeStudySession call that succeeds
    vi.mocked(ResumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-1',
        topic: 'Distributed systems',
        folderId: 'folder-1',
        startedAt: '2026-08-16T10:00:00Z',
        endedAt: '',
      },
      messages: [{ role: 'user', content: 'Hi', createdAt: '2026-08-16T10:00:00Z' }],
    } as never)

    // When resuming a session
    const result = await resumeStudySession('session-1')

    // Then it forwarded the sessionId and returned the session and history
    expect(ResumeStudySession).toHaveBeenCalledWith('session-1')
    expect(result.session.id).toBe('session-1')
    expect(result.messages).toEqual([
      { role: 'user', content: 'Hi', createdAt: '2026-08-16T10:00:00Z' },
    ])
  })
})

describe('moveStudySession', () => {
  it('forwards the sessionId and folderId', async () => {
    // Given a MoveStudySession call that succeeds
    vi.mocked(MoveStudySession).mockResolvedValueOnce()

    // When moving a session to another folder
    await moveStudySession('session-1', 'folder-2')

    // Then it forwarded both arguments
    expect(MoveStudySession).toHaveBeenCalledWith('session-1', 'folder-2')
  })
})

describe('listStudySessionsByFolder', () => {
  it('returns every session in the folder', async () => {
    // Given a ListStudySessionsByFolder call that returns two sessions
    vi.mocked(ListStudySessionsByFolder).mockResolvedValueOnce([
      {
        id: 's-1',
        topic: 'Cache invalidation',
        folderId: 'folder-1',
        startedAt: '2026-08-16T10:00:00Z',
        endedAt: '',
      },
      {
        id: 's-2',
        topic: 'Concurrency patterns',
        folderId: 'folder-1',
        startedAt: '2026-08-15T10:00:00Z',
        endedAt: '2026-08-15T11:00:00Z',
      },
    ] as never)

    // When listing sessions in a folder
    const sessions = await listStudySessionsByFolder('folder-1')

    // Then it forwarded the folderId and returned every session
    expect(ListStudySessionsByFolder).toHaveBeenCalledWith('folder-1')
    expect(sessions).toHaveLength(2)
    expect(sessions[1].endedAt).toBe('2026-08-15T11:00:00Z')
  })
})

describe('requestOpeningTurn', () => {
  it('forwards sessionId and topic', async () => {
    // Given a RequestOpeningTurn call that succeeds
    vi.mocked(RequestOpeningTurn).mockResolvedValueOnce()

    // When requesting the opening turn
    await requestOpeningTurn('session-1', 'Distributed systems')

    // Then it forwarded both arguments
    expect(RequestOpeningTurn).toHaveBeenCalledWith('session-1', 'Distributed systems')
  })
})

describe('sendStudyMessage', () => {
  it('forwards sessionId, topic and content', async () => {
    // Given a SendStudyMessage call that succeeds
    vi.mocked(SendStudyMessage).mockResolvedValueOnce()

    // When sending a message
    await sendStudyMessage('session-1', 'Distributed systems', 'What is CAP theorem?')

    // Then it forwarded every argument
    expect(SendStudyMessage).toHaveBeenCalledWith(
      'session-1',
      'Distributed systems',
      'What is CAP theorem?',
    )
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

describe('deleteStudySession', () => {
  it('forwards the sessionId', async () => {
    // Given a DeleteStudySession call that succeeds
    vi.mocked(DeleteStudySession).mockResolvedValueOnce()

    // When deleting a session
    await deleteStudySession('session-1')

    // Then it forwarded the sessionId
    expect(DeleteStudySession).toHaveBeenCalledWith('session-1')
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
