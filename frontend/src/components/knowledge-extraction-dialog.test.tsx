import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  saveAndApproveExtractedKnowledge,
  saveExtractedKnowledge,
  type KnowledgeItem,
} from '@/lib/knowledge'
import { KnowledgeExtractionDialog } from './knowledge-extraction-dialog'

vi.mock('@/lib/knowledge', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/knowledge')>()
  return {
    ...original,
    saveExtractedKnowledge: vi.fn(),
    saveAndApproveExtractedKnowledge: vi.fn(),
  }
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
    expect(screen.getByLabelText('Select Channels')).toBeChecked()
    expect(screen.getByLabelText('Select Goroutines')).toBeChecked()
    expect(screen.queryByText('Channels property')).not.toBeInTheDocument()
  })

  it('saves only checked candidates without dropping hidden fields', async () => {
    // Given two candidates with the second unchecked
    const items = [candidate('1', 'Channels'), candidate('2', 'Goroutines')]
    vi.mocked(saveExtractedKnowledge).mockResolvedValueOnce({ savedIndices: [0], error: '' })
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open items={items} onClose={onClose} />)
    await user.click(screen.getByLabelText('Select Goroutines'))

    // When saving drafts
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))

    // Then only the complete first candidate is sent and the dialog closes
    await waitFor(() => expect(saveExtractedKnowledge).toHaveBeenCalledWith([items[0]]))
    expect(onClose).toHaveBeenCalled()
  })

  it('restores a candidate and preserves the original order when it is selected again', async () => {
    // Given three candidates with the first temporarily unchecked
    const items = [candidate('1', 'One'), candidate('2', 'Two'), candidate('3', 'Three')]
    vi.mocked(saveExtractedKnowledge).mockResolvedValueOnce({
      savedIndices: [0, 1, 2],
      error: '',
    })
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open items={items} onClose={vi.fn()} />)
    await user.click(screen.getByLabelText('Select One'))

    // When selecting the first candidate again and saving
    await user.click(screen.getByLabelText('Select One'))
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))

    // Then all candidates retain their displayed order
    await waitFor(() => expect(saveExtractedKnowledge).toHaveBeenCalledWith(items))
  })

  it('locks every action and reports progress while saving', async () => {
    // Given a save request that remains in flight
    vi.mocked(saveExtractedKnowledge).mockReturnValueOnce(new Promise<never>(() => {}))
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog open items={[candidate('1', 'Channels')]} onClose={vi.fn()} />,
    )

    // When saving drafts
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))

    // Then candidate selection and both dialog actions are locked
    expect(screen.getByLabelText('Select Channels')).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Dismiss' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
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
    await user.click(screen.getByLabelText('Select Channels'))

    // Then saving is disabled
    expect(screen.getByRole('button', { name: 'Save as drafts' })).toBeDisabled()
  })

  it('shows an empty result without attempting to save', () => {
    // Given extraction found no candidates
    render(<KnowledgeExtractionDialog open items={[]} onClose={vi.fn()} />)

    // Then the empty state is shown and no save action exists
    expect(screen.getByText('No new knowledge found')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save as drafts' })).not.toBeInTheDocument()
  })

  it('retries only exact unsaved candidates after a non-prefix partial failure', async () => {
    // Given three candidates and a first save that persisted only the middle one
    const items = [candidate('1', 'One'), candidate('2', 'Two'), candidate('3', 'Three')]
    vi.mocked(saveExtractedKnowledge)
      .mockResolvedValueOnce({ savedIndices: [1], error: 'knowledge save failed: database locked' })
      .mockResolvedValueOnce({ savedIndices: [0, 1], error: '' })
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open items={items} onClose={vi.fn()} />)

    // When saving and then retrying
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))
    expect(await screen.findByText(/database locked/i)).toBeInTheDocument()
    expect(screen.getByText('Saved')).toBeInTheDocument()
    expect(screen.getByLabelText('Select One')).toBeEnabled()
    expect(screen.getByLabelText('Select Two')).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    // Then the retry excludes exactly the already-saved middle candidate
    await waitFor(() =>
      expect(saveExtractedKnowledge).toHaveBeenNthCalledWith(2, [items[0], items[2]]),
    )
    await waitFor(() => expect(screen.queryByText(/database locked/i)).not.toBeInTheDocument())
  })

  it('retries every candidate after a failure that saved none', async () => {
    // Given a save failure without persisted indices
    const items = [candidate('1', 'One'), candidate('2', 'Two')]
    vi.mocked(saveExtractedKnowledge)
      .mockResolvedValueOnce({ savedIndices: [], error: 'database unavailable' })
      .mockResolvedValueOnce({ savedIndices: [0, 1], error: '' })
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open items={items} onClose={vi.fn()} />)

    // When saving and retrying
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))
    expect(await screen.findByText('database unavailable')).toBeInTheDocument()
    expect(screen.queryByText('Saved')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Try again' }))

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
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))

    // Then a user-safe fallback is shown and retry remains available
    expect(await screen.findByText('Failed to save drafts.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Try again' })).toBeEnabled()
  })

  it('shows a mode-appropriate safe message when "Save as knowledge" rejects with a non-error value', async () => {
    // Given an unexpected binding rejection from the approve-directly path
    vi.mocked(saveAndApproveExtractedKnowledge).mockRejectedValueOnce('unavailable')
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog open items={[candidate('1', 'Channels')]} onClose={vi.fn()} />,
    )

    // When saving via "Save as knowledge"
    await user.click(screen.getByRole('button', { name: 'Save as knowledge' }))

    // Then the fallback names the operation that actually failed, not the
    // other save mode's message
    expect(await screen.findByText('Failed to save as knowledge.')).toBeInTheDocument()
    expect(screen.queryByText('Failed to save drafts.')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Try again' })).toBeEnabled()
  })

  it('saves and approves via "Save as knowledge", directly closing on success', async () => {
    // Given a complete candidate
    const items = [candidate('1', 'Channels'), candidate('2', 'Goroutines')]
    vi.mocked(saveAndApproveExtractedKnowledge).mockResolvedValueOnce({
      savedIndices: [0, 1],
      error: '',
    })
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open items={items} onClose={onClose} />)

    // When saving via "Save as knowledge"
    await user.click(screen.getByRole('button', { name: 'Save as knowledge' }))

    // Then it calls the approve-directly binding (never the draft one) with
    // every selected candidate, and closes the dialog
    await waitFor(() => expect(saveAndApproveExtractedKnowledge).toHaveBeenCalledWith(items))
    expect(saveExtractedKnowledge).not.toHaveBeenCalled()
    expect(onClose).toHaveBeenCalled()
  })

  it('keeps "Save as drafts" at its idle label while "Save as knowledge" is saving', async () => {
    // Given a save-and-approve request that remains in flight
    vi.mocked(saveAndApproveExtractedKnowledge).mockReturnValueOnce(new Promise<never>(() => {}))
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog open items={[candidate('1', 'Channels')]} onClose={vi.fn()} />,
    )

    // When saving via "Save as knowledge"
    await user.click(screen.getByRole('button', { name: 'Save as knowledge' }))

    // Then only that button shows the in-flight label; the draft button
    // keeps its idle label (just disabled, not relabeled) and selection locks
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save as drafts' })).toBeDisabled()
    expect(screen.getByLabelText('Select Channels')).toBeDisabled()
  })

  it('retries the same "Save as knowledge" mode after its own failure', async () => {
    // Given a save-and-approve call that fails once, then succeeds
    const items = [candidate('1', 'Channels')]
    vi.mocked(saveAndApproveExtractedKnowledge)
      .mockResolvedValueOnce({ savedIndices: [], error: 'database unavailable' })
      .mockResolvedValueOnce({ savedIndices: [0], error: '' })
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open items={items} onClose={vi.fn()} />)
    await user.click(screen.getByRole('button', { name: 'Save as knowledge' }))
    expect(await screen.findByText('database unavailable')).toBeInTheDocument()

    // When retrying
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    // Then the retry goes through the same approve-directly binding, not the draft one
    await waitFor(() =>
      expect(saveAndApproveExtractedKnowledge).toHaveBeenNthCalledWith(2, items),
    )
    expect(saveExtractedKnowledge).not.toHaveBeenCalled()
  })
})
