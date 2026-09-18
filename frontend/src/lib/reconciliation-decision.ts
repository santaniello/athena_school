// DecisionState tracks one row's own async apply/resolve call, independent
// of any batch action elsewhere on the screen — see
// specs/phases/phase-02-knowledge-engine/11-knowledge-reconciliation.md.
export interface DecisionState {
  pending: boolean
  done: boolean
  doneLabel: string
  error: string
}

export const idleDecision: DecisionState = { pending: false, done: false, doneLabel: '', error: '' }
