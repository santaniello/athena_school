import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ImportReviewDialog } from './import-review-dialog'
import type { ImportPreview } from '@/lib/ingest'

function preview(overrides: Partial<ImportPreview> = {}): ImportPreview {
  return {
    displayPath: 'eigenvectors-notes.md',
    originalContent: 'eigenvectors r vectors that dont change direction.',
    formattedContent:
      '## Eigenvectors\n\nAn eigenvector keeps its direction under the transformation.',
    formattingFailed: false,
    stats: { headingsBefore: 0, headingsAfter: 1, paragraphsBefore: 1, paragraphsAfter: 1 },
    chunks: [
      {
        heading: 'Eigenvectors',
        content: 'An eigenvector keeps its direction under the transformation.',
        charCount: 61,
        estimatedTokens: 15,
      },
    ],
    skipped: false,
    skippedReason: '',
    ...overrides,
  }
}

describe('ImportReviewDialog', () => {
  it('shows the file name and the formatted/chunks tabs', () => {
    // Given a preview for one file
    render(
      <ImportReviewDialog open={true} preview={preview()} onCancel={vi.fn()} onApprove={vi.fn()} />,
    )

    // Then the reviewed file is named, and both tabs are offered
    expect(screen.getByText(/eigenvectors-notes\.md/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Formatted note' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Chunks (1)' })).toBeInTheDocument()
  })

  it('defaults to the Formatted tab, showing the original and the formatted text side by side', () => {
    // Given a preview whose original and formatted text differ
    render(
      <ImportReviewDialog open={true} preview={preview()} onCancel={vi.fn()} onApprove={vi.fn()} />,
    )

    // Then both are visible without switching tabs
    expect(
      screen.getByText(/eigenvectors r vectors that dont change direction/),
    ).toBeInTheDocument()
    expect(screen.getByText(/An eigenvector keeps its direction/)).toBeInTheDocument()
  })

  it('switches to the Chunks tab and lists every chunk heading', async () => {
    // Given a preview with two chunks
    const user = userEvent.setup()
    const twoChunks = preview({
      chunks: [
        {
          heading: 'Introduction',
          content: 'Intro text long enough to read.',
          charCount: 32,
          estimatedTokens: 8,
        },
        {
          heading: 'Definition',
          content: 'Definition text long enough to read.',
          charCount: 37,
          estimatedTokens: 9,
        },
      ],
    })
    render(
      <ImportReviewDialog open={true} preview={twoChunks} onCancel={vi.fn()} onApprove={vi.fn()} />,
    )

    // When switching to the Chunks tab
    await user.click(screen.getByRole('button', { name: 'Chunks (2)' }))

    // Then every chunk's heading is listed
    expect(screen.getByText('Introduction')).toBeInTheDocument()
    expect(screen.getByText('Definition')).toBeInTheDocument()
  })

  it('expands a chunk to reveal its full text, and collapses it back', async () => {
    // Given a chunk whose content is long enough to be truncated when collapsed
    const user = userEvent.setup()
    const longContent =
      'This is a long passage that goes well past any short preview truncation length, ' +
      'so the collapsed card must cut it off while the expanded card shows every word of it in full.'
    render(
      <ImportReviewDialog
        open={true}
        preview={preview({
          chunks: [
            {
              heading: 'Long',
              content: longContent,
              charCount: longContent.length,
              estimatedTokens: 40,
            },
          ],
        })}
        onCancel={vi.fn()}
        onApprove={vi.fn()}
      />,
    )
    await user.click(screen.getByRole('button', { name: 'Chunks (1)' }))
    expect(screen.queryByText(longContent)).not.toBeInTheDocument()

    // When expanding the chunk
    await user.click(screen.getByRole('button', { name: 'Read full chunk' }))

    // Then its full, untruncated text appears
    expect(screen.getByText(longContent)).toBeInTheDocument()

    // When collapsing it again
    await user.click(screen.getByRole('button', { name: 'Show preview' }))

    // Then the full text is gone again
    expect(screen.queryByText(longContent)).not.toBeInTheDocument()
  })

  it('shows a formatting-unavailable notice instead of the before/after comparison, when formatting failed', () => {
    // Given a preview whose formatter degraded to the original text
    render(
      <ImportReviewDialog
        open={true}
        preview={preview({
          formattingFailed: true,
          formattedContent: 'eigenvectors r vectors that dont change direction.',
        })}
        onCancel={vi.fn()}
        onApprove={vi.fn()}
      />,
    )

    // Then the notice appears, and the original text is shown only once
    expect(screen.getByText(/Formatting is unavailable right now/)).toBeInTheDocument()
    expect(screen.getAllByText(/eigenvectors r vectors that dont change direction/)).toHaveLength(1)
  })

  it('calls onCancel when dismissed via Escape, not just the explicit Cancel button', async () => {
    // Given the dialog is open
    const user = userEvent.setup()
    const onCancel = vi.fn()
    render(
      <ImportReviewDialog
        open={true}
        preview={preview()}
        onCancel={onCancel}
        onApprove={vi.fn()}
      />,
    )

    // When pressing Escape
    await user.keyboard('{Escape}')

    // Then the caller is notified the same way Cancel notifies it —
    // nothing was persisted during preview, so there is nothing to undo
    expect(onCancel).toHaveBeenCalledOnce()
  })

  it('renders nothing when preview is null, even while open', () => {
    // Given no preview yet (e.g. still loading)
    render(<ImportReviewDialog open={true} preview={null} onCancel={vi.fn()} onApprove={vi.fn()} />)

    // Then the dialog's own content never appears
    expect(screen.queryByText('Review import')).not.toBeInTheDocument()
  })

  it('calls onCancel, without calling onApprove, when Cancel is clicked', async () => {
    // Given the dialog is open
    const user = userEvent.setup()
    const onCancel = vi.fn()
    const onApprove = vi.fn()
    render(
      <ImportReviewDialog
        open={true}
        preview={preview()}
        onCancel={onCancel}
        onApprove={onApprove}
      />,
    )

    // When clicking Cancel
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    // Then only onCancel fires
    expect(onCancel).toHaveBeenCalledOnce()
    expect(onApprove).not.toHaveBeenCalled()
  })

  it('calls onApprove with the formatted content when approving', async () => {
    // Given the dialog is open
    const user = userEvent.setup()
    const onApprove = vi.fn()
    render(
      <ImportReviewDialog
        open={true}
        preview={preview({ formattedContent: '## Approved\n\nFinal text.' })}
        onCancel={vi.fn()}
        onApprove={onApprove}
      />,
    )

    // When approving the import
    await user.click(screen.getByRole('button', { name: 'Approve & import' }))

    // Then the caller receives exactly the formatted content — never the
    // original — so what was reviewed is what gets persisted
    expect(onApprove).toHaveBeenCalledWith('## Approved\n\nFinal text.')
  })
})
