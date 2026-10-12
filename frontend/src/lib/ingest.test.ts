import { describe, expect, it, vi } from 'vitest'
import {
  ConfirmImport,
  ConfirmLinkImport,
  ImportFile,
  PickNotesFile,
  PreviewImport,
  PreviewLinkImport,
} from '../../wailsjs/go/desktop/App'
import { BrowserOpenURL, EventsOn } from '../../wailsjs/runtime/runtime'
import {
  confirmImport,
  confirmLinkImport,
  importFile,
  linkErrorMessage,
  openOriginalLink,
  onIngestDone,
  onIngestError,
  onIngestProgress,
  pickNotesFile,
  previewImport,
  previewLinkImport,
} from './ingest'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  PickNotesFile: vi.fn(),
  ImportFile: vi.fn(),
  PreviewImport: vi.fn(),
  ConfirmImport: vi.fn(),
  PreviewLinkImport: vi.fn(),
  ConfirmLinkImport: vi.fn(),
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  BrowserOpenURL: vi.fn(),
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

describe('previewLinkImport', () => {
  it('forwards the session and the pasted URL, returning the mapped preview', async () => {
    // Given a link preview that resolves
    const preview = { displayPath: 'Go Article', sourceUrl: 'https://example.com/a' }
    vi.mocked(PreviewLinkImport).mockResolvedValueOnce(preview as never)

    // When previewing a link
    const result = await previewLinkImport('session-1', 'https://example.com/a')

    // Then the session and URL were forwarded and the preview returned
    expect(PreviewLinkImport).toHaveBeenCalledWith('session-1', 'https://example.com/a')
    expect(result).toBe(preview)
  })
})

describe('confirmLinkImport', () => {
  it('forwards the session, URL and exactly the approved content', async () => {
    // Given a confirm call that resolves
    vi.mocked(ConfirmLinkImport).mockResolvedValueOnce()

    // When confirming a link import
    await confirmLinkImport('session-1', 'https://example.com/a', '# approved')

    // Then every argument was forwarded in order
    expect(ConfirmLinkImport).toHaveBeenCalledWith(
      'session-1',
      'https://example.com/a',
      '# approved',
    )
  })
})

describe('openOriginalLink', () => {
  it('opens the URL in the system browser', () => {
    // Given an http(s) URL
    // When opening the original page
    openOriginalLink('https://example.com/a')

    // Then the Wails runtime opens it externally
    expect(BrowserOpenURL).toHaveBeenCalledWith('https://example.com/a')
  })
})

describe('linkErrorMessage', () => {
  it.each([
    ['web: host is not allowed', 'private or local'],
    ['ingest: fetching https://x: web: unsupported content type', 'web page or text'],
    ['web: page is too large', 'too large'],
    ['web: page has no readable text', 'JavaScript or a login'],
    ['web: invalid URL', 'valid http'],
    ['knowledge index is loading', 'try again'],
  ])('maps %s to friendly copy', (raw, expectedPart) => {
    // Given a raw backend error, as a string (how Wails rejects) or an Error
    // When mapping it to user-facing copy
    // Then the message is understandable and names the actionable cause
    expect(linkErrorMessage(raw)).toContain(expectedPart)
    expect(linkErrorMessage(new Error(raw))).toContain(expectedPart)
  })

  it('falls back to a generic message for an unknown error', () => {
    // Given an error nothing recognizes
    // When mapping it
    // Then the generic copy is used
    expect(linkErrorMessage({ weird: true })).toBe('Failed to read the link. Please try again.')
  })
})
