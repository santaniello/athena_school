import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { getStudySessionSummary } from '@/lib/study'
import { SummaryDialog } from './summary-dialog'

vi.mock('@/lib/study', () => ({
  getStudySessionSummary: vi.fn(),
}))

const SUMMARY = {
  found: true,
  content: '## Progress\n- covered B-trees\n\n## Pending\n- exercise on indexes',
  coversUntilMessageId: 'm3',
  updatedAt: '2026-10-21T12:00:00Z',
}

describe('SummaryDialog', () => {
  it('renders nothing and reads nothing while closed', () => {
    // Given a closed dialog
    // When it renders
    render(<SummaryDialog open={false} sessionId="session-1" onClose={vi.fn()} />)

    // Then no summary is requested
    expect(getStudySessionSummary).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows the summary sections as formatted text once loaded', async () => {
    // Given a session with a summary
    vi.mocked(getStudySessionSummary).mockResolvedValueOnce(SUMMARY)

    // When the dialog opens
    render(<SummaryDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then the sections are rendered as headings and lists
    expect(await screen.findByRole('heading', { name: 'Progress' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Pending' })).toBeInTheDocument()
    expect(screen.getByText('covered B-trees')).toBeInTheDocument()
    expect(getStudySessionSummary).toHaveBeenCalledWith('session-1')
  })

  it('never renders raw HTML or live links from the untrusted summary', async () => {
    // Given a summary carrying raw HTML and a link
    vi.mocked(getStudySessionSummary).mockResolvedValueOnce({
      ...SUMMARY,
      content: '## Progress\n<img src=x alt="tracker">\n\nsee [docs](https://example.com)',
    })

    // When the dialog opens
    render(<SummaryDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then neither an image nor an anchor is created
    await screen.findByRole('heading', { name: 'Progress' })
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.getByText('docs')).toBeInTheDocument()
    expect(screen.getByText(/<img src=x/)).toBeInTheDocument()
  })

  it('never loads a remote image from markdown image syntax in the summary', async () => {
    // Given a summary carrying a markdown image that would call a remote host
    vi.mocked(getStudySessionSummary).mockResolvedValueOnce({
      ...SUMMARY,
      content: '## Progress\n\n![tracker](https://example.com/x.png?d=secret)',
    })

    // When the dialog opens
    render(<SummaryDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then no image element exists, so nothing is fetched, and the alt text stays readable
    await screen.findByRole('heading', { name: 'Progress' })
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(document.querySelector('img')).toBeNull()
    expect(screen.getByText('tracker')).toBeInTheDocument()
  })

  it('says so when the session has no summary', async () => {
    // Given a session without a summary
    vi.mocked(getStudySessionSummary).mockResolvedValueOnce({
      found: false,
      content: '',
      coversUntilMessageId: '',
      updatedAt: '',
    })

    // When the dialog opens
    render(<SummaryDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then an explanation is shown
    expect(await screen.findByText('This session has no summary yet.')).toBeInTheDocument()
  })

  it('shows the failure when the summary cannot be read', async () => {
    // Given a read that fails
    vi.mocked(getStudySessionSummary).mockRejectedValueOnce(new Error('database unavailable'))

    // When the dialog opens
    render(<SummaryDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then the error is shown
    expect(await screen.findByText('database unavailable')).toBeInTheDocument()
  })

  it('falls back to a generic message for a non-Error failure', async () => {
    // Given a read that rejects with a plain string
    vi.mocked(getStudySessionSummary).mockRejectedValueOnce('boom')

    // When the dialog opens
    render(<SummaryDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then the generic message is shown
    expect(await screen.findByText('Failed to load the summary.')).toBeInTheDocument()
  })

  it('calls onClose when the user closes it', async () => {
    // Given an open dialog
    vi.mocked(getStudySessionSummary).mockResolvedValueOnce(SUMMARY)
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<SummaryDialog open sessionId="session-1" onClose={onClose} />)
    await screen.findByRole('heading', { name: 'Progress' })

    // When clicking Close
    await user.click(screen.getAllByRole('button', { name: 'Close' })[0])

    // Then the caller is told
    expect(onClose).toHaveBeenCalled()
  })
})
