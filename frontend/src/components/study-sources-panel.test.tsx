import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StudySourcesPanel } from './study-sources-panel'
import type { StudySource } from '@/lib/study'

const IMPORTED_DOC: StudySource = {
  sourceType: 'imported_doc',
  filePath: 'notes/distributed-systems.md',
  heading: 'CAP theorem',
  concept: '',
  score: 0.68,
}

const USER_NOTE: StudySource = {
  sourceType: 'user_note',
  filePath: '',
  heading: '',
  concept: 'Idempotency',
  score: 0.72,
}

describe('StudySourcesPanel', () => {
  it('shows an empty state when the session has no sources yet', () => {
    // Given no sources cited yet
    render(<StudySourcesPanel sources={[]} />)

    // Then an empty-state message shows instead of a list
    expect(screen.getByText('No sources cited in this session yet.')).toBeInTheDocument()
  })

  it('lists every distinct source with its label', () => {
    // Given a document and a user note
    render(<StudySourcesPanel sources={[IMPORTED_DOC, USER_NOTE]} />)

    // Then both render with their sourceLabel mapping
    expect(screen.getByText('notes/distributed-systems.md')).toBeInTheDocument()
    expect(screen.getByText('CAP theorem')).toBeInTheDocument()
    expect(screen.getByText('User note')).toBeInTheDocument()
    expect(screen.getByText('Idempotency')).toBeInTheDocument()
  })

  it('filters the list by title or subtitle as the user types', async () => {
    // Given both sources rendered
    const user = userEvent.setup()
    render(<StudySourcesPanel sources={[IMPORTED_DOC, USER_NOTE]} />)

    // When searching for the user note's concept
    await user.type(screen.getByPlaceholderText('Search sources'), 'idempot')

    // Then only the matching source remains
    expect(screen.getByText('Idempotency')).toBeInTheDocument()
    expect(screen.queryByText('notes/distributed-systems.md')).not.toBeInTheDocument()
  })

  it('shows a no-match message when the search matches nothing', async () => {
    // Given one source rendered
    const user = userEvent.setup()
    render(<StudySourcesPanel sources={[IMPORTED_DOC]} />)

    // When searching for something that matches nothing
    await user.type(screen.getByPlaceholderText('Search sources'), 'nonexistent')

    // Then a no-match message shows, distinct from the empty-session message
    expect(screen.getByText('No sources match your search.')).toBeInTheDocument()
  })

  it('renders Add and Remove as disabled, not silently inert', () => {
    // Given one source rendered
    render(<StudySourcesPanel sources={[IMPORTED_DOC]} />)

    // Then both controls are visibly disabled
    expect(screen.getByRole('button', { name: 'Add source' })).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Remove notes/distributed-systems.md' }),
    ).toBeDisabled()
  })
})
