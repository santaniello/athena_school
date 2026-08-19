import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { KnowledgeItem } from '@/lib/knowledge'
import { KnowledgeDeleteDialog } from './knowledge-delete-dialog'

function testItem(overrides: Partial<KnowledgeItem> = {}): KnowledgeItem {
  return {
    id: 'item-1',
    topic: 'Go',
    concept: 'Channels',
    definition: 'Typed conduits.',
    properties: [],
    tradeOffs: [],
    relatedConcepts: [],
    source: 'athena',
    status: 'approved',
    createdAt: '2026-08-18T10:00:00Z',
    updatedAt: '2026-08-18T10:00:00Z',
    ...overrides,
  }
}

describe('KnowledgeDeleteDialog', () => {
  it('is closed when there is no item to delete', () => {
    // Given no item selected for deletion
    render(<KnowledgeDeleteDialog item={null} onCancel={vi.fn()} onConfirm={vi.fn()} />)

    // Then no dialog content is shown
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('shows the concept name and no imported-note caveat for an Athena item', () => {
    // Given an Athena-sourced item
    render(
      <KnowledgeDeleteDialog
        item={testItem({ source: 'athena' })}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    )

    // Then the title names the concept and there is no extra file-related copy
    expect(screen.getByText('Delete "Channels"?')).toBeInTheDocument()
    expect(screen.queryByText(/original file/)).not.toBeInTheDocument()
  })

  it('adds the imported-note caveat when the item came from a file', () => {
    // Given an imported-note item
    render(
      <KnowledgeDeleteDialog
        item={testItem({ source: 'imported_doc' })}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    )

    // Then the extra sentence about the untouched source file appears
    expect(screen.getByText(/original file is not deleted/)).toBeInTheDocument()
  })

  it('calls onConfirm when the destructive action is clicked', async () => {
    // Given an open dialog
    const onConfirm = vi.fn()
    const user = userEvent.setup()
    render(<KnowledgeDeleteDialog item={testItem()} onCancel={vi.fn()} onConfirm={onConfirm} />)

    // When confirming the deletion
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    // Then the owner is notified
    expect(onConfirm).toHaveBeenCalledOnce()
  })

  it('calls onCancel when Cancel is clicked', async () => {
    // Given an open dialog
    const onCancel = vi.fn()
    const user = userEvent.setup()
    render(<KnowledgeDeleteDialog item={testItem()} onCancel={onCancel} onConfirm={vi.fn()} />)

    // When cancelling
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    // Then the owner is notified without confirming
    expect(onCancel).toHaveBeenCalledOnce()
  })
})
