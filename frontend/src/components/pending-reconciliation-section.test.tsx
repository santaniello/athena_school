import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  applyPendingReconciliationRelate,
  applyPendingReconciliationUpdate,
  CONFLICT_KEEP_EXISTING,
  listPendingReconciliations,
  rejectPendingReconciliationProposal,
  resolvePendingReconciliationConflict,
  type PendingReconciliation,
} from '@/lib/knowledge'
import { PendingReconciliationSection } from './pending-reconciliation-section'

vi.mock('@/lib/knowledge', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/knowledge')>()
  return {
    ...original,
    listPendingReconciliations: vi.fn(),
    applyPendingReconciliationUpdate: vi.fn(),
    applyPendingReconciliationRelate: vi.fn(),
    resolvePendingReconciliationConflict: vi.fn(),
    rejectPendingReconciliationProposal: vi.fn(),
  }
})

function proposal(overrides: Partial<PendingReconciliation> = {}): PendingReconciliation {
  return {
    id: 'proposal-1',
    action: 'update',
    candidate: {
      id: '',
      topic: 'Distributed Systems',
      concept: 'Eventual consistency',
      definition: 'Converges eventually, with no read-your-writes guarantee.',
      properties: [],
      tradeOffs: [],
      relatedConcepts: [],
      source: 'athena',
      status: 'draft',
      createdAt: '',
      updatedAt: '',
    },
    targetItemId: 'item-target',
    targetConcept: 'Eventual consistency',
    targetStatus: 'approved',
    reason: 'extends the existing definition',
    changes: {
      definition: 'Converges eventually, with no read-your-writes guarantee.',
      properties: [],
      tradeOffs: [],
      relatedConcepts: [],
    },
    stale: false,
    createdAt: '2026-08-28T09:00:00Z',
    ...overrides,
  }
}

describe('PendingReconciliationSection', () => {
  it('renders nothing when there is nothing pending', async () => {
    // Given no pending proposals
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([])

    // When it mounts
    const { container } = render(<PendingReconciliationSection />)

    // Then it renders nothing at all
    await waitFor(() => expect(listPendingReconciliations).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('shows an error when the list fails to load', async () => {
    // Given a failing load
    vi.mocked(listPendingReconciliations).mockRejectedValueOnce(new Error('unavailable'))

    // When it mounts
    render(<PendingReconciliationSection />)

    // Then a load error is shown
    expect(await screen.findByText('Failed to load pending proposals.')).toBeInTheDocument()
  })

  it('applies an update, removes the row, and notifies the badge callback', async () => {
    // Given one pending update proposal
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([proposal()])
    vi.mocked(applyPendingReconciliationUpdate).mockResolvedValueOnce({} as never)
    const onKnowledgeChanged = vi.fn()
    const user = userEvent.setup()
    render(<PendingReconciliationSection onKnowledgeChanged={onKnowledgeChanged} />)
    expect(await screen.findByText('Eventual consistency')).toBeInTheDocument()

    // When applying it
    await user.click(screen.getByRole('button', { name: 'Apply update' }))

    // Then the target proposal id is applied, the row disappears, and the
    // combined badge callback fires
    await waitFor(() => expect(applyPendingReconciliationUpdate).toHaveBeenCalledWith('proposal-1'))
    await waitFor(() => expect(screen.queryByText('Eventual consistency')).not.toBeInTheDocument())
    expect(onKnowledgeChanged).toHaveBeenCalledTimes(1)
  })

  it('keeps a failed decision visible with its error instead of removing it', async () => {
    // Given one pending relate proposal whose apply call fails
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([
      proposal({ action: 'relate', id: 'proposal-2' }),
    ])
    vi.mocked(applyPendingReconciliationRelate).mockRejectedValueOnce(new Error('database locked'))
    const user = userEvent.setup()
    render(<PendingReconciliationSection />)
    await screen.findByRole('button', { name: 'Create & relate' })

    // When applying it and it fails
    await user.click(screen.getByRole('button', { name: 'Create & relate' }))

    // Then the row stays, showing the error, with a retry available
    expect(await screen.findByText('database locked')).toBeInTheDocument()
    expect(screen.getByText('Eventual consistency')).toBeInTheDocument()
  })

  it('rejects a proposal and removes it from the list', async () => {
    // Given one pending conflict proposal
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([
      proposal({ action: 'conflict', id: 'proposal-3' }),
    ])
    vi.mocked(rejectPendingReconciliationProposal).mockResolvedValueOnce(undefined)
    const user = userEvent.setup()
    render(<PendingReconciliationSection />)
    await screen.findByRole('button', { name: 'Reject' })

    // When rejecting it
    await user.click(screen.getByRole('button', { name: 'Reject' }))

    // Then it is rejected and removed from the list
    await waitFor(() =>
      expect(rejectPendingReconciliationProposal).toHaveBeenCalledWith('proposal-3'),
    )
    await waitFor(() => expect(screen.queryByText('Eventual consistency')).not.toBeInTheDocument())
  })

  it('resolves a conflict with the chosen outcome', async () => {
    // Given one pending conflict proposal
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([
      proposal({ action: 'conflict', id: 'proposal-4' }),
    ])
    vi.mocked(resolvePendingReconciliationConflict).mockResolvedValueOnce({} as never)
    const user = userEvent.setup()
    render(<PendingReconciliationSection />)
    await screen.findByRole('button', { name: 'Keep existing' })

    // When keeping the existing item
    await user.click(screen.getByRole('button', { name: 'Keep existing' }))

    // Then that exact resolution is sent for that exact proposal
    await waitFor(() =>
      expect(resolvePendingReconciliationConflict).toHaveBeenCalledWith(
        'proposal-4',
        CONFLICT_KEEP_EXISTING,
      ),
    )
  })

  it('shows a stale proposal as reject-only, without any apply action', async () => {
    // Given a proposal whose target changed since classification
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([
      proposal({ stale: true, targetConcept: '', targetStatus: '' }),
    ])

    // When it renders
    render(<PendingReconciliationSection />)

    // Then only Reject is offered — no apply action for a stale row
    expect(await screen.findByRole('button', { name: 'Reject' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Apply update' })).not.toBeInTheDocument()
    expect(screen.getByText(/changed since it was saved for review/i)).toBeInTheDocument()
  })
})
