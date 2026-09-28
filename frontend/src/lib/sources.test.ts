import { describe, expect, it, vi } from 'vitest'
import { ListSessionSources, RemoveSessionSource } from '../../wailsjs/go/desktop/App'
import { listSessionSources, removeSessionSource } from './sources'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  ListSessionSources: vi.fn(),
  RemoveSessionSource: vi.fn(),
}))

describe('listSessionSources', () => {
  it('forwards the session id and returns every source', async () => {
    // Given a session with two imported documents
    vi.mocked(ListSessionSources).mockResolvedValueOnce([
      { itemId: 'item-1', title: 'Distributed Systems', path: 'notes/ds.md', chunkCount: 12, ingestedAt: '2024-01-01T00:00:00Z' },
      { itemId: 'item-2', title: 'CAP theorem', path: 'cap.md', chunkCount: 4, ingestedAt: '2024-01-02T00:00:00Z' },
    ] as never)

    // When listing the session's sources
    const sources = await listSessionSources('session-1')

    // Then the session id was forwarded and every source is returned
    expect(ListSessionSources).toHaveBeenCalledWith('session-1')
    expect(sources).toEqual([
      { itemId: 'item-1', title: 'Distributed Systems', path: 'notes/ds.md', chunkCount: 12, ingestedAt: '2024-01-01T00:00:00Z' },
      { itemId: 'item-2', title: 'CAP theorem', path: 'cap.md', chunkCount: 4, ingestedAt: '2024-01-02T00:00:00Z' },
    ])
  })

  it('propagates a failure', async () => {
    // Given a call that fails
    vi.mocked(ListSessionSources).mockRejectedValueOnce(new Error('database unavailable'))

    // When listing the session's sources
    // Then the failure propagates
    await expect(listSessionSources('session-1')).rejects.toThrow('database unavailable')
  })
})

describe('removeSessionSource', () => {
  it('forwards the session and item ids', async () => {
    // Given a removal that succeeds
    vi.mocked(RemoveSessionSource).mockResolvedValueOnce()

    // When removing a source
    await removeSessionSource('session-1', 'item-1')

    // Then both ids were forwarded
    expect(RemoveSessionSource).toHaveBeenCalledWith('session-1', 'item-1')
  })

  it('propagates a failure', async () => {
    // Given a removal that fails (e.g. the item belongs to another session)
    vi.mocked(RemoveSessionSource).mockRejectedValueOnce(new Error('ingest: source not found'))

    // When removing it
    // Then the failure propagates
    await expect(removeSessionSource('session-1', 'item-1')).rejects.toThrow('ingest: source not found')
  })
})
