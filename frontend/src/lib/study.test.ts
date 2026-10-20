import { describe, expect, it, vi } from 'vitest'
import {
  CompactStudySession,
  DeleteStudySession,
  GetStudySessionUsage,
  ListStudySessionsByFolder,
  MoveStudySession,
  RequestOpeningTurn,
  ResumeStudySession,
  SendStudyMessage,
  StartStudySession,
} from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  compactStudySession,
  deleteStudySession,
  getStudySessionUsage,
  listStudySessionsByFolder,
  moveStudySession,
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
  startStudySession,
  truncateBreadcrumb,
  type StudyContextUsage,
} from './study'

const CONTEXT_NORMAL: StudyContextUsage = {
  state: 'normal',
  model: '',
  usedTokens: 0,
  contextLength: 0,
  estimated: false,
}

vi.mock('../../wailsjs/go/desktop/App', () => ({
  CompactStudySession: vi.fn(),
  StartStudySession: vi.fn(),
  RequestOpeningTurn: vi.fn(),
  SendStudyMessage: vi.fn(),
  DeleteStudySession: vi.fn(),
  GetStudySessionUsage: vi.fn(),
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
      goal: 'Ace the SQL interview',
      startedAt: '2026-08-16T10:00:00Z',
      context: CONTEXT_NORMAL,
    } as never)

    // When starting a study session in a folder
    const session = await startStudySession(
      'Distributed systems',
      'Ace the SQL interview',
      'folder-1',
    )

    // Then it forwarded the topic, folder id, and goal, and returned the session
    expect(StartStudySession).toHaveBeenCalledWith(
      'Distributed systems',
      'folder-1',
      'Ace the SQL interview',
    )
    expect(session).toEqual({
      id: 'session-1',
      topic: 'Distributed systems',
      folderId: 'folder-1',
      goal: 'Ace the SQL interview',
      startedAt: '2026-08-16T10:00:00Z',
      context: CONTEXT_NORMAL,
    })
  })

  it('defaults folderId to an empty string when omitted', async () => {
    // Given a StartStudySession call that succeeds
    vi.mocked(StartStudySession).mockResolvedValueOnce({
      id: 'session-1',
      topic: 'Distributed systems',
      folderId: 'default',
      goal: 'Ace the SQL interview',
      startedAt: '2026-08-16T10:00:00Z',
      context: CONTEXT_NORMAL,
    } as never)

    // When starting a session without specifying a folder
    await startStudySession('Distributed systems', 'Ace the SQL interview')

    // Then it forwarded an empty folder id, letting the backend fall back
    // to the default folder
    expect(StartStudySession).toHaveBeenCalledWith(
      'Distributed systems',
      '',
      'Ace the SQL interview',
    )
  })
})

describe('getStudySessionUsage', () => {
  it('returns the accumulated usage for the session', async () => {
    // Given a GetStudySessionUsage call that succeeds
    const usage = {
      inputTokens: 10,
      outputTokens: 5,
      cost: 0.01,
      unpricedCalls: 0,
      calls: 2,
      context: { ...CONTEXT_NORMAL, usedTokens: 5000, contextLength: 100000 },
    }
    vi.mocked(GetStudySessionUsage).mockResolvedValueOnce(usage as never)

    // When reading the session usage
    const result = await getStudySessionUsage('session-1')

    // Then it forwarded the sessionId and returned the totals
    expect(GetStudySessionUsage).toHaveBeenCalledWith('session-1')
    expect(result).toEqual(usage)
  })
})

describe('resumeStudySession', () => {
  it('returns the session and its full history', async () => {
    // Given a ResumeStudySession call that succeeds
    vi.mocked(ResumeStudySession).mockResolvedValueOnce({
      session: {
        id: 'session-1',
        topic: 'Distributed systems',
        folderId: 'folder-1',
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
    } as never)

    // When resuming a session
    const result = await resumeStudySession('session-1')

    // Then it forwarded the sessionId and returned the session and history
    expect(ResumeStudySession).toHaveBeenCalledWith('session-1')
    expect(result.session.id).toBe('session-1')
    // toStrictEqual so a dropped id or sources (undefined) cannot slip through
    expect(result.messages).toStrictEqual([
      { id: 'm1', role: 'user', content: 'Hi', createdAt: '2026-08-16T10:00:00Z', sources: [] },
      {
        id: 'm2',
        role: 'assistant',
        content: 'Hello!',
        createdAt: '2026-08-16T10:00:01Z',
        sources: [],
      },
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
        context: CONTEXT_NORMAL,
      },
      {
        id: 's-2',
        topic: 'Concurrency patterns',
        folderId: 'folder-1',
        startedAt: '2026-08-15T10:00:00Z',
        context: CONTEXT_NORMAL,
      },
    ] as never)

    // When listing sessions in a folder
    const sessions = await listStudySessionsByFolder('folder-1')

    // Then it forwarded the folderId and returned every session
    expect(ListStudySessionsByFolder).toHaveBeenCalledWith('folder-1')
    expect(sessions).toHaveLength(2)
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

describe('compactStudySession', () => {
  it('forwards the session id to the binding', async () => {
    // Given a CompactStudySession call that succeeds
    vi.mocked(CompactStudySession).mockResolvedValueOnce()

    // When compacting
    await compactStudySession('session-1')

    // Then the binding received the id
    expect(CompactStudySession).toHaveBeenCalledWith('session-1')
  })

  it('rejects with the binding error when nothing was compacted', async () => {
    // Given a binding that fails
    vi.mocked(CompactStudySession).mockRejectedValueOnce(new Error('study: nothing to compact'))

    // When compacting
    // Then the error reaches the caller
    await expect(compactStudySession('session-1')).rejects.toThrow('nothing to compact')
  })
})

describe('compaction event wrappers', () => {
  it.each([
    ['study:context-compacting', onStudyContextCompacting, { sessionId: 's1' }],
    [
      'study:context-compacted',
      onStudyContextCompacted,
      { sessionId: 's1', usedTokens: 40, contextLength: 100 },
    ],
    [
      'study:compaction-failed',
      onStudyCompactionFailed,
      { sessionId: 's1', reason: 'invalid format' },
    ],
  ])('%s subscribes and forwards the payload', (eventName, subscribe, payload) => {
    // Given a handler and a mocked EventsOn
    const unsubscribe = vi.fn()
    vi.mocked(EventsOn).mockReturnValueOnce(unsubscribe)
    const handler = vi.fn()

    // When subscribing and the event fires
    const result = (subscribe as (h: (e: unknown) => void) => () => void)(handler)
    const [name, callback] = vi.mocked(EventsOn).mock.calls.at(-1)!
    callback(payload)

    // Then it listens to the right event, forwards the payload and hands back the unsubscribe
    expect(name).toBe(eventName)
    expect(handler).toHaveBeenCalledWith(payload)
    expect(result).toBe(unsubscribe)
  })
})

describe('sendStudyMessage', () => {
  it('forwards sessionId, topic, content and sourceMode', async () => {
    // Given a SendStudyMessage call that succeeds
    vi.mocked(SendStudyMessage).mockResolvedValueOnce()

    // When sending a message in notes mode
    await sendStudyMessage('session-1', 'Distributed systems', 'What is CAP theorem?', 'notes')

    // Then it forwarded every argument
    expect(SendStudyMessage).toHaveBeenCalledWith(
      'session-1',
      'Distributed systems',
      'What is CAP theorem?',
      'notes',
    )
  })

  it('forwards a non-default source mode', async () => {
    // Given a SendStudyMessage call that succeeds
    vi.mocked(SendStudyMessage).mockResolvedValueOnce()

    // When sending a message in strict-notes mode
    await sendStudyMessage(
      'session-1',
      'Distributed systems',
      'What is CAP theorem?',
      'strict-notes',
    )

    // Then the chosen mode is forwarded as-is
    expect(SendStudyMessage).toHaveBeenCalledWith(
      'session-1',
      'Distributed systems',
      'What is CAP theorem?',
      'strict-notes',
    )
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
  it('subscribes to the study:chunk event and forwards the structured payload', () => {
    // Given a handler and a mocked EventsOn
    const unsubscribe = vi.fn()
    vi.mocked(EventsOn).mockReturnValueOnce(unsubscribe)
    const handler = vi.fn()

    // When subscribing
    const result = onStudyChunk(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback({ sessionId: 'session-1', content: 'Hello there!' })

    // Then the handler received the sessionId and content, and the
    // unsubscribe function is returned
    expect(EventsOn).toHaveBeenCalledWith('study:chunk', expect.any(Function))
    expect(handler).toHaveBeenCalledWith({ sessionId: 'session-1', content: 'Hello there!' })
    expect(result).toBe(unsubscribe)
  })
})

describe('onStudyDone', () => {
  it('subscribes to the study:done event and forwards the sessionId', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()

    // When subscribing
    onStudyDone(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback({ sessionId: 'session-1' })

    // Then the handler fired with the sessionId
    expect(EventsOn).toHaveBeenCalledWith('study:done', expect.any(Function))
    expect(handler).toHaveBeenCalledWith({ sessionId: 'session-1' })
  })
})

describe('onStudyError', () => {
  it('subscribes to the study:error event and forwards the sessionId, message and code', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()

    // When subscribing
    onStudyError(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback({ sessionId: 'session-1', message: 'upstream failure', code: '' })

    // Then the handler received the sessionId, error message and code
    expect(EventsOn).toHaveBeenCalledWith('study:error', expect.any(Function))
    expect(handler).toHaveBeenCalledWith({
      sessionId: 'session-1',
      message: 'upstream failure',
      code: '',
    })
  })

  it('forwards a stable error code when one is present', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()

    // When subscribing and a pre-persistence error fires
    onStudyError(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback({ sessionId: 'session-1', message: 'blocked', code: 'context_limit_reached' })

    // Then the code is forwarded as-is
    expect(handler).toHaveBeenCalledWith({
      sessionId: 'session-1',
      message: 'blocked',
      code: 'context_limit_reached',
    })
  })
})

describe.each([
  ['onStudyContextNormal', onStudyContextNormal, 'study:context-normal'],
  ['onStudyContextWarning', onStudyContextWarning, 'study:context-warning'],
  ['onStudyContextLimitReached', onStudyContextLimitReached, 'study:context-limit-reached'],
])('%s', (_name, subscribe, eventName) => {
  it(`subscribes to ${'the event'} and forwards the payload`, () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()
    const payload = {
      sessionId: 'session-1',
      usedTokens: 8000,
      contextLength: 10000,
      estimated: true,
    }

    // When subscribing
    subscribe(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback(payload)

    // Then it subscribed to the right event name and forwarded the payload
    expect(EventsOn).toHaveBeenCalledWith(eventName, expect.any(Function))
    expect(handler).toHaveBeenCalledWith(payload)
  })
})

describe('onStudyContextLimitUnavailable', () => {
  it('subscribes to the study:context-limit-unavailable event and forwards the payload', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()

    // When subscribing
    onStudyContextLimitUnavailable(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback({
      sessionId: 'session-1',
      message: "Unable to determine this session's context limit.",
    })

    // Then the handler received the sessionId and message
    expect(EventsOn).toHaveBeenCalledWith('study:context-limit-unavailable', expect.any(Function))
    expect(handler).toHaveBeenCalledWith({
      sessionId: 'session-1',
      message: "Unable to determine this session's context limit.",
    })
  })
})

describe('onStudySources', () => {
  it('subscribes to the study:sources event and forwards the sessionId and sources', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()
    const sources = [
      {
        sourceType: 'imported_doc',
        filePath: 'notes/a.md',
        heading: 'H',
        concept: 'Channels',
        score: 0.68,
      },
    ]

    // When subscribing
    onStudySources(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback({ sessionId: 'session-1', sources })

    // Then the handler received the sessionId and sources
    expect(EventsOn).toHaveBeenCalledWith('study:sources', expect.any(Function))
    expect(handler).toHaveBeenCalledWith({ sessionId: 'session-1', sources })
  })
})

describe('truncateBreadcrumb', () => {
  it('returns the breadcrumb unchanged when already within maxLength', () => {
    expect(truncateBreadcrumb('Física > Cinemática', 60)).toBe('Física > Cinemática')
  })

  it('collapses earlier segments behind an ellipsis, keeping the leaf intact', () => {
    const breadcrumb = 'Arquitetura > Monolito > Tradeoffs > Vantagens'
    expect(truncateBreadcrumb(breadcrumb, 20)).toBe('… > Vantagens')
  })

  it('keeps as many ancestor segments as fit under the budget', () => {
    const breadcrumb = 'Arquitetura > Monolito > Tradeoffs > Vantagens'
    expect(truncateBreadcrumb(breadcrumb, 30)).toBe('… > Tradeoffs > Vantagens')
  })

  it('never exceeds maxLength even when the leaf segment alone already fills it', () => {
    // Given a breadcrumb whose last segment alone is already longer than
    // maxLength — collapsing ancestors can't help here
    const longLeaf = 'A'.repeat(80)
    const breadcrumb = `Parent > ${longLeaf}`

    const result = truncateBreadcrumb(breadcrumb, 60)

    expect(result.length).toBeLessThanOrEqual(60)
    expect(result.endsWith('…')).toBe(true)
  })

  it('does not prepend an ellipsis for a single segment with no ancestors to collapse', () => {
    const longLeaf = 'A'.repeat(80)
    expect(truncateBreadcrumb(longLeaf, 60)).toBe(`${'A'.repeat(59)}…`)
  })
})
