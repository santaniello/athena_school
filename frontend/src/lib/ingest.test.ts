import { describe, expect, it, vi } from 'vitest'
import {
  ConfirmImport,
  ImportFile,
  PickNotesFile,
  PreviewImport,
} from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  confirmImport,
  importFile,
  onIngestDone,
  onIngestError,
  onIngestProgress,
  pickNotesFile,
  previewImport,
} from './ingest'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  PickNotesFile: vi.fn(),
  ImportFile: vi.fn(),
  PreviewImport: vi.fn(),
  ConfirmImport: vi.fn(),
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
}))

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
  it('forwards the owning session and the chosen path', async () => {
    // Given an import that resolves
    vi.mocked(ImportFile).mockResolvedValueOnce()

    // When importing a single file into a session
    await importFile('session-1', '/home/user/notes/go.md')

    // Then both the session and the path were forwarded
    expect(ImportFile).toHaveBeenCalledWith('session-1', '/home/user/notes/go.md')
  })
})

describe('previewImport', () => {
  it('forwards the owning session and the chosen path, returning the mapped preview', async () => {
    // Given a preview call that resolves with a formatted chunk list
    const preview = {
      displayPath: 'go.md',
      originalContent: 'raw',
      formattedContent: '# Go\n\nFormatted.',
      formattingFailed: false,
      stats: { headingsBefore: 0, headingsAfter: 1, paragraphsBefore: 1, paragraphsAfter: 1 },
      chunks: [{ heading: 'Go', content: 'Formatted.', charCount: 10, estimatedTokens: 2 }],
      skipped: false,
      skippedReason: '',
    }
    vi.mocked(PreviewImport).mockResolvedValueOnce(preview as never)

    // When previewing an import
    const result = await previewImport('session-1', '/home/user/notes/go.md')

    // Then both the session and the path were forwarded, and the preview
    // is returned as-is
    expect(PreviewImport).toHaveBeenCalledWith('session-1', '/home/user/notes/go.md')
    expect(result).toEqual(preview)
  })
})

describe('confirmImport', () => {
  it('forwards the owning session, the chosen path and the approved content', async () => {
    // Given a confirm call that resolves
    vi.mocked(ConfirmImport).mockResolvedValueOnce()

    // When confirming an import
    await confirmImport('session-1', '/home/user/notes/go.md', '# Go\n\nFormatted.')

    // Then the session, path and approved content were all forwarded
    expect(ConfirmImport).toHaveBeenCalledWith(
      'session-1',
      '/home/user/notes/go.md',
      '# Go\n\nFormatted.',
    )
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
