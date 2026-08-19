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

  it('restores a candidate and preserves the original order when it is selected again', async () => {
    // Given three candidates with the first temporarily unchecked
    const items = [candidate('1', 'One'), candidate('2', 'Two'), candidate('3', 'Three')]
    vi.mocked(saveExtractedKnowledge).mockResolvedValueOnce(3)
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open items={items} onClose={vi.fn()} />)
    await user.click(screen.getByLabelText('Selecionar One'))

    // When selecting the first candidate again and saving
    await user.click(screen.getByLabelText('Selecionar One'))
    await user.click(screen.getByRole('button', { name: 'Salvar como rascunhos' }))

    // Then all candidates retain their displayed order
    await waitFor(() => expect(saveExtractedKnowledge).toHaveBeenCalledWith(items))
  })

  it('locks every action and reports progress while saving', async () => {
    // Given a save request that remains in flight
    vi.mocked(saveExtractedKnowledge).mockReturnValueOnce(new Promise(() => {}))
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog open items={[candidate('1', 'Channels')]} onClose={vi.fn()} />,
    )

    // When saving drafts
    await user.click(screen.getByRole('button', { name: 'Salvar como rascunhos' }))

    // Then candidate selection and both dialog actions are locked
    expect(screen.getByLabelText('Selecionar Channels')).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Ignorar' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Salvando...' })).toBeDisabled()
  })

  it('closes when the dialog close control is used', async () => {
    // Given an open extraction dialog
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog open items={[candidate('1', 'Channels')]} onClose={onClose} />,
    )

    // When dismissing it through the dialog close control
    await user.click(screen.getByRole('button', { name: 'Close' }))

    // Then the owner is notified exactly once
    expect(onClose).toHaveBeenCalledOnce()
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
    expect(screen.getByText('Salvo')).toBeInTheDocument()
    expect(screen.getByLabelText('Selecionar One')).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Tentar novamente' }))

    // Then the retry excludes the already-saved prefix and closes on success
    await waitFor(() => expect(saveExtractedKnowledge).toHaveBeenNthCalledWith(2, items.slice(1)))
    await waitFor(() => expect(screen.queryByText(/database locked/i)).not.toBeInTheDocument())
  })

  it('retries every candidate after a failure that saved none', async () => {
    // Given a save failure without a partial count
    const items = [candidate('1', 'One'), candidate('2', 'Two')]
    vi.mocked(saveExtractedKnowledge)
      .mockRejectedValueOnce(new Error('database unavailable'))
      .mockResolvedValueOnce(2)
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open items={items} onClose={vi.fn()} />)

    // When saving and retrying
    await user.click(screen.getByRole('button', { name: 'Salvar como rascunhos' }))
    expect(await screen.findByText('database unavailable')).toBeInTheDocument()
    expect(screen.queryByText('Salvo')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Tentar novamente' }))

    // Then the complete selection is retried
    await waitFor(() => expect(saveExtractedKnowledge).toHaveBeenNthCalledWith(2, items))
  })

  it('shows a safe message when saving rejects with a non-error value', async () => {
    // Given an unexpected binding rejection
    vi.mocked(saveExtractedKnowledge).mockRejectedValueOnce('unavailable')
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog open items={[candidate('1', 'Channels')]} onClose={vi.fn()} />,
    )

    // When saving
    await user.click(screen.getByRole('button', { name: 'Salvar como rascunhos' }))

    // Then a user-safe fallback is shown and retry remains available
    expect(await screen.findByText('Falha ao salvar os rascunhos.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Tentar novamente' })).toBeEnabled()
  })
})
