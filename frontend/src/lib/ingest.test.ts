import { describe, expect, it, vi } from 'vitest'
import { ImportFile, ImportNotes, PickNotesFile, PickNotesFolder } from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  importFile,
  importNotes,
  onIngestDone,
  onIngestError,
  onIngestProgress,
  pickNotesFile,
  pickNotesFolder,
} from './ingest'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  PickNotesFolder: vi.fn(),
  ImportNotes: vi.fn(),
  PickNotesFile: vi.fn(),
  ImportFile: vi.fn(),
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
}))

describe('pickNotesFolder', () => {
  it('returns the path chosen by the OS dialog', async () => {
    // Given a folder picker that resolves to a chosen path
    vi.mocked(PickNotesFolder).mockResolvedValueOnce('/home/user/notes')

    // When picking a notes folder
    const path = await pickNotesFolder()

    // Then the chosen path is returned
    expect(path).toBe('/home/user/notes')
  })
})

describe('importNotes', () => {
  it('forwards the chosen path', async () => {
    // Given an import that resolves
    vi.mocked(ImportNotes).mockResolvedValueOnce()

    // When importing a folder
    await importNotes('/home/user/notes')

    // Then the path was forwarded
    expect(ImportNotes).toHaveBeenCalledWith('/home/user/notes')
  })
})

describe('pickNotesFile', () => {
  it('returns the path chosen by the OS dialog', async () => {
    // Given a file picker that resolves to a chosen path
    vi.mocked(PickNotesFile).mockResolvedValueOnce('/home/user/notes/go.md')

    // When picking a notes file
    const path = await pickNotesFile()

    // Then the chosen path is returned
    expect(path).toBe('/home/user/notes/go.md')
  })
})

describe('importFile', () => {
  it('forwards the chosen path', async () => {
    // Given an import that resolves
    vi.mocked(ImportFile).mockResolvedValueOnce()

    // When importing a single file
    await importFile('/home/user/notes/go.md')

    // Then the path was forwarded
    expect(ImportFile).toHaveBeenCalledWith('/home/user/notes/go.md')
  })
})

describe('onIngestProgress', () => {
  it('subscribes to the ingest:progress event and forwards the payload', () => {
    // Given a handler and a mocked EventsOn
    const unsubscribe = vi.fn()
    vi.mocked(EventsOn).mockReturnValueOnce(unsubscribe)
    const handler = vi.fn()
    const progress = { filesProcessed: 1, filesTotal: 10, chunksCreated: 3, currentFile: 'go.md' }

    // When subscribing
    const result = onIngestProgress(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback(progress)

    // Then the handler received the progress payload and the unsubscribe
    // function is returned
    expect(EventsOn).toHaveBeenCalledWith('ingest:progress', expect.any(Function))
    expect(handler).toHaveBeenCalledWith(progress)
    expect(result).toBe(unsubscribe)
  })
})

describe('onIngestDone', () => {
  it('subscribes to the ingest:done event and forwards the summary', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()
    const summary = {
      filesScanned: 10,
      filesIngested: 8,
      filesSkipped: 2,
      filesFailed: 0,
      chunksCreated: 24,
      failures: [],
    }

    // When subscribing
    onIngestDone(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback(summary)

    // Then the handler received the summary
    expect(EventsOn).toHaveBeenCalledWith('ingest:done', expect.any(Function))
    expect(handler).toHaveBeenCalledWith(summary)
  })
})

describe('onIngestError', () => {
  it('subscribes to the ingest:error event and forwards the message', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()

    // When subscribing
    onIngestError(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback('folder does not exist')

    // Then the handler received the error message
    expect(EventsOn).toHaveBeenCalledWith('ingest:error', expect.any(Function))
    expect(handler).toHaveBeenCalledWith('folder does not exist')
  })
})
