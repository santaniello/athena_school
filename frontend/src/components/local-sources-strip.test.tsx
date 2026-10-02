import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LocalSourcesStrip } from './local-sources-strip'
import type { StudySource } from '@/lib/study'

// testSource fills in the fields this test doesn't care about with harmless
// defaults, so each case only spells out what it's actually asserting on.
function testSource(overrides: Partial<StudySource>): StudySource {
  return {
    chunkId: 'chunk-1',
    itemId: 'item-1',
    sourceType: 'imported_doc',
    filePath: '',
    heading: '',
    concept: '',
    score: 0,
    excerpt: '',
    ...overrides,
  }
}

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
      testSource({ filePath: 'notes/a.md', heading: 'Channels', score: 0.68 }),
      testSource({ filePath: 'notes/b.md', heading: 'Buffers', score: 0.5 }),
    ]
    render(<LocalSourcesStrip sources={sources} />)

    // Then a collapsed strip shows the count, with no entries visible yet
    expect(screen.getByText('Local sources (2)')).toBeInTheDocument()
    expect(screen.queryByText(/notes\/a\.md/)).not.toBeInTheDocument()
  })

  it('expands to show each entry on click', async () => {
    // Given a strip with one source, collapsed
    const user = userEvent.setup()
    const sources: StudySource[] = [
      testSource({ filePath: 'notes/a.md', heading: 'Channels', score: 0.68 }),
    ]
    render(<LocalSourcesStrip sources={sources} />)

    // When expanding it
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // Then the entry is shown
    expect(screen.getByText(/notes\/a\.md/)).toBeInTheDocument()
  })

  it('numbers each entry by its 1-based position', async () => {
    // Given two sources
    const user = userEvent.setup()
    const sources: StudySource[] = [
      testSource({ filePath: 'notes/a.md', heading: 'Channels', score: 0.68 }),
      testSource({ filePath: 'notes/b.md', heading: 'Buffers', score: 0.5 }),
    ]
    render(<LocalSourcesStrip sources={sources} />)
    await user.click(screen.getByRole('button', { name: 'Local sources (2)' }))

    // Then each entry is prefixed with its 1-based position
    expect(screen.getByText(/^1\. notes\/a\.md/)).toBeInTheDocument()
    expect(screen.getByText(/^2\. notes\/b\.md/)).toBeInTheDocument()
  })

  it('renders an imported_doc entry with its file path, heading, and a decimal score', async () => {
    // Given one imported_doc source
    const user = userEvent.setup()
    const sources: StudySource[] = [
      testSource({ filePath: 'notes/distributed-systems.md', heading: 'CAP theorem', score: 0.68 }),
    ]
    render(<LocalSourcesStrip sources={sources} />)
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // Then it shows the file path, heading, and a decimal score, never a percentage
    const entry = screen.getByText(/notes\/distributed-systems\.md/).closest('li')!
    expect(entry).toHaveTextContent('notes/distributed-systems.md')
    expect(entry).toHaveTextContent('CAP theorem')
    expect(entry).toHaveTextContent('0.68')
    expect(entry).not.toHaveTextContent('%')
  })

  it('renders a user_note entry as "User note" with its concept and score', async () => {
    // Given one user_note source
    const user = userEvent.setup()
    const sources: StudySource[] = [
      testSource({ sourceType: 'user_note', concept: 'Idempotency', score: 0.72 }),
    ]
    render(<LocalSourcesStrip sources={sources} />)
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // Then it shows "User note", the concept, and the score
    const entry = screen.getByText(/User note/).closest('li')!
    expect(entry).toHaveTextContent('User note')
    expect(entry).toHaveTextContent('Idempotency')
    expect(entry).toHaveTextContent('0.72')
  })

  it('renders an athena entry as "Athena Knowledge" with its concept and score', async () => {
    // Given one athena source
    const user = userEvent.setup()
    const sources: StudySource[] = [
      testSource({ sourceType: 'athena', concept: 'CAP theorem', score: 0.81 }),
    ]
    render(<LocalSourcesStrip sources={sources} />)
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // Then it shows "Athena Knowledge", the concept, and the score
    const entry = screen.getByText(/Athena Knowledge/).closest('li')!
    expect(entry).toHaveTextContent('Athena Knowledge')
    expect(entry).toHaveTextContent('CAP theorem')
    expect(entry).toHaveTextContent('0.81')
  })

  it('renders no separator span when the entry has no subtitle', async () => {
    // Given an imported_doc source with no heading (its subtitle)
    const user = userEvent.setup()
    const source = testSource({ filePath: 'notes/a.md', heading: '', score: 0.68 })
    render(<LocalSourcesStrip sources={[source]} />)
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // Then the entry has exactly one <span> child (the score) after the
    // title, not an empty subtitle separator an empty string would still
    // render unconditionally
    const entry = screen.getByText(/notes\/a\.md/).closest('button')!
    expect(entry.querySelectorAll('span')).toHaveLength(2)
  })

  it('does not throw when an entry is clicked with no onOpenCitation prop', async () => {
    // Given a strip with one source, expanded, and no onOpenCitation prop
    const user = userEvent.setup()
    const source = testSource({ filePath: 'notes/a.md', heading: 'Channels', score: 0.68 })
    render(<LocalSourcesStrip sources={[source]} />)
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // When clicking the entry
    await expect(user.click(screen.getByText(/notes\/a\.md/))).resolves.not.toThrow()
  })

  it('calls onOpenCitation with the resolved source when an entry is clicked', async () => {
    // Given a strip with one source, expanded
    const user = userEvent.setup()
    const source = testSource({ filePath: 'notes/a.md', heading: 'Channels', score: 0.68 })
    const onOpenCitation = vi.fn()
    render(<LocalSourcesStrip sources={[source]} onOpenCitation={onOpenCitation} />)
    await user.click(screen.getByRole('button', { name: 'Local sources (1)' }))

    // When clicking the entry
    await user.click(screen.getByText(/notes\/a\.md/))

    // Then onOpenCitation is called with that exact source
    expect(onOpenCitation).toHaveBeenCalledWith(source)
  })
})
