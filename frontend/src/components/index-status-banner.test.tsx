import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { IndexStatusBanner } from './index-status-banner'
import type { IndexStatus } from '@/lib/knowledge-index'

function status(overrides: Partial<IndexStatus> = {}): IndexStatus {
  return { state: 'ready', hasSnapshot: true, issues: [], lastError: '', ...overrides }
}

describe('IndexStatusBanner', () => {
  it('renders nothing when the index is fully ready', () => {
    // Given a clean ready status
    const { container } = render(
      <IndexStatusBanner
        status={status()}
        continuedWithoutSearch={false}
        retrying={false}
        onRetry={vi.fn()}
        onReview={vi.fn()}
      />,
    )

    // Then nothing renders
    expect(container).toBeEmptyDOMElement()
  })

  it('shows the affected count and a Review action when ready_with_warnings', async () => {
    // Given two isolated chunks
    const user = userEvent.setup()
    const onReview = vi.fn()
    render(
      <IndexStatusBanner
        status={status({
          state: 'ready_with_warnings',
          issues: [
            {
              chunkId: 'c1',
              itemId: 'i1',
              source: 'imported_doc',
              filePath: 'a.md',
              reason: 'missing_item',
            },
            {
              chunkId: 'c2',
              itemId: 'i2',
              source: 'imported_doc',
              filePath: 'b.md',
              reason: 'missing_item',
            },
          ],
        })}
        continuedWithoutSearch={false}
        retrying={false}
        onRetry={vi.fn()}
        onReview={onReview}
      />,
    )

    // Then the count is shown
    expect(screen.getByText(/2 items/)).toBeInTheDocument()

    // When clicking Review
    await user.click(screen.getByRole('button', { name: 'Review' }))

    // Then the caller is notified
    expect(onReview).toHaveBeenCalledOnce()
  })

  it('shows an unavailable warning with a Retry action after continuing without local search', async () => {
    // Given the user chose to continue past a failed load
    const user = userEvent.setup()
    const onRetry = vi.fn()
    render(
      <IndexStatusBanner
        status={status({ state: 'failed', hasSnapshot: false })}
        continuedWithoutSearch={true}
        retrying={false}
        onRetry={onRetry}
        onReview={vi.fn()}
      />,
    )

    // Then the persistent unavailable warning is shown
    expect(screen.getByText(/Local search is unavailable/)).toBeInTheDocument()

    // When retrying from the banner
    await user.click(screen.getByRole('button', { name: 'Retry' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('uses the singular "item" when exactly one chunk is affected', () => {
    // Given exactly one isolated chunk
    render(
      <IndexStatusBanner
        status={status({
          state: 'ready_with_warnings',
          issues: [
            {
              chunkId: 'c1',
              itemId: 'i1',
              source: 'imported_doc',
              filePath: 'a.md',
              reason: 'missing_item',
            },
          ],
        })}
        continuedWithoutSearch={false}
        retrying={false}
        onRetry={vi.fn()}
        onReview={vi.fn()}
      />,
    )

    // Then the count reads "1 item", not "1 items"
    expect(screen.getByText(/1 item /)).toBeInTheDocument()
  })

  it('renders nothing when the index failed but the user has not opted to continue', () => {
    // Given a failed status the failure screen is still gating (not yet
    // dismissed via "Continue without local search")
    const { container } = render(
      <IndexStatusBanner
        status={status({ state: 'failed', hasSnapshot: false })}
        continuedWithoutSearch={false}
        retrying={false}
        onRetry={vi.fn()}
        onReview={vi.fn()}
      />,
    )

    // Then the banner stays silent — the failure screen itself is what's
    // showing, not this persistent banner
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing once the index recovers, even if continuedWithoutSearch is still stale-true', () => {
    // Given a status that is no longer "failed" (e.g. right after a
    // successful retry, before the caller has reset its local opt-in flag)
    const { container } = render(
      <IndexStatusBanner
        status={status({ state: 'ready' })}
        continuedWithoutSearch={true}
        retrying={false}
        onRetry={vi.fn()}
        onReview={vi.fn()}
      />,
    )

    // Then the unavailable warning does not reappear for a healthy index
    expect(container).toBeEmptyDOMElement()
  })

  it('shows a rebuilding indicator while a retry is in flight, with no actions', () => {
    // Given a retry currently running
    render(
      <IndexStatusBanner
        status={status()}
        continuedWithoutSearch={false}
        retrying={true}
        onRetry={vi.fn()}
        onReview={vi.fn()}
      />,
    )

    // Then the rebuilding message is shown, with no Retry/Review actions to
    // avoid firing a second retry mid-flight
    expect(screen.getByText(/Rebuilding knowledge index/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Review' })).not.toBeInTheDocument()
  })
})
