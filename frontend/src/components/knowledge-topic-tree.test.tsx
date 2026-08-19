import { describe, expect, it, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { listKnowledgeTopics } from '@/lib/knowledge'
import { onIngestDone, type IngestSummary } from '@/lib/ingest'
import { KnowledgeTopicTree } from './knowledge-topic-tree'

vi.mock('@/lib/knowledge', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/knowledge')>()
  return { ...original, listKnowledgeTopics: vi.fn() }
})

vi.mock('@/lib/ingest', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/ingest')>()
  return { ...original, onIngestDone: vi.fn() }
})

function stubOnIngestDone() {
  let handler: (summary: IngestSummary) => void = () => {}
  const unsubscribe = vi.fn()
  vi.mocked(onIngestDone).mockImplementation((h) => {
    handler = h
    return unsubscribe
  })
  return { fire: () => act(() => handler({} as IngestSummary)), unsubscribe }
}

describe('KnowledgeTopicTree', () => {
  it('loads and lists every topic alongside "All topics"', async () => {
    // Given two topics
    vi.mocked(listKnowledgeTopics).mockResolvedValueOnce(['Go', 'Kubernetes'])
    stubOnIngestDone()

    // When rendering the tree
    render(<KnowledgeTopicTree selectedTopic={null} onSelectTopic={vi.fn()} />)

    // Then "All topics" and both topics are listed
    expect(screen.getByRole('button', { name: /All topics/ })).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Go' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Kubernetes' })).toBeInTheDocument()
  })

  it('applies the base row styling to every row', async () => {
    // Given a rendered tree with one topic
    vi.mocked(listKnowledgeTopics).mockResolvedValueOnce(['Go'])
    stubOnIngestDone()

    // When rendering it
    render(<KnowledgeTopicTree selectedTopic={null} onSelectTopic={vi.fn()} />)

    // Then both rows carry the shared row styling
    expect(screen.getByRole('button', { name: /All topics/ }).className).toContain(
      'cursor-pointer',
    )
    expect((await screen.findByRole('button', { name: 'Go' })).className).toContain(
      'cursor-pointer',
    )
  })

  it('calls onSelectTopic with the clicked topic', async () => {
    // Given a rendered tree with one topic
    vi.mocked(listKnowledgeTopics).mockResolvedValueOnce(['Go'])
    stubOnIngestDone()
    const onSelectTopic = vi.fn()
    const user = userEvent.setup()
    render(<KnowledgeTopicTree selectedTopic={null} onSelectTopic={onSelectTopic} />)

    // When clicking the topic
    await user.click(await screen.findByRole('button', { name: 'Go' }))

    // Then the topic is reported
    expect(onSelectTopic).toHaveBeenCalledWith('Go')
  })

  it('calls onSelectTopic with null when "All topics" is clicked', async () => {
    // Given a rendered tree with a topic currently selected
    vi.mocked(listKnowledgeTopics).mockResolvedValueOnce(['Go'])
    stubOnIngestDone()
    const onSelectTopic = vi.fn()
    const user = userEvent.setup()
    render(<KnowledgeTopicTree selectedTopic="Go" onSelectTopic={onSelectTopic} />)

    // When clicking "All topics"
    await user.click(screen.getByRole('button', { name: /All topics/ }))

    // Then the filter is cleared
    expect(onSelectTopic).toHaveBeenCalledWith(null)
  })

  it('marks the currently selected topic as active, and "All topics" as not', async () => {
    // Given "Go" is the selected topic
    vi.mocked(listKnowledgeTopics).mockResolvedValueOnce(['Go', 'Kubernetes'])
    stubOnIngestDone()

    // When rendering the tree
    render(<KnowledgeTopicTree selectedTopic="Go" onSelectTopic={vi.fn()} />)

    // Then only "Go" carries the active styling
    const goButton = await screen.findByRole('button', { name: 'Go' })
    expect(goButton.className).toContain('bg-secondary')
    expect(screen.getByRole('button', { name: 'Kubernetes' }).className).not.toContain(
      'bg-secondary',
    )
    expect(screen.getByRole('button', { name: /All topics/ }).className).not.toContain(
      'bg-secondary',
    )
  })

  it('marks "All topics" as active when no topic is selected', async () => {
    // Given no topic is currently selected
    vi.mocked(listKnowledgeTopics).mockResolvedValueOnce(['Go'])
    stubOnIngestDone()

    // When rendering the tree
    render(<KnowledgeTopicTree selectedTopic={null} onSelectTopic={vi.fn()} />)

    // Then "All topics" carries the active styling and "Go" does not
    expect(screen.getByRole('button', { name: /All topics/ }).className).toContain(
      'bg-secondary',
    )
    expect((await screen.findByRole('button', { name: 'Go' })).className).not.toContain(
      'bg-secondary',
    )
  })

  it('reloads topics when a notes import finishes', async () => {
    // Given a tree that has already loaded its initial topics
    vi.mocked(listKnowledgeTopics)
      .mockResolvedValueOnce(['Go'])
      .mockResolvedValueOnce(['Go', 'Kubernetes'])
    const { fire } = stubOnIngestDone()
    render(<KnowledgeTopicTree selectedTopic={null} onSelectTopic={vi.fn()} />)
    await screen.findByRole('button', { name: 'Go' })

    // When a notes import completes
    fire()

    // Then the newly-imported topic appears without a remount
    expect(await screen.findByRole('button', { name: 'Kubernetes' })).toBeInTheDocument()
  })

  it('unsubscribes from ingest:done on unmount', () => {
    // Given a mounted tree
    vi.mocked(listKnowledgeTopics).mockResolvedValueOnce([])
    const { unsubscribe } = stubOnIngestDone()
    const { unmount } = render(<KnowledgeTopicTree selectedTopic={null} onSelectTopic={vi.fn()} />)

    // When it unmounts
    unmount()

    // Then the listener is torn down
    expect(unsubscribe).toHaveBeenCalled()
  })
})
