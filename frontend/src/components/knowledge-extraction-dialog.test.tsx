import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { saveExtractedKnowledge, type KnowledgeItem } from '@/lib/knowledge'
import { KnowledgeExtractionDialog } from './knowledge-extraction-dialog'

vi.mock('@/lib/knowledge', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/knowledge')>()
  return { ...original, saveExtractedKnowledge: vi.fn() }
})

function candidate(id: string, concept: string): KnowledgeItem {
  return {
    id,
    topic: 'Go',
    concept,
    definition: `${concept} definition`,
    properties: [`${concept} property`],
    tradeOffs: [`${concept} trade-off`],
    relatedConcepts: [`${concept} relation`],
    source: 'athena',
    status: 'draft',
    createdAt: '2026-08-18T10:00:00Z',
    updatedAt: '2026-08-18T10:00:00Z',
  }
}

describe('KnowledgeExtractionDialog', () => {
  it('lists concept and definition with every candidate checked', () => {
    // Given extracted candidates
    const items = [candidate('1', 'Channels'), candidate('2', 'Goroutines')]

    // When opening the dialog
    render(<KnowledgeExtractionDialog open items={items} onClose={vi.fn()} />)

    // Then concepts and definitions are visible and all candidates are selected
    expect(screen.getByText('Channels')).toBeInTheDocument()
    expect(screen.getByText('Channels definition')).toBeInTheDocument()
    expect(screen.getByLabelText('Selecionar Channels')).toBeChecked()
    expect(screen.getByLabelText('Selecionar Goroutines')).toBeChecked()
    expect(screen.queryByText('Channels property')).not.toBeInTheDocument()
  })

  it('saves only checked candidates without dropping hidden fields', async () => {
    // Given two candidates with the second unchecked
    const items = [candidate('1', 'Channels'), candidate('2', 'Goroutines')]
    vi.mocked(saveExtractedKnowledge).mockResolvedValueOnce(1)
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open items={items} onClose={onClose} />)
    await user.click(screen.getByLabelText('Selecionar Goroutines'))

    // When saving drafts
    await user.click(screen.getByRole('button', { name: 'Salvar como rascunhos' }))

    // Then only the complete first candidate is sent and the dialog closes
    await waitFor(() => expect(saveExtractedKnowledge).toHaveBeenCalledWith([items[0]]))
    expect(onClose).toHaveBeenCalled()
  })

  it('disables saving when no candidate is checked', async () => {
    // Given one candidate that the user unchecks
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog open items={[candidate('1', 'Channels')]} onClose={vi.fn()} />,
    )
    await user.click(screen.getByLabelText('Selecionar Channels'))

    // Then saving is disabled
    expect(screen.getByRole('button', { name: 'Salvar como rascunhos' })).toBeDisabled()
  })

  it('shows an empty result without attempting to save', () => {
    // Given extraction found no candidates
    render(<KnowledgeExtractionDialog open items={[]} onClose={vi.fn()} />)

    // Then the empty state is shown and no save action exists
    expect(screen.getByText('Nenhum conhecimento novo encontrado')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Salvar como rascunhos' })).not.toBeInTheDocument()
  })

  it('retries only the unsaved remainder after a partial failure', async () => {
    // Given three candidates and a first save that persisted one before failing
    const items = [candidate('1', 'One'), candidate('2', 'Two'), candidate('3', 'Three')]
    vi.mocked(saveExtractedKnowledge)
      .mockRejectedValueOnce(Object.assign(new Error('database locked'), { partialCount: 1 }))
      .mockResolvedValueOnce(2)
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open items={items} onClose={vi.fn()} />)

    // When saving and then retrying
    await user.click(screen.getByRole('button', { name: 'Salvar como rascunhos' }))
    expect(await screen.findByText(/database locked/i)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Tentar novamente' }))

    // Then the retry excludes the already-saved prefix
    await waitFor(() => expect(saveExtractedKnowledge).toHaveBeenNthCalledWith(2, items.slice(1)))
  })
})
