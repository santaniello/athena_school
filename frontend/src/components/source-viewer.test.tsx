import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SourceViewer } from './source-viewer'
import { getSessionSourceDocument } from '@/lib/sources'

vi.mock('@/lib/sources', () => ({
  getSessionSourceDocument: vi.fn(),
}))

const document = {
  itemId: 'item-1',
  title: 'Distributed Systems',
  path: 'notes/ds.md',
  segments: [
    { text: 'Intro. ', chunkId: '' },
    { text: 'The scheduler multiplexes M:N goroutines.', chunkId: 'chunk-1' },
  ],
}

describe('SourceViewer', () => {
  it('shows a loading state while the document loads', () => {
    // Given a document that never resolves
    vi.mocked(getSessionSourceDocument).mockReturnValueOnce(new Promise(() => {}))

    // When the viewer renders
    render(<SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} />)

    // Then it shows a loading message
    expect(screen.getByText('Loading document…')).toBeInTheDocument()
  })

  it('renders the title, path and every segment once loaded', async () => {
    // Given a document with two segments
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(document)

    // When the viewer renders
    render(<SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} />)

    // Then the title/path and both segments' text appear
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())
    expect(screen.getByText('notes/ds.md')).toBeInTheDocument()
    expect(screen.getByText('Intro.', { exact: false })).toBeInTheDocument()
    expect(
      screen.getByText('The scheduler multiplexes M:N goroutines.', { exact: false }),
    ).toBeInTheDocument()
  })

  it('tags each segment with its own chunk id, and leaves a gap segment untagged', async () => {
    // Given the same document
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(document)

    // When the viewer renders
    const { container } = render(
      <SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} />,
    )
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())

    // Then the cited segment carries its chunk id and the gap does not
    const tagged = container.querySelector('[data-chunk-id="chunk-1"]')
    expect(tagged).toHaveTextContent('The scheduler multiplexes M:N goroutines.')
    const gap = screen.getByText('Intro.', { exact: false })
    expect(gap).not.toHaveAttribute('data-chunk-id')
  })

  it('calls onBack when the "Sources" button is clicked', async () => {
    // Given a loaded document and a back handler
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(document)
    const onBack = vi.fn()
    const user = userEvent.setup()

    // When clicking the back button
    render(<SourceViewer sessionId="session-1" itemId="item-1" onBack={onBack} />)
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: /sources/i }))

    // Then the handler fires
    expect(onBack).toHaveBeenCalledOnce()
  })

  it('shows a "re-import to open" message for a document with no stored text', async () => {
    // Given a document imported before spec 2.18
    vi.mocked(getSessionSourceDocument).mockRejectedValueOnce(
      new Error('ingest: source text unavailable'),
    )

    // When the viewer renders
    render(<SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} />)

    // Then it explains that a re-import is needed
    await waitFor(() =>
      expect(screen.getByText(/re-import this document to open it here/i)).toBeInTheDocument(),
    )
  })

  it('shows a "removed" message for a document that no longer exists', async () => {
    // Given a document removed from the session since the citation/row was shown
    vi.mocked(getSessionSourceDocument).mockRejectedValueOnce(new Error('ingest: source not found'))

    // When the viewer renders
    render(<SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} />)

    // Then it explains the source was removed
    await waitFor(() =>
      expect(screen.getByText(/this source was removed from the session/i)).toBeInTheDocument(),
    )
  })

  it('shows a generic error with retry for any other failure', async () => {
    // Given an unrelated failure
    vi.mocked(getSessionSourceDocument).mockRejectedValueOnce(new Error('database unavailable'))

    // When the viewer renders
    render(<SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} />)

    // Then the raw error message and a Retry button appear
    await waitFor(() => expect(screen.getByText('database unavailable')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })

  it('retries the load when Retry is clicked', async () => {
    // Given a failed load followed by a successful retry
    vi.mocked(getSessionSourceDocument).mockRejectedValueOnce(new Error('database unavailable'))
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(document)
    const user = userEvent.setup()

    // When the viewer renders, fails, then Retry is clicked
    render(<SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} />)
    await waitFor(() => expect(screen.getByText('database unavailable')).toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    // Then the document loads successfully
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())
    expect(getSessionSourceDocument).toHaveBeenCalledTimes(2)
  })

  it('highlights the segment matching the given chunkId', async () => {
    // Given a document and a citation pointing at its second segment
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(document)

    // When the viewer renders with that chunkId
    render(
      <SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} chunkId="chunk-1" />,
    )
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())

    // Then only the matching segment carries the highlight class
    const highlighted = screen.getByText('The scheduler multiplexes M:N goroutines.', {
      exact: false,
    })
    expect(highlighted).toHaveClass('citation-highlight')
    const untouched = screen.getByText('Intro.', { exact: false })
    expect(untouched).not.toHaveClass('citation-highlight')
  })

  it('scrolls the matching segment into view once the document loads', async () => {
    // Given a document and a citation pointing at its second segment
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(document)
    const scrollIntoView = vi
      .spyOn(Element.prototype, 'scrollIntoView')
      .mockImplementation(() => {})

    // When the viewer renders with that chunkId
    render(
      <SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} chunkId="chunk-1" />,
    )
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())

    // Then exactly the matching segment was scrolled into view, centered —
    // not the gap segment, and not with default scroll options
    const target = screen.getByText('The scheduler multiplexes M:N goroutines.', { exact: false })
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalledTimes(1))
    expect(scrollIntoView.mock.instances[0]).toBe(target)
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'center' })
  })

  it('re-scrolls and remounts the highlight when reopenToken changes for the same chunkId', async () => {
    // Given a viewer already open at chunk-1
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(document)
    const scrollIntoView = vi
      .spyOn(Element.prototype, 'scrollIntoView')
      .mockImplementation(() => {})
    const { rerender } = render(
      <SourceViewer
        sessionId="session-1"
        itemId="item-1"
        onBack={vi.fn()}
        chunkId="chunk-1"
        reopenToken={1}
      />,
    )
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())
    const firstNode = screen.getByText('The scheduler multiplexes M:N goroutines.', {
      exact: false,
    })
    expect(scrollIntoView).toHaveBeenCalledTimes(1)

    // When re-clicking the exact same citation (sessionId/itemId/chunkId all
    // unchanged, only reopenToken bumped)
    rerender(
      <SourceViewer
        sessionId="session-1"
        itemId="item-1"
        onBack={vi.fn()}
        chunkId="chunk-1"
        reopenToken={2}
      />,
    )

    // Then it scrolls again, and the highlighted segment is a fresh DOM node
    // (remounted, so its fade animation restarts) rather than the same one
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalledTimes(2))
    const secondNode = screen.getByText('The scheduler multiplexes M:N goroutines.', {
      exact: false,
    })
    expect(secondNode).not.toBe(firstNode)
    expect(secondNode).toHaveClass('citation-highlight')
  })

  it('does not scroll anything when no chunkId is given', async () => {
    // Given the same document, opened without a citation (a plain row click)
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(document)
    const scrollIntoView = vi
      .spyOn(Element.prototype, 'scrollIntoView')
      .mockImplementation(() => {})

    // When the viewer renders with no chunkId
    render(<SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} />)
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())

    // Then nothing was scrolled
    expect(scrollIntoView).not.toHaveBeenCalled()
  })

  it('does not scroll or throw when chunkId matches no segment in the document', async () => {
    // Given a chunkId that does not exist in this document (e.g. the cited
    // chunk was re-chunked differently since)
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(document)
    const scrollIntoView = vi
      .spyOn(Element.prototype, 'scrollIntoView')
      .mockImplementation(() => {})

    // When the viewer renders with that chunkId
    render(
      <SourceViewer
        sessionId="session-1"
        itemId="item-1"
        onBack={vi.fn()}
        chunkId="no-such-chunk"
      />,
    )
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())

    // Then nothing was scrolled, and nothing is highlighted
    expect(scrollIntoView).not.toHaveBeenCalled()
    expect(
      screen.getByText('The scheduler multiplexes M:N goroutines.', { exact: false }),
    ).not.toHaveClass('citation-highlight')
  })

  it('renders no highlight when no chunkId is given', async () => {
    // Given the same document, opened without a citation (a plain row click)
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce(document)

    // When the viewer renders
    render(<SourceViewer sessionId="session-1" itemId="item-1" onBack={vi.fn()} />)
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())

    // Then no segment carries the highlight class
    expect(
      screen.getByText('The scheduler multiplexes M:N goroutines.', { exact: false }),
    ).not.toHaveClass('citation-highlight')
  })
})
