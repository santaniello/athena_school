import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CitationChip } from './citation-chip'
import type { StudySource } from '@/lib/study'

function testSource(overrides: Partial<StudySource>): StudySource {
  return {
    chunkId: 'chunk-1',
    itemId: 'item-1',
    sourceType: 'imported_doc',
    filePath: 'notes/a.md',
    heading: 'Channels',
    concept: '',
    score: 0.9,
    excerpt: 'Channels are typed conduits.',
    ...overrides,
  }
}

describe('CitationChip', () => {
  it('renders plain text for an out-of-range index', () => {
    render(<CitationChip citationIndex={1} sources={[]} />)

    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getByText('[1]')).toBeInTheDocument()
  })

  it('renders the chip button for a valid index', () => {
    render(<CitationChip citationIndex={1} sources={[testSource({})]} />)

    expect(screen.getByRole('button', { name: '1' })).toHaveAttribute('data-slot', 'citation-chip')
  })

  it('does not throw when clicked with no onOpenCitation prop', async () => {
    const user = userEvent.setup()
    render(<CitationChip citationIndex={1} sources={[testSource({})]} />)

    await expect(user.click(screen.getByRole('button', { name: '1' }))).resolves.not.toThrow()
  })

  it('calls onOpenCitation with the resolved source when clicked', async () => {
    const user = userEvent.setup()
    const source = testSource({})
    const onOpenCitation = vi.fn()
    render(<CitationChip citationIndex={1} sources={[source]} onOpenCitation={onOpenCitation} />)

    await user.click(screen.getByRole('button', { name: '1' }))

    expect(onOpenCitation).toHaveBeenCalledWith(source)
  })

  it('shows the excerpt in full when it is exactly at the preview length', async () => {
    const user = userEvent.setup()
    const excerpt = 'x'.repeat(300)
    render(<CitationChip citationIndex={1} sources={[testSource({ excerpt })]} />)

    await user.hover(screen.getByRole('button', { name: '1' }))

    expect(await screen.findByText(excerpt)).toBeInTheDocument()
  })

  it('truncates the excerpt with an ellipsis once it exceeds the preview length', async () => {
    const user = userEvent.setup()
    const excerpt = 'x'.repeat(301)
    render(<CitationChip citationIndex={1} sources={[testSource({ excerpt })]} />)

    await user.hover(screen.getByRole('button', { name: '1' }))

    expect(await screen.findByText(`${'x'.repeat(300)}…`)).toBeInTheDocument()
    expect(screen.queryByText(excerpt)).not.toBeInTheDocument()
  })

  it('renders no subtitle paragraph at all when the source has none', async () => {
    const user = userEvent.setup()
    // imported_doc's subtitle is the heading — empty here means no subtitle.
    render(
      <CitationChip citationIndex={1} sources={[testSource({ heading: '', filePath: 'a.md' })]} />,
    )

    await user.hover(screen.getByRole('button', { name: '1' }))
    await screen.findByText('a.md')

    // Then the tooltip renders exactly two paragraphs (title, excerpt) — no
    // empty subtitle paragraph in between, which an empty string would
    // still produce if it rendered unconditionally.
    const tooltip = document.querySelector('[data-slot="tooltip-content"]')!
    expect(tooltip.querySelectorAll('p')).toHaveLength(2)
  })
})
