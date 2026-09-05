import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  acknowledgePendingReconciliationNoChange,
  applyPendingReconciliationCreate,
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
    applyPendingReconciliationCreate: vi.fn(),
    applyPendingReconciliationUpdate: vi.fn(),
    applyPendingReconciliationRelate: vi.fn(),
    resolvePendingReconciliationConflict: vi.fn(),
    acknowledgePendingReconciliationNoChange: vi.fn(),
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

  it('shows no error alert when proposals load successfully', async () => {
    // Given a successful load with one proposal
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([proposal()])

    // When it mounts
    render(<PendingReconciliationSection />)
    await screen.findByText('Eventual consistency')

    // Then no error alert is shown
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByText('Failed to load pending proposals.')).not.toBeInTheDocument()
  })

  it('shows a generic error message when the failure is not an Error instance', async () => {
    // Given an apply call that rejects with a non-Error value
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([proposal()])
    vi.mocked(applyPendingReconciliationUpdate).mockRejectedValueOnce('boom')
    const user = userEvent.setup()
    render(<PendingReconciliationSection />)
    await screen.findByRole('button', { name: 'Apply update' })

    // When applying it
    await user.click(screen.getByRole('button', { name: 'Apply update' }))

    // Then the generic fallback message is shown, not the raw rejection value
    expect(await screen.findByText('Something went wrong. Try again.')).toBeInTheDocument()
  })

  it('shows the exact target concept and status in the comparison line', async () => {
    // Given a proposal with both a target concept and status
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([proposal()])

    // When it renders
    render(<PendingReconciliationSection />)
    await screen.findByText('Eventual consistency')

    // Then the comparison paragraph carries both exact values through
    expect(
      screen.getByText((_, element) => {
        if (!element || element.tagName !== 'P') return false
        const text = element.textContent ?? ''
        return text.includes('Eventual consistency') && text.includes('(approved)')
      }),
    ).toBeInTheDocument()
  })

  it('keeps other rows unaffected while one decision is pending, and removes only the resolved row', async () => {
    // Given two pending proposals — one that already failed once, and
    // another whose apply call has not resolved yet
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([
      proposal({
        id: 'proposal-a',
        action: 'relate',
        candidate: { ...proposal().candidate, concept: 'Proposal A' },
      }),
      proposal({
        id: 'proposal-b',
        action: 'update',
        candidate: { ...proposal().candidate, concept: 'Proposal B' },
      }),
    ])
    vi.mocked(applyPendingReconciliationRelate).mockRejectedValueOnce(new Error('database locked'))
    let resolveUpdate: (value: unknown) => void = () => {}
    vi.mocked(applyPendingReconciliationUpdate).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveUpdate = resolve
      }) as never,
    )
    const user = userEvent.setup()
    render(<PendingReconciliationSection />)
    await screen.findByText('Proposal A')
    await screen.findByText('Proposal B')

    // When A's relate fails
    await user.click(screen.getByRole('button', { name: 'Create & relate' }))
    expect(await screen.findByText('database locked')).toBeInTheDocument()

    // And B's update starts, but has not resolved yet
    await user.click(screen.getByRole('button', { name: 'Apply update' }))

    // Then A's error stays visible — starting B's decision did not wipe A's
    // stored state — and B's own button reflects its own pending state
    expect(screen.getByText('database locked')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Applying...' })).toBeInTheDocument()

    // When B's update resolves
    resolveUpdate({})

    // Then only B is removed — A, still unresolved, remains in the list
    await waitFor(() => expect(screen.queryByText('Proposal B')).not.toBeInTheDocument())
    expect(screen.getByText('Proposal A')).toBeInTheDocument()
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

  it('applies a create proposal at the chosen status', async () => {
    // Given one pending create proposal
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([
      proposal({ action: 'create', id: 'proposal-5' }),
    ])
    vi.mocked(applyPendingReconciliationCreate).mockResolvedValueOnce({} as never)
    const user = userEvent.setup()
    render(<PendingReconciliationSection />)
    expect(await screen.findByText('Eventual consistency')).toBeInTheDocument()

    // When saving it as approved
    await user.click(screen.getByRole('button', { name: 'Save as approved' }))

    // Then that exact proposal is applied at that exact status
    await waitFor(() =>
      expect(applyPendingReconciliationCreate).toHaveBeenCalledWith('proposal-5', 'approved'),
    )
    await waitFor(() => expect(screen.queryByText('Eventual consistency')).not.toBeInTheDocument())
  })

  it('acknowledges a no_change proposal, removing it from the list', async () => {
    // Given one pending no_change proposal
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([
      proposal({ action: 'no_change', id: 'proposal-6' }),
    ])
    vi.mocked(acknowledgePendingReconciliationNoChange).mockResolvedValueOnce(undefined)
    const user = userEvent.setup()
    render(<PendingReconciliationSection />)
    await screen.findByRole('button', { name: 'Acknowledge' })

    // When acknowledging it
    await user.click(screen.getByRole('button', { name: 'Acknowledge' }))

    // Then that exact proposal is acknowledged and removed from the list
    await waitFor(() =>
      expect(acknowledgePendingReconciliationNoChange).toHaveBeenCalledWith('proposal-6'),
    )
    await waitFor(() => expect(screen.queryByText('Eventual consistency')).not.toBeInTheDocument())
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
