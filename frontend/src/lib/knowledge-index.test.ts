import { describe, expect, it, vi } from 'vitest'
import { GetKnowledgeIndexStatus, RetryKnowledgeIndex } from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  getKnowledgeIndexStatus,
  onKnowledgeIndexStatus,
  retryKnowledgeIndex,
} from './knowledge-index'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  GetKnowledgeIndexStatus: vi.fn(),
  RetryKnowledgeIndex: vi.fn(),
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
}))

const readyStatus = {
  state: 'ready' as const,
  hasSnapshot: true,
  issues: [],
  lastError: '',
}

describe('getKnowledgeIndexStatus', () => {
  it('returns the current status', async () => {
    // Given a status query that resolves
    vi.mocked(GetKnowledgeIndexStatus).mockResolvedValueOnce(readyStatus as never)

    // When querying the current status
    const status = await getKnowledgeIndexStatus()

    // Then it is returned as-is
    expect(status).toEqual(readyStatus)
  })
})

describe('retryKnowledgeIndex', () => {
  it('returns the outcome of the retry', async () => {
    // Given a retry that resolves
    vi.mocked(RetryKnowledgeIndex).mockResolvedValueOnce(readyStatus as never)

    // When retrying
    const status = await retryKnowledgeIndex()

    // Then the outcome is returned
    expect(status).toEqual(readyStatus)
  })
})

describe('onKnowledgeIndexStatus', () => {
  it('subscribes to the knowledge-index:status event and forwards the payload', () => {
    // Given a handler and a mocked EventsOn
    const unsubscribe = vi.fn()
    vi.mocked(EventsOn).mockReturnValueOnce(unsubscribe)
    const handler = vi.fn()

    // When subscribing
    const result = onKnowledgeIndexStatus(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback(readyStatus)

    // Then the handler received the status payload and the unsubscribe
    // function is returned
    expect(EventsOn).toHaveBeenCalledWith('knowledge-index:status', expect.any(Function))
    expect(handler).toHaveBeenCalledWith(readyStatus)
    expect(result).toBe(unsubscribe)
  })
})
