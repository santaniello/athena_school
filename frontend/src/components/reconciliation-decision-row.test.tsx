import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import {
  CONFLICT_CREATE_SEPARATELY,
  CONFLICT_KEEP_EXISTING,
  CONFLICT_UPDATE_EXISTING,
  RECONCILE_CONFLICT,
  RECONCILE_CREATE,
  RECONCILE_NO_CHANGE,
  RECONCILE_RELATE,
  RECONCILE_UPDATE,
} from '@/lib/knowledge'
import {
  idleDecision,
  ReconciliationDecisionRow,
  type DecisionState,
} from './reconciliation-decision-row'

function allHandlers() {
  return {
    onApplyCreate: vi.fn(),
    onApplyUpdate: vi.fn(),
    onApplyRelate: vi.fn(),
    onResolveConflict: vi.fn(),
    onAcknowledgeNoChange: vi.fn(),
    onReject: vi.fn(),
    onSaveForReview: vi.fn(),
  }
}

describe('idleDecision', () => {
  it('is the fully-idle starting state', () => {
    expect(idleDecision).toEqual({ pending: false, done: false, doneLabel: '', error: '' })
  })
})

describe('ReconciliationDecisionRow', () => {
  it('shows the display label for each known action', () => {
    const cases: Array<[string, string]> = [
      [RECONCILE_CREATE, 'Create'],
      [RECONCILE_UPDATE, 'Update'],
      [RECONCILE_RELATE, 'Relate'],
      [RECONCILE_CONFLICT, 'Conflict'],
      [RECONCILE_NO_CHANGE, 'No change'],
    ]
    for (const [action, label] of cases) {
      const { unmount } = render(
        <ReconciliationDecisionRow
          concept="X"
          definition="Y"
          action={action}
          state={idleDecision}
        />,
      )
      expect(screen.getByText(label)).toBeInTheDocument()
      unmount()
    }
  })

  it('falls back to the raw action string when it has no known label', () => {
    // Given an action absent from the known label map
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action="unmapped_action"
        state={idleDecision}
      />,
    )

    // Then the raw action string is shown instead of nothing
    expect(screen.getByText('unmapped_action')).toBeInTheDocument()
  })

  it('renders the Stale badge only when stale is true', () => {
    // Given a fresh (non-stale) row
    const { rerender } = render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        state={idleDecision}
      />,
    )

    // Then no Stale badge is shown
    expect(screen.queryByText('Stale')).not.toBeInTheDocument()

    // When it becomes stale
    rerender(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        state={idleDecision}
        stale
      />,
    )

    // Then the Stale badge appears
    expect(screen.getByText('Stale')).toBeInTheDocument()
  })

  it('omits the reason paragraph when there is no reason', () => {
    // Given a row with no reason
    const { container } = render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        state={idleDecision}
      />,
    )

    // Then no italic reason paragraph is rendered — not even an empty one
    expect(container.querySelector('.italic')).not.toBeInTheDocument()
  })

  it('shows the reason when provided', () => {
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        reason="extends the existing definition"
        state={idleDecision}
      />,
    )

    expect(screen.getByText('extends the existing definition')).toBeInTheDocument()
  })

  it('omits the new-definition line when there is none', () => {
    // Given a row with no classified new definition
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        state={idleDecision}
      />,
    )

    // Then no "New definition:" line is rendered — not even an empty one
    expect(screen.queryByText(/New definition:/)).not.toBeInTheDocument()
  })

  it('shows the new definition when provided', () => {
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        newDefinition="A converges-eventually store."
        state={idleDecision}
      />,
    )

    expect(screen.getByText('New definition: A converges-eventually store.')).toBeInTheDocument()
  })

  it('omits the error paragraph when the state has no error', () => {
    // Given an idle state — no error
    const { container } = render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        state={idleDecision}
      />,
    )

    // Then no destructive error paragraph is rendered — not even an empty one
    expect(container.querySelector('p.text-destructive')).not.toBeInTheDocument()
  })

  it('shows the state error when present', () => {
    const state: DecisionState = { ...idleDecision, error: 'database locked' }
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        state={state}
      />,
    )

    expect(screen.getByText('database locked')).toBeInTheDocument()
  })

  it('renders only the create actions for a create row, even when every handler is wired', () => {
    // Given every handler passed at once — the way a live queue screen
    // wires every row identically regardless of that row's own action
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_CREATE}
        state={idleDecision}
        {...allHandlers()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Save as draft' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save as approved' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Acknowledge' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Apply update' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Create & relate' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Keep existing' })).not.toBeInTheDocument()
  })

  it('renders only Acknowledge for a no_change row, even when every handler is wired', () => {
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_NO_CHANGE}
        state={idleDecision}
        {...allHandlers()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Acknowledge' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save as draft' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Apply update' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Create & relate' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Keep existing' })).not.toBeInTheDocument()
  })

  it('renders only Apply update for an update row, even when every handler is wired', () => {
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        state={idleDecision}
        {...allHandlers()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Apply update' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save as draft' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Acknowledge' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Create & relate' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Keep existing' })).not.toBeInTheDocument()
  })

  it('renders only Create & relate for a relate row, even when every handler is wired', () => {
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_RELATE}
        state={idleDecision}
        {...allHandlers()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Create & relate' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save as draft' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Apply update' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Keep existing' })).not.toBeInTheDocument()
  })

  it('renders only the three conflict resolutions for a conflict row, even when every handler is wired', () => {
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_CONFLICT}
        state={idleDecision}
        {...allHandlers()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Keep existing' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Update existing' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create separately' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Apply update' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Create & relate' })).not.toBeInTheDocument()
  })

  it('resolves a conflict with each exact resolution constant', async () => {
    const onResolveConflict = vi.fn()
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_CONFLICT}
        state={idleDecision}
        onResolveConflict={onResolveConflict}
      />,
    )

    screen.getByRole('button', { name: 'Keep existing' }).click()
    screen.getByRole('button', { name: 'Update existing' }).click()
    screen.getByRole('button', { name: 'Create separately' }).click()

    expect(onResolveConflict).toHaveBeenNthCalledWith(1, CONFLICT_KEEP_EXISTING)
    expect(onResolveConflict).toHaveBeenNthCalledWith(2, CONFLICT_UPDATE_EXISTING)
    expect(onResolveConflict).toHaveBeenNthCalledWith(3, CONFLICT_CREATE_SEPARATELY)
  })

  it('applies create at the exact status matching the clicked button', () => {
    const onApplyCreate = vi.fn()
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_CREATE}
        state={idleDecision}
        onApplyCreate={onApplyCreate}
      />,
    )

    screen.getByRole('button', { name: 'Save as draft' }).click()
    screen.getByRole('button', { name: 'Save as approved' }).click()

    expect(onApplyCreate).toHaveBeenNthCalledWith(1, 'draft')
    expect(onApplyCreate).toHaveBeenNthCalledWith(2, 'approved')
  })

  it('shows the pending label while applying an update', () => {
    const state: DecisionState = { ...idleDecision, pending: true }
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        state={state}
        onApplyUpdate={vi.fn()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Applying...' })).toBeInTheDocument()
  })

  it('shows a retry label after a failed update', () => {
    const state: DecisionState = { ...idleDecision, error: 'database locked' }
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_UPDATE}
        state={state}
        onApplyUpdate={vi.fn()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })

  it('shows the pending label while applying a relate', () => {
    const state: DecisionState = { ...idleDecision, pending: true }
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_RELATE}
        state={state}
        onApplyRelate={vi.fn()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Applying...' })).toBeInTheDocument()
  })

  it('shows a retry label after a failed relate', () => {
    const state: DecisionState = { ...idleDecision, error: 'database locked' }
    render(
      <ReconciliationDecisionRow
        concept="X"
        definition="Y"
        action={RECONCILE_RELATE}
        state={state}
        onApplyRelate={vi.fn()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })
})
