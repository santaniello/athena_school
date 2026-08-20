import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { IndexReviewDialog } from './index-review-dialog'
import type { ChunkLoadIssue } from '@/lib/knowledge-index'

function issue(overrides: Partial<ChunkLoadIssue> = {}): ChunkLoadIssue {
  return {
    chunkId: 'chunk-1',
    itemId: 'item-1',
    source: 'imported_doc',
    filePath: 'notes/go.md',
    reason: 'missing_item',
    ...overrides,
  }
}

describe('IndexReviewDialog', () => {
  it('lists each isolated chunk with its file, mapped reason, and recovery guidance', () => {
    // Given one isolated imported-note chunk
    render(
      <IndexReviewDialog
        open={true}
        issues={[issue({ filePath: 'notes/go.md', reason: 'missing_item' })]}
        onClose={vi.fn()}
      />,
    )

    // Then the file path, a plain-English reason, and re-import guidance appear
    expect(screen.getByText('notes/go.md')).toBeInTheDocument()
    expect(
      screen.getByText('The knowledge item this content belonged to no longer exists.'),
    ).toBeInTheDocument()
    expect(
      screen.getByText('Re-import the folder containing this file to fix this.'),
    ).toBeInTheDocument()
  })

  it('never exposes the raw reason code to the user', () => {
    // Given an isolated chunk
    render(
      <IndexReviewDialog
        open={true}
        issues={[issue({ reason: 'malformed_embedding' })]}
        onClose={vi.fn()}
      />,
    )

    // Then only the mapped English copy appears, never the stable code itself
    expect(screen.queryByText('malformed_embedding')).not.toBeInTheDocument()
  })

  it('falls back to the chunk id when the file path is empty (a non-imported source)', () => {
    // Given an isolated chunk with no file path (e.g. an Athena item)
    render(
      <IndexReviewDialog
        open={true}
        issues={[issue({ filePath: '', chunkId: 'chunk-42', source: 'athena' })]}
        onClose={vi.fn()}
      />,
    )

    // Then the chunk id identifies the row, with reindexing guidance instead
    // of re-import guidance
    expect(screen.getByText('chunk-42')).toBeInTheDocument()
    expect(
      screen.getByText('This item is waiting on reindexing support in a future update.'),
    ).toBeInTheDocument()
  })

  it('calls onClose when dismissed', async () => {
    // Given the dialog is open
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<IndexReviewDialog open={true} issues={[issue()]} onClose={onClose} />)

    // When closing it — both the footer's explicit "Close" button and the
    // dialog's built-in X icon control carry the accessible name "Close",
    // so pick the first (footer) one, same as ingest-progress-dialog.test.tsx
    const closeButtons = await screen.findAllByRole('button', { name: 'Close' })
    await user.click(closeButtons[0])

    // Then the caller is notified
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('calls onClose when dismissed via Escape, not just the explicit Close button', async () => {
    // Given the dialog is open
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<IndexReviewDialog open={true} issues={[issue()]} onClose={onClose} />)

    // When pressing Escape
    await user.keyboard('{Escape}')

    // Then the caller is notified the same way
    expect(onClose).toHaveBeenCalledOnce()
  })

  it.each([
    ['source_mismatch', "This content's source no longer matches its knowledge item."],
    ['topic_mismatch', "This content's topic no longer matches its knowledge item."],
    ['status_mismatch', "This content's status no longer matches its knowledge item."],
    ['stale_item', 'This knowledge item changed after this content was last indexed.'],
    ['malformed_embedding', "This content's stored data is corrupted."],
    ['invalid_chunk_id', 'This content has an invalid identifier.'],
    ['unknown_source', 'This content has an unrecognized source.'],
    ['unknown_status', 'This content has an unrecognized status.'],
    ['invalid_vector', "This content's stored data is invalid."],
  ])('maps reason code %s to its plain-English label', (reason, expectedLabel) => {
    // Given an isolated chunk with this exact reason code
    render(<IndexReviewDialog open={true} issues={[issue({ reason })]} onClose={vi.fn()} />)

    // Then the matching plain-English label is shown
    expect(screen.getByText(expectedLabel)).toBeInTheDocument()
  })
})
