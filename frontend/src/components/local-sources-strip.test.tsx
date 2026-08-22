import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LocalSourcesStrip } from './local-sources-strip'
import type { StudySource } from '@/lib/study'

describe('LocalSourcesStrip', () => {
  it('renders nothing when there are no sources', () => {
    // Given an empty source list
    const { container } = render(<LocalSourcesStrip sources={[]} />)

    // Then no strip is rendered
    expect(container).toBeEmptyDOMElement()
  })

  it('renders a collapsed strip with the source count', () => {
    // Given two sources
    const sources: StudySource[] = [
      {
        sourceType: 'imported_doc',
        filePath: 'notes/a.md',
        heading: 'Channels',
        concept: '',
        score: 0.68,
      },
      {
        sourceType: 'imported_doc',
        filePath: 'notes/b.md',
        heading: 'Buffers',
        concept: '',
        score: 0.5,
      },
    ]
    render(<LocalSourcesStrip sources={sources} />)

    // Then a collapsed strip shows the count, with no entries visible yet
    expect(screen.getByText('Local sources (2)')).toBeInTheDocument()
    expect(screen.queryByText('notes/a.md')).not.toBeInTheDocument()
  })

  it('expands to show each entry on click', async () => {
    // Given a strip with one source, collapsed
    const user = userEvent.setup()
    const sources: StudySource[] = [
      {
        sourceType: 'imported_doc',
        filePath: 'notes/a.md',
        heading: 'Channels',
        concept: '',
        score: 0.68,
      },
    ]
    render(<LocalSourcesStrip sources={sources} />)

    // When expanding it
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // Then the entry is shown
    expect(screen.getByText('notes/a.md')).toBeInTheDocument()
  })

  it('renders an imported_doc entry with its file path, heading, and a decimal score', async () => {
    // Given one imported_doc source
    const user = userEvent.setup()
    const sources: StudySource[] = [
      {
        sourceType: 'imported_doc',
        filePath: 'notes/distributed-systems.md',
        heading: 'CAP theorem',
        concept: '',
        score: 0.68,
      },
    ]
    render(<LocalSourcesStrip sources={sources} />)
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // Then it shows the file path, heading, and a decimal score, never a percentage
    const entry = screen.getByText('notes/distributed-systems.md').closest('li')!
    expect(entry).toHaveTextContent('notes/distributed-systems.md')
    expect(entry).toHaveTextContent('CAP theorem')
    expect(entry).toHaveTextContent('0.68')
    expect(entry).not.toHaveTextContent('%')
  })

  it('renders a user_note entry as "User note" with its concept and score', async () => {
    // Given one user_note source
    const user = userEvent.setup()
    const sources: StudySource[] = [
      { sourceType: 'user_note', filePath: '', heading: '', concept: 'Idempotency', score: 0.72 },
    ]
    render(<LocalSourcesStrip sources={sources} />)
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // Then it shows "User note", the concept, and the score
    const entry = screen.getByText('User note').closest('li')!
    expect(entry).toHaveTextContent('User note')
    expect(entry).toHaveTextContent('Idempotency')
    expect(entry).toHaveTextContent('0.72')
  })

  it('renders an athena entry as "Athena Knowledge" with its concept and score', async () => {
    // Given one athena source
    const user = userEvent.setup()
    const sources: StudySource[] = [
      { sourceType: 'athena', filePath: '', heading: '', concept: 'CAP theorem', score: 0.81 },
    ]
    render(<LocalSourcesStrip sources={sources} />)
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // Then it shows "Athena Knowledge", the concept, and the score
    const entry = screen.getByText('Athena Knowledge').closest('li')!
    expect(entry).toHaveTextContent('Athena Knowledge')
    expect(entry).toHaveTextContent('CAP theorem')
    expect(entry).toHaveTextContent('0.81')
  })
})
