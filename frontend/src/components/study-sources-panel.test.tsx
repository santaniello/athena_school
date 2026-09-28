import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StudySourcesPanel } from './study-sources-panel'
import { listSessionSources } from '@/lib/sources'

vi.mock('@/lib/sources', () => ({
  listSessionSources: vi.fn(),
}))

const DISTRIBUTED_SYSTEMS = {
  itemId: 'item-1',
  title: 'Distributed Systems',
  path: 'notes/ds.md',
  chunkCount: 12,
  ingestedAt: '2024-01-01T00:00:00Z',
}

const CAP_THEOREM = {
  itemId: 'item-2',
  title: 'CAP theorem',
  path: 'cap.md',
  chunkCount: 1,
  ingestedAt: '2024-01-02T00:00:00Z',
}

describe('StudySourcesPanel', () => {
  it('shows a loading state before the sources arrive', () => {
    // Given a load that never resolves during this test
    vi.mocked(listSessionSources).mockImplementationOnce(() => new Promise(() => {}))

    // When the panel mounts
    render(<StudySourcesPanel sessionId="session-1" />)

    // Then a loading message shows, not the empty state
    expect(screen.getByText('Loading sources…')).toBeInTheDocument()
    expect(screen.queryByText('No sources imported yet.')).not.toBeInTheDocument()
  })

  it('shows an empty state when the session has no imported documents', async () => {
    // Given a session with nothing imported
    vi.mocked(listSessionSources).mockResolvedValueOnce([])

    // When the panel loads
    render(<StudySourcesPanel sessionId="session-1" />)

    // Then the empty state shows, with its own explanation and Add action
    expect(await screen.findByText('No sources imported yet.')).toBeInTheDocument()
    expect(
      screen.getByText('The chat searches these documents when answering.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add a source' })).toBeInTheDocument()
  })

  it('shows every imported document, oldest first, with its path and chunk count', async () => {
    // Given a session with two imported documents
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])

    // When the panel loads
    render(<StudySourcesPanel sessionId="session-1" />)

    // Then both render with their title, path and chunk count, singular for one chunk
    expect(await screen.findByText('Distributed Systems')).toBeInTheDocument()
    expect(screen.getByText('notes/ds.md · 12 chunks')).toBeInTheDocument()
    expect(screen.getByText('CAP theorem')).toBeInTheDocument()
    expect(screen.getByText('cap.md · 1 chunk')).toBeInTheDocument()
  })

  it('shows the header count for every loaded document', async () => {
    // Given two imported documents
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])

    // When the panel loads
    render(<StudySourcesPanel sessionId="session-1" />)

    // Then the header shows the total count
    expect(await screen.findByText('Sources (2)')).toBeInTheDocument()
  })

  it('filters the list by title or path as the user types', async () => {
    // Given two imported documents, loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When searching for the other document's title
    await user.type(screen.getByPlaceholderText('Search sources'), 'cap theorem')

    // Then only the matching document remains
    expect(screen.getByText('CAP theorem')).toBeInTheDocument()
    expect(screen.queryByText('Distributed Systems')).not.toBeInTheDocument()
  })

  it('matches the search against the path too', async () => {
    // Given two imported documents, loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When searching for a path fragment
    await user.type(screen.getByPlaceholderText('Search sources'), 'notes/ds')

    // Then only the matching document remains
    expect(screen.getByText('Distributed Systems')).toBeInTheDocument()
    expect(screen.queryByText('CAP theorem')).not.toBeInTheDocument()
  })

  it('trims leading and trailing whitespace from the search query', async () => {
    // Given two imported documents, loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When searching with surrounding whitespace around an otherwise
    // matching term
    await user.type(screen.getByPlaceholderText('Search sources'), '  CAP theorem  ')

    // Then it still matches, trimmed
    expect(screen.getByText('CAP theorem')).toBeInTheDocument()
    expect(screen.queryByText('Distributed Systems')).not.toBeInTheDocument()
  })

  it('shows a no-match message when the search matches nothing', async () => {
    // Given one imported document, loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When searching for something that matches nothing
    await user.type(screen.getByPlaceholderText('Search sources'), 'nonexistent')

    // Then a no-match message shows, distinct from the empty-session message
    expect(screen.getByText('No sources match your search.')).toBeInTheDocument()
  })

  it('shows an error with a retry action when loading fails', async () => {
    // Given a load that fails
    vi.mocked(listSessionSources).mockRejectedValueOnce(new Error('database unavailable'))

    // When the panel loads
    render(<StudySourcesPanel sessionId="session-1" />)

    // Then the error shows with a Retry action, instead of the list/empty state
    expect(await screen.findByText('database unavailable')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })

  it('retrying reloads the sources', async () => {
    // Given a load that failed once
    vi.mocked(listSessionSources).mockRejectedValueOnce(new Error('database unavailable'))
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByRole('button', { name: 'Retry' })

    // When retrying, and the second attempt succeeds
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    // Then the list shows
    expect(await screen.findByText('Distributed Systems')).toBeInTheDocument()
  })

  it('reloads when sessionId changes', async () => {
    // Given session-1's document loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    const { rerender } = render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When the panel is handed a different session
    vi.mocked(listSessionSources).mockResolvedValueOnce([CAP_THEOREM])
    rerender(<StudySourcesPanel sessionId="session-2" />)

    // Then it loads and shows that session's documents instead
    expect(await screen.findByText('CAP theorem')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText('Distributed Systems')).not.toBeInTheDocument())
    expect(listSessionSources).toHaveBeenCalledWith('session-2')
  })

  it('renders Add and Remove as disabled, not silently inert', async () => {
    // Given one document loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // Then both controls are visibly disabled — Add and Remove are wired in
    // a later increment (specs/phases/phase-02-knowledge-engine/17-session-sources-panel.md)
    expect(screen.getByRole('button', { name: 'Add source' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Remove Distributed Systems' })).toBeDisabled()
  })
})
